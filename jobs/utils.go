package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	mathrand "math/rand"
	"time"
)

func generateJobID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("job_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func calculateBackoffDelay(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}
	
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
	
	if delay > maxDelay {
		delay = maxDelay
	}
	
	jitter := time.Duration(mathrand.Int63n(int64(delay / 10)))
	
	return delay + jitter
}

func calculateNextRetryAt(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Time {
	delay := calculateBackoffDelay(attempt, baseDelay, maxDelay)
	return time.Now().Add(delay)
}

func getQueuePriority(priority Priority) int {
	switch priority {
	case PriorityCritical:
		return 1000
	case PriorityHigh:
		return 100
	case PriorityNormal:
		return 10
	case PriorityLow:
		return 1
	default:
		return 10
	}
}

func jsonMarshal(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	return json.Marshal(v)
}