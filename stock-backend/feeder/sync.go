// Package feeder 的 sync.go 文件实现了数据同步的核心逻辑。
// 它定义了数据源适配器接口（DataProvider），以及各种数据类型的同步引擎。
//
// 【架构设计 —— 适配器模式（Adapter Pattern）】
// 系统支持多个数据源（Tushare 付费接口、东方财富免费接口），
// 通过 DataProvider 接口统一抽象，采集引擎不需要关心数据从哪里来。
//
// 同步引擎的工作流程：
//  1. 遍历股票列表
//  2. 检查数据库中已有哪些数据（天眼系统）
//  3. 只请求缺失的数据（精确填缝）
//  4. 将数据推入异步落盘管道
package feeder

import (
	"fmt"
	"stock-backend/db"
	"stock-backend/eastmoney"
	"stock-backend/tushare"
	"sync"
	"time"
)

// ==========================================
// 数据源适配器接口 (Adapter Pattern)
// ==========================================

// DataProvider 定义了所有数据源必须实现的接口（协议）。
// 【什么是接口（Interface）？】
// Go 的接口是一组方法签名的集合。任何类型只要实现了接口中的所有方法，
// 就自动满足该接口（隐式实现，不需要像 Java 那样用 implements 声明）。
//
// 这个接口的设计目的：
//   - TushareProvider（付费数据源）和 OpenSourceProvider（免费数据源）都实现这个接口
//   - 采集引擎只依赖接口，不依赖具体实现
//   - 可以在运行时切换数据源，而不需要修改采集逻辑
//
// 方法说明：
//   - FetchStockHistory: 拉取日线行情
//   - FetchDailyBasic:   拉取每日基本面
//   - FetchAdjFactors:   拉取复权因子
//   - FetchIndexDaily:   拉取指数行情
//   - FetchMoneyFlow:    拉取资金流向
//   - FetchStkLimit:     拉取涨跌停价格
//   - FetchCyqPerf:      拉取筹码分布
//   - FetchStkFactorPro: 拉取技术因子
//   - GetName:           返回数据源名称（用于日志和判断权限等级）
type DataProvider interface {
	FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error)
	FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error)
	FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error)
	FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error)
	FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error)
	FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error)
	FetchCyqPerf(tsCode, tradeDate string) ([]tushare.CyqPerf, error)
	FetchStkFactorPro(tsCode, tradeDate string) ([]tushare.StkFactorPro, error)
	GetName() string
}

// ------------------------------------------
// 数据源 A：Tushare 高级付费数据源
// ------------------------------------------
// TushareProvider 是 DataProvider 接口的 Tushare 实现。
// 它是一个"薄代理"，每个方法只是简单地转发调用到 tushare 包的对应函数。
//
// 【为什么需要这层代理？】
//   - 统一接口：所有数据源都实现 DataProvider 接口，采集引擎可以透明切换
//   - 权限标识：GetName() 返回 "Tushare [高级]"，用于判断是否为高权限数据源
//   - 解耦：采集引擎不直接依赖 tushare 包，而是依赖 DataProvider 接口
//
// Go 语法要点 —— 方法接收者：
//   - func (t *TushareProvider) GetName() string { ... }
//   - (t *TushareProvider) 是方法接收者，表示这个方法属于 TushareProvider 类型
//   - *TushareProvider 是指针接收者（推荐），避免值拷贝
//   - 只要 TushareProvider 实现了 DataProvider 的所有方法，就自动满足接口
type TushareProvider struct{} // 空结构体，不需要任何字段

// GetName 返回数据源名称。用于日志输出和权限判断。
func (t *TushareProvider) GetName() string { return "Tushare [高级]" }

// FetchStockHistory 转发调用到 tushare 包的日线拉取函数
func (t *TushareProvider) FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	return tushare.FetchStockHistory(tsCode, startDate, endDate)
}

// FetchAdjFactors 转发调用到 tushare 包的复权因子拉取函数
func (t *TushareProvider) FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	return tushare.FetchAdjFactors(tsCode, startDate, endDate)
}

// FetchDailyBasic 转发调用到 tushare 包的基本面拉取函数
func (t *TushareProvider) FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	return tushare.FetchDailyBasic(tsCode, startDate, endDate)
}

