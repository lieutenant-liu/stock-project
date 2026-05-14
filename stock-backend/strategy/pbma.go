package strategy

import (
	"fmt"
	"log"
	"math"
	"stock-backend/tushare"
	"sync/atomic"
)

// ── PBMA 死因探针（Survival Analyzer）──
// 全局原子计数器，记录每一步的存活/淘汰数量。
var (
	pbmaTotalBars   int64
	pbmaFailKlines  int64 // len(klines) < 30
	pbmaFailFund    int64 // 基本面过滤
	pbmaFailVolMA   int64 // volMa20 <= 0
	pbmaFailAnchor  int64 // 无锚点
	pbmaFailPullbk  int64 // 回调区间无效
	pbmaFailPriceHi int64 // 价格 >= anchorHigh
	pbmaFailShrink  int64 // 缩量不够
	pbmaFailMA20    int64 // MA20 不足或未回踩
	pbmaFailReversl int64 // 非阳线或未站上 MA20
	pbmaFailRoom    int64 // Overhead Room 不足
	pbmaPass        int64 // 成功生成信号
)

// PbmaProbeReport 输出 PBMA 存活分析报告并重置计数器。
func PbmaProbeReport() {
	total := atomic.LoadInt64(&pbmaTotalBars)
	if total == 0 {
		log.Printf("[PBMA探针] 未被调用过，无数据")
		return
	}
	failK := atomic.LoadInt64(&pbmaFailKlines)
	failF := atomic.LoadInt64(&pbmaFailFund)
	failV := atomic.LoadInt64(&pbmaFailVolMA)
	failA := atomic.LoadInt64(&pbmaFailAnchor)
	failP := atomic.LoadInt64(&pbmaFailPullbk)
	failH := atomic.LoadInt64(&pbmaFailPriceHi)
	failS := atomic.LoadInt64(&pbmaFailShrink)
	failM := atomic.LoadInt64(&pbmaFailMA20)
	failR := atomic.LoadInt64(&pbmaFailReversl)
	failO := atomic.LoadInt64(&pbmaFailRoom)
	pass := atomic.LoadInt64(&pbmaPass)

	log.Printf("[PBMA探针] 总调用 %d 次 | 淘汰漏斗:", total)
	log.Printf("  ├─ K线不足(%%%.1f) %d", float64(failK)/float64(total)*100, failK)
	log.Printf("  ├─ 基本面淘汰(%%%.1f) %d", float64(failF)/float64(total)*100, failF)
	log.Printf("  ├─ VolMA=0(%%%.1f) %d", float64(failV)/float64(total)*100, failV)
	log.Printf("  ├─ 无锚点(%%%.1f) %d", float64(failA)/float64(total)*100, failA)
	log.Printf("  ├─ 回调无效(%%%.1f) %d", float64(failP)/float64(total)*100, failP)
	log.Printf("  ├─ 价格>=锚高(%%%.1f) %d", float64(failH)/float64(total)*100, failH)
	log.Printf("  ├─ 缩量不足(%%%.1f) %d", float64(failS)/float64(total)*100, failS)
	log.Printf("  ├─ MA20/回踩(%%%.1f) %d", float64(failM)/float64(total)*100, failM)
	log.Printf("  ├─ 非阳线(%%%.1f) %d", float64(failR)/float64(total)*100, failR)
	log.Printf("  ├─ Room不足(%%%.1f) %d", float64(failO)/float64(total)*100, failO)
	log.Printf("  └─ ✅ 通过(%%%.2f) %d", float64(pass)/float64(total)*100, pass)

	// 重置
	atomic.StoreInt64(&pbmaTotalBars, 0)
	atomic.StoreInt64(&pbmaFailKlines, 0)
	atomic.StoreInt64(&pbmaFailFund, 0)
	atomic.StoreInt64(&pbmaFailVolMA, 0)
	atomic.StoreInt64(&pbmaFailAnchor, 0)
	atomic.StoreInt64(&pbmaFailPullbk, 0)
	atomic.StoreInt64(&pbmaFailPriceHi, 0)
	atomic.StoreInt64(&pbmaFailShrink, 0)
	atomic.StoreInt64(&pbmaFailMA20, 0)
	atomic.StoreInt64(&pbmaFailReversl, 0)
	atomic.StoreInt64(&pbmaFailRoom, 0)
	atomic.StoreInt64(&pbmaPass, 0)
}

// ==========================================
// 策略四：缩量回踩狙击 (PBMA - Pullback Moving Average)
// 专门寻找前期放量大阳线启动，近期连续缩量回调至 MA20 均线附近，
// 且出现止跌反转 K 线的标的。
// ==========================================
type PBMAAnalyzer struct{}

