package tests

import (
	"testing"

	"github.com/kennethnrk/edgernetes-ai/internal/common/modelpath"
)

func TestIsNetworkPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Network URLs
		{"https URL", "https://s3.amazonaws.com/bucket/model.onnx", true},
		{"http URL", "http://minio.local:9000/models/model.onnx", true},
		{"s3 scheme", "s3://my-bucket/models/model.onnx", true},
		{"gs scheme", "gs://gcs-bucket/models/model.onnx", true},
		{"az scheme", "az://container/models/model.onnx", true},
		{"HTTPS uppercase", "HTTPS://example.com/model.onnx", true},
		{"mixed case", "Https://Example.com/model.onnx", true},
		{"with whitespace padding", "  https://example.com/model.onnx  ", true},

		// Local paths
		{"unix absolute", "/home/user/models/model.onnx", false},
		{"unix relative", "./models/model.onnx", false},
		{"windows absolute", `C:\models\model.onnx`, false},
		{"windows UNC", `\\server\share\model.onnx`, false},
		{"just a filename", "model.onnx", false},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modelpath.IsNetworkPath(tt.path)
			if got != tt.want {
				t.Fatalf("IsNetworkPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	if modelpath.Classify("https://example.com/model.onnx") != modelpath.Network {
		t.Fatal("expected Network for https URL")
	}
	if modelpath.Classify("/local/path/model.onnx") != modelpath.Local {
		t.Fatal("expected Local for absolute unix path")
	}
}
