package backtest

import "stock-backend/tushare"

// BacktestConfig 用户可配的回测参数。
type BacktestConfig struct {
	TargetPool      []string `json:"target_pool"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	InitialCapital  float64  `json:"initial_capital"`
	Strategy        string   `json:"strategy"`
	ProfitTakePct   float64  `json:"profit_take_pct"`
	UseMA120Stop    bool     `json:"use_ma120_stop"`
	UseBoxStop      bool     `json:"use_box_stop"`
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

// TheoreticalTrade Phase 1 产出的理论交易信号。
// 包含持仓期间每日收盘价，供 Phase 2 做逐日盯市。
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

// position 当前持仓（引擎内部）。
type position struct {
	TSCode    string
	BuyDate   string
	BuyPrice  float64
	Shares    int
	Strategy  string
	BuyReason string
}

// pendingOrder T+1 挂单买入（信号日产生，次日执行）。
type pendingOrder struct {
	TSCode      string
	SignalDate  string
	Strategy    string
	Reason      string
	SignalPrice float64
}

// dailySnapshot 某个交易日全市场的截面数据（用完即释放）。
type dailySnapshot struct {
	KLines map[string]tushare.DailyKLine // tsCode → 当日K线(已复权)
	Limits map[string]tushare.StkLimit   // tsCode → 当日涨跌停
}

// positionHistory 持仓股票的K线历史缓存（按需加载，逐步追加）。
type positionHistory struct {
	KLines []tushare.DailyKLine // 有序，从建仓前N天到昨天
}
