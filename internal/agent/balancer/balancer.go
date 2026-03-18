package balancer

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// LoadBalancer defines the interface for selecting an endpoint.
type LoadBalancer interface {
	Pick(endpoints []*heartbeatpb.EndpointDetail) (*heartbeatpb.EndpointDetail, error)
}

// WeightedRoundRobin is a load balancer that selects endpoints based on their weight.
type WeightedRoundRobin struct {
	mu            sync.Mutex
	currentIndex  int
	currentWeight float64
}

// NewWeightedRoundRobin creates a new WeightedRoundRobin load balancer.
func NewWeightedRoundRobin() *WeightedRoundRobin {
	return &WeightedRoundRobin{
		currentIndex:  -1,
		currentWeight: 0,
	}
}

// Pick selects an endpoint using a smooth weighted round-robin algorithm.
func (w *WeightedRoundRobin) Pick(endpoints []*heartbeatpb.EndpointDetail) (*heartbeatpb.EndpointDetail, error) {
	healthyEndpoints := FilterHealthy(endpoints)
	if len(healthyEndpoints) == 0 {
		return nil, errors.New("no healthy endpoints available")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	var maxWeight float64
	for _, ep := range healthyEndpoints {
		if ep.Weight > maxWeight {
			maxWeight = ep.Weight
		}
	}

	for {
		w.currentIndex = (w.currentIndex + 1) % len(healthyEndpoints)
		if w.currentIndex == 0 {
			w.currentWeight -= 1.0 // Decrement by a step (could be gcd, but 1.0 works as a simple step)
			if w.currentWeight <= 0 {
				w.currentWeight = maxWeight
				if w.currentWeight == 0 {
					return healthyEndpoints[w.currentIndex], nil // fallback if all weights are 0
				}
			}
		}

		if healthyEndpoints[w.currentIndex].Weight >= w.currentWeight {
			return healthyEndpoints[w.currentIndex], nil
		}
	}
}

// FilterHealthy returns only the healthy endpoints.
func FilterHealthy(endpoints []*heartbeatpb.EndpointDetail) []*heartbeatpb.EndpointDetail {
	var healthy []*heartbeatpb.EndpointDetail
	for _, ep := range endpoints {
		if ep.Healthy {
			healthy = append(healthy, ep)
		}
	}
	return healthy
}
