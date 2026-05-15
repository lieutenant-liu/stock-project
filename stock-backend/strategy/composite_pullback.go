package strategy

import (
	"fmt"
	"math"
	"stock-backend/tushare"
)

// =====================================================
// 复合回踩引擎 (Composite Pullback Engine)
// 捕捉"突破后缩量回踩支撑"的二次买入机会。
// 回踩端强制锁死 EXP 级别过滤器。
// =====================================================

// PullbackAnchor 锚点：记录突破日的位置、量能与支撑线。
type PullbackAnchor struct {
	Index       int     // 锚点在 KLines 中的索引
	Volume      float64 // 锚点日成交量
	SupportLine float64 // 支撑线价位
}

// AnchorFinder 锚点扫描器接口。
type AnchorFinder interface {
	FindAnchor(ctx *SecurityContext) *PullbackAnchor
}

// reject 快速生成观望结果。
func reject(msg string) DiagnoseResult {
	return DiagnoseResult{Signal: "观望 💤", Message: msg}
}

// ─── 通用锚点扫描器 ───
// 依赖 Analyzer 接口，可同时接受原版与实验版策略包装器。

type anchorFinderImpl struct {
	analyzer    Analyzer
	minKLines   int                         // 调用 analyzer 所需的最少 K 线数
	supportFunc func(klines []tushare.DailyKLine, idx int) float64 // 从锚点上下文计算支撑线
}

func (f *anchorFinderImpl) FindAnchor(ctx *SecurityContext) *PullbackAnchor {
	n := len(ctx.KLines)
	scanStart := n - 1 - 20
	if scanStart < f.minKLines {
		scanStart = f.minKLines
	}
	for i := scanStart; i >= n-1-5; i-- {
		sub := &SecurityContext{
			Code:         ctx.Code,
			KLines:       ctx.KLines[:i+1],
			Fundamentals: ctx.Fundamentals,
			MoneyFlows:   ctx.MoneyFlows,
			PEPercentile: ctx.PEPercentile,
			CyqPerf:      ctx.CyqPerf,
		}
		if len(sub.KLines) < f.minKLines {
			continue
		}
		r := f.analyzer.Analyze(sub)
		if r.Signal == "买入 🚀" {
			support := f.supportFunc(sub.KLines, i)
			return &PullbackAnchor{Index: i, Volume: sub.KLines[i].Vol, SupportLine: support}
		}
	}
	return nil
}

// MACBAnchorFinder 均线突破锚点：支撑线 = max(MA30, MA60, MA120)。
type MACBAnchorFinder struct{ baseAnalyzer Analyzer }

func (f *MACBAnchorFinder) FindAnchor(ctx *SecurityContext) *PullbackAnchor {
	impl := &anchorFinderImpl{
		analyzer:  f.baseAnalyzer,
		minKLines: 130,
		supportFunc: func(klines []tushare.DailyKLine, _ int) float64 {
			ma30 := CalcMA(klines, 30)
			ma60 := CalcMA(klines, 60)
			ma120 := CalcMA(klines, 120)
			return math.Max(ma30, math.Max(ma60, ma120))
		},
	}
	return impl.FindAnchor(ctx)
}

// CBBMAnchorFinder 箱体突破锚点：支撑线 = boxUpper。
// 兼容原版 CBBMAnalyzer 和实验版 CBBMExpAnalyzer。
type CBBMAnchorFinder struct{ baseAnalyzer Analyzer }

func (f *CBBMAnchorFinder) FindAnchor(ctx *SecurityContext) *PullbackAnchor {
	impl := &anchorFinderImpl{
		analyzer:  f.baseAnalyzer,
		minKLines: 120,
		supportFunc: func(klines []tushare.DailyKLine, _ int) float64 {
			boxUpper, _ := GetRealBox(klines, 60)
			return boxUpper
		},
	}
	return impl.FindAnchor(ctx)
}

// ─── 复合回踩分析器 ───

type CompositePullbackAnalyzer struct {
	name   string
	finder AnchorFinder
}

