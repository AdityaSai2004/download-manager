package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type rangeTestServer struct {
	data          []byte
	supportsRange bool
	failStart     int64
	failRequests  bool
	mutex         sync.Mutex
	ranges        []ByteRange
}

func (server *rangeTestServer) handler(response http.ResponseWriter, request *http.Request) {
	rangeHeader := request.Header.Get("Range")
	if rangeHeader == "" || !server.supportsRange {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(server.data)
		return
	}
	bounds := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(bounds) != 2 {
		response.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	start, startErr := strconv.ParseInt(bounds[0], 10, 64)
	end, endErr := strconv.ParseInt(bounds[1], 10, 64)
	if startErr != nil || endErr != nil {
		response.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	server.mutex.Lock()
	server.ranges = append(server.ranges, ByteRange{Start: start, End: end})
	server.mutex.Unlock()
	if server.failRequests && start == server.failStart {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	if start < 0 || end < start || end >= int64(len(server.data)) {
		response.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	content := server.data[start : end+1]
	response.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(server.data)))
	response.Header().Set("Content-Length", strconv.Itoa(len(content)))
	response.WriteHeader(http.StatusPartialContent)
	_, _ = response.Write(content)
}

func newRangeTestServer(data []byte, supportsRange bool) (*rangeTestServer, *httptest.Server) {
	rangeServer := &rangeTestServer{data: data, supportsRange: supportsRange, failStart: -1}
	return rangeServer, httptest.NewServer(http.HandlerFunc(rangeServer.handler))
}

func testMetadata(serverURL, filename string, dataSize int64, workerCount int) ParallelMetadata {
	ranges, err := splitByteRanges(dataSize, workerCount)
	if err != nil {
		panic(err)
	}
	metadata := ParallelMetadata{Metadata: Metadata{URL: serverURL, Filename: filename, TotalSize: dataSize}, WorkerCount: workerCount, Workers: make([]WorkerMetadata, len(ranges))}
	for index, byteRange := range ranges {
		metadata.Workers[index] = WorkerMetadata{Index: int64(index), ByteRange: byteRange}
	}
	return metadata
}

func TestWorkersResumeTheirOwnRanges(t *testing.T) {
	data := make([]byte, 4*1024*1024+123)
	for index := range data {
		data[index] = byte(index % 251)
	}
	rangeServer, server := newRangeTestServer(data, true)
	defer server.Close()
	metadata := testMetadata(server.URL, "resume.bin", int64(len(data)), 4)
	partialFile, err := os.CreateTemp(t.TempDir(), "resume-*.part")
	if err != nil {
		t.Fatal(err)
	}
	partialPath := partialFile.Name()
	if err := partialFile.Truncate(int64(len(data))); err != nil {
		t.Fatal(err)
	}
	firstWorker := metadata.Workers[0]
	metadata.Workers[0].Downloaded = firstWorker.ByteRange.End - firstWorker.ByteRange.Start + 1
	if _, err := partialFile.WriteAt(data[firstWorker.ByteRange.Start:firstWorker.ByteRange.End+1], firstWorker.ByteRange.Start); err != nil {
		t.Fatal(err)
	}
	partialWorker := &metadata.Workers[1]
	partialProgress := int64(321)
	partialWorker.Downloaded = partialProgress
	start := partialWorker.ByteRange.Start
	if _, err := partialFile.WriteAt(data[start:start+partialProgress], start); err != nil {
		t.Fatal(err)
	}
	if err := partialFile.Close(); err != nil {
		t.Fatal(err)
	}
	metadata.Downloaded = firstWorker.ByteRange.End - firstWorker.ByteRange.Start + 1 + partialProgress
	metadataPath := partialPath + ".metadata.json"
	if err := saveParallelMetadata(metadata, metadataPath); err != nil {
		t.Fatal(err)
	}
	if err := downloadParallel(metadata, metadataPath, partialPath); err != nil {
		t.Fatal(err)
	}
	result, err := os.ReadFile(partialPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, data) {
		t.Fatal("resumed workers did not reconstruct the original data")
	}
	rangeServer.mutex.Lock()
	defer rangeServer.mutex.Unlock()
	for _, requested := range rangeServer.ranges {
		if requested.Start == firstWorker.ByteRange.Start {
			t.Fatal("completed worker was requested again")
		}
	}
}

func TestWorkerFailureReportsOverallFailure(t *testing.T) {
	data := bytes.Repeat([]byte("parallel-data-"), 300000)
	rangeServer, server := newRangeTestServer(data, true)
	defer server.Close()
	metadata := testMetadata(server.URL, "failure.bin", int64(len(data)), 4)
	rangeServer.failStart = metadata.Workers[1].ByteRange.Start
	rangeServer.failRequests = true
	directory := t.TempDir()
	partPath := directory + "/failure.part"
	if err := downloadParallel(metadata, directory+"/failure.json", partPath); err == nil {
		t.Fatal("expected the overall download to fail")
	}
	if _, err := os.Stat(partPath); err != nil {
		t.Fatal("partial file should remain after worker failure")
	}
}

func TestChangedSourceRejectsExistingMetadata(t *testing.T) {
	expected := testMetadata("https://example.test/file", "file.bin", 100, 2)
	expected.ETag = "old"
	changed := expected
	changed.ETag = "new"
	if metadataMatches(expected, changed) {
		t.Fatal("changed source validators should invalidate metadata")
	}
}

func TestUnsupportedRangePreservesExistingPart(t *testing.T) {
	data := bytes.Repeat([]byte("source-data-"), 100000)
	_, server := newRangeTestServer(data, false)
	defer server.Close()
	metadata := testMetadata(server.URL, "no-range.bin", int64(len(data)), 3)
	directory := t.TempDir()
	partPath := directory + "/no-range.part"
	sentinel := bytes.Repeat([]byte{0xA5}, len(data))
	if err := os.WriteFile(partPath, sentinel, 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveParallelMetadata(metadata, directory+"/no-range.json"); err != nil {
		t.Fatal(err)
	}
	if err := downloadParallel(metadata, directory+"/no-range.json", partPath); err == nil {
		t.Fatal("expected no-range server to fail")
	}
	result, err := os.ReadFile(partPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, sentinel) {
		t.Fatal("unsupported Range response modified partial file")
	}
}

func TestRangedResponseRejectsWrongStatus(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusOK, Status: "200 OK", ContentLength: 10, Body: io.NopCloser(strings.NewReader("0123456789"))}
	if err := validateRangedResponse(response, ByteRange{Start: 0, End: 9}); err == nil {
		t.Fatal("expected 200 response to be rejected")
	}
}
