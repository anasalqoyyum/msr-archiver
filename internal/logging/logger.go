package logging

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// Logger provides synchronized logging for concurrent workers.
type Logger struct {
	mu sync.Mutex
	l  *log.Logger
}

// New creates a thread-safe logger.
func New() *Logger {
	return &Logger{l: log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)}
}

// Infof writes an informational message.
func (lg *Logger) Infof(format string, args ...any) {
	lg.logf("INFO ", format, args...)
}

// Warnf writes a warning message.
func (lg *Logger) Warnf(format string, args ...any) {
	lg.logf("WARN ", format, args...)
}

// Errorf writes an error message.
func (lg *Logger) Errorf(format string, args ...any) {
	lg.logf("ERROR", format, args...)
}

func (lg *Logger) logf(level, format string, args ...any) {
	lg.mu.Lock()
	defer lg.mu.Unlock()
	lg.l.Printf("%s %s", level, fmt.Sprintf(format, args...))
}