func (c *CompositePullbackAnalyzer) Name() string     { return c.name }
func (c *CompositePullbackAnalyzer) MarketTag() string { return "left" }
func (c *CompositePullbackAnalyzer) RequiredData() []string {
	return []string{"klines", "fundamentals", "moneyflow"}
}

func (c *CompositePullbackAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	if len(klines) < 30 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if !checkFundamentalShield(ctx, false) {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	today := klines[len(klines)-1]
	n := len(klines)

	// Step 1: 锚点扫描
	anchor := c.finder.FindAnchor(ctx)
	if anchor == nil {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// Step 2: 回调验证
	pullbackStart := anchor.Index + 1
	pullbackEnd := n - 1
	if pullbackStart >= pullbackEnd || pullbackEnd-pullbackStart < 2 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 价格必须低于锚点高点（处于回调状态）
	if today.Close >= klines[anchor.Index].High {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// Step 3: 支撑线回踩确认 (±3%)
	lowDist := math.Abs(today.Low-anchor.SupportLine) / anchor.SupportLine
	if lowDist > 0.03 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	// 收盘必须站稳支撑线之上
	if today.Close < anchor.SupportLine {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	// 必须是阳线
	if today.Close <= today.Open {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// Step 4: 缩量验证（复用 pbma.go 逻辑）
	totalVol := 0.0
	pullbackDays := 0
	for i := pullbackStart; i < pullbackEnd; i++ {
		totalVol += klines[i].Vol
		pullbackDays++
	}
	if pullbackDays == 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	avgPullbackVol := totalVol / float64(pullbackDays)
	if avgPullbackVol >= 0.5*anchor.Volume {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// Step 5: EXP 过滤器（锁死）
	yesterday := klines[len(klines)-2]
	// 5a. 拦截微幅高开陷阱
	if err := CheckGapTrap(today.Open, yesterday.Close); err != nil {
		return reject("[EXP-P] " + err.Error())
	}
	// 5b. 拦截回踩日无资金承接（左侧逻辑，传入洗盘期均量）
	if err := CheckVolumeTrap(today.Vol, 0, false, avgPullbackVol); err != nil {
		return reject("[EXP-P] " + err.Error())
	}

	// Step 6: 上方空间检查
	room := GetOverheadRoom(klines, today.Close, 120)
	if room < 0.10 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 生成信号
	atr14 := CalcATR(klines, 14)
	volRatio := avgPullbackVol / anchor.Volume * 100
	anchorPctChg := (klines[anchor.Index].Close/klines[anchor.Index-1].Close - 1) * 100

	msg := fmt.Sprintf("🎯 复合回踩确认！锚点日涨幅%.1f%%放量(距今%d天)，回调缩量至%.0f%%，今日最低%.2f精准回踩支撑(%.2f)，阳线站稳。上方空间%.1f%%。",
		anchorPctChg,
		n-1-anchor.Index,
		volRatio,
		today.Low, anchor.SupportLine,
		room*100)

	return DiagnoseResult{
		Code:         code,
		StrategyName: c.Name(),
		LatestPrice:  today.Close,
		Signal:       "买入 🚀",
		Message:      msg,
		BuyPrice:     today.Close,
		SellPrice:    0,
		StopLossPrice: today.Close - 2.0*atr14,
		PEPercentile:  ctx.PEPercentile,
	}
}

func (c *CompositePullbackAnalyzer) EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult {
	if len(history) < 20 {
		return EvaluateHoldResult{}
	}

	buyATR := meta["buy_atr"]

	// 阶段A：-10% 硬止损
	if ok, sellPrice, reason := CheckHardStop(today, pos.BuyPrice, 0.10); ok {
		return EvaluateHoldResult{Sell: true, Price: sellPrice, Reason: reason}
	}

	// 阶段B：Max(2.5*ATR, 12%) 追踪止损
	if ok, sellPrice, reason := CheckTrailingStop(today, history, buyATR, 2.5, 0.12); ok {
		return EvaluateHoldResult{Sell: true, Price: sellPrice, Reason: reason}
	}

	return EvaluateHoldResult{}
}
