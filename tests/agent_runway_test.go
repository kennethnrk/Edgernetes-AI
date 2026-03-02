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

	rawFeatures := []float32{
		6000, // area
		3,    // bedrooms
		2,    // bathrooms
		2,    // stories
		1,    // mainroad (yes=1)
		0,    // guestroom (no=0)
		1,    // basement (yes=1)
		0,    // hotwaterheating (no=0)
		1,    // airconditioning (yes=1)
		2,    // parking
		1,    // prefarea (yes=1)
		2,    // furnishingstatus (furnished=2)
	}

	price, err := runway.ModelInference(modelPath, rawFeatures, true)
	if err != nil {
		t.Fatalf("ModelInference failed: %v", err)
	}

	if math.IsNaN(float64(price)) {
		t.Fatalf("Predicted price is NaN")
	}

	t.Logf("Predicted price: %.2f", price)
}
