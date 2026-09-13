package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

func downloadParallel(metadata ParallelMetadata, metadataFile, partFileName string) error {
	state := &downloadState{
		metadata:     metadata,
		metadataFile: metadataFile,
		started:      time.Now(),
		lastSave:     time.Now(),
	}

	client := &http.Client{}
	for _, worker := range metadata.Workers {
		workerSize := worker.ByteRange.End - worker.ByteRange.Start + 1
		if worker.Downloaded >= workerSize {
			continue
		}
		start := worker.ByteRange.Start + worker.Downloaded
		request, err := http.NewRequest(http.MethodGet, metadata.URL, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, worker.ByteRange.End))
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		validationErr := validateRangedResponse(response, ByteRange{Start: start, End: worker.ByteRange.End})
		_ = response.Body.Close()
		if validationErr != nil {
			return validationErr
		}
		break
	}

	file, err := os.OpenFile(partFileName, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Truncate(metadata.TotalSize); err != nil {
		return err
	}
	if err := saveParallelMetadata(state.metadata, state.metadataFile); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errors := make(chan error, metadata.WorkerCount)
	var waitGroup sync.WaitGroup
	for workerIndex := range metadata.Workers {
		waitGroup.Add(1)
		go downloadWorker(ctx, client, file, state, workerIndex, cancel, errors, &waitGroup)
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			return err
		}
	}

	state.mutex.Lock()
	defer state.mutex.Unlock()
	if state.metadata.Downloaded != state.metadata.TotalSize {
		return fmt.Errorf("download completed with %d of %d bytes", state.metadata.Downloaded, state.metadata.TotalSize)
	}
	return saveParallelMetadata(state.metadata, state.metadataFile)
}