// FetchIndexDaily 转发调用到 tushare 包的指数行情拉取函数
func (t *TushareProvider) FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	return tushare.FetchIndexDaily(tsCode, startDate, endDate)
}

// FetchMoneyFlow 转发调用到 tushare 包的资金流向拉取函数
func (t *TushareProvider) FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	return tushare.FetchMoneyFlow(tsCode, startDate, endDate)
}

// FetchStkLimit 转发调用到 tushare 包的涨跌停拉取函数
func (t *TushareProvider) FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) {
	return tushare.FetchStkLimit(tradeDate)
}

// FetchCyqPerf 转发调用到 tushare 包的筹码分布拉取函数
func (t *TushareProvider) FetchCyqPerf(tsCode, tradeDate string) ([]tushare.CyqPerf, error) {
	return tushare.FetchCyqPerf(tsCode, tradeDate)
}

// FetchStkFactorPro 转发调用到 tushare 包的技术因子拉取函数
func (t *TushareProvider) FetchStkFactorPro(tsCode, tradeDate string) ([]tushare.StkFactorPro, error) {
	return tushare.FetchStkFactorPro(tsCode, tradeDate)
}

// ------------------------------------------
// 数据源 B：开源免费数据源（东方财富）
// ------------------------------------------
// OpenSourceProvider 是 DataProvider 接口的东方财富实现。
// 东方财富提供免费的股票数据接口，不需要付费，但功能有限。
//
// 【与 TushareProvider 的区别】
//   - Tushare：需要付费积分，功能全面（涨跌停、筹码分布、技术因子等）
//   - 东方财富：免费，但不支持涨跌停、筹码分布、技术因子等高级接口
//   - 对于不支持的接口，返回 error（而不是 panic），保证程序不会崩溃
//
// 【为什么需要两个数据源？】
//   - 降低成本：日常增量同步用免费的东方财富接口
//   - 数据补全：东方财富不支持的高级数据，用 Tushare 补充
//   - 故障切换：如果一个数据源出问题，可以切换到另一个
type OpenSourceProvider struct{}

// GetName 返回数据源名称
func (o *OpenSourceProvider) GetName() string {
	return "开源接口 (EastMoney)"
}

// FetchStockHistory 调用东方财富接口拉取日线行情
func (o *OpenSourceProvider) FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	return eastmoney.FetchStockHistory(tsCode, startDate, endDate)
}

// FetchDailyBasic 调用东方财富接口拉取每日基本面数据
func (o *OpenSourceProvider) FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	return eastmoney.FetchDailyBasic(tsCode, startDate, endDate)
}

// FetchAdjFactors 调用东方财富接口逆向推导复权因子
// 【注意】东方财富没有直接提供复权因子接口，需要通过前复权价格反推
func (o *OpenSourceProvider) FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	return eastmoney.FetchDerivedAdjFactors(tsCode, startDate, endDate)
}

// FetchIndexDaily 调用东方财富接口拉取指数行情
func (o *OpenSourceProvider) FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	return eastmoney.FetchIndexDaily(tsCode, startDate, endDate)
}

// FetchMoneyFlow 调用东方财富接口拉取资金流向
func (o *OpenSourceProvider) FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	return eastmoney.FetchMoneyFlow(tsCode, startDate, endDate)
}

// FetchStkLimit 涨跌停数据 —— 东方财富免费接口不支持，返回错误
// 【设计原则】对于不支持的接口，返回明确的错误信息，而不是返回空数据
// 这样调用方可以清楚地知道是"没有数据"还是"不支持该功能"
func (o *OpenSourceProvider) FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) {
	return nil, fmt.Errorf("开源接口暂不支持历史涨跌停绝对价拉取")
}

// FetchCyqPerf 筹码分布 —— 东方财富免费接口不支持
func (o *OpenSourceProvider) FetchCyqPerf(tsCode, tradeDate string) ([]tushare.CyqPerf, error) {
	return nil, fmt.Errorf("开源接口暂不支持筹码分布数据拉取")
}

// FetchStkFactorPro 技术因子 —— 东方财富免费接口不支持
func (o *OpenSourceProvider) FetchStkFactorPro(tsCode, tradeDate string) ([]tushare.StkFactorPro, error) {
	return nil, fmt.Errorf("开源接口暂不支持技术因子专业版数据拉取")
}

