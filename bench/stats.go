package main

import (
	"math/rand"
	"sort"
)

// CI is a bootstrap percentile interval around a point estimate.
type CI struct {
	Mean, Lo, Hi float64
	N            int
}

// bootstrapMean resamples vals with replacement boot times. The generator is
// seeded, and equal-length inputs get the same resample indices, so the
// intervals are reproducible and comparable across arms.
func bootstrapMean(vals []float64, boot int, seed int64) CI {
	n := len(vals)
	if n == 0 {
		return CI{}
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	ci := CI{Mean: sum / float64(n), N: n}
	rng := rand.New(rand.NewSource(seed))
	means := make([]float64, boot)
	for b := range means {
		var s float64
		for i := 0; i < n; i++ {
			s += vals[rng.Intn(n)]
		}
		means[b] = s / float64(n)
	}
	sort.Float64s(means)
	ci.Lo = means[int(0.025*float64(boot))]
	ci.Hi = means[int(0.975*float64(boot))-1]
	return ci
}
