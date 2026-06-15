package sysmon

// ============================================================
// sysmon/manager.go - 系统资源监控
// ============================================================
// 这个文件负责监控系统资源（内存、CPU），并根据硬件环境自动调整参数。
//
// 【为什么需要这个？】
// 项目需要在不同设备上运行：
// - 手机（Termux）：内存有限，需要严格限制
// - 普通 PC：标准配置
// - 高配服务器：可以充分利用资源
// 系统会自动检测环境，选择合适的配置。
//
// 【Go 语言知识点】
// - runtime.MemStats: 内存统计信息
// - runtime.NumCPU(): 获取 CPU 核心数
// - debug.SetGCPercent(): 设置 GC 激进度
// - sync.Once: 保证函数只执行一次
// - /proc/meminfo: Linux 内存信息文件
// ============================================================

import (
	"fmt"      // 格式化输出
	"log"      // 日志
	"os"       // 文件操作
	"runtime"  // 运行时信息
	"runtime/debug" // GC 调试
	"stock-backend/logger" // 项目日志
	"strings"  // 字符串处理
	"sync"     // 同步原语
	"time"     // 时间处理
)

// ------------------------------------------------------------
// 资源档位定义
// ------------------------------------------------------------

// Tier 资源档位枚举。
// 根据设备内存大小分为三个档位：
// - TierMobile: 手机/低配设备（<4GB 内存）
// - TierMid: 普通 PC（4-16GB 内存）
// - TierHigh: 高配服务器（>16GB 内存）
type Tier int

const (
	TierMobile Tier = iota // Termux 或 <4GB：受限保护模式
	TierMid                // 4~16GB：标准 PC
	TierHigh               // >16GB：高配服务器
)

// String 返回档位的可读名称。
func (t Tier) String() string {
	switch t {
	case TierMobile:
		return "Mobile"
	case TierMid:
		return "Mid"
	case TierHigh:
		return "High"
	default:
		return "Unknown"
	}
}

// ResourceConfig 自适应资源调度参数。
// 不同档位使用不同的参数，平衡性能和稳定性。
type ResourceConfig struct {
	ChunkSize  int     // 数据分块大小（每批加载多少只股票）
	MaxWorkers int     // 最大并发 worker 数
	GCPercent  int     // GOGC 值（GC 触发阈值）
	MemHighPct float64 // 触发 GC 的内存占比阈值（%）
	MemCritPct float64 // 触发 FreeOSMemory 的内存占比阈值（%）
}

// EnvInfo 运行环境信息。
type EnvInfo struct {
	IsTermux     bool   // 是否在 Termux（Android）环境
	IsLinux      bool   // 是否在 Linux 环境
	TotalMemMB   int64  // 物理内存总量（MB）
	DetectedTier Tier   // 检测到的资源档位
}

// 全局变量（sync.Once 保证只初始化一次）
var (
	once   sync.Once
	env    EnvInfo         // 环境信息
	config ResourceConfig  // 资源配置
)

// ------------------------------------------------------------
// 初始化
// ------------------------------------------------------------

// Init 初始化资源管理器（sync.Once 保证只执行一次）。
// 【Go 语言知识点：sync.Once】
// sync.Once 保证传入的函数只执行一次，即使被多个 goroutine 同时调用。
// 常用于单例模式和初始化操作。
func Init() {
	once.Do(func() {
		// 检测运行环境
		env = detectEnvironment()
		// 根据环境选择配置
		config = tierConfig(env.DetectedTier)
		// 应用 GC 配置
		applyGOGC(config.GCPercent)

		// 输出环境信息
		log.Printf("[SYSMON] 环境=%s | 内存=%dMB | 档位=%s | Chunk=%d | Workers=%d | GOGC=%d",
			envLabel(env), env.TotalMemMB, env.DetectedTier,
			config.ChunkSize, config.MaxWorkers, config.GCPercent)

		// 启动心跳协程：定期输出内存快照到黑匣子日志
		go startHeartbeat(env)
	})
}

