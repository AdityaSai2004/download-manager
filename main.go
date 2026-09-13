package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Check if this is a resume operation
	if len(os.Args) == 3 && os.Args[1] == "resume" {
		resumeDownload(os.Args[2])
		return
	}

	if len(os.Args) < 3 || len(os.Args) > 5 {
		fmt.Println("Usage:")
		fmt.Println("  New download:    download-manager <url> <workers> [filename]")
		fmt.Println("  Resume download: download-manager resume <metadata.json>")
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
	if len(os.Args) >= 4 {
		filename = os.Args[3]
	}

	folderName := getFilenameWithoutExtension(filename)

	// Create download folder
	if err := os.MkdirAll(folderName, 0755); err != nil {
		fmt.Println("Download failed: could not create folder:", err)
		return
	}

	metadataPath := filepath.Join(folderName, filename+".metadata.json")
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

func resumeDownload(metadataPath string) {
	// Read the metadata file
	metadata, err := readParallelMetadata(metadataPath)
	if err != nil {
		fmt.Println("Resume failed: could not read metadata:", err)
		return
	}

	// Extract folder and filenames from metadata
	filename := metadata.Filename
	folderName := getFilenameWithoutExtension(filename)
	partFileName := filepath.Join(folderName, filename+".part")
	finalFileName := filepath.Join(folderName, filename)

	// Verify the partial file exists and has the correct size
	partInfo, err := os.Stat(partFileName)
	if err != nil {
		fmt.Println("Resume failed: partial file not found:", err)
		return
	}
	if partInfo.Size() < metadata.TotalSize {
		fmt.Printf("Resume failed: partial file size (%d bytes) is less than total size (%d bytes)\n", partInfo.Size(), metadata.TotalSize)
		return
	}

	// Resume the download
	if err := downloadParallel(metadata, metadataPath, partFileName); err != nil {
		fmt.Println("Download failed:", err)
		return
	}

	// Finalize the download
	if err := os.Rename(partFileName, finalFileName); err != nil {
		fmt.Println("Could not finalize download:", err)
		return
	}
	fmt.Println("\nDownload resumed and complete")
}
