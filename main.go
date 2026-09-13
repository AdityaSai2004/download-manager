package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 || len(os.Args) > 5 {
		fmt.Println("Usage: download-manager <url> <workers> [filename] [metadata.json]")
		return
	}

	rawURL := os.Args[1]
	workerCount, err := parseWorkerCount(os.Args[2])
	if err != nil {
		fmt.Println("Download failed:", err)
		return
	}

	response, err := http.Get(rawURL)
	if err != nil {
		fmt.Println("Download failed:", err)
		return
	}
	if response.Body != nil {
		defer response.Body.Close()
	}
	if response.StatusCode != http.StatusOK {
		fmt.Println("Server returned:", response.Status)
		return
	}
	if response.ContentLength <= 0 {
		fmt.Println("Download failed: response ContentLength must be positive")
		return
	}

	filename := fetchFileName(response, rawURL)
	folderName := getFilenameWithoutExtension(filename)
	
	// Create download folder
	if err := os.MkdirAll(folderName, 0755); err != nil {
		fmt.Println("Download failed: could not create folder:", err)
		return
	}

	metadataPath := ""
	if len(os.Args) >= 4 {
		if isMetadataFilename(os.Args[3]) {
			metadataPath = os.Args[3]
		} else {
			filename = os.Args[3]
			folderName = getFilenameWithoutExtension(filename)
			if err := os.MkdirAll(folderName, 0755); err != nil {
				fmt.Println("Download failed: could not create folder:", err)
				return
			}
		}
	}
	if len(os.Args) == 5 {
		metadataPath = os.Args[4]
	}
	if metadataPath == "" {
		metadataPath = filepath.Join(folderName, filename+".metadata.json")
	}

	partFileName := filepath.Join(folderName, filename+".part")
	finalFileName := filepath.Join(folderName, filename)

	ranges, err := splitByteRanges(response.ContentLength, workerCount)
	if err != nil {
		fmt.Println("Download failed:", err)
		return
	}
	metadata := ParallelMetadata{
		Metadata: Metadata{
			URL:          rawURL,
			Filename:     filename,
			TotalSize:    response.ContentLength,
			ETag:         response.Header.Get("ETag"),
			LastModified: response.Header.Get("Last-Modified"),
		},
		WorkerCount: workerCount,
		Workers:     make([]WorkerMetadata, len(ranges)),
	}
	for index, byteRange := range ranges {
		metadata.Workers[index] = WorkerMetadata{Index: int64(index), ByteRange: byteRange}
	}

	existingMetadata, err := readParallelMetadata(metadataPath)
	if err == nil {
		if !metadataMatches(metadata, existingMetadata) {
			fmt.Println("Download failed: existing metadata does not match this download")
			return
		}
		metadata = existingMetadata
	} else if !os.IsNotExist(err) {
		fmt.Println("Download failed: could not read metadata:", err)
		return
	}
	if metadata.Downloaded > 0 {
		partInfo, statErr := os.Stat(partFileName)
		if statErr != nil || partInfo.Size() < metadata.TotalSize {
			fmt.Println("Download failed: metadata progress does not match the partial file")
			return
		}
	}

	if err := response.Body.Close(); err != nil {
		fmt.Println("Download failed: could not close header response:", err)
		return
	}
	if err := downloadParallel(metadata, metadataPath, partFileName); err != nil {
		fmt.Println("Download failed:", err)
		return
	}
	if err := os.Rename(partFileName, finalFileName); err != nil {
		fmt.Println("Could not finalize download:", err)
		return
	}
	fmt.Println("\nDownload complete")
}
