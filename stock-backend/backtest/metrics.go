package backtest

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// CalcSummary 计算回测汇总指标。
func CalcSummary(cfg BacktestConfig, finalAssets float64, tradeLog []TradeRecord, dailyEquity []DailyEquity) BacktestSummary {
	absReturn := (finalAssets - cfg.InitialCapital) / cfg.InitialCapital * 100

	// CAGR 复合年化收益率
	cagr := calcCAGR(cfg.InitialCapital, finalAssets, cfg.StartDate, cfg.EndDate)

	// 胜率
	winCount := 0
	for _, t := range tradeLog {
		if t.PnL > 0 {
			winCount++
		}
	}
	winRate := 0.0
	if len(tradeLog) > 0 {
		winRate = float64(winCount) / float64(len(tradeLog)) * 100
	}

	// 最大回撤
	maxDD := calcMaxDrawdown(dailyEquity)

	// 退出原因分布
	exitReasons := calcExitReasons(tradeLog)

	return BacktestSummary{
		InitialCapital: cfg.InitialCapital,
		FinalAssets:    finalAssets,
		AbsReturnPct:   absReturn,
		CAGR:           cagr,
		WinRate:        winRate,
		MaxDrawdown:    maxDD,
		TotalTrades:    len(tradeLog),
		TradingDays:    len(dailyEquity),
		ExitReasons:    exitReasons,
	}
}

// calcCAGR 计算复合年化收益率。
func calcCAGR(initial, final float64, startDate, endDate string) float64 {
	start, err1 := time.Parse("20060102", startDate)
	end, err2 := time.Parse("20060102", endDate)
	if err1 != nil || err2 != nil || initial <= 0 || final <= 0 {
		return 0
	}
	years := end.Sub(start).Hours() / 24 / 365.25
	if years <= 0 {
		return 0
	}
	return (math.Pow(final/initial, 1.0/years) - 1) * 100
}

// calcMaxDrawdown 从每日净值序列计算最大回撤百分比。
func calcMaxDrawdown(dailyEquity []DailyEquity) float64 {
	if len(dailyEquity) == 0 {
		return 0
	}
	peak := dailyEquity[0].Equity
	maxDD := 0.0
	for _, de := range dailyEquity {
		if de.Equity > peak {
			peak = de.Equity
		}
		if peak > 0 {
			dd := (peak - de.Equity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

// calcExitReasons 统计退出原因分布。
func calcExitReasons(tradeLog []TradeRecord) ExitReasonCount {
	var ec ExitReasonCount
	for _, t := range tradeLog {
		reason := t.SellReason
		classifyExitReason(reason, &ec)
		// 也统计分批减仓的退出原因（在 SellReason 中已合并）
	}
	return ec
}

// classifyExitReason 根据关键字分类退出原因。
func classifyExitReason(reason string, ec *ExitReasonCount) {
	switch {
	case strings.Contains(reason, "半仓止盈"):
		ec.TakeProfit++
	case strings.Contains(reason, "追踪止损"):
		ec.TrailingStop++
	case strings.Contains(reason, "时间止损"):
		ec.TimeExit++
	case strings.Contains(reason, "硬止损") || strings.Contains(reason, "洗盘"):
		ec.HardStop++
	default:
		ec.Other++
	}
}

// PrintSummaryReport 在控制台打印格式化的汇总报告。
func PrintSummaryReport(summary BacktestSummary) {
	log.Println("╔══════════════════════════════════════════════════════════════╗")
	log.Println("║                    回测汇总报告 (Backtest Summary)           ║")
	log.Println("╠══════════════════════════════════════════════════════════════╣")
	log.Printf("║  初始资金:     %12.2f                                ║", summary.InitialCapital)
	log.Printf("║  最终资产:     %12.2f                                ║", summary.FinalAssets)
	log.Printf("║  绝对收益率:   %10.2f%%                                    ║", summary.AbsReturnPct)
	log.Printf("║  年化收益率:   %10.2f%% (CAGR)                              ║", summary.CAGR)
	winCount := int(summary.WinRate * float64(summary.TotalTrades) / 100)
	log.Printf("║  胜率:         %10.2f%% (%d / %d 笔)                       ║", summary.WinRate, winCount, summary.TotalTrades)
	log.Printf("║  最大回撤:     %10.2f%%                                    ║", summary.MaxDrawdown)
	log.Printf("║  交易天数:     %10d                                    ║", summary.TradingDays)
	log.Println("╠══════════════════════════════════════════════════════════════╣")
	log.Println("║  退出原因分布 (Exit Reason Distribution)                    ║")
	log.Println("╠══════════════════════════════════════════════════════════════╣")
	log.Printf("║  半仓止盈 (Take Profit):   %4d 笔                          ║", summary.ExitReasons.TakeProfit)
	log.Printf("║  追踪止损 (Trailing Stop): %4d 笔                          ║", summary.ExitReasons.TrailingStop)
	log.Printf("║  时间止损 (Time Exit):     %4d 笔                          ║", summary.ExitReasons.TimeExit)
	log.Printf("║  硬止损   (Hard Stop):     %4d 笔                          ║", summary.ExitReasons.HardStop)
	log.Printf("║  其他原因 (Other):         %4d 笔                          ║", summary.ExitReasons.Other)
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	// 也输出一份简洁的 JSON 格式（便于前端解析）
	fmt.Printf(`{"summary":{"initial_capital":%.2f,"final_assets":%.2f,"abs_return_pct":%.2f,"cagr":%.2f,"win_rate":%.2f,"max_drawdown":%.2f,"total_trades":%d,"trading_days":%d,"exit_reasons":{"take_profit":%d,"trailing_stop":%d,"time_exit":%d,"hard_stop":%d,"other":%d}}}`+"\n",
		summary.InitialCapital, summary.FinalAssets, summary.AbsReturnPct, summary.CAGR,
		summary.WinRate, summary.MaxDrawdown, summary.TotalTrades, summary.TradingDays,
		summary.ExitReasons.TakeProfit, summary.ExitReasons.TrailingStop,
		summary.ExitReasons.TimeExit, summary.ExitReasons.HardStop, summary.ExitReasons.Other,
	)
}
