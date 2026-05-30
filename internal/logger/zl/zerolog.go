package zl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/leyl1ne/ApiGateway/internal/logger"
	"github.com/rs/zerolog"
)

type ZerologLogger struct {
	log zerolog.Logger
}

func NewZerologLogger(cfg logger.Config) (*ZerologLogger, error) {
	const op = "logger.zl.NewZeroLogger"

	lvl, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	output, err := resolveOutput(cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("%s: failed resolve log output: %w", op, err)
	}

	var writer io.Writer
	switch cfg.Format {
	case "console":
		writer = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "2006-01-02 15:04:05",
		}
	case "json", "":
		writer = output
	default:
		return nil, fmt.Errorf("%s: unknown log format: %s", op, cfg.Format)
	}

	l := zerolog.New(writer).With().Timestamp().Logger()
	return &ZerologLogger{log: l}, nil

}

func resolveOutput(output string) (io.Writer, error) {
	switch output {
	case "", "stderr":
		return os.Stderr, nil
	case "stdout":
		return os.Stdout, nil
	default:
		dir := filepath.Dir(output)
		if dir != "." && dir != "/" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("create log dir %s: %w", dir, err)
			}
		}
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file %s: %w", output, err)
		}
		return f, nil

	}
}

func (z *ZerologLogger) With(fields ...logger.Field) logger.Logger {
	if len(fields) == 0 {
		return z
	}

	ctx := z.log.With()

	for _, f := range fields {
		ctx = ctx.Interface(f.Key, f.Value)
	}

	newLogger := ctx.Logger()

	return &ZerologLogger{
		log: newLogger,
	}
}

func (z *ZerologLogger) Info(msg string, fields ...logger.Field) {
	z.log.Info().Fields(toZerologFields(fields)).Msg(msg)
}

func (z *ZerologLogger) Warn(msg string, fields ...logger.Field) {
	z.log.Warn().Fields(toZerologFields(fields)).Msg(msg)
}

func (z *ZerologLogger) Debug(msg string, fields ...logger.Field) {
	z.log.Debug().Fields(toZerologFields(fields)).Msg(msg)
}

func (z *ZerologLogger) Error(msg string, fields ...logger.Field) {
	z.log.Error().Fields(toZerologFields(fields)).Msg(msg)
}

func (z *ZerologLogger) Fatal(msg string, fields ...logger.Field) {
	z.log.Fatal().Fields(toZerologFields(fields)).Msg(msg)
}

func (z *ZerologLogger) Panic(msg string, fields ...logger.Field) {
	z.log.Panic().Fields(toZerologFields(fields)).Msg(msg)
}

func toZerologFields(fields []logger.Field) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}

	m := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}
