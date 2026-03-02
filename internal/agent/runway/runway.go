package runway

import (
	"log"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
)

func InitRuntime() {

	switch runtime.GOOS {
	case "windows":
		ort.SetSharedLibraryPath(`./assets/onnxruntime_local_windows_x64.dll`)
	// case "linux":
	// 	ort.SetSharedLibraryPath(`./assets/onnxruntime_local_linux_x64.so`)
	// case "darwin":
	// 	ort.SetSharedLibraryPath(`./assets/onnxruntime_local_darwin_x64.dylib`)
	default:
		log.Fatalf("Failed to initialize onnxruntime: Unsupported OS - %s", runtime.GOOS)
	}

	err := ort.InitializeEnvironment()
	if err != nil {
		log.Fatalf("Failed to initialize onnxruntime: %v", err)
	}
}

func CloseRuntime() {
	ort.DestroyEnvironment()
}

var xMeans = []float32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
var xScales = []float32{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
var yMean float32 = 0
var yScale float32 = 1

func scaleFeatures(raw []float32) []float32 {

	scaled := make([]float32, len(raw))
	for i := range raw {
		scaled[i] = (raw[i] - xMeans[i]) / xScales[i]
	}
	return scaled
}

func ModelInference(modelPath string, inputData []float32, scalingEnabled bool) (float32, error) {
	if scalingEnabled {
		inputData = scaleFeatures(inputData)
	}

	outputShape := ort.NewShape(1, 1)
	outputTensor, err := ort.NewTensor(outputShape, []float32{0})
	if err != nil {
		log.Fatalf("Failed to create output tensor: %v", err)
	}
	defer outputTensor.Destroy()

	inputShape := ort.NewShape(1, int64(len(inputData)))
	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		log.Fatalf("Failed to create input tensor: %v", err)
	}
	defer inputTensor.Destroy()

	session, err := ort.NewSession[float32](
		modelPath,
		[]string{"X"},        // input name
		[]string{"variable"}, // output name
		[]*ort.Tensor[float32]{inputTensor},
		[]*ort.Tensor[float32]{outputTensor},
	)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Destroy()

	err = session.Run()
	if err != nil {
		log.Fatalf("Inference failed: %v", err)
	}

	scaledPrediction := outputTensor.GetData()[0]
	if scalingEnabled {
		return scaledPrediction*yScale + yMean, nil
	}
	return scaledPrediction, nil

}
