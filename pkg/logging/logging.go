package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func Init(level, format string) (zerolog.Logger, error) {
	l := parseLevel(level)
	output := os.Stdout

	if format == "json" {
		logger = zerolog.New(output).Level(l).With().Timestamp().Caller().Logger()
	} else {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: output, TimeFormat: time.RFC3339}).Level(l).With().Timestamp().Caller().Logger()
	}

	return logger, nil
}

func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	}
	return zerolog.InfoLevel
}

func Log() *zerolog.Logger {
	return &logger
}
