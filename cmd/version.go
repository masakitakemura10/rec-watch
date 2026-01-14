package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	// ldflags will set these
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "バージョン情報を表示します",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("rec-watch %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Date:   %s\n", date)

		if info, ok := debug.ReadBuildInfo(); ok {
			fmt.Printf("Go:     %s\n", info.GoVersion)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
