package feeder

import (
	"stock-backend/db"
	"stock-backend/tushare"
	"time"
)

func StartSyncMoneyFlow(provider DataProvider, stockCodes []string, start string, end string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "moneyflow", Total: total}
	LogMsg("🌊 [抽水机E] 资金流向引擎启动！当前源:[%s]", provider.GetName())

	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	for i, code := range stockCodes {
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_moneyflow", code, start, end, targetTrust)
		if !needSync {
			summary.Skipped++
			continue
		}
		summary.Targeted++

		var flows []tushare.DailyMoneyFlow
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken()
			flows, err = provider.FetchMoneyFlow(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if err != nil {
			LogMsg("⚠️ [抽水机E %d/%d] %s 报错: %v", i+1, total, code, err)
			summary.Failed++
			continue
		}

		if len(flows) > 0 {
			PushToSink("moneyflow", code, flows)
			summary.Success++
		} else {
			summary.Failed++
		}
	}

	LogMsg("🎉 [抽水机E] 资金流向网络拉取完成！")
	return summary
}

func StartSyncFina(stockCodes []string, start string, end string) SyncSummary {
	total := len(stockCodes)
	summary := SyncSummary{Module: "fina", Total: total}
	LogMsg("🏦 [抽水机F] 季报财务引擎启动！[Tushare专属]")

	for i, code := range stockCodes {
		summary.Targeted++
		var finas []tushare.FinaIndicator
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken()
			finas, err = tushare.FetchFinaIndicators(code, start, end)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if err != nil {
			LogMsg("⚠️ [抽水机F %d/%d] %s 报错: %v", i+1, total, code, err)
			summary.Failed++
			continue
		}

		if len(finas) > 0 {
			PushToSink("fina", code, finas)
			summary.Success++
		} else {
			summary.Failed++
		}
	}

	LogMsg("🎉 [抽水机F] 季报财务拉取完成！")
	return summary
}

func StartSyncIndex(provider DataProvider, startDate, endDate string) SyncSummary {
	summary := SyncSummary{Module: "index", Total: 1}
	LogMsg("📊 [引擎D] 大盘指数同步启动...")

	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("index_daily", "000001.SH", startDate, endDate, targetTrust)
	if !needSync {
		summary.Skipped = 1
		LogMsg("✅ [引擎D] 上证指数严丝合缝，无需重复拉取。")
		return summary
	}
	summary.Targeted = 1

	var indices []tushare.IndexDaily
	var err error
	for retry := 0; retry < 3; retry++ {
		WaitToken()
		indices, err = provider.FetchIndexDaily("000001.SH", actualStart, actualEnd)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		LogMsg("❌ [引擎D] 上证指数拉取失败: %v", err)
		summary.Failed = 1
		return summary
	}

	if len(indices) > 0 {
		saved := db.BatchInsertIndexDaily("000001.SH", indices)
		LogMsg("✅ [引擎D] 上证指数同步完毕，新增/覆盖: %d 条", saved)
		summary.Success = 1
	} else {
		summary.Failed = 1
	}
	return summary
}
