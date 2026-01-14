package updater

import (
	"log"
	"os/exec"
	"strings"
)

func CheckFFmpeg() {
	// 1. Check existence
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Println("⚠️ ffmpeg が見つかりません。インストールを推奨します: `brew install ffmpeg`")
		return
	}

	// 2. Simple version check suggestion (Logic to check for updates is complex without context, so we suggest update if brew is available)
	if _, err := exec.LookPath("brew"); err == nil {
		// Run brew outdated ffmpeg
		cmd := exec.Command("brew", "outdated", "ffmpeg")
		output, err := cmd.CombinedOutput()
		if err == nil && len(output) > 0 && strings.Contains(string(output), "ffmpeg") {
			log.Println("ℹ️ ffmpeg のアップデートが可能です。自動更新は設定されていませんが、以下で更新できます:")
			log.Println("   brew upgrade ffmpeg")
		}
	}
}