// ==========================================
// 日志系统
// ==========================================
// 这个日志系统同时输出到终端（fmt.Print）和内存缓冲（LogBuffer），
// 前端可以通过 API 读取最新的日志信息，实现"赛博控制台"效果。

// LogBuffer 是内存中的日志缓冲区（环形缓冲，最多保留 50 条）
// 【为什么用切片而不是文件？】
//   - 读写速度快（内存操作）
//   - 前端可以实时获取（通过 HTTP API）
//   - 不需要管理日志文件的清理
var (
	LogBuffer []string    // 日志缓冲区
	logMutex  sync.Mutex  // 互斥锁，保护 LogBuffer 的并发安全
)

// SyncSummary 定义了同步任务的统计摘要结构。
// 每个同步引擎（K线、基本面等）执行完毕后，都会返回一个 SyncSummary，
// 包含本次同步的统计数据，用于日志展示和前端可视化。
//
// 字段说明：
//   - Module:   模块名称（如 "kline"、"fund"），标识是哪个引擎
//   - Total:    总股票数（或总日期数）
//   - Targeted: 需要同步的数量（排除了已有的）
//   - Success:  成功同步的数量
//   - Failed:   失败的数量
//   - Skipped:  跳过的数量（数据已完整，无需同步）
type SyncSummary struct {
	Module   string `json:"module"`   // 模块名称
	Total    int    `json:"total"`    // 总数
	Targeted int    `json:"targeted"` // 需要同步的数量
	Success  int    `json:"success"`  // 成功数量
	Failed   int    `json:"failed"`   // 失败数量
	Skipped  int    `json:"skipped"`  // 跳过数量
}

// LogMsg 是自定义的日志输出函数，替代 fmt.Printf。
// 【功能】同时输出到终端和内存缓冲区，实现"双通道日志"。
//
// 参数:
//   - format: 格式化字符串（与 fmt.Sprintf 相同）
//   - a:      可变参数（与 fmt.Sprintf 相同）
//
// Go 语法要点：
//   - format string, a ...interface{}: 可变参数函数
//     ...interface{} 表示接受任意数量、任意类型的参数
//     调用时可以传 LogMsg("hello %s %d", "world", 42)
//   - time.Now().Format("15:04:05"): 格式化当前时间为 "时:分:秒"
//     Go 的时间格式化使用 "15:04:05" 这个特定时间作为模板
func LogMsg(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...) // 格式化消息
	fmt.Print(msg)                   // 终端输出（保底）

	logMutex.Lock() // 加锁保护 LogBuffer
	// 将带时间戳的日志追加到缓冲区
	LogBuffer = append(LogBuffer, time.Now().Format("15:04:05")+" "+msg)
	// 环形缓冲：如果超过 50 条，只保留最新的 50 条
	// LogBuffer[len(LogBuffer)-50:] 表示从倒数第 50 个元素开始截取
	if len(LogBuffer) > 50 {
		LogBuffer = LogBuffer[len(LogBuffer)-50:]
	}
	logMutex.Unlock() // 解锁
}

// GetLogs 供前端读取最新的日志列表。
// 返回 LogBuffer 的副本（而非引用），避免外部修改内部状态。
//
// Go 语法要点：
//   - make([]string, len(LogBuffer)): 创建指定长度的切片
//   - copy(res, LogBuffer): 将 LogBuffer 的内容复制到 res
//     copy 是 Go 内置函数，专门用于切片复制
//   - 注意：这里用的是 Lock() 而非 RLock()，因为 copy 操作需要确保数据一致性
func GetLogs() []string {
	logMutex.Lock()
	defer logMutex.Unlock()
	res := make([]string, len(LogBuffer)) // 创建副本切片
	copy(res, LogBuffer)                  // 复制内容
	return res
}

// ==========================================
// 同步引擎 A：K 线日线行情采集器
// ==========================================

