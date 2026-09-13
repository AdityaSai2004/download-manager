package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type downloadState struct {
	metadata     ParallelMetadata
	metadataFile string
	mutex        sync.Mutex
	started      time.Time
	lastSave     time.Time
}

func (state *downloadState) updateProgress(workerIndex int, downloaded int64, forceSave bool) error {
	state.mutex.Lock()
	defer state.mutex.Unlock()

	state.metadata.Workers[workerIndex].Downloaded = downloaded
	state.metadata.Downloaded = 0
	for _, worker := range state.metadata.Workers {
		state.metadata.Downloaded += worker.Downloaded
	}

	now := time.Now()
	if !forceSave && now.Sub(state.lastSave) < 5*time.Second {
		state.renderLocked()
		return nil
	}
	if err := saveParallelMetadata(state.metadata, state.metadataFile); err != nil {
		return err
	}
	state.lastSave = now
	state.renderLocked()
	return nil
}

func (state *downloadState) renderLocked() {
	percentage := float64(0)
	if state.metadata.TotalSize > 0 {
		percentage = float64(state.metadata.Downloaded) / float64(state.metadata.TotalSize) * 100
	}
	elapsed := time.Since(state.started)
	if elapsed <= 0 {
		elapsed = time.Second
	}
	speed := float64(state.metadata.Downloaded) / elapsed.Seconds() / (1024 * 1024)
	fmt.Printf("\rTotal: [%s] %.2f%% | %d/%d bytes | %.2f MB/s\n", progressBar(percentage), percentage, state.metadata.Downloaded, state.metadata.TotalSize, speed)
	for _, worker := range state.metadata.Workers {
		workerSize := worker.ByteRange.End - worker.ByteRange.Start + 1
		workerPercentage := float64(0)
		if workerSize > 0 {
			workerPercentage = float64(worker.Downloaded) / float64(workerSize) * 100
		}
		fmt.Printf("Worker %d: [%s] %.2f%% | %d/%d bytes\n", worker.Index+1, progressBar(workerPercentage), workerPercentage, worker.Downloaded, workerSize)
	}
}

func progressBar(percentage float64) string {
	const width = 30
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	filled := int(percentage / 100 * width)
	return strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
}
