package log

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestInitLogger(t *testing.T) {
	testLogFile := "test.log"

	_ = os.Remove(testLogFile)

	config := LogConfig{
		Filename:   testLogFile,
		MaxSize:    1,
		MaxBackups: 3,
		MaxAge:     1,
		Compress:   false,
		Level:      zapcore.DebugLevel,
		Console:    false,
	}

	InitLogger(config)

	Logger.Debug("This is a debug message")
	Logger.Info("This is an info message")
	Logger.Warn("This is a warning message")
	Logger.Error("This is an error message")

	Close()

	if _, err := os.Stat(testLogFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created")
	}

	file, err := os.Open(testLogFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	logLines := []string{}
	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	if len(logLines) != 4 {
		t.Errorf("Expected 4 log lines, got %d", len(logLines))
	}

	expectedMessages := []string{"debug message", "info message", "warning message", "error message"}
	for i, msg := range expectedMessages {
		if i < len(logLines) && !strings.Contains(strings.ToLower(logLines[i]), msg) {
			t.Errorf("Log line %d does not contain expected message '%s'", i, msg)
		}
	}

	_ = os.Remove(testLogFile)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultLogConfig()

	if config.Filename != "app.log" {
		t.Errorf("Expected default filename 'app.log', got '%s'", config.Filename)
	}

	if config.MaxSize != 10 {
		t.Errorf("Expected default MaxSize 10, got %d", config.MaxSize)
	}

	if config.Level != zapcore.InfoLevel {
		t.Errorf("Expected default Level InfoLevel, got %v", config.Level)
	}

	if !config.Console {
		t.Errorf("Expected default Console to be true")
	}
}

func TestSimpleInit(t *testing.T) {
	testLogFile := "simple_test.log"
	_ = os.Remove(testLogFile)

	Init(testLogFile, "debug")
	Logger.Info("Simple init test")
	Close()

	if _, err := os.Stat(testLogFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created with simple init")
	}

	_ = os.Remove(testLogFile)
}
