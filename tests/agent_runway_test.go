package tests

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/kennethnrk/edgernetes-ai/internal/agent/runway"
)

func init() {
	// Navigate up to the project root directory
	for {
		if _, err := os.Stat("go.mod"); err == nil {
			break
		}
		if err := os.Chdir(".."); err != nil {
			break
		}
	}
}

func TestScaleFeatures(t *testing.T) {
	raw := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	scaled := runway.ScaleFeatures(raw)

	if len(scaled) != len(raw) {
		t.Fatalf("Expected length %d, got %d", len(raw), len(scaled))
	}

	for i, v := range scaled {
		if v != raw[i] { // Default means=0, scales=1 so values should remain the same
			t.Errorf("Expected %f, got %f at index %d", raw[i], v, i)
		}
	}
}

func TestModelInference(t *testing.T) {
	runway.InitRuntime()
	defer runway.CloseRuntime()

	modelPath := filepath.Join("tests", "test_assets", "mlp_price_predictor_1.onnx")
	replicaID := "test-replica-777"

	// Start the background worker explicitly configured for 2 instances
	err := runway.StartModelWorkers(replicaID, modelPath, 2)
	if err != nil {
		t.Fatalf("Failed to start model workers: %v", err)
	}
	defer runway.StopModelWorkers(replicaID)

	rawFeatures := []float32{
		6000, 3, 2, 2, 1, 0, 1, 0, 1, 2, 1, 2,
	}

	// Run 10 inferences concurrently to ensure thread safety
	errCh := make(chan error, 10)
	priceCh := make(chan float32, 10)

	for i := 0; i < 10; i++ {
		go func() {
			price, err := runway.ModelInference(replicaID, rawFeatures, true)
			if err != nil {
				errCh <- err
				return
			}
			priceCh <- price
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case err := <-errCh:
			t.Fatalf("ModelInference failed concurrently: %v", err)
		case price := <-priceCh:
			if math.IsNaN(float64(price)) {
				t.Fatalf("Predicted price is NaN")
			}
			t.Logf("Predicted price: %.2f", price)
		}
	}
}
