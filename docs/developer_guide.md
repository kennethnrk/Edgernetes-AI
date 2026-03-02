# Developer Guide

## How to setup ONNX Runtime

### Windows

1. Download the ONNX Runtime from [https://onnxruntime.ai/downloads/](https://onnxruntime.ai/downloads/)
2. Extract the downloaded file
3. Copy the `onnxruntime_local_windows_x64.dll` file to the `assets` directory
4. Run `go run main.go`

## How to build protobuf files

1. Run `protoc --go_out=. --go-grpc_out=. api/proto/*.proto` to generate files for all proto definitions into their respective locations.
