package strategy

import (
	"fmt"
	"math"
	"stock-backend/tushare"
)

// =====================================================
// 复合回踩引擎 (Composite Pullback Engine)
// 捕捉"突破后缩量回踩支撑"的二次买入机会。
// 支持 EXP 过滤器开关，用于正交测试归因。
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

type anchorFinderImpl struct {
	analyzer    Analyzer
	minKLines   int
	supportFunc func(klines []tushare.DailyKLine, idx int) float64
}

func (f *anchorFinderImpl) FindAnchor(ctx *SecurityContext) *PullbackAnchor {
	n := len(ctx.KLines)
	for backDays := 5; backDays <= 30; backDays++ {
		i := n - 1 - backDays
		if i < f.minKLines {
			continue
		}
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
	name          string
	finder        AnchorFinder
	useExpFilters bool
}

func (c *CompositePullbackAnalyzer) Name() string     { return c.name }
func (c *CompositePullbackAnalyzer) MarketTag() string { return "right" }
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

	// Step 3: 非对称支撑容差
	toleranceUpper := anchor.SupportLine * 1.05
	toleranceLower := anchor.SupportLine * 0.94
	closeTolerance := anchor.SupportLine * 0.98

	if today.Low > toleranceUpper {
		return reject("未触及支撑带")
	}
	if today.Low < toleranceLower {
		return reject("盘中深度破位")
	}
	if today.Close < closeTolerance {
		return reject("收盘未站稳支撑位")
	}
	// 必须是阳线
	if today.Close <= today.Open {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// Step 4: 缩量验证
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
	if avgPullbackVol >= 0.7*anchor.Volume {
		return reject("回踩期缩量不足")
	}

	// Step 5: EXP 过滤器（条件执行）
	if c.useExpFilters {
		yesterday := klines[len(klines)-2]
		if err := CheckGapTrap(today.Open, yesterday.Close); err != nil {
			return reject("[EXP-P] " + err.Error())
		}
		if err := CheckVolumeTrap(today.Vol, 0, false, avgPullbackVol); err != nil {
			return reject("[EXP-P] " + err.Error())
		}
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
