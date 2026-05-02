package backtest

import (
	"fmt"
	"stock-backend/db"
	"stock-backend/strategy"
	"stock-backend/tushare"
	"strings"
	"time"
)

// Run 执行完整回测，纯内存运算，不写入 SQLite。
func Run(cfg BacktestConfig) (*BacktestResult, error) {
	// ── Phase 1: 参数校验与默认值 ──
	if cfg.TSCode == "" {
		return nil, fmt.Errorf("股票代码不能为空")
	}
	if cfg.InitialCapital <= 0 {
		cfg.InitialCapital = 100000
	}
	if cfg.ProfitTakePct <= 0 {
		cfg.ProfitTakePct = 20.0
	}
	if cfg.Commission < 0 {
		cfg.Commission = 0
	}
	if cfg.Strategy == "" {
		cfg.Strategy = "ALL"
	}

	// ── Phase 2: 数据加载 ──
	lookbackStart := subtractDays(cfg.StartDate, 300)

	klines := db.GetKLinesFromDB(cfg.TSCode, lookbackStart, cfg.EndDate)
	if len(klines) == 0 {
		return nil, fmt.Errorf("未找到 %s 的K线数据", cfg.TSCode)
	}

	adjFactors := db.GetAdjFactorsFromDB(cfg.TSCode, lookbackStart, cfg.EndDate)
	if len(adjFactors) > 0 {
		klines = strategy.ForwardAdjustKLines(klines, adjFactors)
	}

	fundamentals := db.GetFundamentalsFromDB(cfg.TSCode, lookbackStart, cfg.EndDate)
	moneyFlows := db.GetMoneyFlowFromDB(cfg.TSCode, lookbackStart, cfg.EndDate)
	stkLimits := db.GetStkLimitFromDB(cfg.TSCode, lookbackStart, cfg.EndDate)

	fundMap := buildFundMap(fundamentals)
	flowMap := buildFlowMap(moneyFlows)
	limitMap := buildLimitMap(stkLimits)

	// ── Phase 3: 定位模拟起点 ──
	simStartIdx := findStartIndex(klines, cfg.StartDate)
	if simStartIdx < 0 {
		return nil, fmt.Errorf("回测起始日 %s 之前没有足够的历史数据", cfg.StartDate)
	}

	analyzers := selectAnalyzers(cfg.Strategy)

	// ── Phase 4: 正向逐日模拟 ──
	var pos *position
	var pendingBuy *pendingOrder
	var tradeLog []TradeRecord
	var equityCurve []EquityPoint
	cash := cfg.InitialCapital
	peakValue := cfg.InitialCapital
	maxDrawdown := 0.0

	for i := simStartIdx; i < len(klines); i++ {
		today := klines[i]
		window := klines[:i+1]

		// ── A. 执行挂单买入（T+1） ──
		if pendingBuy != nil && i+1 < len(klines) {
			execDay := klines[i+1]
			// 一字涨停检测：开盘即涨停封死，买不到
			if limit, ok := limitMap[execDay.TradeDate]; ok {
				if isOneWordLimitUp(execDay, limit) {
					pendingBuy = nil // 放弃信号
				}
			}
			if pendingBuy != nil {
				buyPrice := execDay.Open
				maxShares := int(cash / (buyPrice * (1 + cfg.Commission)))
				shares := (maxShares / 100) * 100
				if shares >= 100 {
					cost := float64(shares) * buyPrice * (1 + cfg.Commission)
					if cost <= cash {
						pos = &position{
							BuyDate:   execDay.TradeDate,
							BuyPrice:  buyPrice,
							Shares:    shares,
							Strategy:  pendingBuy.Strategy,
							BuyReason: pendingBuy.Reason,
						}
						cash -= cost
					}
				}
				pendingBuy = nil
			}
		}

		// ── B. 检查卖出条件 ──
		if pos != nil {
			if sellSignal, reason := checkSellConditions(today, window, pos, cfg, limitMap); sellSignal {
				sellAmount := float64(pos.Shares) * today.Close * (1 - cfg.Commission)
				pnl := sellAmount - float64(pos.Shares)*pos.BuyPrice*(1+cfg.Commission)
				tradeLog = append(tradeLog, TradeRecord{
					BuyDate:    pos.BuyDate,
					BuyPrice:   pos.BuyPrice,
					SellDate:   today.TradeDate,
					SellPrice:  today.Close,
					Shares:     pos.Shares,
					PnL:        pnl,
					ReturnPct:  (today.Close - pos.BuyPrice) / pos.BuyPrice * 100,
					HoldDays:   countTradingDays(klines, pos.BuyDate, today.TradeDate),
					BuyReason:  pos.BuyReason,
					SellReason: reason,
					Strategy:   pos.Strategy,
				})
				cash += sellAmount
				pos = nil
			}
		}

		// ── C. 检查买入信号 ──
		if pos == nil && pendingBuy == nil {
			ctx := buildContext(cfg.TSCode, window, fundMap, flowMap, today.TradeDate)
			for _, analyzer := range analyzers {
				result := analyzer.Analyze(ctx)
				if containsBuySignal(result.Signal) {
					pendingBuy = &pendingOrder{
						SignalDate:  today.TradeDate,
						Strategy:    analyzer.Name(),
						Reason:      result.Message,
						SignalPrice: today.Close,
					}
					break
				}
			}
		}

		// ── D. 记录净值 ──
		totalValue := cash
		if pos != nil {
			totalValue += float64(pos.Shares) * today.Close
		}
		equityCurve = append(equityCurve, EquityPoint{
			Date:  today.TradeDate,
			Value: totalValue,
		})
		if totalValue > peakValue {
			peakValue = totalValue
		}
		dd := (peakValue - totalValue) / peakValue * 100
		if dd > maxDrawdown {
			maxDrawdown = dd
		}
	}

	// ── Phase 5: 回测结束强制平仓 ──
	if pos != nil {
		lastDay := klines[len(klines)-1]
		sellAmount := float64(pos.Shares) * lastDay.Close * (1 - cfg.Commission)
		pnl := sellAmount - float64(pos.Shares)*pos.BuyPrice*(1+cfg.Commission)
		tradeLog = append(tradeLog, TradeRecord{
			BuyDate:    pos.BuyDate,
			BuyPrice:   pos.BuyPrice,
			SellDate:   lastDay.TradeDate,
			SellPrice:  lastDay.Close,
			Shares:     pos.Shares,
			PnL:        pnl,
			ReturnPct:  (lastDay.Close - pos.BuyPrice) / pos.BuyPrice * 100,
			HoldDays:   countTradingDays(klines, pos.BuyDate, lastDay.TradeDate),
			BuyReason:  pos.BuyReason,
			SellReason: "回测结束平仓",
			Strategy:   pos.Strategy,
		})
		cash += sellAmount
	}

	// ── Phase 6: 统计汇总 ──
	finalAssets := cash
	totalReturn := (finalAssets - cfg.InitialCapital) / cfg.InitialCapital * 100
	winCount, lossCount := 0, 0
	for _, t := range tradeLog {
		if t.PnL > 0 {
			winCount++
		} else {
			lossCount++
		}
	}
	winRate := 0.0
	if len(tradeLog) > 0 {
		winRate = float64(winCount) / float64(len(tradeLog)) * 100
	}

	return &BacktestResult{
		Config:      cfg,
		InitialCap:  cfg.InitialCapital,
		FinalAssets: finalAssets,
		TotalReturn: totalReturn,
		TotalTrades: len(tradeLog),
		WinTrades:   winCount,
		LossTrades:  lossCount,
		WinRate:     winRate,
		MaxDrawdown: maxDrawdown,
		TradeLog:    tradeLog,
		EquityCurve: equityCurve,
		Summary:     buildSummary(cfg, finalAssets, totalReturn, winRate, maxDrawdown, len(tradeLog)),
	}, nil
}

