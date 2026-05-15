package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
	LevelFatal
)

var (
	file *os.File
	mu   sync.Mutex
	levelNames = map[Level]string{
		LevelInfo:  "INFO",
		LevelWarn:  "WARN",
		LevelError: "ERROR",
		LevelFatal: "FATAL",
	}
)

// Init 初始化日志文件。目录不存在时自动创建。
// 每条日志写入后强制 file.Sync()，确保进程被杀前数据已落盘。
func Init(path string) error {
	// 提取目录部分并确保存在
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if err := os.MkdirAll(path[:i], 0755); err != nil {
				return fmt.Errorf("logger mkdir failed: %w", err)
			}
			break
		}
	}

	var err error
	file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("logger init failed: %w", err)
	}

	return nil
}

// goroutineID 从 runtime.Stack 中提取当前 goroutine ID。
func goroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := string(buf[:n])
	// 格式: "goroutine 123 [running]:\n..."
	s = s[len("goroutine "):]
	for i, c := range s {
		if c < '0' || c > '9' {
			return s[:i]
		}
	}
	return "?"
}

// logf 统一写入函数：格式化 → 写入 → 强制刷盘。
func logf(level Level, format string, args ...interface{}) {
	if file == nil {
		return
	}

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	gid := goroutineID()
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] [g%s] %s\n", ts, levelNames[level], gid, msg)

	mu.Lock()
	defer mu.Unlock()
	file.WriteString(line)
	file.Sync()
}

// Info 输出 INFO 级别日志。
func Info(format string, args ...interface{}) { logf(LevelInfo, format, args...) }

// Warn 输出 WARN 级别日志。
func Warn(format string, args ...interface{}) { logf(LevelWarn, format, args...) }

// Error 输出 ERROR 级别日志。
func Error(format string, args ...interface{}) { logf(LevelError, format, args...) }

// Fatal 输出 FATAL 级别日志后退出进程。
func Fatal(format string, args ...interface{}) {
	logf(LevelFatal, format, args...)
	os.Exit(1)
}

// Writer 返回日志文件的 io.Writer，供 io.MultiWriter 使用。
func Writer() io.Writer {
	if file == nil {
		return os.Stderr
	}
	return file
}

// Close 关闭日志文件（优雅退出时调用）。
func Close() {
	if file != nil {
		file.Sync()
		file.Close()
	}
}
