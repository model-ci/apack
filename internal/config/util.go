package config

import (
	"os"
	"path/filepath"
)

const (
	JsonFile = "config.json"
)

func JsonPath(configRoot string) string {
	if configRoot != "" {
		return filepath.Join(configRoot, JsonFile)
	}
	if os.Getuid() == 0 {
		return filepath.Join("/var/lib/apack/", JsonFile)
	} else {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".apack", JsonFile)
	}
}
