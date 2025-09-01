package log

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// L is the global sugared logger used throughout surisink.
var L *zap.SugaredLogger

// InitWithConfig initializes zap logger based on level and format.
// level: debug|info|warn|error
// format: json|console
func InitWithConfig(level, format string) error {
	lvl := zapcore.InfoLevel
	switch strings.ToLower(level) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "info":
		lvl = zapcore.InfoLevel
	case "warn", "warning":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var enc zapcore.Encoder
	if strings.ToLower(format) == "console" {
		enc = zapcore.NewConsoleEncoder(encCfg)
	} else {
		enc = zapcore.NewJSONEncoder(encCfg)
	}

	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), lvl)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	L = logger.Sugar()
	return nil
}

// Sync flushes buffered logs.
func Sync() {
	if L != nil {
		_ = L.Sync()
	}
}
