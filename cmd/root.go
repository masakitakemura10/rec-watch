package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"

	"github.com/mt4110/rec-watch/internal/config"
	"github.com/mt4110/rec-watch/internal/convert"
	"github.com/mt4110/rec-watch/internal/logger"
	"github.com/mt4110/rec-watch/internal/updater"
	"github.com/mt4110/rec-watch/internal/watcher"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "rec-watch [filesOrDirs...]",
	Short: "動画ファイルを一括で1080pのMP4に変換・監視します。",
	Long:  `macOSの画面収録などで作成された動画ファイルを、H.264形式のMP4に一括変換するCLIツール。監視モード(RecWatch)で自動化も可能。`,
	Args:  cobra.ArbitraryArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		// Initialize Config
		loadedCfg, err := config.Load()
		if err != nil {
			log.Printf("設定ファイルの読み込みに失敗しました (デフォルト値を使用します): %v", err)
			loadedCfg = config.NewDefault()
		}
		cfg = loadedCfg

		// Bind Flags to Config (Override config file values with flags if flag is changed)
		// Note: Ideally we should use viper for complex binding, but here we do manual mapping or just use flags if set.
		// For simplicity in this step, we will manually overwrite cfg values with flag values if they are explicitly set.
		// However, cobra flags are already bound to variables in init().
		// We need to merge them.
		// Re-binding flags to config struct fields is tricky without viper.
		// We will update 'cfg' with values from flags.
		updateConfigFromFlags(cmd, cfg)

		// Setup Logger
		logger.Setup(cfg.LogFile)

		// Check Updates
		updater.CheckFFmpeg()
	},
	Run: func(cmd *cobra.Command, args []string) {

		cvt := convert.New(cfg)

		// Watch Mode
		// If --watch is passed, we prioritise watch mode.
		// If WatchDirs is set in config, it uses that.
		// If args are provided with --watch, we treat args as watch targets (overriding or appending to config).
		if flagWatch {
			targets := args
			if len(targets) > 0 {
				cfg.WatchDirs = targets // Override config with CLI args
			}

			// If still empty (no args, no config), default to current dir
			if len(cfg.WatchDirs) == 0 {
				cfg.WatchDirs = []string{"."}
			}

			w := watcher.New(cfg, cvt)
			log.Println("👀 監視モードを開始しました (Ctrl+C で終了)")
			w.Run() // This blocks
			return
		}

		// Legacy support: if config has WatchDirs but flag is NOT set, we do NOT enter watch mode automatically
		// unless we want that behavior. Standard CLI usually requires a flag for long-running processes.
		// So we only watch if flagWatch is true.

		// Batch Mode
		inputPatterns := args
		if len(inputPatterns) == 0 {
			inputPatterns = []string{"."}
		}

		var files []string
		videoExtensions := "{mov,MOV,m4v,mp4,avi,mkv}"
		home, _ := os.UserHomeDir()

		for _, input := range inputPatterns {
			processedInput := input
			if input == "~" {
				processedInput = home
			} else if strings.HasPrefix(input, "~/") {
				processedInput = filepath.Join(home, input[2:])
			}

			var pattern string
			info, err := os.Stat(processedInput)
			if err == nil && info.IsDir() {
				pattern = filepath.Join(processedInput, "**/*."+videoExtensions)
			} else {
				pattern = processedInput
			}

			fsys := os.DirFS(".")
			globPattern := pattern
			isAbs := filepath.IsAbs(pattern)
			if isAbs {
				fsys = os.DirFS("/")
				globPattern, err = filepath.Rel("/", pattern)
				if err != nil {
					log.Printf("警告: パス '%s' の処理に失敗しました: %v", pattern, err)
					continue
				}
			}

			matches, err := doublestar.Glob(fsys, globPattern)
			if err != nil {
				log.Printf("警告: パターン '%s' の検索に失敗しました: %v", pattern, err)
				continue
			}

			if isAbs {
				for i, match := range matches {
					matches[i] = filepath.Join("/", match)
				}
			}

			files = append(files, matches...)
		}

		// Unique
		uniqueFiles := make(map[string]bool)
		var result []string
		for _, f := range files {
			if !uniqueFiles[f] {
				uniqueFiles[f] = true
				result = append(result, f)
			}
		}
		files = result

		if len(files) == 0 {
			log.Println("変換対象が見つかりません。")
			return
		}

		// Keyword Filter
		var filteredFiles []string
		if len(cfg.Keywords) > 0 || len(cfg.IgnoreKeywords) > 0 {
			for _, f := range files {
				name := filepath.Base(f)
				lowerName := strings.ToLower(name)

				// Exclude
				excluded := false
				for _, k := range cfg.IgnoreKeywords {
					if strings.Contains(lowerName, strings.ToLower(k)) {
						excluded = true
						break
					}
				}
				if excluded {
					continue
				}

				// Include (if keywords are set)
				if len(cfg.Keywords) > 0 {
					included := false
					for _, k := range cfg.Keywords {
						if strings.Contains(lowerName, strings.ToLower(k)) {
							included = true
							break
						}
					}
					if !included {
						continue
					}
				}

				filteredFiles = append(filteredFiles, f)
			}
		} else {
			filteredFiles = files
		}

		if len(filteredFiles) == 0 {
			log.Println("フィルタリングの結果、対象ファイルがありません。")
			return
		}

		cvt.ProcessFiles(filteredFiles)
	},
}

