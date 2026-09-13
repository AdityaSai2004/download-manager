package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func validateRangedResponse(response *http.Response, requested ByteRange) error {
	if response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned %s for range %d-%d; expected 206 Partial Content", response.Status, requested.Start, requested.End)
	}

	parts := strings.Split(response.Header.Get("Content-Range"), "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid Content-Range %q", response.Header.Get("Content-Range"))
	}
	byteRange := strings.TrimPrefix(strings.TrimSpace(parts[0]), "bytes ")
	bounds := strings.Split(byteRange, "-")
	if len(bounds) != 2 {
		return fmt.Errorf("invalid Content-Range %q", response.Header.Get("Content-Range"))
	}
	start, err := strconv.ParseInt(bounds[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Content-Range %q", response.Header.Get("Content-Range"))
	}
	end, err := strconv.ParseInt(bounds[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Content-Range %q", response.Header.Get("Content-Range"))
	}
	if start != requested.Start || end != requested.End {
		return fmt.Errorf("Content-Range %q does not match requested range %d-%d", response.Header.Get("Content-Range"), requested.Start, requested.End)
	}

	expectedLength := requested.End - requested.Start + 1
	if response.ContentLength >= 0 && response.ContentLength != expectedLength {
		return fmt.Errorf("ContentLength %d does not match requested range length %d", response.ContentLength, expectedLength)
	}
	return nil
}
