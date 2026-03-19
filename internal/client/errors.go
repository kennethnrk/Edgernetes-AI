package client

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FormatError translates a gRPC error into a human-readable CLI message.
func FormatError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return fmt.Sprintf("error: %v", err)
	}

	switch st.Code() {
	case codes.Unavailable:
		return "error: cannot reach control plane — is it running?"
	case codes.NotFound:
		return fmt.Sprintf("error: not found — %s", st.Message())
	case codes.AlreadyExists:
		return fmt.Sprintf("error: already exists — %s", st.Message())
	case codes.InvalidArgument:
		return fmt.Sprintf("error: invalid input — %s", st.Message())
	case codes.DeadlineExceeded:
		return "error: request timed out"
	default:
		return fmt.Sprintf("error [%s]: %s", st.Code(), st.Message())
	}
}
