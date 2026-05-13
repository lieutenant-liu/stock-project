package feeder

import (
	"math/rand"
	"stock-backend/db"
	"stock-backend/tushare"
	"sync"
	"time"
)

// ==========================================
// 💥 全局调度引擎 (Global Engine - 动态变速版)
// ==========================================

var (
	baseDelay   time.Duration
	delayMutex  sync.RWMutex // 💥 新增：保护动态请求频率的并发锁
	limiterOnce sync.Once
	sinkChan    chan SinkTask
)

// SinkTask 定义异步落盘任务
type SinkTask struct {
	Type   string // "kline", "fund", "adj", "index", "moneyflow", "fina", "limitlist"
	TSCode string
	Data   interface{}
}

// InitGlobalEngine 初始化全局护盾
func InitGlobalEngine(delay time.Duration) {
	limiterOnce.Do(func() {
		SetBaseDelay(int(delay.Milliseconds())) // 初始化时设定初始速度
		sinkChan = make(chan SinkTask, 5000)
		go dataSinkWorker()

		// 初始化随机数种子
		rand.Seed(time.Now().UnixNano())
	})
}

// 💥 新增：动态设置基础延迟 (热重载变速箱)
func SetBaseDelay(delayMs int) {
	delayMutex.Lock()
	defer delayMutex.Unlock()
	baseDelay = time.Duration(delayMs) * time.Millisecond
	LogMsg("🛡️ [系统引擎] 仿生学漏桶请求频率已热更新为: %d 毫秒/发！", delayMs)
}

// WaitToken 仿生学阻塞：带并发安全锁的动态延迟
func WaitToken() {
	// 安全读取当前请求频率
	delayMutex.RLock()
	currentDelay := baseDelay
	delayMutex.RUnlock()

	if currentDelay > 0 {
		// 引入随机抖动：基准时间 + 0~100% 的随机抖动
		jitter := time.Duration(rand.Int63n(int64(currentDelay)))
		time.Sleep(currentDelay + jitter)
	}
}

// PushToSink 将清洗后的数据打入单向汇聚层
func PushToSink(taskType, tsCode string, data interface{}) {
	sinkChan <- SinkTask{
		Type:   taskType,
		TSCode: tsCode,
		Data:   data,
	}
}

// dataSinkWorker 单向落盘守护进程 (系统唯一的写入出口)
func dataSinkWorker() {
	for task := range sinkChan {
		// 所有模块统一从这里落库，便于后续做限流、批量写与失败重试。
		saved := 0
		switch task.Type {
		case "kline":
			if klines, ok := task.Data.([]tushare.DailyKLine); ok {
				saved = db.BatchInsertKLines(task.TSCode, klines)
			}
		case "fund":
			if funds, ok := task.Data.([]tushare.DailyFundamental); ok {
				saved = db.BatchInsertFundamentals(task.TSCode, funds)
			}
		case "adj":
			if adjs, ok := task.Data.([]tushare.AdjFactor); ok {
				saved = db.BatchInsertAdjFactors(task.TSCode, adjs)
			}
		case "index":
			if indices, ok := task.Data.([]tushare.IndexDaily); ok {
				saved = db.BatchInsertIndexDaily(task.TSCode, indices)
			}
			// ... 之前的 kline, fund, adj, index 保持不变 ...
		case "moneyflow":
			if flows, ok := task.Data.([]tushare.DailyMoneyFlow); ok {
				saved = db.BatchInsertMoneyFlow(task.TSCode, flows)
			}
		case "fina":
			if finas, ok := task.Data.([]tushare.FinaIndicator); ok {
				saved = db.BatchInsertFinaIndicators(task.TSCode, finas)
			}
		case "stklimit":
			if limits, ok := task.Data.([]tushare.StkLimit); ok {
				saved = db.BatchInsertStkLimit(limits)
			}
		case "cyqperf":
			if perfs, ok := task.Data.([]tushare.CyqPerf); ok {
				saved = db.BatchInsertCyqPerf(task.TSCode, perfs)
			}
		case "stkfactorpro":
			if factors, ok := task.Data.([]tushare.StkFactorPro); ok {
				saved = db.BatchInsertStkFactorPro(task.TSCode, factors)
			}

		}

		if saved > 0 {
			LogMsg("💾 [Data Sink] %s [%s] 异步落盘成功: %d 条", task.Type, task.TSCode, saved)
		}
	}
}
