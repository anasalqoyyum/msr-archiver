package metadata

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Input holds metadata write parameters.
type Input struct {
	FilePath     string
	FileType     string
	Album        string
	Title        string
	AlbumArtists []string
	Artists      []string
	TrackNumber  int
	CoverPath    string
	LyricPath    string
}

// Apply writes metadata tags, cover art, and optional lyrics by remuxing with ffmpeg.
func Apply(ctx context.Context, in Input) error {
	lyrics := ""
	if in.LyricPath != "" {
		b, err := os.ReadFile(in.LyricPath)
		if err != nil {
			return fmt.Errorf("read lyric file %s: %w", in.LyricPath, err)
		}
		lyrics = string(b)
	}

	args := []string{"-y", "-i", in.FilePath}
	coverEnabled := in.CoverPath != ""
	if coverEnabled {
		args = append(args, "-i", in.CoverPath)
	}

	args = append(args, "-map", "0:a")
	if coverEnabled {
		args = append(args, "-map", "1:v")
	}
	args = append(args, "-c:a", "copy")

	if coverEnabled {
		if in.FileType == ".mp3" {
			args = append(args,
				"-c:v", "mjpeg",
				"-id3v2_version", "3",
				"-disposition:v", "attached_pic",
				"-metadata:s:v", "title=Cover",
				"-metadata:s:v", "comment=Cover (front)",
			)
		} else {
			args = append(args,
				"-c:v", "png",
				"-disposition:v", "attached_pic",
			)
		}
	}

	albumArtists := strings.Join(in.AlbumArtists, "")
	artists := strings.Join(in.Artists, "")

	args = append(args,
		"-metadata", "album="+in.Album,
		"-metadata", "title="+in.Title,
		"-metadata", "album_artist="+albumArtists,
		"-metadata", "albumartist="+albumArtists,
		"-metadata", "artist="+artists,
		"-metadata", fmt.Sprintf("track=%d", in.TrackNumber),
	)

	if lyrics != "" {
		args = append(args, "-metadata", "lyrics="+lyrics)
	}

	tmpPath := filepath.Join(filepath.Dir(in.FilePath), ".tmp-metadata-"+filepath.Base(in.FilePath))
	args = append(args, tmpPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg metadata write failed: %w: %s", err, stderr.String())
	}

	if err := os.Rename(tmpPath, in.FilePath); err != nil {
		return fmt.Errorf("replace output file with metadata version: %w", err)
	}

	return nil
}
