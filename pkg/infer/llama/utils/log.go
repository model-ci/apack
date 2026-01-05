package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var LevelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

var levelColors = map[LogLevel]string{
	DEBUG: "\033[36m", // Cyan
	INFO:  "\033[32m", // Green
	WARN:  "\033[33m", // Yellow
	ERROR: "\033[31m", // Red
	FATAL: "\033[35m", // Magenta
}

const colorReset = "\033[0m"

type Logger struct {
	level      LogLevel
	output     io.Writer
	colorized  bool
	prefix     string
	timeFormat string
}

func NewLogger(level LogLevel, output io.Writer) *Logger {
	return &Logger{
		level:      level,
		output:     output,
		colorized:  isTerminal(output),
		timeFormat: "2006-01-02 15:04:05",
	}
}

func NewFileLogger(level LogLevel, filename string) (*Logger, error) {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		level:      level,
		output:     file,
		colorized:  false,
		timeFormat: "2006-01-02 15:04:05",
	}, nil
}

func (l *Logger) SetPrefix(prefix string) {
	l.prefix = prefix
}

func (l *Logger) SetTimeFormat(format string) {
	l.timeFormat = format
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	os.Exit(1)
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	now := time.Now()
	message := fmt.Sprintf(format, args...)

	_, file, line, ok := runtime.Caller(2)
	var caller string
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	} else {
		caller = "unknown"
	}

	var logMessage string
	if l.colorized {
		color := levelColors[level]
		logMessage = fmt.Sprintf("%s[%s]%s %s %s%s %s: %s\n",
			color,
			LevelNames[level],
			colorReset,
			now.Format(l.timeFormat),
			l.prefix,
			caller,
			LevelNames[level],
			message)
	} else {
		logMessage = fmt.Sprintf("[%s] %s %s%s %s: %s\n",
			LevelNames[level],
			now.Format(l.timeFormat),
			l.prefix,
			caller,
			LevelNames[level],
			message)
	}

	l.output.Write([]byte(logMessage))
}

var defaultLogger = NewLogger(INFO, os.Stdout)

func SetLevel(level LogLevel) {
	defaultLogger.level = level
}

func SetOutput(output io.Writer) {
	defaultLogger.output = output
	defaultLogger.colorized = isTerminal(output)
}

func SetPrefix(prefix string) {
	defaultLogger.SetPrefix(prefix)
}

func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	defaultLogger.Fatal(format, args...)
}

func ParseLevel(levelStr string) (LogLevel, error) {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN", "WARNING":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return INFO, fmt.Errorf("unknown log level: %s", levelStr)
	}
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return f == os.Stdout || f == os.Stderr
	}
	return false
}

type Fields map[string]interface{}

func (l *Logger) WithFields(fields Fields) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

type FieldLogger struct {
	logger *Logger
	fields Fields
}

func (fl *FieldLogger) Debug(format string, args ...interface{}) {
	fl.log(DEBUG, format, args...)
}

func (fl *FieldLogger) Info(format string, args ...interface{}) {
	fl.log(INFO, format, args...)
}

func (fl *FieldLogger) Warn(format string, args ...interface{}) {
	fl.log(WARN, format, args...)
}

func (fl *FieldLogger) Error(format string, args ...interface{}) {
	fl.log(ERROR, format, args...)
}

func (fl *FieldLogger) Fatal(format string, args ...interface{}) {
	fl.log(FATAL, format, args...)
	os.Exit(1)
}

func (fl *FieldLogger) log(level LogLevel, format string, args ...interface{}) {
	if level < fl.logger.level {
		return
	}

	message := fmt.Sprintf(format, args...)

	var fieldStr strings.Builder
	for k, v := range fl.fields {
		fieldStr.WriteString(fmt.Sprintf(" %s=%v", k, v))
	}

	fl.logger.log(level, "%s%s", message, fieldStr.String())
}

func WithFields(fields Fields) *FieldLogger {
	return defaultLogger.WithFields(fields)
}
