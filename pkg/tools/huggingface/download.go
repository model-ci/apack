package huggingface

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/pkg/progress"
	"golang.org/x/sync/errgroup"
)

const (
	LargeFileThreshold = 100 * 1024 * 1024
	MinChunkSize       = 10 * 1024 * 1024
	MaxChunkCount      = 100
	DefaultConcurrency = 5
	DefaultMaxRetries  = 5
)

type Downloader struct {
	URL         string
	DestPath    string
	TotalSize   int64
	Concurrency int
	MaxRetries  int
	Client      *http.Client
	Progress    *progress.ProgressWriter
}

// NewDownloader creates a Downloader with optional progress tracking
func NewDownloader(url, destPath string, totalSize int64, pw *progress.ProgressWriter) *Downloader {
	return &Downloader{
		URL:         url,
		DestPath:    destPath,
		TotalSize:   totalSize,
		Concurrency: DefaultConcurrency,
		MaxRetries:  DefaultMaxRetries,
		Progress:    pw,
		Client:      createOptimizedClient(),
	}
}

func (d *Downloader) SetConcurrency(n int) *Downloader {
	if n > 0 {
		d.Concurrency = n
	}
	return d
}

func (d *Downloader) SetMaxRetries(n int) *Downloader {
	d.MaxRetries = n
	return d
}

// Start initiates the download process
func (d *Downloader) Start(ctx context.Context) error {
	// 0. Pre-check: If complete, mark progress done
	if info, err := os.Stat(d.DestPath); err == nil && info.Size() == d.TotalSize {
		if d.Progress != nil {
			d.Progress.MarkCompleted()
		}
		log.Logger.Debugf("File %s already exists and is complete.", d.DestPath)
		return nil
	}

	// 1. Small file strategy
	if d.TotalSize < LargeFileThreshold {
		return d.downloadPartWithRetry(ctx, d.DestPath, 0, d.TotalSize, false)
	}

	// 2. Large file strategy
	return d.downloadConcurrent(ctx)
}

func (d *Downloader) downloadConcurrent(ctx context.Context) error {
	chunkSize := d.calculateChunkSize()
	partCount := int(math.Ceil(float64(d.TotalSize) / float64(chunkSize)))

	// === SYNC PROGRESS FOR RESUME ===
	// Calculate total bytes already on disk from partial chunks
	if d.Progress != nil {
		var existingTotal int64
		for i := 0; i < partCount; i++ {
			info, err := os.Stat(d.getPartFilename(i))
			if err == nil {
				existingTotal += info.Size()
			}
		}
		// "Fast-forward" the progress bar to match local state
		d.syncProgress(existingTotal)
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(d.Concurrency)

	for i := 0; i < partCount; i++ {
		index := i
		start := int64(index) * chunkSize
		end := start + chunkSize
		if end > d.TotalSize {
			end = d.TotalSize
		}

		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			partFile := d.getPartFilename(index)
			// Pass 'true' for isPart
			if err := d.downloadPartWithRetry(ctx, partFile, start, end, true); err != nil {
				return fmt.Errorf("chunk %d failed: %w", index, err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return d.mergeParts(partCount)
}

func (d *Downloader) downloadPartWithRetry(ctx context.Context, filename string, rangeStart, rangeEnd int64, isPart bool) error {
	var lastErr error

	for attempt := 0; attempt <= d.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			jitter := time.Duration(rand.IntN(1000)) * time.Millisecond
			sleepTime := backoff + jitter

			if sleepTime > 30*time.Second {
				sleepTime = 30 * time.Second
			}

			log.Logger.Debugf("Retrying chunk... attempt %d, wait %v\n", attempt, sleepTime)

			select {
			case <-time.After(sleepTime):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = d.downloadPart(ctx, filename, rangeStart, rangeEnd, isPart)
		if lastErr == nil {
			return nil
		}

		if !d.isRetryableError(lastErr) {
			return lastErr
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (d *Downloader) isRetryableError(err error) bool {
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	if netErr, ok := err.(net.Error); ok && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	return true
}

func (d *Downloader) downloadPart(ctx context.Context, filename string, rangeStart, rangeEnd int64, isPart bool) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, _ := file.Stat()
	currentLocalSize := stat.Size()
	expectedPartSize := rangeEnd - rangeStart

	// For single-file mode resume, sync progress if needed
	if !isPart && d.Progress != nil && currentLocalSize > 0 {
		// Simple approach: sync what we found locally
		d.syncProgress(currentLocalSize)
	}

	if currentLocalSize >= expectedPartSize {
		return nil
	}

	downloadStart := rangeStart + currentLocalSize
	req, err := http.NewRequestWithContext(ctx, "GET", d.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", downloadStart, rangeEnd-1))

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	// === PROGRESS INTEGRATION ===
	var reader io.Reader = resp.Body
	if d.Progress != nil {
		// TeeReader splits the stream: writes to file AND updates progress
		// Concurrent writes to d.Progress are safe due to its internal mutex
		reader = io.TeeReader(resp.Body, d.Progress)
	}

	_, err = io.Copy(file, reader)
	return err
}

// syncProgress updates the progress writer to reflect bytes already downloaded (resume scenario)
func (d *Downloader) syncProgress(bytes int64) {
	if bytes <= 0 {
		return
	}
	// Create a dummy buffer to "feed" the progress writer
	// Since ProgressWriter.Write only increments counters, the content doesn't matter
	const bufSize = 32 * 1024
	buf := make([]byte, bufSize)

	remaining := bytes
	for remaining > 0 {
		toWrite := remaining
		if toWrite > bufSize {
			toWrite = bufSize
		}
		// This triggers pw.written += n
		d.Progress.Write(buf[:toWrite])
		remaining -= toWrite
	}
}

// ... (mergeParts, calculateChunkSize, getPartFilename implementation remains same) ...
func (d *Downloader) mergeParts(count int) error {
	outFile, err := os.Create(d.DestPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for i := 0; i < count; i++ {
		partFile := d.getPartFilename(i)
		part, err := os.Open(partFile)
		if err != nil {
			return fmt.Errorf("failed to open chunk %s: %w", partFile, err)
		}
		if _, err := io.Copy(outFile, part); err != nil {
			part.Close()
			return err
		}
		part.Close()
		os.Remove(partFile)
	}
	return nil
}

func (d *Downloader) calculateChunkSize() int64 {
	chunkSize := d.TotalSize / MaxChunkCount
	if chunkSize < MinChunkSize {
		chunkSize = MinChunkSize
	}
	const MB = 1024 * 1024
	if remainder := chunkSize % MB; remainder != 0 {
		chunkSize += MB - remainder
	}
	return chunkSize
}

func (d *Downloader) getPartFilename(index int) string {
	return fmt.Sprintf("%s.part.%d", d.DestPath, index)
}

// createOptimizedClient configures timeouts specifically for large file downloads
func createOptimizedClient() *http.Client {
	return &http.Client{
		// IMPORTANT: Set to 0 (no limit) to allow large file downloads to complete.
		// A fixed timeout (e.g. 60s) would kill the connection even if data is flowing.
		Timeout: 0,

		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second, // Max time to establish TCP connection
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second, // Close connection if idle for 90s
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second, // Max time to wait for server first byte
		},
	}
}