// ── 辅助函数 ──

func subtractDays(dateStr string, n int) string {
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return dateStr
	}
	return t.AddDate(0, 0, -n).Format("20060102")
}

func buildFundMap(funds []tushare.DailyFundamental) map[string]tushare.DailyFundamental {
	m := make(map[string]tushare.DailyFundamental, len(funds))
	for _, f := range funds {
		m[f.TradeDate] = f
	}
	return m
}

func buildFlowMap(flows []tushare.DailyMoneyFlow) map[string]tushare.DailyMoneyFlow {
	m := make(map[string]tushare.DailyMoneyFlow, len(flows))
	for _, f := range flows {
		m[f.TradeDate] = f
	}
	return m
}

func buildLimitMap(limits []tushare.StkLimit) map[string]tushare.StkLimit {
	m := make(map[string]tushare.StkLimit, len(limits))
	for _, l := range limits {
		m[l.TradeDate] = l
	}
	return m
}

func findStartIndex(klines []tushare.DailyKLine, startDate string) int {
	for i, k := range klines {
		if k.TradeDate >= startDate {
			return i
		}
	}
	return -1
}

func buildContext(
	code string,
	window []tushare.DailyKLine,
	fundMap map[string]tushare.DailyFundamental,
	flowMap map[string]tushare.DailyMoneyFlow,
	currentDate string,
) *strategy.SecurityContext {
	// 获取 <= currentDate 的最新基本面
	var latestFund tushare.DailyFundamental
	var funds []tushare.DailyFundamental
	for _, k := range window {
		if f, ok := fundMap[k.TradeDate]; ok && k.TradeDate <= currentDate {
			latestFund = f
		}
	}
	if latestFund.TSCode != "" {
		funds = append(funds, latestFund)
	}

	// 获取 <= currentDate 的最新资金流向
	var latestFlow tushare.DailyMoneyFlow
	var flows []tushare.DailyMoneyFlow
	for _, k := range window {
		if f, ok := flowMap[k.TradeDate]; ok && k.TradeDate <= currentDate {
			latestFlow = f
		}
	}
	if latestFlow.TSCode != "" {
		flows = append(flows, latestFlow)
	}

	pePercentile := db.GetPEPercentile(code, currentDate, 750)

	return &strategy.SecurityContext{
		Code:         code,
		KLines:       window,
		Fundamentals: funds,
		MoneyFlows:   flows,
		PEPercentile: pePercentile,
	}
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

func countTradingDays(klines []tushare.DailyKLine, fromDate, toDate string) int {
	count := 0
	for _, k := range klines {
		if k.TradeDate > fromDate && k.TradeDate <= toDate {
			count++
		}
	}
	return count
}

func buildSummary(cfg BacktestConfig, finalAssets, totalReturn, winRate, maxDrawdown float64, trades int) string {
	return fmt.Sprintf(
		"%s 回测 %s ~ %s | 初始资金 %.0f → 最终 %.0f | 收益率 %.2f%% | 胜率 %.1f%% (%d笔) | 最大回撤 %.2f%%",
		cfg.Strategy, cfg.StartDate, cfg.EndDate,
		cfg.InitialCapital, finalAssets, totalReturn,
		winRate, trades, maxDrawdown,
	)
}

