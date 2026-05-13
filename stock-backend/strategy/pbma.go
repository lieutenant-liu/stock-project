package strategy

import (
	"fmt"
	"math"
	"stock-backend/tushare"
)

// ==========================================
// 策略四：缩量回踩狙击 (PBMA - Pullback Moving Average)
// 专门寻找前期放量大阳线启动，近期连续缩量回调至 MA20 均线附近，
// 且出现止跌反转 K 线的标的。
// ==========================================
type PBMAAnalyzer struct{}

func (p *PBMAAnalyzer) Name() string           { return "缩量回踩狙击 (PBMA)" }
func (p *PBMAAnalyzer) RequiredData() []string { return []string{"klines", "fundamentals"} }

func (p *PBMAAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	if len(klines) < 30 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 基本面过滤
	if !checkFundamentalShield(ctx, false) {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	today := klines[len(klines)-1]
	n := len(klines)

	// =====================================================
	// Step A: 寻找前锋锚点 (Anchor Detection)
	// 过去 15 个交易日内（不含今天），寻找放量大阳线
	// =====================================================
	volMa20 := CalcVolMA(klines, 20)
	if volMa20 <= 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	anchorIdx := -1
	anchorHigh := 0.0
	anchorVol := 0.0
	scanStart := n - 1 - 15
	if scanStart < 0 {
		scanStart = 0
	}

	for i := scanStart; i < n-1; i++ {
		k := klines[i]
		if k.PreClose <= 0 {
			continue
		}
		pctChg := (k.Close - k.PreClose) / k.PreClose
		isBigYang := pctChg > 0.05 && k.Close > k.Open
		isVolumeSurge := k.Vol > 1.5*volMa20
		if isBigYang && isVolumeSurge {
			anchorIdx = i
			anchorHigh = k.High
			anchorVol = k.Vol
		}
	}

	if anchorIdx < 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// =====================================================
	// Step B: 极度缩量洗盘确认 (Volume Contraction)
	// 从锚点日之后到昨天，平均成交量 < 0.5 * AnchorVol
	// =====================================================
	pullbackStart := anchorIdx + 1
	pullbackEnd := n - 1 // 昨天（不含今天）
	if pullbackStart >= pullbackEnd {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 价格必须低于锚点最高价（处于回调状态）
	if today.Close >= anchorHigh {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 计算回调期间平均成交量
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

	if avgPullbackVol >= 0.5*anchorVol {
		return DiagnoseResult{Signal: "观望 💤"} // 缩量不够极致
	}

	// =====================================================
	// Step C: 均线支撑位测试 (Support Test)
	// 今天最低价回踩到 MA20 附近 (±3%)
	// =====================================================
	ma20 := CalcMA(klines, 20)
	if ma20 <= 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	lowDist := math.Abs(today.Low-ma20) / ma20
	if lowDist >= 0.03 {
		return DiagnoseResult{Signal: "观望 💤"} // 没有回踩到均线
	}

	// =====================================================
	// Step D: 右侧止跌反转 (Right-Side Reversal)
	// 今天必须是阳线，且收盘站在 MA20 之上
	// =====================================================
	isYangLine := today.Close > today.Open
	closeAboveMA20 := today.Close > ma20
	if !isYangLine || !closeAboveMA20 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// =====================================================
	// 全部通过，生成买入信号
	// =====================================================
	room := GetOverheadRoom(klines, today.Close, 120)
	if room < 0.10 {
		return DiagnoseResult{Signal: "观望 💤"} // 上方空间不足
	}

	// 计算 ATR 用于止损参考
	atr14 := CalcATR(klines, 14)

	volRatio := avgPullbackVol / anchorVol * 100
	msg := fmt.Sprintf("🎯 缩量回踩狙击！锚点日涨幅%.1f%%放量(距今%d天)，回调期缩量至锚点%.0f%%，今日最低%.2f精准回踩MA20(%.2f)，阳线反转确认。上方空间%.1f%%。",
		(anchorHigh/klines[anchorIdx].PreClose-1)*100,
		n-1-anchorIdx,
		volRatio,
		today.Low, ma20,
		room*100)

	return DiagnoseResult{
		Code:         code,
		StrategyName: p.Name(),
		LatestPrice:  today.Close,
		Signal:       "买入 🚀",
		Message:      msg,
		BuyPrice:     today.Close,
		SellPrice:    0, // 不设固定止盈，由追踪止损守护
		StopLossPrice: today.Close - 2.0*atr14,
		PEPercentile:  ctx.PEPercentile,
	}
}

func (p *PBMAAnalyzer) EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult {
	if len(history) < 20 {
		return EvaluateHoldResult{}
	}

	// 计算买入时的 ATR（用于追踪止损）
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