// startHeartbeat 定期记录内存快照，作为进程死亡前的"心电图"。
// 【用途】
// 如果程序崩溃，可以通过最后的心跳日志判断是内存溢出还是其他原因。
func startHeartbeat(env EnvInfo) {
	interval := 1 * time.Second // 默认每秒记录一次
	if env.IsTermux {
		interval = 5 * time.Second // Termux 下降低 I/O 压力
	}

	// 创建定时器
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 定时记录内存快照
	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m) // 读取内存统计

		// 记录到日志
		logger.Info("[HEARTBEAT] Alloc: %dMB, Sys: %dMB, NumGC: %d, Goroutines: %d",
			m.HeapAlloc/1024/1024, // 堆内存分配量（MB）
			m.Sys/1024/1024,       // 系统内存总量（MB）
			m.NumGC,               // GC 次数
			runtime.NumGoroutine()) // goroutine 数量
	}
}

// envLabel 返回环境标签。
func envLabel(e EnvInfo) string {
	if e.IsTermux {
		return "Termux"
	}
	if e.IsLinux {
		return "Linux"
	}
	return "Unknown"
}

// ------------------------------------------------------------
// 环境检测
// ------------------------------------------------------------

// detectEnvironment 嗅探宿主环境。
// 【检测策略】
// 1. 检查环境变量（PREFIX, TERMUX_VERSION）
// 2. 检查 /proc/version 内核版本字符串（PRoot 穿透）
// 3. 检查 Android 特有文件（/system/build.prop）
// 4. 开发者强制后门（FORCE_MOBILE=1）
func detectEnvironment() EnvInfo {
	info := EnvInfo{
		IsLinux:    true, // 当前目标平台
		TotalMemMB: 4096, // 默认 4GB
	}

	// 1. Termux 检测：PREFIX 或 TERMUX_VERSION 环境变量
	prefix := os.Getenv("PREFIX")
	termuxVer := os.Getenv("TERMUX_VERSION")
	if strings.Contains(prefix, "com.termux") || termuxVer != "" {
		info.IsTermux = true
	}

	// 2. PRoot 穿透：通过内核版本字符串检测 Android 底层
	//    PRoot 抹除了环境变量，但 /proc/version 透传宿主内核信息
	if !info.IsTermux {
		if data, err := os.ReadFile("/proc/version"); err == nil {
			if strings.Contains(strings.ToLower(string(data)), "android") {
				info.IsTermux = true
			}
		}
	}

	// 3. 终极穿透：检查 Android 特有的物理文件是否被 proot-distro 挂载
	//    /system/build.prop 和 /system/bin/app_process 是 Android 独有的文件，
	//    proot-distro 会将宿主的 /system 目录透传进容器，无法伪造
	if !info.IsTermux {
		if _, err := os.Stat("/system/build.prop"); err == nil {
			info.IsTermux = true
		} else if _, err := os.Stat("/system/bin/app_process"); err == nil {
			info.IsTermux = true
		}
	}

	// 4. 开发者强制后门：FORCE_MOBILE=1 强制进入受限模式
	if !info.IsTermux && os.Getenv("FORCE_MOBILE") == "1" {
		info.IsTermux = true
	}

	// 5. 读取物理内存（仅非 Termux 时有意义）
	if !info.IsTermux {
		if memMB, err := readMemTotalMB("/proc/meminfo"); err == nil {
			info.TotalMemMB = memMB
		}
	}

	// 6. Termux 关键补丁：强制覆盖虚拟内存上限
	//    Android LMKD 在 ~1.5GB 就物理抹杀进程，/proc/meminfo 报告的是物理 RAM（如 8GB），
	//    必须限制为 1024MB 确保泄压红线在 768MB（75%），安全避开 LMKD。
	if info.IsTermux {
		info.TotalMemMB = 1024
	}

	// 7. 档位分类（Termux 强制 Mobile）
	if info.IsTermux {
		info.DetectedTier = TierMobile
	} else {
		info.DetectedTier = classifyTier(info.TotalMemMB)
	}

	return info
}

