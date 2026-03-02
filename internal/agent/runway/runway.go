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

func ScaleFeatures(raw []float32) []float32 {

	scaled := make([]float32, len(raw))
	for i := range raw {
		scaled[i] = (raw[i] - xMeans[i]) / xScales[i]
	}
	return scaled
}
