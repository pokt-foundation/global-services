package logger

import (
	logger "github.com/sirupsen/logrus"
)

// Log is a custom instance of the logrus logger
var Log = logger.New()

func init() {
	// Log as JSON instead of the default ASCII formatter.
	Log.SetFormatter(&logger.JSONFormatter{})
}
