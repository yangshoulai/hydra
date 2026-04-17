package proxy

import "math/rand/v2"

const defaultRouteWeight int64 = 100

func normalizedWeight(weight int) int64 {
	if weight > 0 {
		return int64(weight)
	}
	return defaultRouteWeight
}

func weightedRandomIndex(weights []int64) int {
	if len(weights) == 0 {
		return -1
	}

	totalWeight := int64(0)
	for _, weight := range weights {
		if weight > 0 {
			totalWeight += weight
		}
	}

	if totalWeight <= 0 {
		return 0
	}

	return weightedIndexForDraw(weights, rand.Int64N(totalWeight))
}

func weightedIndexForDraw(weights []int64, draw int64) int {
	if len(weights) == 0 {
		return -1
	}

	lastPositiveIndex := -1
	for index, weight := range weights {
		if weight <= 0 {
			continue
		}
		lastPositiveIndex = index
		if draw < weight {
			return index
		}
		draw -= weight
	}

	if lastPositiveIndex >= 0 {
		return lastPositiveIndex
	}
	return 0
}
