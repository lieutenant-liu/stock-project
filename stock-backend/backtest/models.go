package backtest

import "stock-backend/strategy"

// BacktestConfig 用户可配的回测参数。
type BacktestConfig struct {
	TargetPool      []string `json:"target_pool"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	InitialCapital  float64  `json:"initial_capital"`
	Strategy        string   `json:"strategy"`
	Commission      float64  `json:"commission"`
	PositionSizePct float64  `json:"position_size_pct"`
}

// BacktestResult 完整回测结果。
type BacktestResult struct {
	Config      BacktestConfig `json:"config"`
	InitialCap  float64        `json:"initial_capital"`
	FinalAssets float64        `json:"final_assets"`
	TotalReturn float64        `json:"total_return_pct"`
	TotalTrades int            `json:"total_trades"`
	WinTrades   int            `json:"win_trades"`
	LossTrades  int            `json:"loss_trades"`
	WinRate     float64        `json:"win_rate_pct"`
	MaxDrawdown float64        `json:"max_drawdown_pct"`
	TradeLog    []TradeRecord  `json:"trade_log"`
	EquityCurve []EquityPoint  `json:"equity_curve"`
	Summary     string         `json:"summary"`
}

// TradeRecord 一笔完整的买入-卖出交易记录。
type TradeRecord struct {
	TSCode     string  `json:"ts_code"`
	BuyDate    string  `json:"buy_date"`
	BuyPrice   float64 `json:"buy_price"`
	SellDate   string  `json:"sell_date"`
	SellPrice  float64 `json:"sell_price"`
	Shares     int     `json:"shares"`
	PnL        float64 `json:"pnl"`
	ReturnPct  float64 `json:"return_pct"`
	HoldDays   int     `json:"hold_days"`
	BuyReason  string  `json:"buy_reason"`
	SellReason string  `json:"sell_reason"`
	Strategy   string  `json:"strategy"`
}

// EquityPoint 每日净值点。
type EquityPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// TheoreticalTrade Phase 1 产出的已闭环完整交易。
// Phase 2 仅按 SellDate/SellPrice 执行资金回笼，不做任何判断。
type TheoreticalTrade struct {
	Code          string             `json:"code"`
	BuyDate       string             `json:"buy_date"`
	BuyPrice      float64            `json:"buy_price"`
	SellDate      string             `json:"sell_date"`
	SellPrice     float64            `json:"sell_price"`
	Strategy      string             `json:"strategy"`
	BuyReason     string             `json:"buy_reason"`
	SellReason    string             `json:"sell_reason"`
	HoldingPrices map[string]float64 `json:"-"` // trade_date → Close，仅持仓期间
}

// ── Phase 1 内部状态结构（不导出）──

// pendingSignal T+1 挂单买入（信号日产生，次日执行）。
type pendingSignal struct {
	code       string
	signalDate string
	strategy   string
	reason     string
	buyResult  strategy.DiagnoseResult // 买入时的完整诊断结果（含策略参数）
	meta       map[string]float64      // EvaluateHold 所需的策略元数据
}

// holdState 当前持仓状态（买入后持续跟踪直到卖出）。
type holdState struct {
	buyDate           string
	buyIdx            int
	buyPrice          float64
	strategy          string
	reason            string
	buyResult         strategy.DiagnoseResult
	meta              map[string]float64
	trailingStopArmed bool    // 已触发止损条件但因流动性不足无法执行，后续有流动性立即卖出
	highWatermark     float64 // 持仓期间最高收盘价
	stageBActive      bool    // 是否已激活阶段B (利润锁定，启用追踪止损)
	buyATR            float64 // 买入时的基础 ATR 值 (用于自适应止损)
}
