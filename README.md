# Download Manager v4

v4 keeps the parallel ranged downloader behavior from v3, but separates the implementation into focused modules:

- `main.go`: CLI orchestration and download finalization
- `model.go`: metadata models, worker configuration, and range splitting
- `metadata.go`: atomic metadata persistence and loading
- `range.go`: HTTP ranged-response validation
- `filename.go`: output filename and content-type inference
- `progress.go`: aggregate and per-worker progress state/rendering
- `worker.go`: one worker's ranged request and positional writes
- `download.go`: preflight, worker coordination, cancellation, and completion checks
- `main_test.go`: resume and failure behavior tests

Run it with:

```text
go run ./v4 <url> <workers> [filename] [metadata.json]
```

The source must provide a positive `ContentLength` and honor byte ranges with matching `206 Partial Content` and `Content-Range` responses.
