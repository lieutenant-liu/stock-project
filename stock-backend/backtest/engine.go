package backtest

import (
	"fmt"
	"log"
	"sort"
	"stock-backend/db"
	"stock-backend/strategy"
	"stock-backend/tushare"
	"strings"
	"time"
)

// Run 执行组合级回测（按需查询架构，不预加载全量数据）。
func Run(cfg BacktestConfig) (*BacktestResult, error) {
	// ── Phase 1: 参数校验与默认值 ──
	log.Printf("[回测] 启动 | 策略=%s 区间=%s~%s 资金=%.0f", cfg.Strategy, cfg.StartDate, cfg.EndDate, cfg.InitialCapital)
	if len(cfg.TargetPool) == 0 {
		cfg.TargetPool = db.GetAllStockCodes()
		log.Printf("[回测] 自动加载全市场 %d 只", len(cfg.TargetPool))
	}
	if len(cfg.TargetPool) == 0 {
		return nil, fmt.Errorf("股票池为空，请先同步股票基础数据")
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
	if cfg.PositionSizePct <= 0 || cfg.PositionSizePct > 1 {
		cfg.PositionSizePct = 0.20
	}

	// ── Phase 2: 仅加载交易日历 ──
	tradingDays := db.GetTradingDays(cfg.StartDate, cfg.EndDate)
	if len(tradingDays) == 0 {
		return nil, fmt.Errorf("回测区间 %s ~ %s 内无交易日", cfg.StartDate, cfg.EndDate)
	}
	log.Printf("[回测] 交易日 %d 天 (%s ~ %s)", len(tradingDays), tradingDays[0], tradingDays[len(tradingDays)-1])

	analyzers := selectAnalyzers(cfg.Strategy)
	lookbackStart := subtractDays(cfg.StartDate, 300)

	// 建立 TargetPool set，用于快速过滤
	poolSet := make(map[string]bool, len(cfg.TargetPool))
	for _, code := range cfg.TargetPool {
		poolSet[code] = true
	}

	// ── Phase 3: 定位模拟起点 ──
	simStartIdx := 0
	for i, d := range tradingDays {
		if d >= cfg.StartDate {
			simStartIdx = i
			break
		}
	}

	// ── Phase 4: 正向逐日模拟（按需查询） ──
	tSimStart := time.Now()
	positions := make(map[string]*position)          // tsCode → 持仓
	posHistories := make(map[string]*positionHistory) // tsCode → K线历史缓存
	var pendingBuys []pendingOrder
	var tradeLog []TradeRecord
	var equityCurve []EquityPoint
	cash := cfg.InitialCapital
	peakValue := cfg.InitialCapital
	maxDrawdown := 0.0
	totalDays := len(tradingDays) - simStartIdx

	// 上一个交易日，用于建仓时加载历史的结束日期
	prevDate := func(d int) string {
		if d > 0 {
			return tradingDays[d-1]
		}
		return lookbackStart
	}

	for d := simStartIdx; d < len(tradingDays); d++ {
		todayDate := tradingDays[d]
		dayNum := d - simStartIdx + 1
		if dayNum%50 == 0 || dayNum == totalDays {
			log.Printf("[回测] 模拟进度 %d/%d 天 | 日期=%s | 持仓=%d | 净值=%.0f",
				dayNum, totalDays, todayDate, len(positions), cash)
		}

		// ── A. 加载今日截面数据 ──
		snapKLines := db.GetAllKLinesForDate(todayDate)
		snapLimits := db.GetAllLimitsForDate(todayDate)

		// ── B. 执行挂单买入（T+1，用 today 的 Open）──
		sort.Slice(pendingBuys, func(i, j int) bool {
			ki, okI := snapKLines[pendingBuys[i].TSCode]
			kj, okJ := snapKLines[pendingBuys[j].TSCode]
			if !okI {
				return false
			}
			if !okJ {
				return true
			}
			return ki.Open < kj.Open
		})

		for _, pb := range pendingBuys {
			stockKline, hasData := snapKLines[pb.TSCode]
			if !hasData {
				continue
			}

			// 一字涨停检测
			if limit, ok := snapLimits[pb.TSCode]; ok {
				if isOneWordLimitUp(stockKline, limit) {
					continue
				}
			}

			// 已持仓则跳过
			if _, held := positions[pb.TSCode]; held {
				continue
			}

			buyPrice := stockKline.Open
			targetAlloc := cfg.InitialCapital * cfg.PositionSizePct
			maxShares := int(targetAlloc / (buyPrice * (1 + cfg.Commission)))
			shares := (maxShares / 100) * 100

			if shares < 100 {
				continue
			}
			cost := float64(shares) * buyPrice * (1 + cfg.Commission)
			if cost > cash {
				continue
			}

			// 建仓：加载该股票的历史K线（用于后续卖出判断）
			history := db.GetKLinesWithAdj(pb.TSCode, lookbackStart, prevDate(d))
			posHistories[pb.TSCode] = &positionHistory{KLines: history}

			positions[pb.TSCode] = &position{
				TSCode:    pb.TSCode,
				BuyDate:   todayDate,
				BuyPrice:  buyPrice,
				Shares:    shares,
				Strategy:  pb.Strategy,
				BuyReason: pb.Reason,
			}
			cash -= cost
		}
		pendingBuys = nil

		// ── C. 检查持仓卖出 ──
		for tsCode, pos := range positions {
			todayKline, hasData := snapKLines[tsCode]
			if !hasData {
				continue
			}

			// 构建窗口：历史K线 + 今天K线
			hist := posHistories[tsCode]
			window := make([]tushare.DailyKLine, len(hist.KLines), len(hist.KLines)+1)
			copy(window, hist.KLines)
			window = append(window, todayKline)

			limitToday, hasLimit := snapLimits[tsCode]
			var limitMap map[string]tushare.StkLimit
			if hasLimit {
				limitMap = map[string]tushare.StkLimit{todayDate: limitToday}
			}

			if sellSignal, reason := checkSellConditions(todayKline, window, pos, cfg, limitMap); sellSignal {
				sellAmount := float64(pos.Shares) * todayKline.Close * (1 - cfg.Commission)
				pnl := sellAmount - float64(pos.Shares)*pos.BuyPrice*(1+cfg.Commission)
				tradeLog = append(tradeLog, TradeRecord{
					TSCode:     tsCode,
					BuyDate:    pos.BuyDate,
					BuyPrice:   pos.BuyPrice,
					SellDate:   todayDate,
					SellPrice:  todayKline.Close,
					Shares:     pos.Shares,
					PnL:        pnl,
					ReturnPct:  (todayKline.Close - pos.BuyPrice) / pos.BuyPrice * 100,
					HoldDays:   countDaysBetween(tradingDays, pos.BuyDate, todayDate),
					BuyReason:  pos.BuyReason,
					SellReason: reason,
					Strategy:   pos.Strategy,
				})
				cash += sellAmount
				delete(positions, tsCode)
				delete(posHistories, tsCode)
			} else {
				// 未卖出，追加今天的K线到历史缓存
				hist.KLines = append(hist.KLines, todayKline)
			}
		}

		// ── D. 扫描买入信号 ──
		for tsCode, kline := range snapKLines {
			// 只扫描池内股票
			if !poolSet[tsCode] {
				continue
			}
			// 跳过已持仓/已挂单
			if _, held := positions[tsCode]; held {
				continue
			}
			alreadyPending := false
			for _, pb := range pendingBuys {
				if pb.TSCode == tsCode {
					alreadyPending = true
					break
				}
			}
			if alreadyPending {
				continue
			}

			// 快速过滤：停牌或涨幅不足
			if kline.Vol <= 0 {
				continue
			}
			if kline.PctChg < 3.0 {
				continue
			}

			// 按需加载该股票的上下文（临时，用完即释放）
			ctx := buildOnDemandContext(tsCode, lookbackStart, todayDate)
			if ctx == nil || len(ctx.KLines) < 120 {
				continue
			}

			for _, analyzer := range analyzers {
				result := analyzer.Analyze(ctx)
				if containsBuySignal(result.Signal) {
					pendingBuys = append(pendingBuys, pendingOrder{
						TSCode:      tsCode,
						SignalDate:  todayDate,
						Strategy:    analyzer.Name(),
						Reason:      result.Message,
						SignalPrice: kline.Close,
					})
					break
				}
			}
		}

		// ── E. 记录净值 ──
		totalValue := cash
		for tsCode, pos := range positions {
			if klineToday, ok := snapKLines[tsCode]; ok {
				totalValue += float64(pos.Shares) * klineToday.Close
			} else {
				totalValue += float64(pos.Shares) * pos.BuyPrice
			}
		}
		equityCurve = append(equityCurve, EquityPoint{Date: todayDate, Value: totalValue})
		if totalValue > peakValue {
			peakValue = totalValue
		}
		dd := (peakValue - totalValue) / peakValue * 100
		if dd > maxDrawdown {
			maxDrawdown = dd
		}

		// 破产风控
		if totalValue < cfg.InitialCapital*0.5 {
			for tsCode, pos := range positions {
				klineToday, ok := snapKLines[tsCode]
				if !ok {
					continue
				}
				sellAmount := float64(pos.Shares) * klineToday.Close * (1 - cfg.Commission)
				pnl := sellAmount - float64(pos.Shares)*pos.BuyPrice*(1+cfg.Commission)
				tradeLog = append(tradeLog, TradeRecord{
					TSCode:     tsCode,
					BuyDate:    pos.BuyDate,
					BuyPrice:   pos.BuyPrice,
					SellDate:   todayDate,
					SellPrice:  klineToday.Close,
					Shares:     pos.Shares,
					PnL:        pnl,
					ReturnPct:  (klineToday.Close - pos.BuyPrice) / pos.BuyPrice * 100,
					HoldDays:   countDaysBetween(tradingDays, pos.BuyDate, todayDate),
					BuyReason:  pos.BuyReason,
					SellReason: "破产清算",
					Strategy:   pos.Strategy,
				})
				cash += sellAmount
			}
			positions = make(map[string]*position)
			posHistories = make(map[string]*positionHistory)
			break
		}
	}
	log.Printf("[回测] 模拟完成 | %d 天 | 持仓=%d | 交易=%d 笔 | 耗时 %s",
		totalDays, len(positions), len(tradeLog), time.Since(tSimStart).Round(time.Millisecond))

	// ── Phase 5: 回测结束强制平仓 ──
	if len(positions) > 0 {
		lastDate := tradingDays[len(tradingDays)-1]
		lastSnap := db.GetAllKLinesForDate(lastDate)
		for tsCode, pos := range positions {
			klineLast, ok := lastSnap[tsCode]
			if !ok {
				continue
			}
			sellAmount := float64(pos.Shares) * klineLast.Close * (1 - cfg.Commission)
			pnl := sellAmount - float64(pos.Shares)*pos.BuyPrice*(1+cfg.Commission)
			tradeLog = append(tradeLog, TradeRecord{
				TSCode:     tsCode,
				BuyDate:    pos.BuyDate,
				BuyPrice:   pos.BuyPrice,
				SellDate:   lastDate,
				SellPrice:  klineLast.Close,
				Shares:     pos.Shares,
				PnL:        pnl,
				ReturnPct:  (klineLast.Close - pos.BuyPrice) / pos.BuyPrice * 100,
				HoldDays:   countDaysBetween(tradingDays, pos.BuyDate, lastDate),
				BuyReason:  pos.BuyReason,
				SellReason: "回测结束平仓",
				Strategy:   pos.Strategy,
			})
			cash += sellAmount
		}
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

	log.Printf("[回测] 全部完成 | 收益率=%.2f%% 胜率=%.1f%% 回撤=%.2f%% 交易=%d笔",
		totalReturn, winRate, maxDrawdown, len(tradeLog))

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
		Summary:     buildPortfolioSummary(cfg, finalAssets, totalReturn, winRate, maxDrawdown, len(tradeLog), len(cfg.TargetPool)),
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

// buildOnDemandContext 按需加载单只股票的完整策略分析上下文。
// 加载完毕后数据由调用方持有，函数本身不缓存。
func buildOnDemandContext(code, lookbackStart, currentDate string) *strategy.SecurityContext {
	klines := db.GetKLinesWithAdj(code, lookbackStart, currentDate)
	if len(klines) == 0 {
		return nil
	}

	// 截取到昨天的数据（分析发生在收盘后，用到昨天为止的历史）
	var window []tushare.DailyKLine
	for _, k := range klines {
		if k.TradeDate < currentDate {
			window = append(window, k)
		}
	}
	// 如果截取后不足，用全部（可能 currentDate 本身也需要）
	if len(window) == 0 {
		window = klines
	}

	// 获取最新基本面
	fundamentals := db.GetFundamentalsFromDB(code, lookbackStart, currentDate)
	var latestFund tushare.DailyFundamental
	for _, f := range fundamentals {
		if f.TradeDate <= currentDate {
			latestFund = f
		}
	}
	var funds []tushare.DailyFundamental
	if latestFund.TSCode != "" {
		funds = append(funds, latestFund)
	}

	// 获取最新资金流向
	moneyFlows := db.GetMoneyFlowFromDB(code, lookbackStart, currentDate)
	var latestFlow tushare.DailyMoneyFlow
	for _, f := range moneyFlows {
		if f.TradeDate <= currentDate {
			latestFlow = f
		}
	}
	var flows []tushare.DailyMoneyFlow
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
