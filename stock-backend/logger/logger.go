package logger

// ============================================================
// logger/logger.go - 日志系统
// ============================================================
// 这个文件实现了文件日志系统，将运行日志写入文件。
//
// 【为什么需要文件日志？】
// 1. 终端日志会随窗口关闭丢失
// 2. 文件日志可以事后分析问题
// 3. 支持日志级别分类（INFO/WARN/ERROR/FATAL）
//
// 【Go 语言知识点】
// - os.OpenFile: 打开文件（支持追加写入）
// - sync.Mutex: 互斥锁，保证并发安全
// - io.Writer 接口: 实现此接口可以自定义输出目标
// - iota: 常量生成器，自动递增
// ============================================================

import (
	"fmt"    // 格式化输出
	"io"     // IO 接口
	"os"     // 文件操作
	"runtime" // 运行时信息（获取 goroutine ID）
	"sync"   // 同步原语（互斥锁）
	"time"   // 时间处理
)

// ------------------------------------------------------------
// 日志级别定义
// ------------------------------------------------------------

// Level 日志级别类型。
type Level int

// 日志级别常量。
// 【Go 语言知识点：iota】
// iota 是常量生成器，从 0 开始自动递增。
// LevelInfo = 0, LevelWarn = 1, LevelError = 2, LevelFatal = 3
const (
	LevelInfo  Level = iota // 普通信息
	LevelWarn               // 警告
	LevelError              // 错误
	LevelFatal              // 致命错误（会退出程序）
)

// 全局变量
var (
	file       *os.File          // 日志文件句柄
	mu         sync.Mutex        // 互斥锁，保证并发写入安全
	levelNames = map[Level]string{ // 级别名称映射
		LevelInfo:  "INFO",
		LevelWarn:  "WARN",
		LevelError: "ERROR",
		LevelFatal: "FATAL",
	}
)

// ------------------------------------------------------------
// 初始化与关闭
// ------------------------------------------------------------

// Init 初始化日志文件。目录不存在时自动创建。
// 每条日志写入后强制 file.Sync()，确保进程被杀前数据已落盘。
//
// 【Go 语言知识点：os.MkdirAll】
// os.MkdirAll 会递归创建目录，如果目录已存在不会报错。
// 例如：os.MkdirAll("logs/sub", 0755) 会创建 logs/ 和 logs/sub/ 两个目录。
func Init(path string) error {
	// 提取目录部分并确保存在
	// 从路径末尾向前找第一个 '/'
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			// 创建目录（递归创建，如 "logs/blackbox.log" → 创建 "logs/"）
			if err := os.MkdirAll(path[:i], 0755); err != nil {
				return fmt.Errorf("logger mkdir failed: %w", err)
			}
			break
		}
	}

	// 打开日志文件
	// O_APPEND: 追加写入（不覆盖原有内容）
	// O_CREATE: 文件不存在时创建
	// O_WRONLY: 只写模式
	var err error
	file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("logger init failed: %w", err)
	}

	return nil
}

// ------------------------------------------------------------
// 内部工具函数
// ------------------------------------------------------------

// goroutineID 从 runtime.Stack 中提取当前 goroutine ID。
// 【为什么需要 goroutine ID？】
// 并发程序中可能有多个 goroutine 同时写日志。
// 记录 goroutine ID 可以追踪是哪个协程产生的日志。
func goroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false) // 获取当前 goroutine 的堆栈
	s := string(buf[:n])
	// 堆栈格式: "goroutine 123 [running]:\n..."
	s = s[len("goroutine "):] // 跳过 "goroutine " 前缀
	for i, c := range s {
		if c < '0' || c > '9' {
			return s[:i] // 提取数字部分
		}
	}
	return "?"
}

// logf 统一写入函数：格式化 → 写入 → 强制刷盘。
// 【Go 语言知识点：可变参数】
// args ...interface{} 表示接受任意数量的参数。
// 调用时可以传入任意个参数，如 logf(LevelInfo, "name=%s age=%d", "Tom", 25)
func logf(level Level, format string, args ...interface{}) {
	if file == nil {
		return // 日志文件未初始化，跳过
	}

	// 格式化日志行
	ts := time.Now().Format("2006-01-02 15:04:05.000") // 时间戳
	gid := goroutineID()                                // goroutine ID
	msg := fmt.Sprintf(format, args...)                  // 格式化消息
	line := fmt.Sprintf("[%s] [%s] [g%s] %s\n", ts, levelNames[level], gid, msg)

	// 加锁写入（保证并发安全）
	mu.Lock()
	defer mu.Unlock()
	file.WriteString(line)
	file.Sync() // 强制刷盘，确保数据写入磁盘
}

// ------------------------------------------------------------
// 公开日志函数
// ------------------------------------------------------------

// Info 输出 INFO 级别日志。
// 用于记录普通信息，如"数据库初始化完成"、"同步任务启动"等。
func Info(format string, args ...interface{}) { logf(LevelInfo, format, args...) }

// Warn 输出 WARN 级别日志。
// 用于记录警告信息，如"Token 即将过期"、"网络请求超时"等。
func Warn(format string, args ...interface{}) { logf(LevelWarn, format, args...) }

// Error 输出 ERROR 级别日志。
// 用于记录错误信息，如"数据库查询失败"、"API 调用异常"等。
func Error(format string, args ...interface{}) { logf(LevelError, format, args...) }

// Fatal 输出 FATAL 级别日志后退出进程。
// 用于记录致命错误，程序无法继续运行。
func Fatal(format string, args ...interface{}) {
	logf(LevelFatal, format, args...)
	os.Exit(1) // 退出程序
}

// Writer 返回日志文件的 io.Writer，供 io.MultiWriter 使用。
// 【使用场景】
// main.go 中使用 io.MultiWriter 将标准 log 包的输出同时写入终端和日志文件：
// log.SetOutput(io.MultiWriter(os.Stderr, logger.Writer()))
func Writer() io.Writer {
	if file == nil {
		return os.Stderr // 未初始化时输出到标准错误
	}
	return file
}

// Close 关闭日志文件（优雅退出时调用）。
// 在 main.go 的 defer 中调用，确保日志文件正确关闭。
func Close() {
	if file != nil {
		file.Sync()  // 先刷盘
		file.Close() // 再关闭
	}
}
