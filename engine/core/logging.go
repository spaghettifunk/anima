package core

import (
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

var once sync.Once

type logger struct {
	*log.Logger
}

var singleton *logger

func getLogger() *logger {
	if singleton == nil {
		once.Do(
			func() {
				l := log.NewWithOptions(os.Stderr, log.Options{
					ReportCaller:    true,
					ReportTimestamp: true,
					TimeFormat:      time.RFC3339,
					Prefix:          "Engine 🏎️ ",
				})
				// TODO: configurable
				l.SetLevel(log.DebugLevel)
				singleton = &logger{l}
			})
	}
	return singleton
}

func LogDebug(msg string, args ...interface{}) {
	getLogger().Debugf(msg, args...)
}

func LogInfo(msg string, args ...interface{}) {
	getLogger().Infof(msg, args...)
}

func LogWarn(msg string, args ...interface{}) {
	getLogger().Warnf(msg, args...)
}

func LogError(msg string, args ...interface{}) {
	getLogger().Errorf(msg, args...)
}

func LogFatal(msg string, args ...interface{}) {
	getLogger().Fatalf(msg, args...)
}
