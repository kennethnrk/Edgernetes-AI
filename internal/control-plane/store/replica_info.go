package store

import (
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

type ReplicaInfo struct {
	ID            string                       `json:"id"`
	ModelID       string                       `json:"model_id"`
	Name          string                       `json:"name"`
	Status        constants.ModelReplicaStatus `json:"status"`
	ErrorCode     int                          `json:"error_code"`
	ErrorMessage  string                       `json:"error_message"`
	LastHeartbeat time.Time                    `json:"last_heartbeat"`
}
