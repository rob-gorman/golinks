package logger

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
)

type Logger interface {
	Debug(string, ...any)
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)

	AddContext(...any) Logger
	NewWithContext(...any) Logger
}

func Default(writers ...io.Writer) Logger {
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	wr := toMultiWriter(writers...)
	opt := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	handler := slog.NewTextHandler(wr, opt)

	logger := slog.New(handler)
	return &loggerT{Logger: logger}
}

type loggerT struct {
	*slog.Logger
}

func (l *loggerT) AddContext(args ...any) Logger {
	l.Logger = l.Logger.With(args...)
	return l
}

func (l *loggerT) NewWithContext(args ...any) Logger {
	return &loggerT{
		Logger: l.Logger.With(args...),
	}
}

func (l *loggerT) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

func (l *loggerT) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

func (l *loggerT) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

func (l *loggerT) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

func toMultiWriter(wrs ...io.Writer) io.Writer {
	if len(wrs) == 0 {
		return os.Stderr // default out
	}

	if len(wrs) == 1 {
		return wrs[0] // single writer
	}

	// sanitize nil writers because MultiWriter panics on write to nil
	var valid []io.Writer
	for _, w := range wrs {
		if w == nil {
			continue // skip nil writers
		}
		valid = append(valid, w)
	}

	if len(valid) == 0 {
		return os.Stderr
	}

	if len(valid) == 1 {
		return valid[0]
	}

	return io.MultiWriter(valid...)
}

// very simple thread-safe buffer for our logger output, if needed
type Buffer struct {
	buf   *bytes.Buffer
	mutex sync.RWMutex
}

func (buffer *Buffer) Read(dataBytes []byte) (n int, err error) {
	buffer.mutex.RLock()
	defer buffer.mutex.RUnlock()
	if buffer.buf == nil {
		buffer.buf = new(bytes.Buffer)
	}
	return buffer.buf.Read(dataBytes)
}

func (buffer *Buffer) String() string {
	buffer.mutex.RLock()
	defer buffer.mutex.RUnlock()
	return buffer.buf.String()
}

func (buffer *Buffer) Write(dataBytes []byte) (n int, err error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	if buffer.buf == nil {
		buffer.buf = &bytes.Buffer{}
	}
	return buffer.buf.Write(dataBytes)
}
