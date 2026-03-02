package runway

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

// InferenceJob describes an inference request holding unscaled input data
// and channels to asynchronously pass the prediction or error back to the caller.
type InferenceJob struct {
	InputData      []float32
	ScalingEnabled bool
	Result         chan float32
	Err            chan error
}

// ModelWorker holds the ONNX Session and the job queue for a specific replica.
type ModelWorker struct {
	Session *ort.DynamicAdvancedSession
	Queue   chan *InferenceJob
	Quit    chan struct{}
}

// workerRegistry keeps track of all running model workers by ReplicaID.
var (
	workerRegistry = make(map[string]*ModelWorker)
	registryMu     sync.RWMutex
)

// StartModelWorkers preloads an ONNX model into memory and spins up the
// specified number of goroutines to perform inference sequentially pulled from a queue.
func StartModelWorkers(replicaID string, modelPath string, instanceCount int) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := workerRegistry[replicaID]; exists {
		return fmt.Errorf("workers for replica %s are already running", replicaID)
	}

	if instanceCount <= 0 {
		instanceCount = 1
	}

	// 1. Preload the Model
	// We create an Advanced Session because it allows dynamic creation of distinct input/output tensors
	// per inference job natively, which prevents memory corruption across concurrent goroutines.
	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"X"},
		[]string{"variable"},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to load model %s: %w", modelPath, err)
	}

	queue := make(chan *InferenceJob, 100) // 100 backlog capacity
	quit := make(chan struct{})

	// 2. Start workers
	for i := 0; i < instanceCount; i++ {
		go func(workerID int) {
			log.Printf("Started worker %d for replica %s", workerID, replicaID)
			for {
				select {
				case <-quit:
					log.Printf("Worker %d for replica %s shutting down", workerID, replicaID)
					return
				case job := <-queue:
					prediction, err := processJob(session, job)
					if err != nil {
						job.Err <- err
					} else {
						job.Result <- prediction
					}
				}
			}
		}(i)
	}

	// 3. Register the worker pool
	workerRegistry[replicaID] = &ModelWorker{
		Session: session,
		Queue:   queue,
		Quit:    quit,
	}

	return nil
}

// StopModelWorkers stops the worker pool and unloads the model from memory.
func StopModelWorkers(replicaID string) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	worker, exists := workerRegistry[replicaID]
	if !exists {
		return fmt.Errorf("replica %s is not running", replicaID)
	}

	// Signal all workers to shut down
	close(worker.Quit)

	// Clean up resources
	err := worker.Session.Destroy()
	delete(workerRegistry, replicaID)

	return err
}

// ModelInference safely submits an inference request to the correct worker pool.
func ModelInference(replicaID string, inputData []float32, scalingEnabled bool) (float32, error) {
	registryMu.RLock()
	worker, exists := workerRegistry[replicaID]
	registryMu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("model replica %s is not currently loaded", replicaID)
	}

	job := &InferenceJob{
		InputData:      inputData,
		ScalingEnabled: scalingEnabled,
		Result:         make(chan float32, 1),
		Err:            make(chan error, 1),
	}

	// Submit job (non-blocking if queue isn't full)
	select {
	case worker.Queue <- job:
	default:
		return 0, errors.New("inference worker queue is full")
	}

	// Wait for response
	select {
	case res := <-job.Result:
		return res, nil
	case err := <-job.Err:
		return 0, err
	case <-time.After(5 * time.Second):
		return 0, errors.New("timeout waiting for inference result")
	}
}

// processJob encapsulates the actual tensor mapping and inference for a single job request.
func processJob(session *ort.DynamicAdvancedSession, job *InferenceJob) (float32, error) {
	data := job.InputData
	if job.ScalingEnabled {
		data = ScaleFeatures(data)
	}

	// Create Output Tensor
	outputShape := ort.NewShape(1, 1)
	outputTensor, err := ort.NewTensor(outputShape, []float32{0})
	if err != nil {
		return 0, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Create Input Tensor
	inputShape := ort.NewShape(1, int64(len(data)))
	inputTensor, err := ort.NewTensor(inputShape, data)
	if err != nil {
		return 0, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	// Execute Inference
	err = session.Run([]ort.Value{inputTensor}, []ort.Value{outputTensor})
	if err != nil {
		return 0, fmt.Errorf("inference run failed: %w", err)
	}

	scaledPrediction := outputTensor.GetData()[0]
	if job.ScalingEnabled {
		return scaledPrediction*yScale + yMean, nil
	}
	return scaledPrediction, nil
}
