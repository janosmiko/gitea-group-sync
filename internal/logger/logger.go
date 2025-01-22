package logger

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Configure() {
	logLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))

	// Set default level if environment variable is not set
	if logLevel == "" {
		logLevel = "INFO"
	}

	debug := os.Getenv("DEBUG")
	if debug == "true" {
		logLevel = "DEBUG"
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		// If parsing fails, default to info level
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return trimPath(file) + ":" + strconv.Itoa(line)
	}

	partsOrder := []string{"time", "level", "tag", "message"}
	if level <= zerolog.DebugLevel {
		partsOrder = []string{"time", "level", "caller", "tag", "message"}
	}

	logger := log.Output(zerolog.ConsoleWriter{
		Out:           os.Stderr,
		TimeFormat:    time.RFC3339,
		PartsOrder:    partsOrder,
		FieldsExclude: []string{"tag"},
	}).With().Str("tag", "").Logger()

	if level <= zerolog.DebugLevel {
		logger = logger.With().Caller().Logger()
	}

	log.Logger = logger
}

func CronLogger() PrintfLogger {
	return &cronLogger{logger: log.Logger}
}

type cronLogger struct {
	logger zerolog.Logger
}

func (l *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info().Fields(keysAndValues).Msg(msg)
}

func (l *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.logger.Error().Err(err).Fields(keysAndValues).Msg(msg)
}

func (l *cronLogger) Printf(format string, v ...interface{}) {
	l.logger.Debug().Msgf("process is still running: "+format, v...)
}

type PrintfLogger interface {
	Printf(string, ...interface{})
}

var _ cron.Logger = &cronLogger{}
var _ PrintfLogger = &cronLogger{}

func New() Logger {
	return Logger{Logger: log.Logger}
}

type Logger struct {
	zerolog.Logger
}

func (l Logger) Tag(tag string) Logger {
	l.Logger = l.Logger.With().Str("tag", fmt.Sprintf("[%s]", tag)).Logger()

	return l
}

func trimPath(path string) string {
	if !strings.Contains(path, "gitea-ldap-sync/") {
		return path
	}

	parts := strings.Split(path, "gitea-ldap-sync/")
	if len(parts) > 1 {
		return parts[1]
	}

	return path
}
