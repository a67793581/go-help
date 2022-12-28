package zap_help

import "go.uber.org/zap"

// NewLogger returns a new Logger.
//
// By default, Loggers info at zap's InfoLevel.
func NewLogger(l *zap.Logger) *Logger {
	logger := &Logger{
		log: l.Sugar(),
	}
	return logger
}

// Logger adapts zap's Logger to be compatible with help.Logger.
type Logger struct {
	log *zap.SugaredLogger
}

// Debug implements help.Logger.
func (l *Logger) Debug(args ...interface{}) {
	l.log.Debugln(args...)
}

// Debugf implements help.Logger.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log.Debugf(format, args...)
}

// Info implements help.Logger.
func (l *Logger) Info(args ...interface{}) {
	l.log.Infoln(args...)
}

// Infof implements help.Logger.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log.Infof(format, args...)
}

// Warn implements help.Logger.
func (l *Logger) Warn(args ...interface{}) {
	l.log.Warnln(args...)
}

// Warnf implements help.Logger.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log.Warnf(format, args...)
}

// Error implements help.Logger.
func (l *Logger) Error(args ...interface{}) {
	l.log.Errorln(args...)
}

// Errorf implements help.Logger.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log.Errorf(format, args...)
}

// Fatal implements help.Logger.
func (l *Logger) Fatal(args ...interface{}) {
	l.log.Fatalln(args...)
}

// Fatalf implements help.Logger.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log.Fatalf(format, args...)
}
