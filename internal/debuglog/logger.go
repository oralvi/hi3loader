package debuglog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func New(dir, name string) (*Logger, error) {
	if dir == "" {
		dir = "logs"
	}
	if name == "" {
		name = "debug.log"
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Join(wd, dir)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(baseDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return &Logger{
		file: file,
		path: path,
	}, nil
}

func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Logger) Writef(format string, args ...any) {
	if l == nil || l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	line := fmt.Sprintf(
		"%s %s\n",
		time.Now().Format(time.RFC3339Nano),
		fmt.Sprintf(format, args...),
	)
	_, _ = l.file.WriteString(line)
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	err := l.file.Close()
	l.file = nil
	return err
}
