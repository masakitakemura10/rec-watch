package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "初期セットアップを行います",
	Long:  `必要なディレクトリ(~/Desktop/ScreenRecordings)の作成と、LaunchAgent(plist)の生成・登録を行います。`,
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("ホームディレクトリの取得に失敗: %v", err)
		}

		// 1. Create Directory
		targetDir := filepath.Join(home, "Desktop", "ScreenRecordings")
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Printf("ディレクトリ作成失敗: %v", err)
		} else {
			log.Printf("✅ ディレクトリを確認: %s", targetDir)
		}

		// 2. Generate plist
		username := os.Getenv("USER")
		if username == "" {
			username = filepath.Base(home)
		}

		// Find executable path
		execPath, err := os.Executable()
		if err != nil {
			execPath = "/usr/local/bin/rec-watch" // fallback
		}

		plistPath := filepath.Join(home, "Library/LaunchAgents/com.user.recwatch.plist")

		tmpl := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.recwatch</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecPath}}</string>
        <string>--watch</string>
        <string>{{.WatchDir}}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>
`
		data := struct {
			ExecPath string
			WatchDir string
			LogPath  string
		}{
			ExecPath: execPath,
			WatchDir: targetDir,
			LogPath:  filepath.Join(home, "Library/Logs/rec-watch.log"),
		}

		f, err := os.Create(plistPath)
		if err != nil {
			log.Fatalf("plistファイルの作成に失敗: %v", err)
		}

		t := template.Must(template.New("plist").Parse(tmpl))
		if err := t.Execute(f, data); err != nil {
			f.Close()
			log.Fatalf("plistの書き込みに失敗: %v", err)
		}
		f.Close()
		log.Printf("✅ plistファイルを作成: %s", plistPath)

		// 3. Launchctl load
		log.Println("LaunchAgentをロードしますか？ (y/n)")
		var response string
		fmt.Scanln(&response)
		if response == "y" || response == "Y" {
			// Unload first just in case
			exec.Command("launchctl", "unload", plistPath).Run()

			cmd := exec.Command("launchctl", "load", plistPath)
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("❌ launchctl load 失敗: %v\n%s", err, string(output))
			} else {
				log.Println("✅ launchctl load 成功！ rec-watchがバックグラウンドで起動しました。")
			}
		} else {
			log.Println("スキップしました。手動で実行する場合は以下のコマンドを入力してください:")
			fmt.Printf("launchctl load %s\n", plistPath)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
