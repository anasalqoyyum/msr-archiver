package download

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newDownloader(handler roundTripFunc) *Downloader {
	return New(&http.Client{Transport: handler})
}

func response(status int, contentType, body string) *http.Response {
	h := make(http.Header)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode:    status,
		Body:          io.NopCloser(strings.NewReader(body)),
		Header:        h,
		ContentLength: int64(len(body)),
	}
}

func TestDownloadToFileSuccess(t *testing.T) {
	d := newDownloader(func(req *http.Request) (*http.Response, error) {
		return response(200, "audio/mpeg", "abc123"), nil
	})

	ctx := context.Background()
	outPath := filepath.Join(t.TempDir(), "nested", "song.bin")

	contentType, err := d.DownloadToFile(ctx, "https://example.test/audio", outPath)
	if err != nil {
		t.Fatalf("DownloadToFile failed: %v", err)
	}
	if !strings.Contains(contentType, "audio/mpeg") {
		t.Fatalf("unexpected content type: %q", contentType)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file was not created: %v", err)
	}
	if string(b) != "abc123" {
		t.Fatalf("unexpected file contents: %q", string(b))
	}
}

func TestDownloadToFileStatusError(t *testing.T) {
	d := newDownloader(func(req *http.Request) (*http.Response, error) {
		return response(http.StatusBadGateway, "", ""), nil
	})

	_, err := d.DownloadToFile(context.Background(), "https://example.test/bad", filepath.Join(t.TempDir(), "x"))
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestDownloadToFileWithProgress(t *testing.T) {
	body := strings.Repeat("x", 128)
	d := newDownloader(func(req *http.Request) (*http.Response, error) {
		return response(200, "audio/flac", body), nil
	})

	var updates []ProgressUpdate
	_, err := d.DownloadToFileWithProgress(
		context.Background(),
		"https://example.test/audio",
		filepath.Join(t.TempDir(), "song.bin"),
		func(update ProgressUpdate) {
			updates = append(updates, update)
		},
	)
	if err != nil {
		t.Fatalf("DownloadToFileWithProgress failed: %v", err)
	}
	if len(updates) == 0 {
		t.Fatalf("expected progress updates, got none")
	}

	last := updates[len(updates)-1]
	if last.BytesWritten != int64(len(body)) {
		t.Fatalf("unexpected bytes written: %d", last.BytesWritten)
	}
	if last.TotalBytes != int64(len(body)) {
		t.Fatalf("unexpected total bytes: %d", last.TotalBytes)
	}
}

func TestDownloadSongMP3(t *testing.T) {
	d := newDownloader(func(req *http.Request) (*http.Response, error) {
		return response(200, "audio/mpeg", "fake-mp3"), nil
	})

	dir := t.TempDir()

	path, fileType, err := d.DownloadSong(context.Background(), dir, "my song", "https://example.test/song")
	if err != nil {
		t.Fatalf("DownloadSong failed: %v", err)
	}
	if fileType != ".mp3" {
		t.Fatalf("expected .mp3 file type, got %q", fileType)
	}
	if !strings.HasSuffix(path, "my_song.mp3") {
		t.Fatalf("unexpected output path: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected mp3 output to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "my_song.wav")); !os.IsNotExist(err) {
		t.Fatalf("wav file should not remain, got err=%v", err)
	}
}

func TestDownloadSongWithProgressReturnsStats(t *testing.T) {
	body := "fake-mp3"
	d := newDownloader(func(req *http.Request) (*http.Response, error) {
		return response(200, "audio/mpeg", body), nil
	})

	var latest ProgressUpdate
	path, fileType, result, err := d.DownloadSongWithProgress(
		context.Background(),
		t.TempDir(),
		"my song",
		"https://example.test/song",
		func(update ProgressUpdate) {
			latest = update
		},
	)
	if err != nil {
		t.Fatalf("DownloadSongWithProgress failed: %v", err)
	}
	if fileType != ".mp3" {
		t.Fatalf("expected .mp3 file type, got %q", fileType)
	}
	if !strings.HasSuffix(path, "my_song.mp3") {
		t.Fatalf("unexpected output path: %s", path)
	}
	if result.BytesWritten != int64(len(body)) {
		t.Fatalf("unexpected bytes written: %d", result.BytesWritten)
	}
	if latest.BytesWritten != int64(len(body)) {
		t.Fatalf("expected final progress bytes %d, got %d", len(body), latest.BytesWritten)
	}
}
