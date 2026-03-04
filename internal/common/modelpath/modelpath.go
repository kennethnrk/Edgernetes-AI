// Package modelpath provides utilities for classifying model file paths
// as either local filesystem paths or network blob URLs.
//
// Both the control plane and edge agents import this package to decide
// whether a model file needs to be transferred via gRPC streaming or
// can be fetched directly from a remote URL.
package modelpath

import "strings"

// PathType represents the location type of a model file path.
type PathType int

const (
	// Local indicates a path on the local filesystem (absolute or relative).
	Local PathType = iota
	// Network indicates a remote URL (HTTP, HTTPS, S3, GCS, Azure Blob, etc.).
	Network
)

// networkPrefixes lists the URL scheme prefixes that are treated as network paths.
var networkPrefixes = []string{
	"http://",
	"https://",
	"s3://",
	"gs://",
	"az://",
}

// IsNetworkPath returns true if the given path is a remote/network URL
// (e.g. https://..., s3://..., gs://..., az://...).
// It returns false for local filesystem paths, relative paths, empty strings,
// and anything that does not start with a recognized network scheme.
func IsNetworkPath(path string) bool {
	return Classify(path) == Network
}

// Classify returns the PathType for the given model file path.
func Classify(path string) PathType {
	lower := strings.ToLower(strings.TrimSpace(path))
	for _, prefix := range networkPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return Network
		}
	}
	return Local
}
