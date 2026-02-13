package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"msr-archiver/internal/audio"
)

// ProgressUpdate carries per-file download progress information.
type ProgressUpdate struct {
	BytesWritten int64
	TotalBytes   int64
}

// ProgressFunc receives throttled progress updates while a file is downloading.
type ProgressFunc func(ProgressUpdate)

// FileDownloadResult describes a completed file download.
type FileDownloadResult struct {
	ContentType  string
	BytesWritten int64
	Duration     time.Duration
}

// Downloader streams files from HTTP endpoints.
type Downloader struct {
	httpClient *http.Client
}

// New creates a Downloader.
func New(httpClient *http.Client) *Downloader {
	return &Downloader{httpClient: httpClient}
}

// DownloadToFile downloads a URL to a given destination path.
func (d *Downloader) DownloadToFile(ctx context.Context, url, dstPath string) (string, error) {
	result, err := d.DownloadToFileWithProgress(ctx, url, dstPath, nil)
	if err != nil {
		return "", err
	}
	return result.ContentType, nil
}

// DownloadToFileWithProgress downloads a URL to a destination path and reports progress.
func (d *Downloader) DownloadToFileWithProgress(ctx context.Context, url, dstPath string, progress ProgressFunc) (FileDownloadResult, error) {
	started := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return FileDownloadResult{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return FileDownloadResult{}, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return FileDownloadResult{}, fmt.Errorf("download %s: unexpected status %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return FileDownloadResult{}, fmt.Errorf("create parent dirs: %w", err)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return FileDownloadResult{}, fmt.Errorf("create file %s: %w", dstPath, err)
	}
	defer out.Close()

	totalBytes := resp.ContentLength
	if totalBytes <= 0 {
		totalBytes = -1
	}

	bytesWritten, err := copyWithProgress(out, resp.Body, totalBytes, progress)
	if err != nil {
		return FileDownloadResult{}, fmt.Errorf("write file %s: %w", dstPath, err)
	}

	return FileDownloadResult{
		ContentType:  resp.Header.Get("Content-Type"),
		BytesWritten: bytesWritten,
		Duration:     time.Since(started),
	}, nil
}

// DownloadSong downloads a song and converts WAV to FLAC to match previous behavior.
func (d *Downloader) DownloadSong(ctx context.Context, dir, name, sourceURL string) (string, string, error) {
	path, fileType, _, err := d.DownloadSongWithProgress(ctx, dir, name, sourceURL, nil)
	if err != nil {
		return "", "", err
	}
	return path, fileType, nil
}

// DownloadSongWithProgress downloads a song and reports file progress.
func (d *Downloader) DownloadSongWithProgress(ctx context.Context, dir, name, sourceURL string, progress ProgressFunc) (string, string, FileDownloadResult, error) {
	base := filepath.Join(dir, MakeValid(name))
	wavPath := base + ".wav"
	mp3Path := base + ".mp3"

	dl, err := d.DownloadToFileWithProgress(ctx, sourceURL, wavPath, progress)
	if err != nil {
		return "", "", FileDownloadResult{}, err
	}

	contentType := dl.ContentType
	if strings.Contains(strings.ToLower(contentType), "audio/mpeg") {
		if err := os.Rename(wavPath, mp3Path); err != nil {
			return "", "", FileDownloadResult{}, fmt.Errorf("rename to mp3: %w", err)
		}
		return mp3Path, ".mp3", dl, nil
	}

	flacPath := base + ".flac"
	if err := audio.WAVToFLAC(ctx, wavPath, flacPath); err != nil {
		return "", "", FileDownloadResult{}, err
	}
	if err := os.Remove(wavPath); err != nil {
		return "", "", FileDownloadResult{}, fmt.Errorf("remove wav file: %w", err)
	}

	return flacPath, ".flac", dl, nil
}

func copyWithProgress(dst io.Writer, src io.Reader, totalBytes int64, progress ProgressFunc) (int64, error) {
	buf := make([]byte, 32*1024)
	var bytesWritten int64
	var lastProgress time.Time

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if written > 0 {
				bytesWritten += int64(written)
			}
			if writeErr != nil {
				return bytesWritten, writeErr
			}
			if written != n {
				return bytesWritten, io.ErrShortWrite
			}

			if progress != nil {
				now := time.Now()
				if lastProgress.IsZero() || now.Sub(lastProgress) >= 700*time.Millisecond {
					progress(ProgressUpdate{BytesWritten: bytesWritten, TotalBytes: totalBytes})
					lastProgress = now
				}
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				if progress != nil {
					progress(ProgressUpdate{BytesWritten: bytesWritten, TotalBytes: totalBytes})
				}
				return bytesWritten, nil
			}
			return bytesWritten, readErr
		}
	}
}
