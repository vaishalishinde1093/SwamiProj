package util

import "math"

// Distribute distributes items equally across batches using generics
// This works with any slice type and distributes as evenly as possible
func Distribute[T any](items []T, maxPerBatch int) [][]T {
    if len(items) == 0 {
        return [][]T{}
    }

    if len(items) <= maxPerBatch {
        return [][]T{items}
    }

    // Calculate number of batches needed
    numBatches := int(math.Ceil(float64(len(items)) / float64(maxPerBatch)))
    // Distribute as evenly as possible
    baseSize := len(items) / numBatches
    remainder := len(items) % numBatches

    batches := make([][]T, numBatches)
    startIdx := 0

    for i := 0; i < numBatches; i++ {
        size := baseSize
        if i < remainder {
            size++
        }
        endIdx := startIdx + size
        batches[i] = items[startIdx:endIdx]
        startIdx = endIdx
    }

    return batches
}
