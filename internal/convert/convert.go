package convert

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mt4110/rec-watch/internal/config"
	"github.com/mt4110/rec-watch/internal/split"
)

// SendNotification sends a desktop notification
func SendNotification(title, message, filePath string) {
	if _, err := exec.LookPath("terminal-notifier"); err == nil {
		args := []string{"-title", title, "-message", message, "-sound", "default"}
		if filePath != "" {
			u := url.URL{Scheme: "file", Path: filePath}
			args = append(args, "-open", u.String())
		}
		exec.Command("terminal-notifier", args...).Run()
		return
	}
	// Fallback
	script := fmt.Sprintf(`tell application "System Events" to display notification "%s" with title "%s" sound name "default"`, message, title)
	exec.Command("osascript", "-e", script).Run()
}

type Converter struct {
	Cfg *config.Config
}

func New(cfg *config.Config) *Converter {
	return &Converter{Cfg: cfg}
}

func (c *Converter) ProcessFiles(files []string) {
	// 出力ディレクトリを作成
	baseOut, _ := filepath.Abs(c.Cfg.DestDir)
	batchDir := baseOut
	if c.Cfg.BatchStamp {
		batchDir = filepath.Join(baseOut, nowStamp())
	}
	if err := os.MkdirAll(batchDir, 0755); err != nil {
		log.Fatalf("出力ディレクトリの作成に失敗: %v", err)
	}

	log.Printf("変換対象: %d件", len(files))
	log.Printf("出力先: %s", batchDir)
	log.Printf("並列実行数: %d", c.Cfg.Concurrent)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, c.Cfg.Concurrent)

	for _, inPath := range files {
		wg.Add(1)
		semaphore <- struct{}{} // 実行枠を確保

		go func(inPath string) {
			defer func() {
				<-semaphore // 実行枠を解放
				wg.Done()
			}()
			if _, err := c.Convert(inPath, batchDir); err != nil {
				log.Printf("❌ 変換失敗: %s -> %v", inPath, err)
			}
		}(inPath)
	}

	wg.Wait()
	log.Println("✅ すべて完了")
}

func (c *Converter) Convert(inPath string, outDir string) (string, error) {
	// Check for Parallel Split Mode
	// Threshold: e.g. 1GB (1024*1024*1024 bytes)
	// For testing, let's say 500MB or if requested via config

	shouldSplit := c.Cfg.ParallelSplit

	// If auto-detect logic is needed:
	// info, _ := os.Stat(inPath)
	// if info.Size() > 1*1024*1024*1024 { shouldSplit = true }

	if shouldSplit {
		// GPU mode overrides Split (checked inside ConvertSplit or here)
		if c.Cfg.GPU {
			// GPU mode doesn't need split usually, but user asked for comparison.
			// Using GPU on split chunks is possible but maybe inefficient startup overhead.
			// Let's stick to: if GPU, linear GPU. If CPU, maybe Split.
			// But for now, let's implement Split Logic here.
			// Actually, let's keep it simple: ConvertSplit calls ConvertOne for chunks.
			return c.ConvertSplit(inPath, outDir)
		}
		return c.ConvertSplit(inPath, outDir)
	}

	return c.ConvertOne(inPath, outDir)
}

