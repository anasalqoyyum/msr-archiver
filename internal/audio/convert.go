package audio

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CheckFFmpeg verifies ffmpeg is available.
func CheckFFmpeg(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg is required but unavailable: %w", err)
	}
	return nil
}

// WAVToFLAC converts a wav file into flac using ffmpeg.
func WAVToFLAC(ctx context.Context, wavPath, flacPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", wavPath, "-vn", "-compression_level", "12", flacPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg wav->flac failed: %w: %s", err, stderr.String())
	}
	return nil
}
