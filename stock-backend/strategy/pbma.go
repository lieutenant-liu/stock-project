package strategy

import (
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
	// TODO: 核心买入逻辑
	// 1. 前期放量大阳线检测（近 20 日内出现涨幅 > 5% 且成交量 > 2x MA20 量的阳线）
	// 2. 连续缩量回调至 MA20 附近（当前量 < 0.5x MA20 量，价格距 MA20 < 3%）
	// 3. 止跌反转 K 线确认（十字星、锤子线、阳包阴等）
	return DiagnoseResult{Signal: "观望 💤"}
}

func (p *PBMAAnalyzer) EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult {
	// TODO: 持仓评估逻辑
	// 止损：跌破 MA20 或回踩低点
	// 止盈：前高压力位或策略动态追踪
	return EvaluateHoldResult{}
}
