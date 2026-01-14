package cmd

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "初期セットアップの設定を削除します",
	Long:  `LaunchAgent(plist)のアンロードと削除を行います。`,
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("ホームディレクトリの取得に失敗: %v", err)
		}

		plistPath := filepath.Join(home, "Library/LaunchAgents/com.user.recwatch.plist")

		// 1. Unload
		log.Printf("LaunchAgentをアンロードしています: %s", plistPath)
		if output, err := exec.Command("launchctl", "unload", plistPath).CombinedOutput(); err != nil {
			log.Printf("⚠️ アンロードに失敗しました (すでにロードされていない可能性があります): %v\n%s", err, string(output))
		} else {
			log.Println("✅ アンロード成功")
		}

		// 2. Remove plist
		if _, err := os.Stat(plistPath); err == nil {
			if err := os.Remove(plistPath); err != nil {
				log.Fatalf("❌ plistファイルの削除に失敗: %v", err)
			}
			log.Println("✅ plistファイルを削除しました")
		} else {
			log.Println("⚠️ plistファイルが見つかりません")
		}

		log.Println("アンインストール完了 (ログファイルと出力ディレクトリ、rec-watchバイナリ自体は残っています)")
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