func (c *Converter) ConvertOne(inPath string, outDir string) (string, error) {

	// ファイルの更新日時を取得してファイル名にする
	info, err := os.Stat(inPath)
	var timeStamp string
	if err != nil {
		timeStamp = time.Now().Format("2006-01-02_15-04-05")
	} else {
		timeStamp = info.ModTime().Format("2006-01-02_15-04-05")
	}

	outPath := filepath.Join(outDir, fmt.Sprintf("%s.mp4", timeStamp))

	vf := "scale=1920:1080:force_original_aspect_ratio=decrease"
	if !c.Cfg.NoPad {
		vf += ",pad=1920:1080:(ow-iw)/2:(oh-ih)/2"
	}

	ffmpegPath := "ffmpeg"
	if c.Cfg.FFmpegBin != "" {
		ffmpegPath = c.Cfg.FFmpegBin
	}

	ffmpegArgs := []string{
		"-i", inPath,
	}

	// Codec Selection
	if c.Cfg.GPU {
		// macOS VideoToolbox
		ffmpegArgs = append(ffmpegArgs, "-c:v", "h264_videotoolbox")
		// Bitrate or Quality control for GPU
		// Apple's HW encoder uses -q:v (0-100) or -b:v.
		// CRF doesn't work directly.
		// Mapping simplistic CRF to Quality is hard.
		// Let's use a quality setting or default.
		// q:v 60-80 is usually good.
		// Let's assume some default if not specified/mapped.
		// For simplicity, we just use default or mapped CRF inverse?
		// Higher CRF = Lower Quality.
		// Higher q:v = Higher Quality.
		// q = 100 - CRF*2 ? (Roughly)
		q := 70 // default
		if c.Cfg.CRF > 0 {
			// Map CRF 20 -> 80, CRF 30 -> 60
			q = 100 - (c.Cfg.CRF * 2)
			if q < 1 {
				q = 1
			}
		}
		ffmpegArgs = append(ffmpegArgs, "-q:v", fmt.Sprintf("%d", q))
	} else {
		// CPU x264
		ffmpegArgs = append(ffmpegArgs, "-vcodec", "libx264")
		ffmpegArgs = append(ffmpegArgs, "-preset", c.Cfg.Preset)
		ffmpegArgs = append(ffmpegArgs, "-crf", fmt.Sprintf("%d", c.Cfg.CRF))
	}

	ffmpegArgs = append(ffmpegArgs,
		"-vf", vf,
		"-movflags", "+faststart",
	)

	if c.Cfg.FPS > 0 {
		ffmpegArgs = append(ffmpegArgs, "-r", fmt.Sprintf("%d", c.Cfg.FPS))
	}

	if c.Cfg.Mute {
		ffmpegArgs = append(ffmpegArgs, "-an")
	} else {
		ffmpegArgs = append(ffmpegArgs, "-acodec", "aac", "-b:a", "128k", "-ac", "2")
	}

	ffmpegArgs = append(ffmpegArgs, outPath)

	log.Printf("▶ 変換: %s -> %s", inPath, outPath)
	startTime := time.Now()

	if c.Cfg.DryRun {
		// cmdLen := fmt.Sprintf("%s %s", ffmpegPath, fmt.Sprint(ffmpegArgs))
		log.Printf("[DryRun] Command: %s %v", ffmpegPath, ffmpegArgs)
		return outPath, nil // Return success for dry-run
	}

	cmd := exec.Command(ffmpegPath, ffmpegArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg実行エラー: %v\n%s", err, string(output))
	}

	// Stats collecting
	duration := time.Since(startTime).Seconds()
	infoOut, _ := os.Stat(outPath)
	convertedSize := infoOut.Size()
	originalSize := info.Size()

	logEntry := struct {
		Type          string  `json:"type"`
		Input         string  `json:"input"`
		Output        string  `json:"output"`
		DurationSec   float64 `json:"duration_sec"`
		OriginalSize  int64   `json:"original_size"`
		ConvertedSize int64   `json:"converted_size"`
		SizeDiff      int64   `json:"size_diff"`
		Timestamp     string  `json:"timestamp"`
	}{
		Type:          "conversion_result",
		Input:         inPath,
		Output:        outPath,
		DurationSec:   duration,
		OriginalSize:  originalSize,
		ConvertedSize: convertedSize,
		SizeDiff:      originalSize - convertedSize,
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	if jsonBytes, err := json.Marshal(logEntry); err == nil {
		// Logger writes to file, we use a special prefix or just raw JSON line
		// Since we use std log which adds date/time prefix, it might break pure JSON lines if we are not careful.
		// However, for simplicity, we'll just log the JSON string. The stats command will have to handle the log prefix.
		// Or we can assume the stats command filters lines that look like JSON.
		log.Println(string(jsonBytes))
	}

	if !c.Cfg.NoTrash && !c.Cfg.DryRun {

		if err := moveToTrash(inPath); err != nil {
			log.Printf("🗑 ゴミ箱への移動に失敗: %s -> %v", inPath, err)
		}
	}

	return outPath, nil
}

func moveToTrash(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("osascript", "-e", `tell application "Finder" to move POSIX file "`+absPath+`" to trash`)
		return cmd.Run()
	case "linux":
		if _, err := exec.LookPath("gio"); err == nil {
			cmd := exec.Command("gio", "trash", absPath)
			return cmd.Run()
		}
		return fmt.Errorf("gio command not found")
	case "windows":
		psCmd := fmt.Sprintf("Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile('%s', [Microsoft.VisualBasic.FileIO.UIOption]::OnlyErrorDialogs, [Microsoft.VisualBasic.FileIO.RecycleOption]::SendToRecycleBin)", absPath)
		cmd := exec.Command("powershell", "-Command", psCmd)
		return cmd.Run()
	default:
		return fmt.Errorf("%s はサポートされていないOSです", runtime.GOOS)
	}
}

