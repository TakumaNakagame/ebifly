package room

import (
	"math"
)

type Stats struct {
	Average      *float64       `json:"average"` // nil when no numeric votes
	Distribution map[string]int `json:"distribution"`
}

// ComputeStats computes the average (numeric cards only) and distribution
// (all cards including "?" and "coffee") from revealed votes.
// Spectators are expected to be filtered out before calling.
func ComputeStats(values []string) Stats {
	dist := map[string]int{}
	var sum float64
	var n int
	for _, v := range values {
		dist[v]++
		if num, ok := NumericValue(v); ok {
			sum += num
			n++
		}
	}
	var avg *float64
	if n > 0 {
		a := math.Round(sum/float64(n)*100) / 100
		avg = &a
	}
	return Stats{Average: avg, Distribution: dist}
}
