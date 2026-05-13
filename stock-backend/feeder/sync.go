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
// 💥 数据源适配器接口 (Adapter Pattern)
// ==========================================

// DataProvider 定义了所有情报供应商必须遵守的协议
type DataProvider interface {
	FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error)
	FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error)
	FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) // 💥 新增
	FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error)
	FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error)
	FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) // 💥 挂载涨跌停榜武器
	FetchCyqPerf(tsCode, tradeDate string) ([]tushare.CyqPerf, error)
	FetchStkFactorPro(tsCode, tradeDate string) ([]tushare.StkFactorPro, error)
	GetName() string
}

// ------------------------------------------
// 驱动 A：Tushare 高级付费付费数据源 (全量历史基石)
type TushareProvider struct{}

func (t *TushareProvider) GetName() string { return "Tushare [高级]" }
func (t *TushareProvider) FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	return tushare.FetchStockHistory(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	return tushare.FetchAdjFactors(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	return tushare.FetchDailyBasic(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	return tushare.FetchIndexDaily(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	return tushare.FetchMoneyFlow(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) {
	return tushare.FetchStkLimit(tradeDate)
}
func (t *TushareProvider) FetchCyqPerf(tsCode, tradeDate string) ([]tushare.CyqPerf, error) {
	return tushare.FetchCyqPerf(tsCode, tradeDate)
}
func (t *TushareProvider) FetchStkFactorPro(tsCode, tradeDate string) ([]tushare.StkFactorPro, error) {
	return tushare.FetchStkFactorPro(tsCode, tradeDate)
}

// ------------------------------------------
// 驱动 B：开源免费轻步兵 (东方财富)
// ------------------------------------------
type OpenSourceProvider struct{}

func (o *OpenSourceProvider) GetName() string {
	return "开源接口 (EastMoney)"
}

func (o *OpenSourceProvider) FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	return eastmoney.FetchStockHistory(tsCode, startDate, endDate)
}

// 修改 feeder/sync.go 中的 OpenSourceProvider 基本面方法
func (o *OpenSourceProvider) FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	// 💥 接入东财开源基本面榨取引擎
	return eastmoney.FetchDailyBasic(tsCode, startDate, endDate)
}

func (o *OpenSourceProvider) FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	// 💥 启用东财逆向推导引擎替代 Tushare
	return eastmoney.FetchDerivedAdjFactors(tsCode, startDate, endDate)
}

// 💥 挂载新武器：大盘指数
func (o *OpenSourceProvider) FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	return eastmoney.FetchIndexDaily(tsCode, startDate, endDate)
}

// 💥 挂载新武器：资金流向
func (o *OpenSourceProvider) FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	return eastmoney.FetchMoneyFlow(tsCode, startDate, endDate)
}
func (o *OpenSourceProvider) FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) {
	// 开源降级版暂不支持，保护接口一致性
	return nil, fmt.Errorf("开源接口暂不支持历史涨跌停绝对价拉取")
}
func (o *OpenSourceProvider) FetchCyqPerf(tsCode, tradeDate string) ([]tushare.CyqPerf, error) {
	return nil, fmt.Errorf("开源接口暂不支持筹码分布数据拉取")
}
func (o *OpenSourceProvider) FetchStkFactorPro(tsCode, tradeDate string) ([]tushare.StkFactorPro, error) {
	return nil, fmt.Errorf("开源接口暂不支持技术因子专业版数据拉取")
}

// ==========================================

// ==========================================
// 💥 赛博控制台日志系统
// ==========================================
var (
	LogBuffer []string
	logMutex  sync.Mutex
)

type SyncSummary struct {
	// 统一同步统计结构：用于日志展示、自动任务步骤落库和前端可视化。
	Module   string `json:"module"`
	Total    int    `json:"total"`
	Targeted int    `json:"targeted"`
	Success  int    `json:"success"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped"`
}

// LogMsg 替代 fmt.Printf，同时输出到终端和前端内存环！
func LogMsg(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Print(msg) // 依然在终端输出一份保底

	logMutex.Lock()
	LogBuffer = append(LogBuffer, time.Now().Format("15:04:05")+" "+msg)
	// 永远只保留最新的 50 条日志，防止撑爆内存！
	if len(LogBuffer) > 50 {
		LogBuffer = LogBuffer[len(LogBuffer)-50:]
	}
	logMutex.Unlock()
}

// GetLogs 供前端读取最新日志
func GetLogs() []string {
	logMutex.Lock()
	defer logMutex.Unlock()
	res := make([]string, len(LogBuffer))
	copy(res, LogBuffer)
	return res
}

