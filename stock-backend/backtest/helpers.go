package backtest

import (
	"fmt"
	"stock-backend/strategy"
	"stock-backend/tushare"
	"strings"
	"time"
)

// subtractDays 从日期字符串减去 n 天。
func subtractDays(dateStr string, n int) string {
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return dateStr
	}
	return t.AddDate(0, 0, -n).Format("20060102")
}

func selectAnalyzers(name string) []strategy.Analyzer {
	switch strings.ToUpper(name) {
	case "MACB":
		return []strategy.Analyzer{&strategy.MACBAnalyzer{}}
	case "CBBM":
		return []strategy.Analyzer{&strategy.CBBMAnalyzer{}}
	default:
		return strategy.GetActiveAnalyzers()
	}
}

func containsBuySignal(signal string) bool {
	return strings.Contains(signal, "买入")
}

func isOneWordLimitUp(k tushare.DailyKLine, limit tushare.StkLimit) bool {
	return k.Open == k.High && k.High == k.Low && k.Low == k.Close &&
		k.Close >= limit.UpLimit && limit.UpLimit > 0
}

// countDaysBetween 计算 tradingDays 中 (fromDate, toDate] 的交易日数。
func countDaysBetween(tradingDays []string, fromDate, toDate string) int {
	count := 0
	for _, d := range tradingDays {
		if d > fromDate && d <= toDate {
			count++
		}
	}
	return count
}

func buildPortfolioSummary(cfg BacktestConfig, finalAssets, totalReturn, winRate, maxDrawdown float64, trades, poolSize int) string {
	return fmt.Sprintf(
		"%s 回测 %s ~ %s | 池%d只 | 初始资金 %.0f → 最终 %.0f | 收益率 %.2f%% | 胜率 %.1f%% (%d笔) | 最大回撤 %.2f%%",
		cfg.Strategy, cfg.StartDate, cfg.EndDate, poolSize,
		cfg.InitialCapital, finalAssets, totalReturn,
		winRate, trades, maxDrawdown,
	)
}
