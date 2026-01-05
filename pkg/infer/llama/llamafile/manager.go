package llamafile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/utils"
)

type Manager struct {
	customPath string
	tempPath   string
}

func NewManager(customPath string) (*Manager, error) {
	return &Manager{
		customPath: customPath,
	}, nil
}

func (m *Manager) GetBinaryPath() (string, error) {
	if m.customPath == "" {
		m.tempPath = os.TempDir()
		m.customPath = filepath.Join(m.tempPath, LlamafileBinaryName)
	} else {
		if filepath.Base(m.customPath) != LlamafileBinaryName {
			m.customPath = filepath.Join(m.customPath, LlamafileBinaryName)
		}
	}

	return m.customPath, buildBin(m.customPath)
}

func buildBin(file string) error {
	if utils.FileExist(file) {
		return nil
	}

	binary, _, err := getPlatformBinary()
	if err != nil {
		return err
	}

	if err := os.WriteFile(file, binary, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}
	return nil
}

func (m *Manager) Cleanup() error {
	if m.tempPath != "" {
		return os.Remove(m.tempPath)
	}
	return nil
}

func getPlatformBinary() ([]byte, string, error) {
	return getEmbeddedBinary()
}
