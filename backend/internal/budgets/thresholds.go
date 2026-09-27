package budgets

import "sort"

func ThresholdsCrossed(previous, current, limit float64, thresholds []float64) []float64 {
	if limit <= 0 || current < 0 {
		return nil
	}
	values := append([]float64(nil), thresholds...)
	sort.Float64s(values)
	result := []float64{}
	for _, threshold := range values {
		if threshold <= 0 {
			continue
		}
		boundary := limit * threshold / 100
		if previous < boundary && current >= boundary {
			result = append(result, threshold)
		}
	}
	return result
}