// StartSyncKLine 是 K 线日线行情的同步引擎。
// 【核心功能】遍历股票列表，检查每只股票的日线数据是否有缺失（"空洞"），
// 只拉取缺失部分的数据（"精确填缝"），避免重复请求已有数据。
//
// 【工作流程】
//  1. 遍历所有股票代码
//  2. 调用 db.GetDailySyncTaskRange 检查数据库中已有哪些数据
//  3. 如果数据完整，跳过；如果有缺失，计算需要拉取的精确区间
//  4. 调用数据源拉取数据（最多重试 3 次）
//  5. 成功则推入异步落盘管道；失败则记录日志
//  6. 特殊处理：如果 API 返回空数据（停牌），填入"幽灵标记"防止死循环
//
// 参数:
//   - provider:    数据源（TushareProvider 或 OpenSourceProvider）
//   - stockCodes:  股票代码列表，如 ["000001.SZ", "600000.SH", ...]
//   - targetStart: 目标起始日期，格式 "20060102"
//   - targetEnd:   目标结束日期，格式 "20060102"
//
// 返回值:
//   - SyncSummary: 同步统计摘要（总数、成功数、失败数、跳过数）
func StartSyncKLine(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "kline", Total: total}
	LogMsg("🚀 [采集器A] K线引擎启动！当前源:[%s]\n", provider.GetName())

	// 根据数据源类型确定可信度权重
	// Tushare 权重 100（权威），东方财富权重 50（免费）
	// 这个权重会影响天眼系统的数据优先级判断
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	// 遍历每只股票，逐个检查和补全数据
	for i, code := range stockCodes {
		// 【天眼系统】调用数据库层的智能检测函数，
		// 返回实际需要同步的精确区间 [actualStart, actualEnd]
		// 如果数据已完整，needSync = false，直接跳过
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_klines", code, targetStart, targetEnd, targetTrust)
		if !needSync {
			summary.Skipped++
			continue // 数据完整，静默跳过
		}
		summary.Targeted++ // 标记为需要同步

		LogMsg("⏳ [K线 %d/%d] 发现空洞，准备拉取 %s...", i+1, total, code)

		// 【重试机制】网络请求可能失败，最多重试 3 次
		var history []tushare.DailyKLine
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 接入全局漏桶限速器，等待允许发送请求
			history, err = provider.FetchStockHistory(code, actualStart, actualEnd)
			if err == nil {
				break // 请求成功，跳出重试循环
			}
			time.Sleep(2 * time.Second) // 失败后等待 2 秒再重试
		}

		// 根据请求结果分三种情况处理
		if err == nil && len(history) > 0 {
			// 情况 1：成功获取到数据，推入异步落盘管道
			PushToSink("kline", code, history)
			summary.Success++
		} else if err == nil && len(history) == 0 {
			// =======================================================
			// 情况 2：API 请求成功但返回空数据 —— 说明这只股票停牌了
			//
			// 【为什么需要"幽灵标记"？】
			// 如果不处理，下次同步时天眼系统会认为这里还有空洞，
			// 又去请求，又返回空，形成死循环。
			// 解决方案：填入一个 TrustLevel=-1 的"幽灵数据"占位，
			// 天眼系统会认为这里已有数据（虽然权重低），不再重复请求。
			// 以后如果有了真实数据（TrustLevel=100），UPSERT 会自动覆盖幽灵数据。
			// =======================================================
			LogMsg("⚠️ [K线] %s 在 %s~%s 期间无数据(停牌)，填入幽灵标记防止死循环...", code, actualStart, actualEnd)

			// 查询这段时间内所有的交易日（排除周末和节假日）
			query := `SELECT cal_date FROM trade_calendar WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?`
			rows, _ := db.DB.Query(query, actualStart, actualEnd)
			var ghostKLines []tushare.DailyKLine
			for rows.Next() {
				var d string
				if rows.Scan(&d) == nil {
					// 为每个交易日创建一条"幽灵 K 线"（所有价格为 0）
					ghostKLines = append(ghostKLines, tushare.DailyKLine{
						TSCode: code, TradeDate: d,
						Open: 0, Close: 0, High: 0, Low: 0, Vol: 0, Amount: 0,
						DataSource: "SYSTEM_GHOST", // 标记为系统生成的幽灵数据
						TrustLevel: -1,             // 极低权重，真实数据可随时覆盖
					})
				}
			}
			rows.Close() // 关闭数据库游标，释放连接

			if len(ghostKLines) > 0 {
				PushToSink("kline", code, ghostKLines)
				summary.Success++
			} else {
				summary.Failed++
			}
		} else {
			// 情况 3：请求彻底失败（3 次重试都失败）
			LogMsg("❌ [K线 %d/%d] %s 失败: %v", i+1, total, code, err)
			summary.Failed++
		}
	}
	LogMsg("🎉 [采集器A] K线填缝网络拉取阶段完成！(请等待 Sink 落盘)\n")
	return summary
}

