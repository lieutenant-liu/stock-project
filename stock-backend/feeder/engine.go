// Package feeder 是数据采集模块的核心引擎，负责：
//  1. 全局请求限速（漏桶算法）—— 防止 API 请求过快被封禁
//  2. 异步数据落盘（Channel + Worker）—— 采集和写入分离，提高效率
//  3. 动态请求频率调整 —— 运行时可以热更新请求间隔
package feeder

// ==================== 导入说明 ====================
// math/rand:       随机数生成，用于请求间隔的随机抖动（避免固定节奏被限流）
// stock-backend/db: 数据库操作包，提供批量写入函数
// stock-backend/tushare: Tushare 数据结构包，提供 DailyKLine 等类型定义
// sync:            并发原语，提供 Once（单次执行）、RWMutex（读写锁）
// time:            时间处理，用于延迟、超时等
import (
	"math/rand"
	"stock-backend/db"
	"stock-backend/tushare"
	"sync"
	"time"
)

// ==========================================
// 全局调度引擎 (Global Engine)
// ==========================================
// 这个模块实现了两个核心机制：
//  1. 请求限速器（漏桶算法）：控制对 Tushare API 的请求频率，防止被封 IP
//  2. 数据汇聚管道（Channel + Worker）：采集线程只管拉数据，写入由后台 Worker 统一处理

var (
	baseDelay   time.Duration  // 基础请求间隔（毫秒），如 200ms 表示每秒最多 5 次请求
	delayMutex  sync.RWMutex   // 读写锁，保护 baseDelay 的并发安全（运行时可热更新）
	limiterOnce sync.Once      // 保证 InitGlobalEngine 只执行一次（单例模式）
	sinkChan    chan SinkTask   // 数据汇聚通道，采集线程通过它将数据发送给后台 Worker
)

// SinkTask 定义了异步落盘任务的数据结构。
// 采集线程不直接写数据库，而是将数据包装成 SinkTask 发送到 channel，
// 由后台的 dataSinkWorker 统一消费和写入。
//
// 【为什么要这样设计？】
//   - 采集线程（生产者）可以全速拉数据，不用等待数据库写入完成
//   - 写入线程（消费者）统一处理，便于做批量写入、限流、失败重试
//   - 生产者和消费者解耦，架构更清晰
//
// 字段说明：
//   - Type:   数据类型标识，决定写入哪张表
//             "kline"=日线, "fund"=基本面, "adj"=复权因子,
//             "index"=指数, "moneyflow"=资金流向, "fina"=财务指标,
//             "stklimit"=涨跌停, "cyqperf"=筹码分布, "stkfactorpro"=技术因子
//   - TSCode: 股票代码（用于日志和数据库路由）
//   - Data:   实际数据（interface{} 可存放任意类型，消费时通过类型断言还原）
type SinkTask struct {
	Type   string      // 数据类型标识
	TSCode string      // 股票代码
	Data   interface{} // 实际数据（类型不定，消费时断言）
}

// InitGlobalEngine 初始化全局调度引擎（单例模式）。
// 这个函数在整个程序生命周期中只会执行一次（通过 sync.Once 保证）。
//
// 初始化内容：
//   1. 设置基础请求间隔
//   2. 创建数据汇聚 channel（带 5000 缓冲）
//   3. 启动后台数据写入 Worker
//   4. 初始化随机数种子
//
// 参数:
//   - delay: 基础请求间隔，如 200 * time.Millisecond 表示每 200ms 允许一次请求
//
// Go 语法要点：
//   - sync.Once: 保证传入的函数只执行一次，即使被多个协程同时调用
//     这是 Go 实现单例模式的标准方式
//   - make(chan SinkTask, 5000): 创建带缓冲的 channel，容量 5000
//     缓冲 channel 允许生产者在消费者忙时继续发送（最多积压 5000 条）
//     如果缓冲满了，生产者会被阻塞（背压机制）
//   - go dataSinkWorker(): 用 go 关键字启动一个新协程（轻量级线程）
//     协程是 Go 并发的核心，比操作系统线程轻量得多
func InitGlobalEngine(delay time.Duration) {
	limiterOnce.Do(func() {
		SetBaseDelay(int(delay.Milliseconds())) // 设置初始请求间隔
		sinkChan = make(chan SinkTask, 5000)     // 创建带 5000 缓冲的 channel
		go dataSinkWorker()                      // 启动后台写入 Worker（独立协程）

		// 初始化随机数种子（基于当前时间的纳秒数，确保每次运行结果不同）
		rand.Seed(time.Now().UnixNano())
	})
}

// SetBaseDelay 动态设置请求间隔（热更新，不需要重启程序）。
// 【使用场景】可以在运行时调整请求频率：
//   - 遇到限流时增大间隔（如从 200ms 调到 500ms）
//   - 限流解除后减小间隔（提高采集速度）
//
// 参数:
//   - delayMs: 新的请求间隔（毫秒），如 200 表示每 200ms 一次请求
//
// Go 语法要点：
//   - time.Duration(delayMs) * time.Millisecond: 将整数毫秒转换为 time.Duration 类型
//     time.Duration 底层是 int64（纳秒），乘以 time.Millisecond 做单位转换
func SetBaseDelay(delayMs int) {
	delayMutex.Lock()         // 获取写锁
	defer delayMutex.Unlock() // 函数结束时释放写锁
	baseDelay = time.Duration(delayMs) * time.Millisecond
	LogMsg("🛡️ [系统引擎] 仿生学漏桶请求频率已热更新为: %d 毫秒/发！", delayMs)
}

