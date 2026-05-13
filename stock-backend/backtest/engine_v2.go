package backtest

import (
	"fmt"
	"log"
	"sort"
	"stock-backend/db"
	"stock-backend/stockutil"
	"stock-backend/strategy"
	"stock-backend/tushare"
	"strings"
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

	// 全局板块过滤：仅保留主板股票（排除创业板/科创板/北交所）
	var mainBoardPool []string
	for _, code := range cfg.TargetPool {
		if stockutil.IsValidMainBoardCode(code) {
			mainBoardPool = append(mainBoardPool, code)
		}
	}
	if len(mainBoardPool) != len(cfg.TargetPool) {
		log.Printf("[回测V2] 板块过滤: %d → %d 只主板股票", len(cfg.TargetPool), len(mainBoardPool))
	}
	cfg.TargetPool = mainBoardPool

	if len(cfg.TargetPool) == 0 {
		return nil, fmt.Errorf("股票池为空，请先同步股票基础数据")
	}
	if cfg.InitialCapital <= 0 {
		cfg.InitialCapital = 100000
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

const chunkSize = 100

func phase1SignalMining(cfg BacktestConfig) []TheoreticalTrade {
	analyzers := selectAnalyzers(cfg.Strategy)
	lookbackStart := subtractDays(cfg.StartDate, 365) // 确保至少 250 个交易日的预热数据

	// 1. 宏观风控预计算（1次查询）
	indexData := db.GetIndexDailyForBacktest("000001.SH", lookbackStart, cfg.EndDate)
	isBullMarket := buildMarketRegimeMap(indexData)
	isStrongMarket := buildStrongMarketMap(indexData)
	log.Printf("[回测V2] 大盘数据 %d 天, 安全日 %d 天, 强势日 %d 天", len(indexData), countTrue(isBullMarket), countTrue(isStrongMarket))

	// 2. 分块批量加载 + 纯内存策略运算
	var signals []TheoreticalTrade
	total := len(cfg.TargetPool)
	processed := 0

	for i := 0; i < total; i += chunkSize {
		end := i + chunkSize
		if end > total {
			end = total
		}
		chunk := cfg.TargetPool[i:end]

		// 5 条批量 SQL（代替 400 条单股查询）
		klinesMap := db.BatchGetKLinesWithAdj(chunk, lookbackStart, cfg.EndDate)
		fundsMap := db.BatchGetFundamentals(chunk, lookbackStart, cfg.EndDate)
		flowsMap := db.BatchGetMoneyFlow(chunk, lookbackStart, cfg.EndDate)
		limitsMap := db.BatchGetStkLimit(chunk, lookbackStart, cfg.EndDate)
		cyqPerfMap := db.BatchGetCyqPerf(chunk, lookbackStart, cfg.EndDate)

		// 纯内存策略运算
		for _, code := range chunk {
			klines := klinesMap[code]
			if len(klines) < 130 {
				continue
			}

			funds := fundsMap[code]
			flows := flowsMap[code]
			limits := limitsMap[code]
			cyqPerfs := cyqPerfMap[code]

			limitMap := make(map[string]tushare.StkLimit, len(limits))
			for _, l := range limits {
				limitMap[l.TradeDate] = l
			}

			trades := mineStockSignals(code, klines, funds, flows, limitMap, cyqPerfs, cfg, analyzers, isBullMarket, isStrongMarket)
			signals = append(signals, trades...)
		}

		processed += len(chunk)
		log.Printf("[回测V2] 信号开采进度 %d/%d | 信号=%d 笔", processed, total, len(signals))

		// klinesMap/fundsMap/flowsMap/limitsMap 在此作用域结束，GC 可回收
	}

	return signals
}

// mineStockSignals 对单只股票执行信号开采。
// 遍历该股的全部 K 线，在内存中模拟完整交易闭环：
// 检测买入 → 持仓跟踪(EvaluateHold) → 策略动态止盈止损 → 产出已闭环交易。
// Phase 2 不再做任何卖出判断，仅执行资金记账。
func mineStockSignals(
	code string,
	klines []tushare.DailyKLine,
	funds []tushare.DailyFundamental,
	flows []tushare.DailyMoneyFlow,
	limitMap map[string]tushare.StkLimit,
	cyqPerfs []tushare.CyqPerf,
	cfg BacktestConfig,
	analyzers []strategy.Analyzer,
	isBullMarket map[string]bool,
	isStrongMarket map[string]bool,
) []TheoreticalTrade {
	var trades []TheoreticalTrade
	var pending *pendingSignal // T+1 挂单
	var hold *holdState        // 当前持仓

	// 构建 date→fund/flow/cyq map（用于快速查找最新基本面/资金流/筹码分布）
	fundMap := make(map[string]tushare.DailyFundamental, len(funds))
	for _, f := range funds {
		fundMap[f.TradeDate] = f
	}
	flowMap := make(map[string]tushare.DailyMoneyFlow, len(flows))
	for _, f := range flows {
		flowMap[f.TradeDate] = f
	}
	cyqMap := make(map[string]tushare.CyqPerf, len(cyqPerfs))
	for _, c := range cyqPerfs {
		cyqMap[c.TradeDate] = c
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

		// ── A. 执行挂单买入（T+1：信号日 < 今天）──
		if pending != nil && pending.signalDate < todayDate {
			// 一字涨停检测：买不进
			if limit, ok := limitMap[todayDate]; ok {
				if isOneWordLimitUp(today, limit) {
					pending = nil
					continue
				}
			}

			buyPrice := today.Open
			targetAlloc := cfg.InitialCapital * cfg.PositionSizePct
			maxShares := int(targetAlloc / (buyPrice * (1 + cfg.Commission)))
			shares := (maxShares / 100) * 100

			if shares >= 100 {
				hold = &holdState{
					buyDate:   todayDate,
					buyIdx:    i,
					buyPrice:  buyPrice,
					strategy:  pending.strategy,
					reason:    pending.reason,
					buyResult: pending.buyResult,
					meta:      pending.meta,
				}
			}
			pending = nil
		}

		// ── B. 持仓评估：两阶段动态止损 + 策略动态止盈止损 ──
		if hold != nil {
			// 0. T+1 保护约束：买入当天不允许任何卖出评估
			if i == hold.buyIdx {
				continue
			}

			// 更新高水位
			if today.Close > hold.highWatermark {
				hold.highWatermark = today.Close
			}

			// B1. Armed 状态最高优先级
			if hold.trailingStopArmed {
				if today.High > today.Low && today.Vol > 0 {
					reason := fmt.Sprintf("跌停打开，集合竞价出逃（开盘价 %.2f）", today.Open)
					trades = append(trades, buildClosedTrade(code, hold, i, today.Open, reason, klines))
					hold = nil
					continue // 【核心修复】：必须跳过本日
				}
				continue // 一字跌停，继续武装，跳过本日
			}

			// 计算最大浮盈比例，决定是否激活阶段B
			maxGainPct := (hold.highWatermark - hold.buyPrice) / hold.buyPrice * 100
			if maxGainPct >= 15.0 {
				hold.stageBActive = true
			}

			holdingHistory := klines[hold.buyIdx : i+1]

			if hold.stageBActive {
				// 阶段B：利润锁定期，启用12%高水位追踪止损
				if strategy.IsTrailingStopTriggered(today, holdingHistory, 0.12) {
					if today.High == today.Low || today.Vol == 0 {
						hold.trailingStopArmed = true
						continue // 跌停锁死，等明天
					}
					if ok, sellPrice, reason := strategy.CheckTrailingStop(today, holdingHistory, 0.12); ok {
						trades = append(trades, buildClosedTrade(code, hold, i, sellPrice, reason, klines))
						hold = nil
						continue // 【核心修复】：必须跳过本日
					}
				}
			} else {
				// 阶段A：利润缓冲期，仅执行-8%绝对硬止损
				if ok, sellPrice, reason := strategy.CheckHardStop(today, hold.buyPrice, 0.08); ok {
					trades = append(trades, buildClosedTrade(code, hold, i, sellPrice, reason, klines))
					hold = nil
					continue
				}
			}

			// B3. 常规策略 EvaluateHold
			// 执行到这里，hold 绝对不可能为 nil
			fullHistory := klines[:i+1]
			for _, analyzer := range analyzers {
				if analyzer.Name() == hold.strategy {
					pos := &strategy.Position{
						Code:      code,
						BuyDate:   hold.buyDate,
						BuyPrice:  hold.buyPrice,
						Strategy:  hold.strategy,
						BuyResult: hold.buyResult,
					}
					eval := analyzer.EvaluateHold(pos, today, fullHistory, hold.meta)
					if eval.Sell {
						trades = append(trades, buildClosedTrade(code, hold, i, eval.Price, eval.Reason, klines))
						hold = nil
					}
					break // 退出 analyzers 循环
				}
			}

			// 【防同日再入隔离】如果在 B3 卖出了，hold 变为空，必须跳过本日的 Section C (买入扫描)
			if hold == nil {
				continue
			}
		}

		// ── C. 扫描买入信号（仅当未持仓、无挂单、且大盘安全时）──
		if hold == nil && pending == nil && isBullMarket[todayDate] {
			// 停牌过滤（零成交 = 停牌，无法买入）
			if today.Vol <= 0 {
				continue
			}

			// 构建策略上下文
			ctx := buildStockContext(code, klines, funds, flows, fundMap, flowMap, cyqMap, i, todayDate)
			if ctx == nil || len(ctx.KLines) < 120 {
				continue
			}

			// 弱势环境下仅允许左侧策略 (DSS)，屏蔽右侧突破策略 (MACB/CBBM)
			eligible := analyzers
			if !isStrongMarket[todayDate] {
				eligible = eligible[:0]
				for _, a := range analyzers {
					if strings.Contains(a.Name(), "DSS") {
						eligible = append(eligible, a)
					}
				}
				if len(eligible) == 0 {
					continue
				}
			}

			for _, analyzer := range eligible {
				result := analyzer.Analyze(ctx)
				if containsBuySignal(result.Signal) {
					pending = &pendingSignal{
						code:       code,
						signalDate: todayDate,
						strategy:   analyzer.Name(),
						reason:     result.Message,
						buyResult:  result,
						meta:       buildStrategyMeta(analyzer.Name(), result, klines, i),
					}
					break
				}
			}
		}
	}

	// ── D. 回测结束强制平仓 ──
	if hold != nil {
		lastIdx := len(klines) - 1
		lastDate := klines[lastIdx].TradeDate
		if lastDate >= cfg.StartDate && lastDate <= cfg.EndDate {
			trades = append(trades, buildClosedTrade(code, hold, lastIdx, klines[lastIdx].Close, "回测结束平仓", klines))
		}
	}

	return trades
}

// buildClosedTrade 构建一笔已闭环的完整交易。
func buildClosedTrade(code string, hold *holdState, sellIdx int, sellPrice float64, sellReason string, klines []tushare.DailyKLine) TheoreticalTrade {
	holdingPrices := make(map[string]float64, sellIdx-hold.buyIdx+1)
	for j := hold.buyIdx; j <= sellIdx; j++ {
		holdingPrices[klines[j].TradeDate] = klines[j].Close
	}
	return TheoreticalTrade{
		Code:          code,
		BuyDate:       hold.buyDate,
		BuyPrice:      hold.buyPrice,
		SellDate:      klines[sellIdx].TradeDate,
		SellPrice:     sellPrice,
		Strategy:      hold.strategy,
		BuyReason:     hold.reason,
		SellReason:    sellReason,
		HoldingPrices: holdingPrices,
	}
}

// buildStrategyMeta 为 EvaluateHold 构建策略所需的元数据。
func buildStrategyMeta(strategyName string, buyResult strategy.DiagnoseResult, klines []tushare.DailyKLine, signalIdx int) map[string]float64 {
	meta := make(map[string]float64)

	switch {
	case strings.Contains(strategyName, "CBBM"):
		// CBBM 需要 boxUpper 用于动态止损计算
		if signalIdx >= 60 {
			boxUpper, _ := strategy.GetRealBox(klines[:signalIdx+1], 60)
			meta["box_upper"] = boxUpper
		}

	case strings.Contains(strategyName, "DSS"):
		// DSS 需要 boxUpper, boxLower, atr14
		if signalIdx >= 60 {
			boxUpper, boxLower := strategy.GetRealBox(klines[:signalIdx+1], 60)
			meta["box_upper"] = boxUpper
			meta["box_lower"] = boxLower
			meta["atr14"] = strategy.CalcATR(klines[:signalIdx+1], 14)
		}
	}
	// MACB 无需额外 meta，其 EvaluateHold 仅依赖动态计算的 MA
	return meta
}

// buildStockContext 构建单只股票在指定日期的策略上下文。
func buildStockContext(
	code string,
	klines []tushare.DailyKLine,
	funds []tushare.DailyFundamental,
	flows []tushare.DailyMoneyFlow,
	fundMap map[string]tushare.DailyFundamental,
	flowMap map[string]tushare.DailyMoneyFlow,
	cyqMap map[string]tushare.CyqPerf,
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

	// 筹码分布（当日数据）
	var cyqPerf *tushare.CyqPerf
	if c, ok := cyqMap[currentDate]; ok {
		cyqPerf = &c
	}

	// PE 分位数
	pePercentile := calcPEPercentileFromFunds(funds, currentDate)

	return &strategy.SecurityContext{
		Code:         code,
		KLines:       window,
		Fundamentals: fundsSlice,
		MoneyFlows:   flowsSlice,
		PEPercentile: pePercentile,
		CyqPerf:      cyqPerf,
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
// 对齐实盘 CheckMarketEnvironment 的规则 1（暴跌风控）和规则 2（趋势风控）。
// true = 允许开新仓，false = 屏蔽新买入信号（但不强制平仓）。
func buildMarketRegimeMap(indexData []tushare.IndexDaily) map[string]bool {
	result := make(map[string]bool, len(indexData))

	for i := 0; i < len(indexData); i++ {
		date := indexData[i].TradeDate

		// 数据不足，默认放行
		if i < 20 {
			result[date] = true
			continue
		}

		// 规则 1: 暴跌风控 — 大盘单日跌幅 >= 1.5%
		if indexData[i].PctChg <= -1.5 {
			result[date] = false
			continue
		}

		// 规则 2: 趋势风控 — 收盘 < MA20 且 MA20 拐头向下
		sum20 := 0.0
		for j := i - 19; j <= i; j++ {
			sum20 += indexData[j].Close
		}
		ma20 := sum20 / 20.0

		sumPrev20 := 0.0
		for j := i - 20; j <= i-1; j++ {
			sumPrev20 += indexData[j].Close
		}
		prevMa20 := sumPrev20 / 20.0

		if indexData[i].Close < ma20 && ma20 < prevMa20 {
			result[date] = false
			continue
		}

		result[date] = true
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

// buildStrongMarketMap 构建大盘强弱 map。
// 对齐实盘 CheckMarketEnvironment 的规则 4：上证 Close > MA60 为强势市场。
// true = 强势市场（允许右侧突破策略 MACB/CBBM），false = 弱势市场（仅允许左侧策略 DSS）。
func buildStrongMarketMap(indexData []tushare.IndexDaily) map[string]bool {
	result := make(map[string]bool, len(indexData))

	for i := 0; i < len(indexData); i++ {
		date := indexData[i].TradeDate

		// 数据不足 60 天，默认视为强势（不阻拦）
		if i < 59 {
			result[date] = true
			continue
		}

		// 计算 MA60
		sum60 := 0.0
		for j := i - 59; j <= i; j++ {
			sum60 += indexData[j].Close
		}
		ma60 := sum60 / 60.0

		// 强势 = 收盘价在 MA60 之上
		result[date] = indexData[i].Close >= ma60
	}

	return result
}

// selectAnalyzers 和 containsBuySignal 复用 engine.go 中的实现（models.go 同包）
// subtractDays、countDaysBetween、buildPortfolioSummary 同理复用