// ==========================================
// 同步引擎 B：每日基本面指标采集器
// ==========================================

// StartSyncFund 是每日基本面指标的同步引擎。
// 【功能】与 StartSyncKLine 类似，但采集的是基本面数据（PE、PB、市值等）。
//
// 【与 K 线引擎的区别】
//   - 数据表不同：daily_fundamentals（不是 daily_klines）
//   - 重试间隔更长：5 秒（基本面接口更敏感，需要更长的冷静期）
//   - 没有幽灵标记机制（基本面数据为空通常不是停牌，可能是数据延迟）
//
// 参数:
//   - provider:    数据源
//   - stockCodes:  股票代码列表
//   - targetStart: 目标起始日期
//   - targetEnd:   目标结束日期
//
// 返回值:
//   - SyncSummary: 同步统计摘要
func StartSyncFund(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "fund", Total: total}
	LogMsg("💎 [采集器B] 基本面引擎启动！\n")

	// 根据数据源类型确定可信度权重
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	for i, code := range stockCodes {
		// 天眼系统：检查基本面数据的缺失区间
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_fundamentals", code, targetStart, targetEnd, targetTrust)

		if !needSync {
			summary.Skipped++
			continue // 数据完整，跳过
		}
		summary.Targeted++

		LogMsg("⏳ [基本面 %d/%d] 发现空洞！定向拉取 %s (%s~%s)...\n", i+1, total, code, actualStart, actualEnd)

		// 重试机制：最多 3 次，每次失败等待 5 秒
		var funds []tushare.DailyFundamental
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 接入全局漏桶限速器
			funds, err = provider.FetchDailyBasic(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			LogMsg("⚠️ [基本面] %s 报错: %v。重试 %d/3...\n", code, err, retry+1)
			time.Sleep(5 * time.Second) // 基本面接口更敏感，等待更长
		}

		if err != nil {
			LogMsg("❌ [基本面 %d/%d] %s 彻底失败！交由对账员下次处理。\n", i+1, total, code)
			summary.Failed++
			continue
		}

		if len(funds) > 0 {
			// 成功获取数据，推入异步落盘管道
			// 第一个参数 "fund" 告诉 Sink Worker 写入 daily_fundamentals 表
			PushToSink("fund", code, funds)
			summary.Success++
		} else {
			LogMsg("⚠️ [基本面 %d/%d] %s 返回 0 条数据\n", i+1, total, code)
			summary.Failed++
		}
	}
	LogMsg("🎉 [采集器B] 基本面网络拉取填缝完成！(等待后台 Sink 落盘)\n")
	return summary
}

// ==========================================
// 同步引擎 C：复权因子采集器
// ==========================================

// StartSyncAdjFactors 是复权因子的同步引擎。
// 【功能】遍历股票列表，检查每只股票的复权因子数据是否有缺失，只拉取缺失部分。
//
// 【复权因子的重要性】
// 复权因子是技术分析的基础。没有复权因子，历史价格在分红/送股时会出现断裂，
// 导致技术指标（如均线、MACD）计算错误。
//
// 参数:
//   - provider:    数据源
//   - stockCodes:  股票代码列表
//   - targetStart: 目标起始日期
//   - targetEnd:   目标结束日期
//
// 返回值:
//   - SyncSummary: 同步统计摘要
func StartSyncAdjFactors(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "adj", Total: total}
	LogMsg("🧬 [采集器C] 复权因子引擎启动！当前源:[%s]\n", provider.GetName())

	// 根据数据源类型确定可信度权重
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	for i, code := range stockCodes {
		// 天眼系统：检查复权因子数据的缺失区间
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("adj_factors", code, targetStart, targetEnd, targetTrust)
		if !needSync {
			summary.Skipped++
			continue // 数据完整，跳过
		}
		summary.Targeted++

		LogMsg("⏳ [复权 %d/%d] 发现断层！正在拉取并推导 %s (%s~%s)...", i+1, total, code, actualStart, actualEnd)

		// 重试机制：最多 3 次，每次失败等待 3 秒
		var factors []tushare.AdjFactor
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 接入全局漏桶限速器
			factors, err = provider.FetchAdjFactors(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			time.Sleep(3 * time.Second)
		}

		if err != nil {
			LogMsg("❌ [复权 %d/%d] %s 彻底失败！", i+1, total, code)
			summary.Failed++
			continue
		}

		if len(factors) > 0 {
			// 成功获取数据，推入异步落盘管道
			PushToSink("adj", code, factors)
			summary.Success++
		} else {
			summary.Failed++
		}
	}
	LogMsg("🎉 [采集器C] 复权因子网络拉取完成！\n")
	return summary
}

