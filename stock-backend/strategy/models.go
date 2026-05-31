package strategy

import (
	"stock-backend/tushare"
)

// DiagnoseResult 工业级诊断报告（标准化输出）
type DiagnoseResult struct {
	Code          string  `json:"code"`
	StrategyName  string  `json:"strategy_name"`
	LatestPrice   float64 `json:"latest_price"`
	Signal        string  `json:"signal"`          // 信号："买入 🚀", "观望 💤", "卖出 🛑"
	Message       string  `json:"message"`         // 详细战报
	BuyPrice      float64 `json:"buy_price"`       // 🎯 建议买入价
	SellPrice     float64 `json:"sell_price"`      // 🎯 建议止盈价
	StopLossPrice float64 `json:"stop_loss_price"` // 🎯 硬性止损价
	PEPercentile  float64 `json:"pe_percentile"`   // 买入时的估值分位
}

// 💥 架构升级：机构级多维数据上下文 (Security Context)
type SecurityContext struct {
	Code         string
	StockName    string
	KLines       []tushare.DailyKLine
	Fundamentals []tushare.DailyFundamental
	MoneyFlows   []tushare.DailyMoneyFlow // 2000积分王牌：资金流向数据
	PEPercentile float64                  // 💥 新增：动态估值历史分位 (0.0 ~ 1.0)
	CyqPerf      *tushare.CyqPerf         // 💥 筹码分布数据 (5000积分高阶)
	LatestFina   *tushare.FinaIndicator   // 最新季报财务指标 (ROE, NetProfitYoY)
}

// Analyzer 多态策略引擎接口定义 (Strategy Pattern)
type Analyzer interface {
	Name() string
	MarketTag() string                           // "right" (右侧突破) 或 "left" (左侧抄底/回踩)
	RequiredData() []string                      // 声明需要的数据："klines", "fundamentals", "moneyflow"
	Analyze(ctx *SecurityContext) DiagnoseResult // 💥 接口升维：接收上下文
	EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult
}

// Position 回测持仓信息（供 EvaluateHold 使用）。
type Position struct {
	Code      string
	BuyDate   string
	BuyPrice  float64
	Strategy  string
	BuyResult DiagnoseResult // 买入时的完整诊断结果（含策略参数）
}

// EvaluateHoldResult 每日持仓评估结果。
type EvaluateHoldResult struct {
	Sell   bool    // 是否触发卖出
	Price  float64 // 实际卖出价格（仅 Sell=true 时有效）
	Reason string  // 卖出原因
}
