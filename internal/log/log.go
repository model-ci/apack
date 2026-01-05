package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *zap.SugaredLogger

type LogConfig struct {
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
	Level      zapcore.Level
	Console    bool
}

func DefaultLogConfig() LogConfig {
	return LogConfig{
		Filename:   "app.log",
		MaxSize:    10,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
		Level:      zapcore.InfoLevel,
		Console:    true,
	}
}

func InitLogger(config LogConfig) {
	encoder := getEncoder()

	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && lvl >= config.Level
	})

	var cores []zapcore.Core

	if config.Filename != "" {

		fileWriter := getLogWriter(config)
		fileCore := zapcore.NewCore(encoder, fileWriter, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= config.Level
		}))

		cores = append(cores, fileCore)
		config.Console = false
	}

	if config.Console {
		stdoutSyncer := zapcore.AddSync(os.Stdout)
		stdoutCore := zapcore.NewCore(encoder, stdoutSyncer, lowPriority)

		stderrSyncer := zapcore.AddSync(os.Stderr)
		stderrCore := zapcore.NewCore(encoder, stderrSyncer, highPriority)

		cores = append(cores, stdoutCore, stderrCore)
	}

	core := zapcore.NewTee(cores...)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	Logger = logger.Sugar()
}

func Init(filename, level string) {
	config := DefaultLogConfig()
	config.Filename = filename
	l, err := zapcore.ParseLevel(level)
	if err != nil {
		panic(err)
	}
	config.Level = l
	InitLogger(config)
}

func Close() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLogWriter(config LogConfig) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   config.Filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}
	return zapcore.AddSync(lumberJackLogger)
}
