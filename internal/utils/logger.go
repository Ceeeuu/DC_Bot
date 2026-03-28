package utils

import "github.com/sirupsen/logrus"

func NewLogger(level string) *logrus.Logger {
	logger := logrus.New()

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lvl = logrus.InfoLevel
	}

	logger.SetLevel(lvl)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return logger
}
