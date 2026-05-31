package strategy

import "stock-backend/tushare"

// =====================================================
// 实验版策略包装器 (Experimental Strategy Wrappers)
// 嵌入原始策略，委托 Analyze()，仅在买入信号时叠加实验过滤器。
// =====================================================

// ─── CBBM-EXP：中枢强势突破-实验 ───

type CBBMExpAnalyzer struct{ base *CBBMAnalyzer }

func (c *CBBMExpAnalyzer) Name() string        { return "中枢强势突破-实验 (CBBM-EXP)" }
func (c *CBBMExpAnalyzer) MarketTag() string    { return c.base.MarketTag() }
func (c *CBBMExpAnalyzer) RequiredData() []string { return c.base.RequiredData() }

func (c *CBBMExpAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	result := c.base.Analyze(ctx)
	if result.Signal != "买入 🚀" {
		return result
	}

	today := ctx.KLines[len(ctx.KLines)-1]
	yesterday := ctx.KLines[len(ctx.KLines)-2]
	volMa20 := CalcVolMA(ctx.KLines, 20)

	if err := CheckGapTrap(today.Open, yesterday.Close); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}
	if err := CheckVolumeTrap(today.Vol, volMa20, true, 0); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}
	if err := CheckBodyRatio(today); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}
	return result
}

func (c *CBBMExpAnalyzer) EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult {
	return c.base.EvaluateHold(pos, today, history, meta)
}

// ─── PBMA-EXP：缩量回踩狙击-实验 ───

type PBMAExpAnalyzer struct{ base *PBMAAnalyzer }

func (p *PBMAExpAnalyzer) Name() string        { return "缩量回踩狙击-实验 (PBMA-EXP)" }
func (p *PBMAExpAnalyzer) MarketTag() string    { return p.base.MarketTag() }
func (p *PBMAExpAnalyzer) RequiredData() []string { return p.base.RequiredData() }

func (p *PBMAExpAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	result := p.base.Analyze(ctx)
	if result.Signal != "买入 🚀" {
		return result
	}

	klines := ctx.KLines
	n := len(klines)
	today := klines[n-1]
	yesterday := klines[n-2]
	volMa20 := CalcVolMA(klines, 20)

	if err := CheckGapTrap(today.Open, yesterday.Close); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}

	// 复用 PBMA 锚点检测 + 回调均量计算，获取真实 avgPullbackVol
	avgPullbackVol := 0.0
	if volMa20 > 0 {
		anchorIdx := -1
		scanStart := n - 1 - 15
		if scanStart < 1 {
			scanStart = 1
		}
		for i := scanStart; i < n-1; i++ {
			prevClose := klines[i-1].Close
			if prevClose <= 0 {
				continue
			}
			pctChg := (klines[i].Close - prevClose) / prevClose
			if pctChg > 0.05 && klines[i].Close > klines[i].Open && klines[i].Vol > 1.5*volMa20 {
				anchorIdx = i
			}
		}
		if anchorIdx >= 0 {
			pullbackStart := anchorIdx + 1
			pullbackEnd := n - 1
			if pullbackStart < pullbackEnd {
				totalVol := 0.0
				for i := pullbackStart; i < pullbackEnd; i++ {
					totalVol += klines[i].Vol
				}
				avgPullbackVol = totalVol / float64(pullbackEnd-pullbackStart)
			}
		}
	}

	if err := CheckVolumeTrap(today.Vol, volMa20, false, avgPullbackVol); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}
	if err := CheckBodyRatio(today); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}
	return result
}

func (p *PBMAExpAnalyzer) EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult {
	return p.base.EvaluateHold(pos, today, history, meta)
}
