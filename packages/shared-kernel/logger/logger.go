package logger

import (
	"log"
)

type Logger interface {
	Printf(format string, v ...any)
}

func Std() Logger {
	return log.Default()
}

type Noop struct{}

func (Noop) Printf(string, ...any) {}
