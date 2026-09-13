package main

import (
	"net/http"
	"net/url"
	"path"
	"strings"
)

// getFilenameWithoutExtension returns the filename without its extension
func getFilenameWithoutExtension(filename string) string {
	ext := path.Ext(filename)
	if ext == "" {
		return filename
	}
	return filename[:len(filename)-len(ext)]
}

func isMetadataFilename(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".metadata.json")
}

func extensionFromContentType(contentType string) string {
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch contentType {
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	case "text/html":
		return ".html"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	default:
		return ""
	}
}

func extractFilename(contentDisposition string) string {
	for _, part := range strings.Split(contentDisposition, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename=") {
			filename := strings.Trim(strings.TrimPrefix(part, "filename="), `"`)
			if filename != "" {
				return filename
			}
		}
	}
	return ""
}

func fetchFileName(response *http.Response, rawURL string) string {
	if filename := extractFilename(response.Header.Get("Content-Disposition")); filename != "" {
		return filename
	}
	if parsedURL, err := url.Parse(rawURL); err == nil {
		filename := path.Base(parsedURL.Path)
		if filename != "." && filename != "/" && filename != "" && path.Ext(filename) != "" {
			return filename
		}
	}
	if extension := extensionFromContentType(response.Header.Get("Content-Type")); extension != "" {
		return "download" + extension
	}
	return "download"
}
