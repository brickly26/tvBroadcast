// services/fragmenter.go
package services

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EnsureFragmented rewrites inPath to outPath if the file is not already
// fragmented (very naive: looks for "moof").  It returns the output path.
func EnsureFragmented(inPath string) (string, error) {
	// if it already contains "moof" within first few MB, assume fMP4
	f, err := os.Open(inPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 4*1024*1024)
	n, _ := f.Read(buf)
	if strings.Contains(string(buf[:n]), "moof") {
		return inPath, nil // already fragmented
	}

	// otherwise rewrite -> *.frag.mp4 alongside the original
	outPath := strings.TrimSuffix(inPath, ".mp4") + ".frag.mp4"
	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", inPath,
		"-c", "copy",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof+dash",
		outPath,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w", err)
	}
	return outPath, nil
}