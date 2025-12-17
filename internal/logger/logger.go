package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
	colorCyan   = "\033[36m"
)

func init() {
	// Configure console writer with custom level names and colors
	writer := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		FormatLevel: func(i interface{}) string {
			level := strings.ToUpper(fmt.Sprintf("%s", i))
			switch level {
			case "DEBUG":
				return fmt.Sprintf("%s%sDBG%s", colorDim, colorCyan, colorReset)
			case "INFO":
				return fmt.Sprintf("%s%sRUN%s", colorBold, colorGreen, colorReset)
			case "WARN":
				return fmt.Sprintf("%s%sWRN%s", colorBold, colorYellow, colorReset)
			case "ERROR":
				return fmt.Sprintf("%s%sERR%s", colorBold, colorRed, colorReset)
			case "FATAL":
				return fmt.Sprintf("%s%sDIE%s", colorBold, colorRed, colorReset)
			default:
				return level
			}
		},
	}
	log.Logger = zerolog.New(writer).With().Timestamp().Logger()
}

// Debugf logs a debug message
func Debugf(msg string, args ...interface{}) {
	log.Debug().Msgf(msg, args...)
}

// Infof logs an info message
func Infof(msg string, args ...interface{}) {
	log.Info().Msgf(msg, args...)
}

// Warnf logs a warning message
func Warnf(msg string, args ...interface{}) {
	log.Warn().Msgf(msg, args...)
}

// Errorf logs an error message with the error
func Errorf(err error, msg string, args ...interface{}) {
	log.Error().Err(err).Msgf(msg, args...)
}

// RecoverErrorf logs the message as an error.
// To be used instead of Errorf when you don't have the error type (usually in recover)
func RecoverErrorf(msg string, args ...interface{}) {
	log.Error().Msgf(msg, args...)
}

// Fatalf logs a fatal message and exits
func Fatalf(err error, msg string, args ...interface{}) {
	log.Fatal().Err(err).Msgf(msg, args...)
}
