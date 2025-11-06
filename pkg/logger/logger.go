package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

// Logger is a structured logger
type Logger struct {
	level      LogLevel
	writer     io.Writer
	structured bool // JSON output if true
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger(INFO, os.Stdout, false)
}

// NewLogger creates a new logger instance
func NewLogger(level LogLevel, writer io.Writer, structured bool) *Logger {
	return &Logger{
		level:      level,
		writer:     writer,
		structured: structured,
	}
}

// SetDefault sets the default logger
func SetDefault(logger *Logger) {
	defaultLogger = logger
}

// Log logs a message with the given level and fields
func (l *Logger) Log(level LogLevel, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Fields:    fields,
	}

	if l.structured {
		l.logJSON(entry)
	} else {
		l.logText(entry)
	}
}

// LogError logs an error message
func (l *Logger) LogError(level LogLevel, message string, err error, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Fields:    fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	if l.structured {
		l.logJSON(entry)
	} else {
		l.logText(entry)
	}
}

func (l *Logger) logJSON(entry LogEntry) {
	data, _ := json.Marshal(entry)
	fmt.Fprintln(l.writer, string(data))
}

func (l *Logger) logText(entry LogEntry) {
	msg := fmt.Sprintf("[%s] %s: %s", entry.Timestamp, entry.Level, entry.Message)

	if len(entry.Fields) > 0 {
		msg += fmt.Sprintf(" %+v", entry.Fields)
	}

	if entry.Error != "" {
		msg += fmt.Sprintf(" error=%s", entry.Error)
	}

	fmt.Fprintln(l.writer, msg)
}

// Convenience methods for default logger

func Debug(message string, fields map[string]interface{}) {
	defaultLogger.Log(DEBUG, message, fields)
}

func Info(message string, fields map[string]interface{}) {
	defaultLogger.Log(INFO, message, fields)
}

func Warn(message string, fields map[string]interface{}) {
	defaultLogger.Log(WARN, message, fields)
}

func Error(message string, err error, fields map[string]interface{}) {
	defaultLogger.LogError(ERROR, message, err, fields)
}

func Fatal(message string, err error, fields map[string]interface{}) {
	defaultLogger.LogError(FATAL, message, err, fields)
	os.Exit(1)
}

// WithFields creates a logger with default fields
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

func WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: defaultLogger,
		fields: fields,
	}
}

func (f *FieldLogger) Debug(message string) {
	f.logger.Log(DEBUG, message, f.fields)
}

func (f *FieldLogger) Info(message string) {
	f.logger.Log(INFO, message, f.fields)
}

func (f *FieldLogger) Warn(message string) {
	f.logger.Log(WARN, message, f.fields)
}

func (f *FieldLogger) Error(message string, err error) {
	f.logger.LogError(ERROR, message, err, f.fields)
}

func (f *FieldLogger) Fatal(message string, err error) {
	f.logger.LogError(FATAL, message, err, f.fields)
	os.Exit(1)
}
