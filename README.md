# Download Manager

A parallel HTTP file downloader written in Go using HTTP range requests and concurrent workers.

## Features

- **Parallel Ranged Downloading**: Split one file into byte ranges and download those ranges concurrently using multiple workers
- **Resume Capability**: Save download metadata to resume interrupted downloads
- **Range Request Support**: Utilizes HTTP 206 Partial Content responses for efficient chunked downloading
- **Progress Tracking**: Real-time progress reporting for both aggregate and per-worker downloads
- **Atomic Metadata Persistence**: Safely save and recover download state
- **Flexible Output**: Automatic filename inference from HTTP headers or custom naming
- **Test Coverage**: Comprehensive tests for resume and failure scenarios

## Architecture

The project is organized into focused, single-responsibility modules:

| Module         | Purpose                                                             |
| -------------- | ------------------------------------------------------------------- |
| `main.go`      | CLI orchestration and download finalization                         |
| `download.go`  | Preflight checks, worker coordination, cancellation, and completion |
| `worker.go`    | Individual worker's ranged request and positional writes            |
| `model.go`     | Metadata models, worker configuration, and range splitting logic    |
| `metadata.go`  | Atomic metadata persistence and loading                             |
| `range.go`     | HTTP ranged-response validation                                     |
| `filename.go`  | Output filename and content-type inference                          |
| `progress.go`  | Aggregate and per-worker progress state/rendering                   |
| `main_test.go` | Resume and failure behavior tests                                   |

## Prerequisites

- Go 1.16 or higher
- Network access to download from HTTP/HTTPS sources

## Installation

Clone the repository:

```bash
git clone https://github.com/yourusername/download-manager.git
cd download-manager/v4
```

Build the binary:

```bash
go build -o download-manager
```

## Usage

### Basic Download

```bash
go run . <url> <workers> [filename]
```

### Resume a Download

Resume using only the metadata file (no URL or worker count needed):

```bash
go run . resume <metadata.json>
```

### Examples

Download a file using 4 concurrent workers:

```bash
go run . https://example.com/largefile.zip 4
```

Download with a custom filename:

```bash
go run . https://example.com/largefile.zip 4 myfile.zip
```

Resume a download using just the metadata file:

```bash
go run . resume largefile/largefile.zip.metadata.json
```

After completion, all files will be organized in the folder:

- `largefile/largefile.zip` - The final downloaded file
- `largefile/largefile.zip.part` - Removed after successful completion
- `largefile/largefile.zip.metadata.json` - Download metadata and state

## How It Works

1. **Folder Organization**: Creates a folder with the name of the download file (without extension) to contain all download-related files
2. **Preflight Check**: Verifies the server supports range requests and provides content length
3. **Range Splitting**: Divides the file into chunks based on the number of workers
4. **Parallel Download**: Workers simultaneously download their assigned byte ranges
5. **Positional Write**: Each worker writes to its specific position in the partial file (.part)
6. **Progress Tracking**: Real-time monitoring of individual and aggregate progress with periodic metadata saves
7. **Atomic Completion**: Finalizes the download by renaming the partial file to the final filename
8. **Resume Support**: Metadata file contains all download information, enabling resume with a single metadata file reference

### Directory Structure

When downloading a file named `largefile.zip`, the following directory structure is created:

```
largefile/
  ├── largefile.zip                    # Final downloaded file
  ├── largefile.zip.part               # Partial file (during download)
  └── largefile.zip.metadata.json      # Download metadata and progress
```

### Resume Capability

The metadata file contains the complete download state including:

- Original URL
- Filename
- Total file size
- Download progress per worker
- ETag and Last-Modified headers for validation

To resume an interrupted download, simply provide the metadata file path:

```bash
download-manager resume <path/to/file.metadata.json>
```

The tool will automatically extract all necessary information and continue downloading from where it left off.

## Requirements

The download source must:

- Provide a positive `ContentLength` header
- Support byte range requests (HTTP 206 Partial Content)
- Return matching `Content-Range` responses for each range request

## Testing

Run the test suite to verify resume and failure behavior:

```bash
go test -v
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