// ==========================================
// 同步引擎 G：涨跌停价格采集器
// ==========================================

// StartSyncStkLimit 是涨跌停价格的同步引擎。
// 【与其他引擎的区别】
//   - K 线/基本面/复权因子：按"股票"维度遍历，每只股票拉取一个日期区间
//   - 涨跌停：按"日期"维度遍历，每个交易日拉取全市场所有股票的涨跌停数据
//
// 【工作流程】
//  1. 调用 db.GetMarketMissingDates 检查哪些交易日缺少涨跌停数据
//  2. 遍历缺失的日期，逐日请求全市场数据
//  3. 如果某天返回空数据，填入幽灵标记防止死循环
//
// 参数:
//   - provider:    数据源（需要支持 FetchStkLimit 接口）
//   - targetStart: 目标起始日期
//   - targetEnd:   目标结束日期
//
// 返回值:
//   - SyncSummary: 同步统计摘要
func StartSyncStkLimit(provider DataProvider, targetStart string, targetEnd string) SyncSummary {
	summary := SyncSummary{Module: "stklimit"}
	LogMsg("🔥 [采集器G] 涨跌停引擎启动！当前源:[%s]", provider.GetName())

	// 调用数据库层的日历检测器，找出所有缺少涨跌停数据的交易日
	missingDates := db.GetMarketMissingDates("daily_stk_limit", targetStart, targetEnd)

	if len(missingDates) == 0 {
		LogMsg("✅ [采集器G] 目标区间涨跌停数据严丝合缝，无需重复拉取！")
		summary.Skipped = 1
		return summary
	}

	summary.Total = len(missingDates)
	summary.Targeted = len(missingDates)
	LogMsg("⏳ [采集器G] 发现 %d 个交易日缺失数据，开始逐日补齐...", len(missingDates))
	totalSaved := 0

	// 遍历每个缺失的日期，逐日请求全市场涨跌停数据
	for i, d := range missingDates {
		WaitToken() // 接入全局漏桶限速器

		limits, err := provider.FetchStkLimit(d)
		if err != nil {
			LogMsg("⚠️ [采集器G] %s 报错: %v", d, err)
			time.Sleep(2 * time.Second) // 遇到错误冷静 2 秒
			summary.Failed++
			continue
		}

		if len(limits) > 0 {
			// 成功获取数据，推入异步落盘管道
			// TSCode 设为 "ALL" 表示这是全市场的数据（不是单只股票）
			PushToSink("stklimit", "ALL", limits)
			totalSaved += len(limits)
			summary.Success++
		} else {
			// 没有数据（可能是非交易日），填入幽灵标记防止死循环
			ghost := []tushare.StkLimit{{
				TradeDate:  d,
				TSCode:     "GHOST",           // 幽灵标记
				DataSource: "SYSTEM_GHOST",    // 系统生成
				TrustLevel: -1,                // 极低权重
			}}
			PushToSink("stklimit", "ALL", ghost)
			summary.Success++
		}

		// 每处理 50 天或最后一天时，输出进度汇报
		if (i+1)%50 == 0 || i == len(missingDates)-1 {
			LogMsg("🔄 [采集器G] 进度汇报: 已处理 %d/%d 天，累计发现 %d 个标的...", i+1, len(missingDates), totalSaved)
		}
	}
	LogMsg("🎉 [采集器G] 涨跌停底座历史补齐完成！共推入真实数据: %d 条！", totalSaved)
	return summary
}
