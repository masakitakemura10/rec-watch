package split

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sort"
)

// Splitter handles file segmentation
type Splitter struct {
	FFmpegBin string
}

func New(ffmpegBin string) *Splitter {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	return &Splitter{FFmpegBin: ffmpegBin}
}

// Split divides the input file into chunks in the output directory.
// Returns a list of generated chunk file paths.
// segmentTime is in seconds (e.g., 300 for 5 minutes).
func (s *Splitter) Split(inFile string, outDir string, segmentTime int) ([]string, error) {
	// Pattern for output segments: chunk_000.mp4, chunk_001.mp4...
	// We use .mp4 container for segments to keep it simple, or .ts if keyframe issues
	// But .mp4 with 'segment' muxer and reset_timestamps should be fine for re-concatenating if we re-encode them.
	// Actually, if we re-encode chunks, we can't concatenate them naively unless they are identical params.
	// But 'concat' demuxer works well for same-codec files.

	outPattern := filepath.Join(outDir, "chunk_%03d.mp4")

	args := []string{
		"-i", inFile,
		"-c", "copy",
		"-map", "0",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", segmentTime),
		"-reset_timestamps", "1", // Important for independent chunks
		outPattern,
	}

	cmd := exec.Command(s.FFmpegBin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("split failed: %v\n%s", err, string(output))
	}

	// Verify and collect output files
	files, err := filepath.Glob(filepath.Join(outDir, "chunk_*.mp4"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	log.Printf("🔪 分割完了: %s -> %d チャンク", filepath.Base(inFile), len(files))
	return files, nil
}
