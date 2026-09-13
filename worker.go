package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

func downloadWorker(ctx context.Context, client *http.Client, file *os.File, state *downloadState, workerIndex int, cancel context.CancelFunc, errors chan<- error, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	worker := state.metadata.Workers[workerIndex]
	workerSize := worker.ByteRange.End - worker.ByteRange.Start + 1
	if worker.Downloaded >= workerSize {
		return
	}

	start := worker.ByteRange.Start + worker.Downloaded
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, state.metadata.URL, nil)
	if err != nil {
		errors <- err
		cancel()
		return
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, worker.ByteRange.End))
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() == nil {
			errors <- err
			cancel()
		}
		return
	}
	defer response.Body.Close()

	requested := ByteRange{Start: start, End: worker.ByteRange.End}
	if err := validateRangedResponse(response, requested); err != nil {
		errors <- err
		cancel()
		return
	}

	buffer := make([]byte, 1024*1024)
	downloaded := worker.Downloaded
	remaining := workerSize - downloaded
	for remaining > 0 {
		readSize := int64(len(buffer))
		if readSize > remaining {
			readSize = remaining
		}
		n, readErr := io.ReadFull(response.Body, buffer[:readSize])
		if n > 0 {
			if _, err := file.WriteAt(buffer[:n], worker.ByteRange.Start+downloaded); err != nil {
				errors <- err
				cancel()
				return
			}
			downloaded += int64(n)
			remaining -= int64(n)
			if err := state.updateProgress(workerIndex, downloaded, false); err != nil {
				errors <- err
				cancel()
				return
			}
		}
		if readErr != nil {
			errors <- fmt.Errorf("worker %d received %d of %d bytes: %w", worker.Index+1, downloaded-worker.Downloaded, workerSize-worker.Downloaded, readErr)
			cancel()
			return
		}
	}

	var extra [1]byte
	if n, readErr := response.Body.Read(extra[:]); n != 0 || (readErr != io.EOF && readErr != nil) {
		errors <- fmt.Errorf("worker %d received more bytes than its assigned range", worker.Index+1)
		cancel()
		return
	}
	if err := state.updateProgress(workerIndex, downloaded, true); err != nil {
		errors <- err
		cancel()
	}
}