func (p *PBMAAnalyzer) Name() string           { return "缩量回踩狙击 (PBMA)" }
func (p *PBMAAnalyzer) MarketTag() string      { return "left" }
func (p *PBMAAnalyzer) RequiredData() []string { return []string{"klines", "fundamentals"} }

func (p *PBMAAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	atomic.AddInt64(&pbmaTotalBars, 1)

	if len(klines) < 30 {
		atomic.AddInt64(&pbmaFailKlines, 1)
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 基本面过滤
	if !checkFundamentalShield(ctx, false) {
		atomic.AddInt64(&pbmaFailFund, 1)
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
		atomic.AddInt64(&pbmaFailVolMA, 1)
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
		prevClose := klines[i-1].Close // 前一日收盘价（PreClose 字段在 BatchGetKLinesWithAdj 中未被查询，始终为 0）
		if prevClose <= 0 {
			continue
		}
		pctChg := (k.Close - prevClose) / prevClose
		isBigYang := pctChg > 0.05 && k.Close > k.Open
		isVolumeSurge := k.Vol > 1.5*volMa20
		if isBigYang && isVolumeSurge {
			anchorIdx = i
			anchorHigh = k.High
			anchorVol = k.Vol
		}
	}

	if anchorIdx < 0 {
		atomic.AddInt64(&pbmaFailAnchor, 1)
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// =====================================================
	// Step B: 极度缩量洗盘确认 (Volume Contraction)
	// 从锚点日之后到昨天，平均成交量 < 0.5 * AnchorVol
	// =====================================================
	pullbackStart := anchorIdx + 1
	pullbackEnd := n - 1 // 昨天（不含今天）
	if pullbackStart >= pullbackEnd {
		atomic.AddInt64(&pbmaFailPullbk, 1)
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 价格必须低于锚点最高价（处于回调状态）
	if today.Close >= anchorHigh {
		atomic.AddInt64(&pbmaFailPriceHi, 1)
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
		atomic.AddInt64(&pbmaFailPullbk, 1)
		return DiagnoseResult{Signal: "观望 💤"}
	}
	avgPullbackVol := totalVol / float64(pullbackDays)

	if avgPullbackVol >= 0.5*anchorVol {
		atomic.AddInt64(&pbmaFailShrink, 1)
		return DiagnoseResult{Signal: "观望 💤"} // 缩量不够极致
	}

	// =====================================================
	// Step C: 均线支撑位测试 (Support Test)
	// 今天最低价回踩到 MA20 附近 (±3%)
	// =====================================================
	ma20 := CalcMA(klines, 20)
	if ma20 <= 0 {
		atomic.AddInt64(&pbmaFailMA20, 1)
		return DiagnoseResult{Signal: "观望 💤"}
	}

	lowDist := math.Abs(today.Low-ma20) / ma20
	if lowDist >= 0.03 {
		atomic.AddInt64(&pbmaFailMA20, 1)
		return DiagnoseResult{Signal: "观望 💤"} // 没有回踩到均线
	}

	// =====================================================
	// Step D: 右侧止跌反转 (Right-Side Reversal)
	// 今天必须是阳线，且收盘站在 MA20 之上
	// =====================================================
	isYangLine := today.Close > today.Open
	closeAboveMA20 := today.Close > ma20
	if !isYangLine || !closeAboveMA20 {
		atomic.AddInt64(&pbmaFailReversl, 1)
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// =====================================================
	// 全部通过，生成买入信号
	// =====================================================
	room := GetOverheadRoom(klines, today.Close, 120)
	if room < 0.03 {
		atomic.AddInt64(&pbmaFailRoom, 1)
		return DiagnoseResult{Signal: "观望 💤"} // 上方空间不足
	}

	// 计算 ATR 用于止损参考
	atr14 := CalcATR(klines, 14)

	volRatio := avgPullbackVol / anchorVol * 100
	anchorPctChg := (klines[anchorIdx].Close/klines[anchorIdx-1].Close - 1) * 100
	msg := fmt.Sprintf("🎯 缩量回踩狙击！锚点日涨幅%.1f%%放量(距今%d天)，回调期缩量至锚点%.0f%%，今日最低%.2f精准回踩MA20(%.2f)，阳线反转确认。上方空间%.1f%%。",
		anchorPctChg,
		n-1-anchorIdx,
		volRatio,
		today.Low, ma20,
		room*100)

	atomic.AddInt64(&pbmaPass, 1)
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