// WaitToken 是请求限速的核心函数，每次发送 API 请求前必须调用。
// 【限速原理 —— 漏桶算法 + 随机抖动】
//
// 漏桶算法：无论请求来得多快，都以固定速率"漏出"请求，多余的请求被阻塞等待。
// 随机抖动：在基础间隔上叠加 0~100% 的随机时间，避免所有请求都卡在同一个时间点发出。
//
// 例如：baseDelay = 200ms，那么实际等待时间 = 200ms + rand(0~200ms) = 200~400ms
// 这样既保证了平均频率不超过限制，又让请求时间分散，更像人类操作。
//
// Go 语法要点：
//   - rand.Int63n(n): 生成 [0, n) 范围内的随机 int64
//   - time.Sleep(duration): 让当前协程休眠指定时间
func WaitToken() {
	// 安全读取当前请求间隔（使用读锁，不阻塞其他读操作）
	delayMutex.RLock()
	currentDelay := baseDelay
	delayMutex.RUnlock()

	if currentDelay > 0 {
		// 计算随机抖动：0 到 currentDelay 之间的随机值
		jitter := time.Duration(rand.Int63n(int64(currentDelay)))
		// 休眠 = 基础间隔 + 随机抖动
		time.Sleep(currentDelay + jitter)
	}
}

// PushToSink 将采集到的数据推入异步落盘管道。
// 这是所有采集线程往数据库写数据的唯一入口。
//
// 【工作流程】
//  采集线程 -> PushToSink() -> sinkChan -> dataSinkWorker() -> db.BatchInsertXxx()
//
// 采集线程调用这个函数后立即返回，不等待数据库写入完成（异步非阻塞）。
// 如果 channel 缓冲满了（积压 5000 条），调用会被阻塞（背压保护）。
//
// 参数:
//   - taskType: 数据类型标识（如 "kline"、"fund"），决定写入哪张表
//   - tsCode:   股票代码（用于日志记录）
//   - data:     实际数据（interface{} 类型，消费时通过类型断言还原）
//
// Go 语法要点：
//   - sinkChan <- SinkTask{...}: 向 channel 发送数据
//     如果 channel 有空闲缓冲，立即返回；如果满了，阻塞等待
//     这是 Go CSP 并发模型的核心操作
func PushToSink(taskType, tsCode string, data interface{}) {
	sinkChan <- SinkTask{
		Type:   taskType,
		TSCode: tsCode,
		Data:   data,
	}
}

// dataSinkWorker 是后台数据写入的守护协程（消费者）。
// 这个函数作为独立协程运行，从 sinkChan 中不断读取任务并写入数据库。
//
// 【设计要点】
//   - 整个系统只有一个写入出口，便于统一管理数据库连接
//   - for task := range sinkChan 会一直阻塞直到有数据或 channel 被关闭
//   - 使用 type switch 根据 task.Type 路由到不同的数据库写入函数
//
// Go 语法要点：
//   - for task := range channel: 从 channel 持续读取数据，直到 channel 被关闭
//     这是 Go 中消费者协程的标准写法
//   - switch task.Type { case "kline": ... }: 类型路由，根据字符串分发到不同处理逻辑
//   - task.Data.([]tushare.DailyKLine): 类型断言，将 interface{} 还原为具体类型
//     ok 变量为 false 表示类型不匹配（安全降级，不会 panic）
func dataSinkWorker() {
	// 不断从 channel 读取任务，直到 channel 被关闭
	for task := range sinkChan {
		// 所有模块统一从这里落库，便于后续做限流、批量写与失败重试
		saved := 0 // 记录本次写入的记录数

		// 根据数据类型路由到对应的数据库写入函数
		switch task.Type {
		case "kline": // 日线行情
			// 类型断言：将 interface{} 还原为 []tushare.DailyKLine
			// 如果 task.Data 不是这个类型，ok 为 false，跳过写入
			if klines, ok := task.Data.([]tushare.DailyKLine); ok {
				saved = db.BatchInsertKLines(task.TSCode, klines)
			}
		case "fund": // 基本面指标
			if funds, ok := task.Data.([]tushare.DailyFundamental); ok {
				saved = db.BatchInsertFundamentals(task.TSCode, funds)
			}
		case "adj": // 复权因子
			if adjs, ok := task.Data.([]tushare.AdjFactor); ok {
				saved = db.BatchInsertAdjFactors(task.TSCode, adjs)
			}
		case "index": // 指数行情
			if indices, ok := task.Data.([]tushare.IndexDaily); ok {
				saved = db.BatchInsertIndexDaily(task.TSCode, indices)
			}
		case "moneyflow": // 资金流向
			if flows, ok := task.Data.([]tushare.DailyMoneyFlow); ok {
				saved = db.BatchInsertMoneyFlow(task.TSCode, flows)
			}
		case "fina": // 财务指标
			if finas, ok := task.Data.([]tushare.FinaIndicator); ok {
				saved = db.BatchInsertFinaIndicators(task.TSCode, finas)
			}
		case "stklimit": // 涨跌停价格
			if limits, ok := task.Data.([]tushare.StkLimit); ok {
				saved = db.BatchInsertStkLimit(limits)
			}
		case "cyqperf": // 筹码分布
			if perfs, ok := task.Data.([]tushare.CyqPerf); ok {
				saved = db.BatchInsertCyqPerf(task.TSCode, perfs)
			}
		case "stkfactorpro": // 技术因子
			if factors, ok := task.Data.([]tushare.StkFactorPro); ok {
				saved = db.BatchInsertStkFactorPro(task.TSCode, factors)
			}
		}

		// 如果成功写入了数据，记录日志
		if saved > 0 {
			LogMsg("💾 [Data Sink] %s [%s] 异步落盘成功: %d 条", task.Type, task.TSCode, saved)
		}
	}
}
