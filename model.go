package main

import (
	"fmt"
	"strconv"
)

type Metadata struct {
	URL          string `json:"url"`
	Filename     string `json:"filename"`
	TotalSize    int64  `json:"total_size"`
	Downloaded   int64  `json:"downloaded"`
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

type ByteRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type WorkerMetadata struct {
	Index      int64     `json:"index"`
	ByteRange  ByteRange `json:"byte_range"`
	Downloaded int64     `json:"downloaded"`
}

type ParallelMetadata struct {
	Metadata
	WorkerCount int              `json:"worker_count"`
	Workers     []WorkerMetadata `json:"workers"`
}

func splitByteRanges(totalSize int64, workerCount int) ([]ByteRange, error) {
	if totalSize <= 0 {
		return nil, fmt.Errorf("total size must be positive")
	}
	if workerCount <= 0 {
		return nil, fmt.Errorf("worker count must be positive")
	}
	if int64(workerCount) > totalSize {
		return nil, fmt.Errorf("worker count cannot exceed total size")
	}

	partSize := totalSize / int64(workerCount)
	ranges := make([]ByteRange, workerCount)
	for index := range ranges {
		start := int64(index) * partSize
		end := start + partSize - 1
		if index == workerCount-1 {
			end = totalSize - 1
		}
		ranges[index] = ByteRange{Start: start, End: end}
	}
	return ranges, nil
}

func parseWorkerCount(value string) (int, error) {
	workerCount, err := strconv.Atoi(value)
	if err != nil || workerCount <= 0 {
		return 0, fmt.Errorf("worker count must be a positive integer")
	}
	return workerCount, nil
}

func metadataMatches(expected, actual ParallelMetadata) bool {
	if expected.URL != actual.URL || expected.Filename != actual.Filename ||
		expected.TotalSize != actual.TotalSize || expected.ETag != actual.ETag ||
		expected.LastModified != actual.LastModified || expected.WorkerCount != actual.WorkerCount ||
		len(expected.Workers) != len(actual.Workers) {
		return false
	}

	var downloadedTotal int64
	for index := range expected.Workers {
		expectedWorker := expected.Workers[index]
		actualWorker := actual.Workers[index]
		workerSize := actualWorker.ByteRange.End - actualWorker.ByteRange.Start + 1
		if expectedWorker.Index != actualWorker.Index || expectedWorker.ByteRange != actualWorker.ByteRange ||
			actualWorker.Downloaded < 0 || actualWorker.Downloaded > workerSize {
			return false
		}
		downloadedTotal += actualWorker.Downloaded
	}
	return downloadedTotal == actual.Downloaded
}