// ==========================================
// 💥 引擎 A：专属 K 线采集器
// 💥 引擎 A：专属 K 线采集器 (V3.0 动态请求频率版)
// 💥 引擎 A：专属 K 线采集器 (黎明补齐版：精确填缝)
func StartSyncKLine(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "kline", Total: total}
	LogMsg("🚀 [采集器A] K线引擎启动！当前源:[%s]\n", provider.GetName())
	// 💥 补上这段火力权重判定
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}
	for i, code := range stockCodes {
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_klines", code, targetStart, targetEnd, targetTrust)
		if !needSync {
			summary.Skipped++
			continue // 静默跳过，减少日志噪音
		}
		summary.Targeted++

		LogMsg("⏳ [K线 %d/%d] 发现空洞，准备拉取 %s...", i+1, total, code)

		var history []tushare.DailyKLine
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 💥 接入全局漏桶，等待开火指令
			history, err = provider.FetchStockHistory(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if err == nil && len(history) > 0 {
			// 💥 数据打入汇聚管线
			PushToSink("kline", code, history)
			summary.Success++
		} else if err == nil && len(history) == 0 {
			// =======================================================
			// 💥 终极修复：停牌股补漏机制！
			// 如果 API 请求成功，但确实没有数据，说明这天停牌了。
			// 塞入伪造的幽灵 K 线，权重设为 -1，防止以后天天重复拉取！
			// =======================================================
			LogMsg("⚠️ [K线] %s 在 %s~%s 期间无数据(停牌)，填入幽灵标记防止死循环...", code, actualStart, actualEnd)

			// 查出这期间所有的交易日
			query := `SELECT cal_date FROM trade_calendar WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?`
			rows, _ := db.DB.Query(query, actualStart, actualEnd)
			var ghostKLines []tushare.DailyKLine
			for rows.Next() {
				var d string
				if rows.Scan(&d) == nil {
					ghostKLines = append(ghostKLines, tushare.DailyKLine{
						TSCode: code, TradeDate: d,
						Open: 0, Close: 0, High: 0, Low: 0, Vol: 0, Amount: 0,
						DataSource: "SYSTEM_GHOST",
						TrustLevel: -1, // 极低权重，只要以后有了真实数据，立刻会被 UPSERT 覆盖！
					})
				}
			}
			rows.Close()
			if len(ghostKLines) > 0 {
				PushToSink("kline", code, ghostKLines)
				summary.Success++
			} else {
				summary.Failed++
			}
		} else {
			LogMsg("❌ [K线 %d/%d] %s 失败: %v", i+1, total, code, err)
			summary.Failed++
		}
	}
	LogMsg("🎉 [采集器A] K线填缝网络拉取阶段完成！(请等待 Sink 落盘)\n")
	return summary
}

// 💥 引擎 B：专属基本面采集器 (黎明补齐版：异步单点汇聚)
func StartSyncFund(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "fund", Total: total}
	LogMsg("💎 [采集器B] 基本面引擎启动！\n")
	// 💥 补上这段火力权重判定
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}
	for i, code := range stockCodes {
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_fundamentals", code, targetStart, targetEnd, targetTrust)

		if !needSync {
			summary.Skipped++
			continue // 静默跳过，避免刷屏
		}
		summary.Targeted++

		LogMsg("⏳ [基本面 %d/%d] 发现空洞！定向拉取 %s (%s~%s)...\n", i+1, total, code, actualStart, actualEnd)

		var funds []tushare.DailyFundamental
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 💥 接入全局漏桶防超速
			funds, err = provider.FetchDailyBasic(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			LogMsg("⚠️ [基本面] %s 报错: %v。重试 %d/3...\n", code, err, retry+1)
			time.Sleep(5 * time.Second)
		}

		if err != nil {
			LogMsg("❌ [基本面 %d/%d] %s 彻底失败！交由对账员下次处理。\n", i+1, total, code)
			summary.Failed++
			continue
		}

		if len(funds) > 0 {
			// 💥 正确做法：打入异步管线，Worker 立即转身去拉下一个股票！
			// 参数 1: 必须是 "fund" 字符串，让 Sink 路由知道调哪个表
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

// 💥 引擎 C：专属复权因子采集器
func StartSyncAdjFactors(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "adj", Total: total}
	LogMsg("🧬 [采集器C] 复权因子引擎启动！当前源:[%s]\n", provider.GetName())

	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	for i, code := range stockCodes {
		// 💥 接入天眼系统！
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("adj_factors", code, targetStart, targetEnd, targetTrust)
		if !needSync {
			summary.Skipped++
			continue // 静默跳过，保护 Tushare 积分！
		}
		summary.Targeted++

		LogMsg("⏳ [复权 %d/%d] 发现断层！正在拉取并推导 %s (%s~%s)...", i+1, total, code, actualStart, actualEnd)

		var factors []tushare.AdjFactor
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken()
			factors, err = provider.FetchAdjFactors(code, actualStart, actualEnd) // 使用精确区间
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
			PushToSink("adj", code, factors)
			summary.Success++
		} else {
			summary.Failed++
		}
	}
	LogMsg("🎉 [采集器C] 复权因子网络拉取完成！\n")
	return summary
}

// 💥 引擎 G：专属涨跌榜采集器 (通过日历智能推导区间)
func StartSyncStkLimit(provider DataProvider, targetStart string, targetEnd string) SyncSummary {
	summary := SyncSummary{Module: "stklimit"}
	LogMsg("🔥 [采集器G] 涨跌停引擎启动！当前源:[%s]", provider.GetName())

	// 调用我们刚刚写好的日历检测器，直接锁定所有空洞日期！
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

	for i, d := range missingDates {
		WaitToken() // 依然经过限速漏桶

		limits, err := provider.FetchStkLimit(d)
		if err != nil {
			LogMsg("⚠️ [采集器G] %s 报错: %v", d, err)
			time.Sleep(2 * time.Second) // 遇到错误冷静两秒
			summary.Failed++
			continue
		}

		if len(limits) > 0 {
			PushToSink("stklimit", "ALL", limits)
			totalSaved += len(limits)
			summary.Success++
		} else {
			// 打上幽灵标记防死循环
			ghost := []tushare.StkLimit{{
				TradeDate:  d,
				TSCode:     "GHOST",
				DataSource: "SYSTEM_GHOST",
				TrustLevel: -1,
			}}
			PushToSink("stklimit", "ALL", ghost)
			summary.Success++
		}

		if (i+1)%50 == 0 || i == len(missingDates)-1 {
			LogMsg("🔄 [采集器G] 进度汇报: 已处理 %d/%d 天，累计发现 %d 个标的...", i+1, len(missingDates), totalSaved)
		}
	}
	LogMsg("🎉 [采集器G] 涨跌停底座历史补齐完成！共推入真实数据: %d 条！", totalSaved)
	return summary
}