// Temporary variables for flags
var (
	flagDest           string
	flagCRF            int
	flagPreset         string
	flagFPS            int
	flagMute           bool
	flagKeywords       []string
	flagIgnoreKeywords []string
	flagNoPad          bool
	flagStampPerFile   bool
	flagNoTrash        bool
	flagBatchStamp     bool
	flagFFmpegBin      string
	flagConcurrent     int
	flagWatch          bool
	flagNotify         bool
	flagDryRun         bool
	flagProfile        string
	flagParallelSplit  bool
	flagGPU            bool
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Define flags
	rootCmd.Flags().StringVar(&flagDest, "dest", "", "出力先ディレクトリ")
	rootCmd.Flags().IntVar(&flagCRF, "crf", 0, "CRF値 (品質)")
	rootCmd.Flags().StringVar(&flagPreset, "preset", "", "エンコードプリセット")
	rootCmd.Flags().IntVar(&flagFPS, "fps", 0, "フレームレート (0で無効)")
	rootCmd.Flags().BoolVar(&flagMute, "mute", false, "音声をミュートする")
	rootCmd.Flags().StringSliceVar(&flagKeywords, "keywords", []string{}, "ファイル名に含まれるキーワードでフィルタ")
	rootCmd.Flags().StringSliceVar(&flagIgnoreKeywords, "ignore-keywords", []string{}, "ファイル名に含まれるキーワードを除外") // New
	rootCmd.Flags().BoolVar(&flagNoPad, "no-pad", false, "1080pにリサイズする際に黒帯を追加しない")
	rootCmd.Flags().BoolVar(&flagStampPerFile, "stamp-per-file", false, "個別のファイル名にタイムスタンプを追加する")
	rootCmd.Flags().BoolVar(&flagNoTrash, "no-trash", false, "変換元のファイルをゴミ箱に移動しない")
	rootCmd.Flags().BoolVar(&flagBatchStamp, "batch-stamp", true, "出力先ディレクトリをタイムスタンプ付きで作成する (default true)")
	rootCmd.Flags().StringVar(&flagFFmpegBin, "ffmpeg-bin", "", "ffmpegのバイナリパスを明示的に指定する")
	rootCmd.Flags().IntVar(&flagConcurrent, "concurrent", 0, "並列実行数")
	rootCmd.Flags().BoolVar(&flagWatch, "watch", false, "指定したディレクトリを監視して自動変換する")
	rootCmd.Flags().BoolVar(&flagNotify, "notify", true, "変換完了時にデスクトップ通知を送る")
	rootCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "実行せずにコマンドを表示する")
	rootCmd.Flags().StringVar(&flagProfile, "profile", "", "使用するプロファイル名")
	rootCmd.Flags().BoolVar(&flagParallelSplit, "parallel-split", false, "動画を分割して並列変換する（大容量ファイル向け・爆速）")
	rootCmd.Flags().BoolVar(&flagGPU, "gpu", false, "GPU(VideoToolbox)を使用して変換する（超爆速・画質/圧縮率はCPUに劣る）")
}

func updateConfigFromFlags(cmd *cobra.Command, c *config.Config) {
	flags := cmd.Flags()

	// 1. Apply Profile first if exists
	if flags.Changed("profile") {
		entry, ok := c.Profiles[flagProfile]
		if ok {
			// Apply profile settings (only if they are non-zero/valid)
			if entry.CRF > 0 {
				c.CRF = entry.CRF
			}
			if entry.Preset != "" {
				c.Preset = entry.Preset
			}
			log.Printf("ℹ️ プロファイル '%s' を適用しました (CRF: %d, Preset: %s)", flagProfile, c.CRF, c.Preset)
		} else {
			log.Printf("⚠️ プロファイル '%s' は見つかりませんでした。デフォルト設定を使用します。", flagProfile)
		}
	}

	if flags.Changed("dest") {
		c.DestDir = flagDest
	}
	if flags.Changed("crf") {
		c.CRF = flagCRF
	}
	if flags.Changed("preset") {
		c.Preset = flagPreset
	}
	if flags.Changed("fps") {
		c.FPS = flagFPS
	}
	if flags.Changed("mute") {
		c.Mute = flagMute
	}
	if flags.Changed("keywords") {
		c.Keywords = flagKeywords
	}
	if flags.Changed("ignore-keywords") {
		c.IgnoreKeywords = flagIgnoreKeywords
	}
	if flags.Changed("no-pad") {
		c.NoPad = flagNoPad
	}
	if flags.Changed("stamp-per-file") {
		c.StampPerFile = flagStampPerFile
	}
	if flags.Changed("no-trash") {
		c.NoTrash = flagNoTrash
	}
	if flags.Changed("batch-stamp") {
		c.BatchStamp = flagBatchStamp
	}
	if flags.Changed("ffmpeg-bin") {
		c.FFmpegBin = flagFFmpegBin
	}
	if flags.Changed("concurrent") {
		c.Concurrent = flagConcurrent
	}
	// Notify is default true, so we need careful handling if user passed --notify=false
	if flags.Changed("notify") {
		c.Notify = flagNotify
	}
	if flags.Changed("dry-run") {
		c.DryRun = flagDryRun
	}
	if flags.Changed("parallel-split") {
		c.ParallelSplit = flagParallelSplit
	}
	if flags.Changed("gpu") {
		c.GPU = flagGPU
	}

	// Watch logic overlap
	if flagWatch {
		// If watch flag is on, we determine the watch dir.
		// Use the first arg if available, else current dir.
		// cmd.Flags().Args() logic...
	}
}
