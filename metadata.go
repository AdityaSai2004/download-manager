package main

import (
	"encoding/json"
	"os"
)

func saveParallelMetadata(metadata ParallelMetadata, filename string) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	temporaryFile := filename + ".tmp"
	if err := os.WriteFile(temporaryFile, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(temporaryFile, filename); err != nil {
		_ = os.Remove(temporaryFile)
		return err
	}
	return nil
}

func readParallelMetadata(filename string) (ParallelMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ParallelMetadata{}, err
	}
	defer file.Close()

	var metadata ParallelMetadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return ParallelMetadata{}, err
	}
	return metadata, nil
}
