package cmd

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "環境の診断を行います",
	Long:  `FFmpegのインストール状況、ログディレクトリの権限、plistの状態などをチェックします。`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("🏥 環境診断を開始します...")
		hasError := false

		// 1. ffmpeg check
		if path, err := exec.LookPath("ffmpeg"); err != nil {
			log.Println("❌ ffmpeg が見つかりません。 `brew install ffmpeg` を実行してください。")
			hasError = true
		} else {
			log.Printf("✅ ffmpeg found: %s", path)
			// version check (simple)
			if out, err := exec.Command("ffmpeg", "-version").Output(); err == nil {
				// Print first line
				var firstLine string
				for _, b := range out {
					if b == '\n' {
						break
					}
					firstLine += string(b)
				}
				log.Printf("   Version: %s", firstLine)
			}
		}

		// 2. terminal-notifier check
		if path, err := exec.LookPath("terminal-notifier"); err != nil {
			log.Println("⚠️ terminal-notifier が見つかりません。通知をクリックしてファイルを開く機能が動作しません。 (推奨: `brew install terminal-notifier`)")
		} else {
			log.Printf("✅ terminal-notifier found: %s", path)
		}

		// 3. Log Directory check
		home, _ := os.UserHomeDir()
		logDir := filepath.Join(home, "Library/Logs")
		if info, err := os.Stat(logDir); err != nil {
			log.Printf("⚠️ ログディレクトリ (%s) にアクセスできません: %v", logDir, err)
		} else if !info.IsDir() {
			log.Printf("⚠️ %s はディレクトリではありません", logDir)
		} else {
			// Write check
			testFile := filepath.Join(logDir, "rec-watch-write-test")
			if f, err := os.Create(testFile); err != nil {
				log.Printf("❌ ログディレクトリへの書き込み権限がありません: %v", err)
				hasError = true
			} else {
				f.Close()
				os.Remove(testFile)
				log.Println("✅ ログディレクトリ権限 OK")
			}
		}

		// 4. Plist check
		plistPath := filepath.Join(home, "Library/LaunchAgents/com.user.recwatch.plist")
		if _, err := os.Stat(plistPath); err != nil {
			log.Println("ℹ️ LaunchAgent設定 (plist) は見つかりませんでした (init未実行)")
		} else {
			log.Printf("✅ plist found: %s", plistPath)
			// Check if loaded
			cmd := exec.Command("launchctl", "list")
			out, _ := cmd.Output()
			if err == nil {
				// grep com.user.recwatch
				// Simple string search
				if contains(out, []byte("com.user.recwatch")) {
					log.Println("✅ LaunchAgent is loaded (launchctl list confirms)")
				} else {
					log.Println("⚠️ plistはありますが、ロードされていません (`launchctl load` が必要かもしれません)")
				}
			}
		}

		if hasError {
			log.Println("\n❌ いくつかの問題が見つかりました。修正してください。")
			os.Exit(1)
		} else {
			log.Println("\n✅ 診断完了: 概ね問題なさそうです！")
		}
	},
}

func contains(b []byte, sub []byte) bool {
	for i := 0; i < len(b)-len(sub)+1; i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