func nowStamp() string {
	return time.Now().Format("20060102")
}

func (c *Converter) ConvertSplit(inPath string, outDir string) (string, error) {
	log.Printf("🚀 並列分割モードで処理開始: %s", filepath.Base(inPath))

	if c.Cfg.DryRun {
		log.Printf("[DryRun] Would split %s into chunks...", inPath)
	}

	// Temp Dir
	tmpDir, err := os.MkdirTemp("", "rec-watch-split-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// 1. Split
	// Split into e.g. 5 minutes (300s) chunks? Or shorter for more parallelism?
	// 5 mins is good balance.
	s := split.New(c.Cfg.FFmpegBin)
	var chunks []string
	if c.Cfg.DryRun {
		// Mock chunks
		chunks = []string{
			filepath.Join(tmpDir, "chunk_000.mp4"),
			filepath.Join(tmpDir, "chunk_001.mp4"),
			filepath.Join(tmpDir, "chunk_002.mp4"),
		}
	} else {
		chunks, err = s.Split(inPath, tmpDir, 300)
		if err != nil {
			return "", err
		}
	}

	// 2. Parallel Transcode chunks
	// We want to reuse ConvertOne logic but output to tmpDir
	// We need 'wait group' again here, basically sub-scheduling.
	// NOTE: This will use available CPU. Since we are inside a worker, this might spawn more threads.
	// If global concurrency is limited, we might be blocked if we use the same semaphore?
	// But ConvertOne doesn't use semaphore, ProcessFiles does.
	// So we can spawn goroutines here freely, but we should be careful about exploding CPU usage.
	// FFmpeg x264 already uses multi-threads.
	// Running multiple FFmpegs on multicores is beneficial but too many is bad.
	// For "Split" mode, we assume this file takes over the machine.

	type result struct {
		index int
		path  string
		err   error
	}

	results := make([]result, len(chunks))
	var wg sync.WaitGroup
	// Limit sub-parallelism to e.g. 4 or Config.Concurrent
	// If main loop has concurrency, this might be recursive.
	// For v0.5.0, we assume --parallel-split is used with --concurrent=1 for the main loop effectively,
	// OR we manage it.

	sem := make(chan struct{}, c.Cfg.Concurrent) // Reuse configured concurrency or e.g. 4
	if c.Cfg.Concurrent == 0 {
		sem = make(chan struct{}, 4)
	}

	log.Printf("⚡️ %d個のチャンクを %d並列で変換中...", len(chunks), cap(sem))

	for i, chunk := range chunks {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, chunkPath string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			// Output name in temp dir
			// ConvertOne adds timestamp to name, we need to preserve order.
			// We can hack ConvertOne or just craft args manually...
			// ConvertOne is designed for final output.
			// Let's just create a helper or use ConvertOne but rename result?
			// ConvertOne uses logic to ensure unique name in outDir.
			// If we pass tmpDir as outDir, it will work.

			// ACTUALLY: ConvertOne applies scaling/padding/crf.
			// We want exactly that.
			// But we need to keep filenames sortable or track them.
			// ConvertOne returns outPath.

			chuckOutDir := filepath.Join(tmpDir, "converted")
			os.MkdirAll(chuckOutDir, 0755)

			// We forcibly use the same basename to keep 'chunk_001' part for sorting?
			// But ConvertOne changes name to timestamp... that breaks sorting order!
			// ConvertOne: `outPath := filepath.Join(outDir, fmt.Sprintf("%s.mp4", timeStamp))`
			// This is BAD for Split mode. We need filename preservation.

			// Fix: We need a lower level function `doConvert(in, out)` used by ConvertOne.
			// Refactoring ConvertOne slightly.

			outFile := filepath.Join(chuckOutDir, filepath.Base(chunkPath)) // chunk_000.mp4

			// Use internal private method if we refactor, or just Copy/Paste logic for V1?
			// Let's refactor ConvertOne to use `convertFile(in, out)`
			err := c.convertFile(chunkPath, outFile)
			results[i] = result{index: i, path: outFile, err: err}
			if err != nil {
				log.Printf("⚠️ チャンク変換失敗: %s: %v", chunkPath, err)
			}
		}(i, chunk)
	}
	wg.Wait()

	// Check errors
	var convertedChunks []string
	for _, res := range results {
		if res.err != nil {
			return "", fmt.Errorf("chunk %d failed: %v", res.index, res.err)
		}
		convertedChunks = append(convertedChunks, res.path)
	}

	// 3. Merge (Concat)
	// Create concat list
	listFile := filepath.Join(tmpDir, "concat.txt")
	f, err := os.Create(listFile)
	if err != nil {
		return "", err
	}

	for _, chunk := range convertedChunks {
		// path should be absolute or relative. Absolute is safest.
		abs, _ := filepath.Abs(chunk)
		f.WriteString(fmt.Sprintf("file '%s'\n", abs))
	}
	f.Close()

	// Final Output Path (using same logic as ConvertOne for naming)
	// We need to determine final output name.
	info, _ := os.Stat(inPath)
	timeStamp := info.ModTime().Format("2006-01-02_15-04-05")
	finalOutPath := filepath.Join(outDir, fmt.Sprintf("%s.mp4", timeStamp))

	log.Println("🔗 チャンクを結合中...")

	// ffmpeg -f concat -safe 0 -i list.txt -c copy out.mp4
	mergeArgs := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c", "copy",
		finalOutPath,
	}

	cmd := exec.Command(c.Cfg.FFmpegBin, mergeArgs...)
	if c.Cfg.FFmpegBin == "" {
		cmd = exec.Command("ffmpeg", mergeArgs...)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("merge failed: %v\n%s", err, string(out))
	}

	// 4. Logging & Trash (Standard process) - Handled by caller 'ProcessFiles' if we returned simple error?
	// Wait, ConvertOne handled Trash. ConvertSplit should too.
	// Duplicated logic from ConvertOne end.

	// Stats collecting (manually for now or reuse)
	// ...

	// Trash
	if !c.Cfg.NoTrash && !c.Cfg.DryRun {
		if err := moveToTrash(inPath); err != nil {
			log.Printf("🗑 ゴミ箱への移動に失敗: %s -> %v", inPath, err)
		}
	}

	return finalOutPath, nil
}

