package zap_help

import "go.uber.org/zap"

// NewLogger returns a new Logger.
//
// By default, Loggers info at zap's InfoLevel.
func NewLogger(l *zap.Logger) *Logger {
	logger := &Logger{
		log:    l.Sugar(),
		info:   (*zap.SugaredLogger).Info,
		infof:  (*zap.SugaredLogger).Infof,
		warn:   (*zap.SugaredLogger).Warn,
		warnf:  (*zap.SugaredLogger).Warnf,
		error:  (*zap.SugaredLogger).Error,
		errorf: (*zap.SugaredLogger).Errorf,
		fatal:  (*zap.SugaredLogger).Fatal,
		fatalf: (*zap.SugaredLogger).Fatalf,
	}
	return logger
}

// Logger adapts zap's Logger to be compatible with help.Logger.
type Logger struct {
	log    *zap.SugaredLogger
	info   func(*zap.SugaredLogger, ...interface{})
	infof  func(*zap.SugaredLogger, string, ...interface{})
	warn   func(*zap.SugaredLogger, ...interface{})
	warnf  func(*zap.SugaredLogger, string, ...interface{})
	error  func(*zap.SugaredLogger, ...interface{})
	errorf func(*zap.SugaredLogger, string, ...interface{})
	fatal  func(*zap.SugaredLogger, ...interface{})
	fatalf func(*zap.SugaredLogger, string, ...interface{})
}

// Info implements help.Logger.
func (l *Logger) Info(args ...interface{}) {
	l.info(l.log, args...)
}

// Infof implements help.Logger.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.infof(l.log, format, args...)
}

// Warn implements help.Logger.
func (l *Logger) Warn(args ...interface{}) {
	l.info(l.log, args...)
}

// Warnf implements help.Logger.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.infof(l.log, format, args...)
}

// Error implements help.Logger.
func (l *Logger) Error(args ...interface{}) {
	l.info(l.log, args...)
}

// Errorf implements help.Logger.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.infof(l.log, format, args...)
}

// Fatal implements help.Logger.
func (l *Logger) Fatal(args ...interface{}) {
	l.fatal(l.log, args...)
}

// Fatalf implements help.Logger.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.fatalf(l.log, format, args...)
}
