package sysmon

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"stock-backend/logger"
	"strings"
	"sync"
	"time"
)

// Tier 资源档位枚举。
type Tier int

const (
	TierMobile Tier = iota // Termux 或 <4GB：受限保护模式
	TierMid                // 4~16GB：标准 PC
	TierHigh               // >16GB：高配服务器
)

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
type ResourceConfig struct {
	ChunkSize  int
	MaxWorkers int
	GCPercent  int     // GOGC 值
	MemHighPct float64 // 触发 GC 的内存占比阈值
	MemCritPct float64 // 触发 FreeOSMemory 的内存占比阈值
}

// EnvInfo 运行环境信息。
type EnvInfo struct {
	IsTermux     bool
	IsLinux      bool
	TotalMemMB   int64
	DetectedTier Tier
}

var (
	once   sync.Once
	env    EnvInfo
	config ResourceConfig
)

// Init 初始化资源管理器（sync.Once 保证只执行一次）。
func Init() {
	once.Do(func() {
		env = detectEnvironment()
		config = tierConfig(env.DetectedTier)
		applyGOGC(config.GCPercent)
		log.Printf("[SYSMON] 环境=%s | 内存=%dMB | 档位=%s | Chunk=%d | Workers=%d | GOGC=%d",
			envLabel(env), env.TotalMemMB, env.DetectedTier,
			config.ChunkSize, config.MaxWorkers, config.GCPercent)

		// 启动心跳协程：定期输出内存快照到黑匣子日志
		go startHeartbeat(env)
	})
}

// startHeartbeat 定期记录内存快照，作为进程死亡前的"心电图"。
func startHeartbeat(env EnvInfo) {
	interval := 1 * time.Second
	if env.IsTermux {
		interval = 5 * time.Second // Termux 下降低 I/O 压力
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		logger.Info("[HEARTBEAT] Alloc: %dMB, Sys: %dMB, NumGC: %d, Goroutines: %d",
			m.HeapAlloc/1024/1024, m.Sys/1024/1024, m.NumGC, runtime.NumGoroutine())
	}
}

func envLabel(e EnvInfo) string {
	if e.IsTermux {
		return "Termux"
	}
	if e.IsLinux {
		return "Linux"
	}
	return "Unknown"
}

// detectEnvironment 嗅探宿主环境。
// PRoot 会抹除 Termux 环境变量，因此额外通过 /proc/version 内核版本字符串穿透伪装。
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
func readMemTotalMB(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			var kb int64
			if _, err := fmt.Sscanf(line, "MemTotal: %d kB", &kb); err == nil {
				return kb / 1024, nil
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found in %s", path)
}

// classifyTier 根据物理内存大小分类档位。
func classifyTier(memMB int64) Tier {
	switch {
	case memMB <= 3*1024:
		return TierMobile
	case memMB <= 16*1024:
		return TierMid
	default:
		return TierHigh
	}
}

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
			MaxWorkers: runtime.NumCPU() * 2,
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
func applyGOGC(pct int) {
	debug.SetGCPercent(pct)
}

// GetChunkSize 返回自适应 chunk 大小。
func GetChunkSize() int {
	Init()
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

// CheckMemoryBackpressure 检查内存水位，超过阈值时被动等待自然 GC。
// 返回 true 表示触发了泄压，调用方应额外 sleep 等待回收。
// 注意：绝不调用 runtime.GC() 或 debug.FreeOSMemory()，PRoot 下会引发 madvise 风暴导致系统重启。
func CheckMemoryBackpressure() bool {
	Init()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	totalBytes := uint64(env.TotalMemMB) * 1024 * 1024
	if totalBytes == 0 {
		return false
	}

	heapPct := float64(m.HeapAlloc) / float64(totalBytes) * 100

	if heapPct >= config.MemHighPct {
		log.Printf("[SYSMON] 内存水位告警 %.1f%% (HeapAlloc=%dMB / Total=%dMB) → 等待自然 GC",
			heapPct, m.HeapAlloc/1024/1024, env.TotalMemMB)
		time.Sleep(1 * time.Second)
		return true
	}

	return false
}
