package backtest

import (
	"context"
	"fmt"
	"log"
	"sort"
	"stock-backend/db"
	"stock-backend/stockutil"
	"stock-backend/sysmon"
	"stock-backend/strategy"
	"stock-backend/tushare"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RunV2 执行两阶段解耦回测。
// Phase 1: 按股票遍历，每只股票仅 4 次 DB 查询，产出理论交易信号。
// Phase 2: 按日历推演，0 次 DB 查询，纯内存计算逐日精确净值。
func RunV2(ctx context.Context, cfg BacktestConfig) (*BacktestResult, error) {
	sysmon.Init()
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

	// ── 读取引擎子策略配置 ──
	engineCfg, err := db.GetEngineConfig()
	if err != nil {
		log.Printf("[回测V2] 引擎配置读取失败，使用默认全开: %v", err)
		engineCfg = db.EngineConfig{EnableRegimeRouter: true, EnableSignalAllocator: true, Enable3DExit: true}
	}
	log.Printf("[回测V2] 引擎子策略 | 大盘路由=%v 资金裁决=%v 3D退出=%v",
		engineCfg.EnableRegimeRouter, engineCfg.EnableSignalAllocator, engineCfg.Enable3DExit)

	// ── Phase 1: Signal Mining ──
	tPhase1 := time.Now()
	signals := phase1SignalMining(ctx, cfg, engineCfg)
	log.Printf("[回测V2] Phase 1 完成 | 信号=%d 笔 | 耗时 %s", len(signals), time.Since(tPhase1).Round(time.Millisecond))
	strategy.PbmaProbeReport() // 死因探针：输出 PBMA 各步骤淘汰统计

	// ── Phase 2: Portfolio Simulation ──
	tPhase2 := time.Now()
	result, err := phase2PortfolioSim(ctx, cfg, signals, engineCfg)
	if err != nil {
		return nil, err
	}
	log.Printf("[回测V2] Phase 2 完成 | 耗时 %s", time.Since(tPhase2).Round(time.Millisecond))

	return result, nil
}

// ─────────────────────────────────────────────
// Phase 1: Signal Mining（按股票遍历）
// ─────────────────────────────────────────────

// MaxSignalsPerTask 单任务信号上限，防止 OOM（Termux 安全阈值）。
const MaxSignalsPerTask = 100000

// MaxGroupSignals 计算组级信号容量上限。
// 任务越多，每个任务分到的配额越少，防止多任务聚合导致 OOM。
func MaxGroupSignals(taskCount int) int {
	if taskCount <= 1 {
		return MaxSignalsPerTask
	}
	// 组级上限 = 200000，但每个任务至少 20000
	perTask := MaxSignalsPerTask / taskCount
	if perTask < 20000 {
		perTask = 20000
	}
	return perTask * taskCount
}

// ChunkData 一个 chunk 的批量查询结果（用完即弃，控制内存峰值）。
type ChunkData struct {
	Codes      []string
	KLinesMap  map[string][]tushare.DailyKLine
	FundsMap   map[string][]tushare.DailyFundamental
	FlowsMap   map[string][]tushare.DailyMoneyFlow
	LimitsMap  map[string][]tushare.StkLimit
	CyqPerfMap map[string][]tushare.CyqPerf
	FinaMap    map[string][]tushare.FinaIndicator
}

// LoadChunk 从 DB 加载单个 chunk 的全部数据（6条SQL）。
func LoadChunk(codes []string, lookbackStart, endDate string) ChunkData {
	return ChunkData{
		Codes:      codes,
		KLinesMap:  db.BatchGetKLinesWithAdj(codes, lookbackStart, endDate),
		FundsMap:   db.BatchGetFundamentals(codes, lookbackStart, endDate),
		FlowsMap:   db.BatchGetMoneyFlow(codes, lookbackStart, endDate),
		LimitsMap:  db.BatchGetStkLimit(codes, lookbackStart, endDate),
		CyqPerfMap: db.BatchGetCyqPerf(codes, lookbackStart, endDate),
		FinaMap:    db.BatchGetFinaIndicators(codes, lookbackStart, endDate),
	}
}

// MineChunkSignals 对单个 chunk 的数据执行信号开采（纯内存，无DB查询）。
func MineChunkSignals(
	cfg BacktestConfig,
	chunk ChunkData,
	analyzers []strategy.Analyzer,
	isBullMarket map[string]bool,
	isStrongMarket map[string]bool,
	engineCfg db.EngineConfig,
) []TheoreticalTrade {
	var signals []TheoreticalTrade
	for _, code := range chunk.Codes {
		klines := chunk.KLinesMap[code]
		if len(klines) < 30 { // P1: 降低门槛，让 PBMA(需~30根) 通过；MACB/CBBM 由自身内部检查过滤
			continue
		}
		funds := chunk.FundsMap[code]
		flows := chunk.FlowsMap[code]
		limits := chunk.LimitsMap[code]
		cyqPerfs := chunk.CyqPerfMap[code]

		limitMap := make(map[string]tushare.StkLimit, len(limits))
		for _, l := range limits {
			limitMap[l.TradeDate] = l
		}

		trades := mineStockSignals(code, klines, funds, flows, limitMap, cyqPerfs, chunk.FinaMap[code], cfg, analyzers, isBullMarket, isStrongMarket, engineCfg)
		signals = append(signals, trades...)
	}
	return signals
}

func phase1SignalMining(ctx context.Context, cfg BacktestConfig, engineCfg db.EngineConfig) []TheoreticalTrade {
	chunkSize := sysmon.GetChunkSize()
	maxWorkers := sysmon.GetMaxWorkers()
	analyzers := SelectAnalyzers(cfg.Strategy)
	lookbackStart := SubtractDays(cfg.StartDate, 365)

	// 1. 宏观风控预计算（1次查询）
	indexData := db.GetIndexDailyForBacktest("000001.SH", lookbackStart, cfg.EndDate)
	isBullMarket := BuildMarketRegimeMap(indexData)
	isStrongMarket := BuildStrongMarketMap(indexData)
	log.Printf("[回测V2] 大盘数据 %d 天, 安全日 %d 天, 强势日 %d 天 | Workers=%d", len(indexData), countTrue(isBullMarket), countTrue(isStrongMarket), maxWorkers)

	// 2. 并发分块加载 + 纯内存策略运算
	var signals []TheoreticalTrade
	var mu sync.Mutex // 保护 signals 切片
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	total := len(cfg.TargetPool)
	var processed int64

	for i := 0; i < total; i += chunkSize {
		// P2: 优雅退出检查（调度前）
		select {
		case <-ctx.Done():
			log.Printf("[回测V2] 收到取消指令，停止 Phase 1 调度 (已分发 %d/%d)", i, total)
			wg.Wait()
			return signals
		default:
		}

		// P2: 内存背压（调度前）
		if sysmon.CheckMemoryBackpressure() {
			time.Sleep(1 * time.Second)
		}

		end := i + chunkSize
		if end > total {
			end = total
		}
		codes := cfg.TargetPool[i:end]

		wg.Add(1)
		sem <- struct{}{} // 获取令牌（满时阻塞）

		go func(codes []string) {
			defer wg.Done()
			defer func() { <-sem }() // 释放令牌

			chunk := LoadChunk(codes, lookbackStart, cfg.EndDate)
			chunkSignals := MineChunkSignals(cfg, chunk, analyzers, isBullMarket, isStrongMarket, engineCfg)

			mu.Lock()
			// P1: 信号上限防 OOM
			if len(signals)+len(chunkSignals) > MaxSignalsPerTask {
				log.Printf("[回测V2] 信号数已达上限 %d，丢弃本 chunk 信号", MaxSignalsPerTask)
				mu.Unlock()
			} else {
				signals = append(signals, chunkSignals...)
				newTotal := len(signals)
				mu.Unlock()
				n := int(atomic.AddInt64(&processed, int64(len(codes))))
				log.Printf("[回测V2] 信号开采进度 %d/%d | 信号=%d 笔", n, total, newTotal)
			}
		}(codes)
	}

	wg.Wait()

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
	finas []tushare.FinaIndicator,
	cfg BacktestConfig,
	analyzers []strategy.Analyzer,
	isBullMarket map[string]bool,
	isStrongMarket map[string]bool,
	engineCfg db.EngineConfig,
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

			buyPrice := today.Open * (1 + SlippageRate) // 买入滑点
			targetAlloc := cfg.InitialCapital * cfg.PositionSizePct
			maxShares := int(targetAlloc / (buyPrice * (1 + cfg.Commission)))
			shares := (maxShares / 100) * 100

			if shares >= 100 {
				// 计算买入时的基础 ATR（用于自适应止损）
				buyATR := 0.0
				if i >= 14 {
					buyATR = strategy.CalcATR(klines[:i+1], 14)
				}

				hold = &holdState{
					buyDate:   todayDate,
					buyIdx:    i,
					buyPrice:  buyPrice,
					strategy:  pending.strategy,
					reason:    pending.reason,
					buyResult: pending.buyResult,
					meta:      pending.meta,
					buyATR:    buyATR,
					peakClose: buyPrice,
					daysHeld:  0,
					halfSold:  false,
					totalQty:  shares,
					score:     pending.score,
				}
			}
			pending = nil
		}

		// ── B. 持仓评估：3D退出状态机 + 策略动态止盈止损 ──
		if hold != nil {
			// 0. T+1 保护约束：买入当天不允许任何卖出评估
			if i == hold.buyIdx {
				continue
			}

			// 每日持仓递增
			hold.daysHeld++

			// 更新高水位（两套：highWatermark 用于 Stage B，peakClose 用于 3D 退出）
			if today.Close > hold.highWatermark {
				hold.highWatermark = today.Close
			}
			if today.Close > hold.peakClose {
				hold.peakClose = today.Close
			}

			// B1. Armed 状态最高优先级（跌停出逃）
			if hold.trailingStopArmed {
				if today.High > today.Low && today.Vol > 0 {
					reason := fmt.Sprintf("跌停打开，集合竞价出逃（开盘价 %.2f）", today.Open)
					trades = append(trades, buildClosedTrade(code, hold, i, today.Open, reason, klines))
					hold = nil
					continue
				}
				continue
			}

			// B2. 退出逻辑分支：Enable3DExit 开关实现新旧逻辑绝对隔离
			if engineCfg.Enable3DExit {
				// ── 新版 3D 退出状态机（唯一退出判定者）──
				sell, reason, partial := eval3DExit(today, hold, hold.strategy, klines[:i+1], engineCfg.ExitProfile)
				if sell {
					// 跌停封死检测：基于前复权价格的真实跌幅判定（避免未复权 DownLimit 跨维度对比）
					isLimitDown := false
					if i > 0 {
						prevClose := klines[i-1].Close
						if prevClose > 0 {
							pctChg := (today.Close - prevClose) / prevClose
							// 跌幅 >= 9.5% 且收盘价等于最低价，认定为实质性跌停无法卖出
							if pctChg <= -0.095 && today.Close == today.Low {
								isLimitDown = true
							}
							// ST 股窄幅跌停兜底：单日振幅为零且跌幅达到 4.8%
							if !isLimitDown && pctChg <= -0.048 && today.High == today.Low {
								isLimitDown = true
							}
						}
					}
					if isLimitDown {
						hold.trailingStopArmed = true
						continue
					}
					if partial {
						qty := (hold.totalQty / 200) * 100 // 取整到100股一手
						if qty < 100 {
							qty = 100
						}
						hold.partialSells = append(hold.partialSells, PartialSellEvent{
							SellDate:  todayDate,
							SellPrice: today.Close,
							SellQty:   qty,
							Reason:    reason,
						})
						hold.totalQty -= qty
						hold.halfSold = true
						hold.reason = reason
						continue
					}
					trades = append(trades, buildClosedTrade(code, hold, i, today.Close, reason, klines))
					hold = nil
					continue
				}
				// eval3DExit 决定不卖出 → 直接跳到下一天，绝不触碰旧版逻辑
				continue
			}

			// ── 旧版历史兜底逻辑（Enable3DExit=false 时生效）──
			// B3. 两阶段止损（Stage A: -10% 硬止损 / Stage B: 追踪止损）
			holdingHistory := klines[hold.buyIdx : i+1]
			if !hold.halfSold {
				maxGainPct := (hold.highWatermark - hold.buyPrice) / hold.buyPrice * 100
				if maxGainPct >= 15.0 {
					hold.stageBActive = true
				}
				if hold.stageBActive {
					if strategy.IsTrailingStopTriggered(today, holdingHistory, hold.buyATR, 2.5, 0.12) {
						if today.High == today.Low || today.Vol == 0 {
							hold.trailingStopArmed = true
							continue
						}
						if ok, sellPrice, trailReason := strategy.CheckTrailingStop(today, holdingHistory, hold.buyATR, 2.5, 0.12); ok {
							trades = append(trades, buildClosedTrade(code, hold, i, sellPrice, trailReason, klines))
							hold = nil
							continue
						}
					}
				} else {
					// Stage A：宽幅护底期，执行 -10% 硬止损
					if ok, sellPrice, reason := strategy.CheckHardStop(today, hold.buyPrice, 0.10); ok {
						trades = append(trades, buildClosedTrade(code, hold, i, sellPrice, reason, klines))
						hold = nil
						continue
					}
				}
			}

			// B4. 常规策略 EvaluateHold（旧版模式下的策略级退出）
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
					break
				}
			}

			// 【防同日再入隔离】
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
			ctx := BuildStockContext(code, klines, funds, flows, fundMap, flowMap, cyqMap, finas, i, todayDate)
			if ctx == nil || len(ctx.KLines) < 30 { // P1: 降低门槛，各策略内部自行检查所需最小长度
				continue
			}

			// 弱势环境下仅允许左侧策略（缩量回踩/深海动量），屏蔽右侧突破策略
			eligible := analyzers
			if engineCfg.EnableRegimeRouter && !isStrongMarket[todayDate] {
				eligible = make([]strategy.Analyzer, 0, len(analyzers))
				for _, a := range analyzers {
					if a.MarketTag() == "left" {
						eligible = append(eligible, a)
					}
				}
				if len(eligible) == 0 {
					continue
				}
			}

			for _, analyzer := range eligible {
				result := analyzer.Analyze(ctx)
				if ContainsBuySignal(result.Signal) {
					pending = &pendingSignal{
						code:       code,
						signalDate: todayDate,
						strategy:   analyzer.Name(),
						reason:     result.Message,
						buyResult:  result,
						meta:       buildStrategyMeta(analyzer.Name(), result, klines, i),
						score:      signalScore(analyzer.Name(), klines, i),
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
	sellPrice *= (1 - SlippageRate) // 卖出滑点
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
		Score:         hold.score,
		PartialSells:  hold.partialSells,
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

	case strings.Contains(strategyName, "PBMA"):
		// PBMA 需要 buyATR 用于两阶段 ATR 动态止损
		if signalIdx >= 14 {
			meta["buy_atr"] = strategy.CalcATR(klines[:signalIdx+1], 14)
		}
	}
	// MACB 无需额外 meta，其 EvaluateHold 仅依赖动态计算的 MA
	return meta
}

// BuildStockContext 构建单只股票在指定日期的策略上下文。
func BuildStockContext(
	code string,
	klines []tushare.DailyKLine,
	funds []tushare.DailyFundamental,
	flows []tushare.DailyMoneyFlow,
	fundMap map[string]tushare.DailyFundamental,
	flowMap map[string]tushare.DailyMoneyFlow,
	cyqMap map[string]tushare.CyqPerf,
	finas []tushare.FinaIndicator,
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

	// 最新季报财务指标（反向线性扫描，兼容周末公告日期）
	var latestFina *tushare.FinaIndicator
	for j := len(finas) - 1; j >= 0; j-- {
		if finas[j].AnnDate <= currentDate {
			latestFina = &finas[j]
			break
		}
	}

	// PE 分位数
	pePercentile := CalcPEPercentileFromFunds(funds, currentDate)

	return &strategy.SecurityContext{
		Code:         code,
		KLines:       window,
		Fundamentals: fundsSlice,
		MoneyFlows:   flowsSlice,
		PEPercentile: pePercentile,
		CyqPerf:      cyqPerf,
		LatestFina:   latestFina,
	}
}

// CalcPEPercentileFromFunds 从已加载的基本面数据中计算 PE 分位数。
func CalcPEPercentileFromFunds(funds []tushare.DailyFundamental, endDate string) float64 {
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

func phase2PortfolioSim(ctx context.Context, cfg BacktestConfig, signals []TheoreticalTrade, engineCfg db.EngineConfig) (*BacktestResult, error) {
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
	var dailyEquity []DailyEquity
	peakValue := cfg.InitialCapital
	maxDrawdown := 0.0

	// 4. 按天循环日历（0次DB查询）
	for dayIdx, today := range tradingDays {
		// P2: 优雅退出 + 内存背压（每10天检查一次）
		if dayIdx%10 == 0 {
			select {
			case <-ctx.Done():
				log.Printf("[回测V2] Phase 2 收到取消指令，终止于 %s", today)
				return nil, ctx.Err()
			default:
			}
			if sysmon.CheckMemoryBackpressure() {
				time.Sleep(1 * time.Second)
			}
		}

		// A. 卖出处理（含分批减仓）
		for code, ap := range activePositions {
			// A1. 检查分批减仓事件（PartialSells）
			if len(ap.trade.PartialSells) > 0 && ap.trade.SellDate != today {
				for _, ps := range ap.trade.PartialSells {
					if ps.SellDate == today {
						sellQty := ps.SellQty
						if sellQty > ap.shares {
							sellQty = ap.shares
						}
						sellAmount := float64(sellQty) * ps.SellPrice * (1 - cfg.Commission)
						pnl := sellAmount - float64(sellQty)*ap.trade.BuyPrice*(1+cfg.Commission)
						tradeLog = append(tradeLog, TradeRecord{
							TSCode:     code,
							BuyDate:    ap.trade.BuyDate,
							BuyPrice:   ap.trade.BuyPrice,
							SellDate:   today,
							SellPrice:  ps.SellPrice,
							Shares:     sellQty,
							PnL:        pnl,
							ReturnPct:  (ps.SellPrice - ap.trade.BuyPrice) / ap.trade.BuyPrice * 100,
							HoldDays:   countDaysBetween(tradingDays, ap.trade.BuyDate, today),
							BuyReason:  ap.trade.BuyReason,
							SellReason: ps.Reason,
							Strategy:   ap.trade.Strategy,
							Score:      ap.trade.Score,
						})
						cash += sellAmount
						ap.shares -= sellQty
						ap.buyCost -= float64(sellQty) * ap.trade.BuyPrice * (1 + cfg.Commission)
					}
				}
			}

			// A2. 最终清仓（到达 ExitDate）
			if ap.trade.SellDate == today {
				sellAmount := float64(ap.shares) * ap.trade.SellPrice * (1 - cfg.Commission)
				pnl := sellAmount - ap.buyCost
				tradeLog = append(tradeLog, TradeRecord{
					TSCode:       code,
					BuyDate:      ap.trade.BuyDate,
					BuyPrice:     ap.trade.BuyPrice,
					SellDate:     today,
					SellPrice:    ap.trade.SellPrice,
					Shares:       ap.shares,
					PnL:          pnl,
					ReturnPct:    (ap.trade.SellPrice - ap.trade.BuyPrice) / ap.trade.BuyPrice * 100,
					HoldDays:     countDaysBetween(tradingDays, ap.trade.BuyDate, today),
					BuyReason:    ap.trade.BuyReason,
					SellReason:   ap.trade.SellReason,
					Strategy:     ap.trade.Strategy,
					Score:        ap.trade.Score,
					PartialSells: ap.trade.PartialSells,
				})
				cash += sellAmount
				delete(activePositions, code)
			}
		}

		// B. 计算当前总资产（用于动态头寸 + 逐日盯市）
		holdingValue := 0.0
		for _, ap := range activePositions {
			if closePrice, ok := ap.trade.HoldingPrices[today]; ok {
				holdingValue += float64(ap.shares) * closePrice
			} else {
				holdingValue += ap.buyCost // fallback
			}
		}
		totalEquity := cash + holdingValue

		// C. 买入处理（动态头寸调度 + 按 score 降序优先占用资金）
		todaySignals := signalsByDate[today]
		if engineCfg.EnableSignalAllocator {
			sort.SliceStable(todaySignals, func(i, j int) bool {
				return todaySignals[i].Score > todaySignals[j].Score
			})
		}
		for _, signal := range todaySignals {
			if _, held := activePositions[signal.Code]; held {
				continue
			}

			shares := CalculatePositionShares(
				totalEquity, cash, signal.BuyPrice, cfg.Commission,
				signal.Strategy, signal.Score, cfg.PositionSizePct,
			)
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

		// D. 逐日盯市 (Daily MTM)
		totalValue := cash + holdingValue
		dailyEquity = append(dailyEquity, DailyEquity{Date: today, Equity: totalValue, Cash: cash})

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

	// 构建 equityCurve（向后兼容）
	equityCurve := make([]EquityPoint, len(dailyEquity))
	for i, de := range dailyEquity {
		equityCurve[i] = EquityPoint{Date: de.Date, Value: de.Equity}
	}

	// 计算专业汇总指标并打印报告
	summary := CalcSummary(cfg, finalAssets, tradeLog, dailyEquity)
	PrintSummaryReport(summary)

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
		Metrics:     summary,
	}, nil
}

// ─────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────

// buildMarketRegimeMap 构建大盘宏观风控 map。
// 对齐实盘 CheckMarketEnvironment 的规则 1（暴跌风控）和规则 2（趋势风控）。
// true = 允许开新仓，false = 屏蔽新买入信号（但不强制平仓）。
func BuildMarketRegimeMap(indexData []tushare.IndexDaily) map[string]bool {
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
// 对齐实盘 CheckMarketEnvironment：Close >= MA60 即为强势。
// true = 强势市场（允许右侧突破策略 MACB/CBBM），false = 弱势市场（仅允许左侧策略 DSS）。
func BuildStrongMarketMap(indexData []tushare.IndexDaily) map[string]bool {
	n := len(indexData)
	result := make(map[string]bool, n)

	// 预计算 MA60 数组
	ma60Arr := make([]float64, n)
	for i := 0; i < n; i++ {
		if i >= 59 {
			sum := 0.0
			for j := i - 59; j <= i; j++ {
				sum += indexData[j].Close
			}
			ma60Arr[i] = sum / 60.0
		}
	}

	for i := 0; i < n; i++ {
		date := indexData[i].TradeDate

		// 数据不足 60 天，默认视为强势（不阻拦）
		if i < 59 {
			result[date] = true
			continue
		}

		result[date] = indexData[i].Close >= ma60Arr[i]
	}

	return result
}

// SelectAnalyzers 和 ContainsBuySignal 已导出，供 signallab 包复用。
// SubtractDays、countDaysBetween、buildPortfolioSummary 同理。
