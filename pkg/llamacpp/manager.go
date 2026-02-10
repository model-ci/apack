package llamacpp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/utils"
)

type Manager struct {
	customPath string
	tempPath   string
	binaryPath string
}

func NewManager(customPath string) (*Manager, error) {
	return &Manager{
		customPath: customPath,
	}, nil
}

func (m *Manager) GenBinaryPath() (string, error) {
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

func (m *Manager) SetBinaryPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := os.Setenv(types.ENV_LLAMACPP_BINARY_PATH, absPath); err != nil {
		return err
	}

	m.binaryPath = absPath

	return nil
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
