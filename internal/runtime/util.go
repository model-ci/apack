package runtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func findModelFile(absPath string) (string, error) {
	stat, err := os.Lstat(absPath)
	if err != nil {
		return "", err
	}
	if stat.Mode().IsRegular() {
		return absPath, nil
	} else if !stat.IsDir() {
		return "", fmt.Errorf("could not find model file in %s: path is not regular file or directory", absPath)
	}

	modelPath := ""
	if err := filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".gguf") && d.Type().IsRegular() {
			if modelPath == "" {
				modelPath = path
			} else {
				return fmt.Errorf("multiple model files found: %s and %s", modelPath, path)
			}
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("error searching for model file in %s: %w", absPath, err)
	} else if modelPath == "" {
		return "", fmt.Errorf("could not find model file in %s", absPath)
	}
	return modelPath, nil
}
