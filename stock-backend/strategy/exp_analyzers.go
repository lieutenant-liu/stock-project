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

	today := ctx.KLines[len(ctx.KLines)-1]
	yesterday := ctx.KLines[len(ctx.KLines)-2]
	volMa20 := CalcVolMA(ctx.KLines, 20)

	if err := CheckGapTrap(today.Open, yesterday.Close); err != nil {
		return DiagnoseResult{Signal: "观望 💤", Message: "[EXP] " + err.Error()}
	}
	// PBMA 是左侧策略，avgPullbackVol 无法从包装器获取，传 0 跳过左侧检查
	if err := CheckVolumeTrap(today.Vol, volMa20, false, 0); err != nil {
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
