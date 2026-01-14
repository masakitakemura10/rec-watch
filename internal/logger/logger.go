package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

var rotator *lumberjack.Logger

func Setup(logFilePath string) {
	if logFilePath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			logFilePath = filepath.Join(home, "Library", "Logs", "rec-watch.log")
		} else {
			logFilePath = "rec-watch.log"
		}
	}

	fmt.Printf("Log file: %s\n", logFilePath)

	// Lumberjack logger for rotation
	rotator = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     7,    // days
		Compress:   true, // gzip
	}

	// MultiWriter to write to both stdout and file
	mw := io.MultiWriter(os.Stdout, rotator)

	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func MuteStdout() {
	if rotator != nil {
		log.SetOutput(rotator)
	}
}