// readMemTotalMB 从 /proc/meminfo 解析物理内存总量（MB）。
// 【Linux 知识】
// /proc/meminfo 是 Linux 虚拟文件，包含内存使用信息。
// 格式示例：MemTotal:        8167848 kB
func readMemTotalMB(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	// 逐行查找 "MemTotal:"
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			var kb int64
			// 解析数值（单位 kB）
			if _, err := fmt.Sscanf(line, "MemTotal: %d kB", &kb); err == nil {
				return kb / 1024, nil // 转换为 MB
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found in %s", path)
}

// classifyTier 根据物理内存大小分类档位。
func classifyTier(memMB int64) Tier {
	switch {
	case memMB <= 3*1024: // <= 3GB
		return TierMobile
	case memMB <= 16*1024: // <= 16GB
		return TierMid
	default: // > 16GB
		return TierHigh
	}
}

// ------------------------------------------------------------
// 配置与查询
// ------------------------------------------------------------

// tierConfig 根据档位返回资源配置。
func tierConfig(tier Tier) ResourceConfig {
	switch tier {
	case TierMobile:
		return ResourceConfig{
			ChunkSize:  50, // 大块加载降低跨边界 I/O 频率，SQLite 批量读取效率最大化
			MaxWorkers: 4,  // 4 核并发，8 核平板用一半算力
			GCPercent:  80, // 降低 GC 抢占，把 CPU 算力让给指标计算
			MemHighPct: 60,
			MemCritPct: 75,
		}
	case TierHigh:
		return ResourceConfig{
			ChunkSize:  500,
			MaxWorkers: runtime.NumCPU() * 2, // 充分利用 CPU
			GCPercent:  85,
			MemHighPct: 85,
			MemCritPct: 92,
		}
	default: // TierMid
		return ResourceConfig{
			ChunkSize:  100,
			MaxWorkers: runtime.NumCPU(),
			GCPercent:  75,
			MemHighPct: 75,
			MemCritPct: 85,
		}
	}
}

// applyGOGC 设置 Go GC 激进度。
// 【Go 语言知识点：GOGC】
// GOGC 控制 GC 触发频率：
// - GOGC=100（默认）：堆内存增长 100% 时触发 GC
// - GOGC=50：堆内存增长 50% 时触发 GC（更频繁）
// - GOGC=200：堆内存增长 200% 时触发 GC（更少频率）
func applyGOGC(pct int) {
	debug.SetGCPercent(pct)
}

// GetChunkSize 返回自适应 chunk 大小。
func GetChunkSize() int {
	Init() // 确保已初始化
	return config.ChunkSize
}

// GetMaxWorkers 返回最大并发 worker 数。
func GetMaxWorkers() int {
	Init()
	return config.MaxWorkers
}

// GetConfig 返回完整资源配置快照。
func GetConfig() ResourceConfig {
	Init()
	return config
}

// GetEnv 返回环境信息快照。
func GetEnv() EnvInfo {
	Init()
	return env
}

// ------------------------------------------------------------
// 内存泄压
// ------------------------------------------------------------

// CheckMemoryBackpressure 检查内存水位，超过阈值时被动等待自然 GC。
// 返回 true 表示触发了泄压，调用方应额外 sleep 等待回收。
//
// 【重要注意事项】
// 绝不调用 runtime.GC() 或 debug.FreeOSMemory()！
// 在 PRoot 环境下，这些操作会引发 madvise 风暴，导致系统重启。
// 只能被动等待 Go 运行时自然触发 GC。
func CheckMemoryBackpressure() bool {
	Init()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 计算物理内存总量
	totalBytes := uint64(env.TotalMemMB) * 1024 * 1024
	if totalBytes == 0 {
		return false
	}

	// 计算堆内存占比
	heapPct := float64(m.HeapAlloc) / float64(totalBytes) * 100

	// 超过阈值，触发泄压
	if heapPct >= config.MemHighPct {
		log.Printf("[SYSMON] 内存水位告警 %.1f%% (HeapAlloc=%dMB / Total=%dMB) → 等待自然 GC",
			heapPct, m.HeapAlloc/1024/1024, env.TotalMemMB)
		time.Sleep(1 * time.Second) // 等待 1 秒，让 GC 有机会运行
		return true
	}

	return false
}
