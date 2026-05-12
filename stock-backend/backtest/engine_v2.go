package backtest

import (
	"fmt"
	"log"
	"sort"
	"stock-backend/db"
	"stock-backend/strategy"
	"stock-backend/tushare"
	"time"
)

// RunV2 执行两阶段解耦回测。
// Phase 1: 按股票遍历，每只股票仅 4 次 DB 查询，产出理论交易信号。
// Phase 2: 按日历推演，0 次 DB 查询，纯内存计算逐日精确净值。
func RunV2(cfg BacktestConfig) (*BacktestResult, error) {
	log.Printf("[回测V2] 启动 | 策略=%s 区间=%s~%s 资金=%.0f", cfg.Strategy, cfg.StartDate, cfg.EndDate, cfg.InitialCapital)

	// ── 参数校验与默认值 ──
	if len(cfg.TargetPool) == 0 {
		cfg.TargetPool = db.GetAllStockCodes()
		log.Printf("[回测V2] 自动加载全市场 %d 只", len(cfg.TargetPool))
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

	// ── Phase 1: Signal Mining ──
	tPhase1 := time.Now()
	signals := phase1SignalMining(cfg)
	log.Printf("[回测V2] Phase 1 完成 | 信号=%d 笔 | 耗时 %s", len(signals), time.Since(tPhase1).Round(time.Millisecond))

	// ── Phase 2: Portfolio Simulation ──
	tPhase2 := time.Now()
	result, err := phase2PortfolioSim(cfg, signals)
	if err != nil {
		return nil, err
	}
	log.Printf("[回测V2] Phase 2 完成 | 耗时 %s", time.Since(tPhase2).Round(time.Millisecond))

	return result, nil
}

// ─────────────────────────────────────────────
// Phase 1: Signal Mining（按股票遍历）
// ─────────────────────────────────────────────

func phase1SignalMining(cfg BacktestConfig) []TheoreticalTrade {
	analyzers := selectAnalyzers(cfg.Strategy)
	lookbackStart := subtractDays(cfg.StartDate, 300)

	// 1. 宏观风控预计算（1次查询）
	indexData := db.GetIndexDailyForBacktest("000001.SH", lookbackStart, cfg.EndDate)
	isBullMarket := buildMarketRegimeMap(indexData)
	log.Printf("[回测V2] 大盘数据 %d 天, 牛市日 %d 天", len(indexData), countTrue(isBullMarket))

	// 2. 按股票遍历
	var signals []TheoreticalTrade
	total := len(cfg.TargetPool)

	for idx, code := range cfg.TargetPool {
		if (idx+1)%500 == 0 || idx+1 == total {
			log.Printf("[回测V2] 信号开采进度 %d/%d | 信号=%d 笔", idx+1, total, len(signals))
		}

		// 单股内存闭环：4次DB查询
		klines := db.GetKLinesWithAdj(code, lookbackStart, cfg.EndDate)
		if len(klines) < 130 {
			continue
		}

		funds := db.GetFundamentalsFromDB(code, lookbackStart, cfg.EndDate)
		flows := db.GetMoneyFlowFromDB(code, lookbackStart, cfg.EndDate)
		limits := db.GetStkLimitFromDB(code, lookbackStart, cfg.EndDate)

		limitMap := make(map[string]tushare.StkLimit, len(limits))
		for _, l := range limits {
			limitMap[l.TradeDate] = l
		}

		// 内层循环：模拟该股时间推移，产出理论交易
		trades := mineStockSignals(code, klines, funds, flows, limitMap, cfg, analyzers, isBullMarket)
		signals = append(signals, trades...)

		// klines/funds/flows/limits 在此作用域结束，GC 可回收
	}

	return signals
}

// mineStockSignals 对单只股票执行信号开采。
// 遍历该股的全部 K 线，在内存中模拟时间推移，产出理论交易。
func mineStockSignals(
	code string,
	klines []tushare.DailyKLine,
	funds []tushare.DailyFundamental,
	flows []tushare.DailyMoneyFlow,
	limitMap map[string]tushare.StkLimit,
	cfg BacktestConfig,
	analyzers []strategy.Analyzer,
	isBullMarket map[string]bool,
) []TheoreticalTrade {
	var trades []TheoreticalTrade
	var pendingBuy *pendingOrder
	var held bool
	var buyIdx int
	var buyStrategy, buyReason string

	// 构建 date→fund map（用于快速查找最新基本面）
	fundMap := make(map[string]tushare.DailyFundamental, len(funds))
	for _, f := range funds {
		fundMap[f.TradeDate] = f
	}
	flowMap := make(map[string]tushare.DailyMoneyFlow, len(flows))
	for _, f := range flows {
		flowMap[f.TradeDate] = f
	}

	for i := 0; i < len(klines); i++ {
		today := klines[i]
		todayDate := today.TradeDate

		if todayDate < cfg.StartDate {
			continue
		}
		if todayDate > cfg.EndDate {
			break
		}

		// 大盘风控检查
		if !isBullMarket[todayDate] {
			// 大盘不安全，取消挂单
			if pendingBuy != nil {
				pendingBuy = nil
			}
			continue
		}

		// 执行挂单买入（T+1：信号日 < 今天）
		if pendingBuy != nil && pendingBuy.SignalDate < todayDate {
			// 一字涨停检测
			if limit, ok := limitMap[todayDate]; ok {
				if isOneWordLimitUp(today, limit) {
					pendingBuy = nil
					continue
				}
			}

			buyPrice := today.Open
			targetAlloc := cfg.InitialCapital * cfg.PositionSizePct
			maxShares := int(targetAlloc / (buyPrice * (1 + cfg.Commission)))
			shares := (maxShares / 100) * 100

			if shares >= 100 {
				held = true
				buyIdx = i
				buyStrategy = pendingBuy.Strategy
				buyReason = pendingBuy.Reason
			}
			pendingBuy = nil
		}

		// 检查卖出条件
		if held {
			window := klines[:i+1]

			// 构建持仓对象用于卖出判断
			pos := &position{
				TSCode:    code,
				BuyDate:   klines[buyIdx].TradeDate,
				BuyPrice:  klines[buyIdx].Open,
				Strategy:  buyStrategy,
				BuyReason: buyReason,
			}

			if sellSignal, reason := checkSellConditions(today, window, pos, cfg, limitMap); sellSignal {
				// 收集持仓期间每日价格
				holdingPrices := make(map[string]float64, i-buyIdx+1)
				for j := buyIdx; j <= i; j++ {
					holdingPrices[klines[j].TradeDate] = klines[j].Close
				}

				trades = append(trades, TheoreticalTrade{
					Code:          code,
					BuyDate:       klines[buyIdx].TradeDate,
					BuyPrice:      klines[buyIdx].Open,
					SellDate:      todayDate,
					SellPrice:     today.Close,
					Strategy:      buyStrategy,
					BuyReason:     buyReason,
					SellReason:    reason,
					HoldingPrices: holdingPrices,
				})
				held = false
				continue
			}
		}

		// 扫描买入信号（仅当未持仓且无挂单时）
		if !held && pendingBuy == nil {
			// 快速过滤
			if today.Vol <= 0 || today.PctChg < 3.0 {
				continue
			}

			// 构建策略上下文
			ctx := buildStockContext(code, klines, funds, flows, fundMap, flowMap, i, todayDate)
			if ctx == nil || len(ctx.KLines) < 120 {
				continue
			}

			for _, analyzer := range analyzers {
				result := analyzer.Analyze(ctx)
				if containsBuySignal(result.Signal) {
					pendingBuy = &pendingOrder{
						TSCode:      code,
						SignalDate:  todayDate,
						Strategy:    analyzer.Name(),
						Reason:      result.Message,
						SignalPrice: today.Close,
					}
					break
				}
			}
		}
	}

	// 回测结束强制平仓
	if held {
		lastIdx := len(klines) - 1
		lastDate := klines[lastIdx].TradeDate
		if lastDate >= cfg.StartDate && lastDate <= cfg.EndDate {
			holdingPrices := make(map[string]float64, lastIdx-buyIdx+1)
			for j := buyIdx; j <= lastIdx; j++ {
				holdingPrices[klines[j].TradeDate] = klines[j].Close
			}

			trades = append(trades, TheoreticalTrade{
				Code:          code,
				BuyDate:       klines[buyIdx].TradeDate,
				BuyPrice:      klines[buyIdx].Open,
				SellDate:      lastDate,
				SellPrice:     klines[lastIdx].Close,
				Strategy:      buyStrategy,
				BuyReason:     buyReason,
				SellReason:    "回测结束平仓",
				HoldingPrices: holdingPrices,
			})
		}
	}

	return trades
}

// buildStockContext 构建单只股票在指定日期的策略上下文。
func buildStockContext(
	code string,
	klines []tushare.DailyKLine,
	funds []tushare.DailyFundamental,
	flows []tushare.DailyMoneyFlow,
	fundMap map[string]tushare.DailyFundamental,
	flowMap map[string]tushare.DailyMoneyFlow,
	currentIdx int,
	currentDate string,
) *strategy.SecurityContext {
	// K线窗口：到昨天为止（分析发生在收盘后）
	window := klines[:currentIdx]
	if len(window) == 0 {
		window = klines[:currentIdx+1]
	}

	// 最新基本面
	var latestFund tushare.DailyFundamental
	for i := currentIdx; i >= 0; i-- {
		if f, ok := fundMap[klines[i].TradeDate]; ok {
			latestFund = f
			break
		}
	}
	var fundsSlice []tushare.DailyFundamental
	if latestFund.TSCode != "" {
		fundsSlice = append(fundsSlice, latestFund)
	}

	// 最新资金流向
	var latestFlow tushare.DailyMoneyFlow
	for i := currentIdx; i >= 0; i-- {
		if f, ok := flowMap[klines[i].TradeDate]; ok {
			latestFlow = f
			break
		}
	}
	var flowsSlice []tushare.DailyMoneyFlow
	if latestFlow.TSCode != "" {
		flowsSlice = append(flowsSlice, latestFlow)
	}

	// PE 分位数
	pePercentile := calcPEPercentileFromFunds(funds, currentDate)

	return &strategy.SecurityContext{
		Code:         code,
		KLines:       window,
		Fundamentals: fundsSlice,
		MoneyFlows:   flowsSlice,
		PEPercentile: pePercentile,
	}
}

// calcPEPercentileFromFunds 从已加载的基本面数据中计算 PE 分位数。
func calcPEPercentileFromFunds(funds []tushare.DailyFundamental, endDate string) float64 {
	if len(funds) < 100 {
		return 0.5
	}

	var peList []float64
	var currentPE float64
	found := false

	for _, f := range funds {
		if f.TradeDate <= endDate && f.PE > 0 {
			peList = append(peList, f.PE)
			currentPE = f.PE
			found = true
		}
	}

	if !found || len(peList) < 100 {
		return 0.5
	}

	sort.Float64s(peList)

	rank := 0
	for i, pe := range peList {
		if currentPE <= pe {
			rank = i
			break
		}
	}

	return float64(rank) / float64(len(peList))
}

// ─────────────────────────────────────────────
// Phase 2: Portfolio Simulation（按日历推演）
// ─────────────────────────────────────────────

type activePosition struct {
	trade   TheoreticalTrade
	shares  int
	buyCost float64
}

func phase2PortfolioSim(cfg BacktestConfig, signals []TheoreticalTrade) (*BacktestResult, error) {
	// 1. 获取交易日历
	tradingDays := db.GetTradingDays(cfg.StartDate, cfg.EndDate)
	if len(tradingDays) == 0 {
		return nil, fmt.Errorf("回测区间 %s ~ %s 内无交易日", cfg.StartDate, cfg.EndDate)
	}
	log.Printf("[回测V2] Phase 2 | 交易日 %d 天", len(tradingDays))

	// 2. 按 BuyDate 挂载信号
	signalsByDate := make(map[string][]TheoreticalTrade, len(tradingDays))
	for _, t := range signals {
		signalsByDate[t.BuyDate] = append(signalsByDate[t.BuyDate], t)
	}

	// 3. 初始化组合状态
	cash := cfg.InitialCapital
	activePositions := make(map[string]*activePosition)
	var tradeLog []TradeRecord
	var equityCurve []EquityPoint
	peakValue := cfg.InitialCapital
	maxDrawdown := 0.0

	// 4. 按天循环日历（0次DB查询）
	for _, today := range tradingDays {
		// A. 卖出处理
		for code, ap := range activePositions {
			if ap.trade.SellDate == today {
				sellAmount := float64(ap.shares) * ap.trade.SellPrice * (1 - cfg.Commission)
				pnl := sellAmount - ap.buyCost
				tradeLog = append(tradeLog, TradeRecord{
					TSCode:     code,
					BuyDate:    ap.trade.BuyDate,
					BuyPrice:   ap.trade.BuyPrice,
					SellDate:   today,
					SellPrice:  ap.trade.SellPrice,
					Shares:     ap.shares,
					PnL:        pnl,
					ReturnPct:  (ap.trade.SellPrice - ap.trade.BuyPrice) / ap.trade.BuyPrice * 100,
					HoldDays:   countDaysBetween(tradingDays, ap.trade.BuyDate, today),
					BuyReason:  ap.trade.BuyReason,
					SellReason: ap.trade.SellReason,
					Strategy:   ap.trade.Strategy,
				})
				cash += sellAmount
				delete(activePositions, code)
			}
		}

		// B. 买入处理
		for _, signal := range signalsByDate[today] {
			if _, held := activePositions[signal.Code]; held {
				continue
			}

			targetAlloc := cfg.InitialCapital * cfg.PositionSizePct
			maxShares := int(targetAlloc / (signal.BuyPrice * (1 + cfg.Commission)))
			shares := (maxShares / 100) * 100
			if shares < 100 {
				continue
			}

			cost := float64(shares) * signal.BuyPrice * (1 + cfg.Commission)
			if cost > cash {
				continue
			}

			cash -= cost
			activePositions[signal.Code] = &activePosition{
				trade:   signal,
				shares:  shares,
				buyCost: cost,
			}
		}

		// C. 逐日盯市 (Daily MTM)
		holdingValue := 0.0
		for _, ap := range activePositions {
			if closePrice, ok := ap.trade.HoldingPrices[today]; ok {
				holdingValue += float64(ap.shares) * closePrice
			} else {
				holdingValue += ap.buyCost // fallback
			}
		}
		totalValue := cash + holdingValue
		equityCurve = append(equityCurve, EquityPoint{Date: today, Value: totalValue})

		if totalValue > peakValue {
			peakValue = totalValue
		}
		dd := (peakValue - totalValue) / peakValue * 100
		if dd > maxDrawdown {
			maxDrawdown = dd
		}
	}

	// 5. 统计汇总
	finalAssets := cash
	for _, ap := range activePositions {
		// 残余持仓按最后价格计算（不应发生，因为 Phase 1 已强制平仓）
		finalAssets += ap.buyCost
	}

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

	log.Printf("[回测V2] 全部完成 | 收益率=%.2f%% 胜率=%.1f%% 回撤=%.2f%% 交易=%d笔",
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

// ─────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────

// buildMarketRegimeMap 构建大盘宏观风控 map。
// MA60 > MA120 且指数在 MA60 上方视为牛市。
func buildMarketRegimeMap(indexData []tushare.IndexDaily) map[string]bool {
	result := make(map[string]bool, len(indexData))

	for i := 0; i < len(indexData); i++ {
		date := indexData[i].TradeDate

		// 计算 MA60
		if i < 59 {
			result[date] = true // 数据不足，默认放行
			continue
		}
		sum60 := 0.0
		for j := i - 59; j <= i; j++ {
			sum60 += indexData[j].Close
		}
		ma60 := sum60 / 60.0

		// 计算 MA120
		ma120 := 0.0
		if i >= 119 {
			sum120 := 0.0
			for j := i - 119; j <= i; j++ {
				sum120 += indexData[j].Close
			}
			ma120 = sum120 / 120.0
		}

		// 牛市判断：MA60 > MA120 且收盘价在 MA60 上方
		if ma120 > 0 {
			result[date] = indexData[i].Close > ma60 && ma60 > ma120
		} else {
			result[date] = indexData[i].Close > ma60
		}
	}

	return result
}

func countTrue(m map[string]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}

// selectAnalyzers 和 containsBuySignal 复用 engine.go 中的实现（models.go 同包）
// subtractDays、countDaysBetween、buildPortfolioSummary 同理复用