// Low level conversion logic
func (c *Converter) convertFile(inPath, outPath string) error {
	vf := "scale=1920:1080:force_original_aspect_ratio=decrease"
	if !c.Cfg.NoPad {
		vf += ",pad=1920:1080:(ow-iw)/2:(oh-ih)/2"
	}

	ffmpegPath := "ffmpeg"
	if c.Cfg.FFmpegBin != "" {
		ffmpegPath = c.Cfg.FFmpegBin
	}

	ffmpegArgs := []string{
		"-i", inPath,
	}

	// Codec Logic Reused
	if c.Cfg.GPU {
		ffmpegArgs = append(ffmpegArgs, "-c:v", "h264_videotoolbox")
		q := 70
		if c.Cfg.CRF > 0 {
			q = 100 - (c.Cfg.CRF * 2)
			if q < 1 {
				q = 1
			}
		}
		ffmpegArgs = append(ffmpegArgs, "-q:v", fmt.Sprintf("%d", q))
	} else {
		ffmpegArgs = append(ffmpegArgs, "-vcodec", "libx264")
		ffmpegArgs = append(ffmpegArgs, "-preset", c.Cfg.Preset)
		ffmpegArgs = append(ffmpegArgs, "-crf", fmt.Sprintf("%d", c.Cfg.CRF))
	}

	ffmpegArgs = append(ffmpegArgs,
		"-vf", vf,
		"-movflags", "+faststart",
	)

	// Audio
	if c.Cfg.Mute {
		ffmpegArgs = append(ffmpegArgs, "-an")
	} else {
		ffmpegArgs = append(ffmpegArgs, "-acodec", "aac", "-b:a", "128k", "-ac", "2")
	}

	ffmpegArgs = append(ffmpegArgs, outPath)

	if c.Cfg.DryRun {
		log.Printf("[DryRun chunk] %v", ffmpegArgs)
		return nil
	}

	cmd := exec.Command(ffmpegPath, ffmpegArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg error: %v\n%s", err, string(out))
	}
	return nil
}
