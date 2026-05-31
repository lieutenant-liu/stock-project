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
	Config      BacktestConfig  `json:"config"`
	InitialCap  float64         `json:"initial_capital"`
	FinalAssets float64         `json:"final_assets"`
	TotalReturn float64         `json:"total_return_pct"`
	TotalTrades int             `json:"total_trades"`
	WinTrades   int             `json:"win_trades"`
	LossTrades  int             `json:"loss_trades"`
	WinRate     float64         `json:"win_rate_pct"`
	MaxDrawdown float64         `json:"max_drawdown_pct"`
	TradeLog    []TradeRecord   `json:"trade_log"`
	EquityCurve []EquityPoint   `json:"equity_curve"`
	Summary     string          `json:"summary"`
	Metrics     BacktestSummary `json:"metrics"` // 专业组合级指标
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
	SellReason  string             `json:"sell_reason"`
	Strategy    string             `json:"strategy"`
	Score       int                `json:"score"`
	PartialSells []PartialSellEvent `json:"partial_sells"`
}

// EquityPoint 每日净值点。
type EquityPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// DailyEquity 每日账户净值明细（含现金拆分）。
type DailyEquity struct {
	Date   string  `json:"date"`
	Equity float64 `json:"equity"` // 总净值 = Cash + Holdings
	Cash   float64 `json:"cash"`
}

// ExitReasonCount 退出原因分布统计。
type ExitReasonCount struct {
	TakeProfit   int `json:"take_profit"`    // 半仓止盈
	TrailingStop int `json:"trailing_stop"`  // 追踪止损
	TimeExit     int `json:"time_exit"`      // 时间止损
	HardStop     int `json:"hard_stop"`      // 洗盘硬止损 / 硬止损
	Other        int `json:"other"`          // 其他原因
}

// BacktestSummary 回测汇总指标。
type BacktestSummary struct {
	InitialCapital float64          `json:"initial_capital"`
	FinalAssets    float64          `json:"final_assets"`
	AbsReturnPct   float64          `json:"abs_return_pct"`   // 绝对收益率
	CAGR           float64          `json:"cagr"`             // 复合年化收益率
	WinRate        float64          `json:"win_rate"`         // 胜率
	MaxDrawdown    float64          `json:"max_drawdown"`     // 最大回撤
	TotalTrades    int              `json:"total_trades"`
	TradingDays    int              `json:"trading_days"`
	ExitReasons    ExitReasonCount  `json:"exit_reasons"`     // 退出原因分布
}

// TradeTelemetry 交易遥测数据（Phase 1 信号开采阶段采集）。
// 用于后续分析信号通过/被拒原因、策略命中率、过滤器效率等。
type TradeTelemetry struct {
	Code          string  `json:"code"`
	Date          string  `json:"date"`
	Strategy      string  `json:"strategy"`
	SignalPassed  bool    `json:"signal_passed"`   // 是否最终产出买入信号
	RejectReason  string  `json:"reject_reason"`   // 被拒原因（如 "overhead_room<3%", "vol_contraction_fail"）
	KLineLen      int     `json:"kline_len"`       // 当时可用 K 线数量
	OverheadRoom  float64 `json:"overhead_room"`   // GetOverheadRoom 值
	VolRatio      float64 `json:"vol_ratio"`       // 量比
	PEPercentile  float64 `json:"pe_percentile"`   // PE 分位数
}

// PartialSellEvent 分批减仓记录。
type PartialSellEvent struct {
	SellDate  string  `json:"sell_date"`
	SellPrice float64 `json:"sell_price"`
	SellQty   int     `json:"sell_qty"` // 减仓股数
	Reason    string  `json:"reason"`
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
	HoldingPrices map[string]float64 `json:"-"`        // trade_date → Close，仅持仓期间
	Score         int                `json:"score"`         // 信号评分（越高越优先）
	PartialSells  []PartialSellEvent `json:"partial_sells"` // 分批减仓记录
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
	score      int                     // 信号评分（越高越优先）
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
	trailingStopArmed  bool    // 已触发止损条件但因流动性不足无法执行，后续有流动性立即卖出
	highWatermark     float64 // 持仓期间最高收盘价
	stageBActive      bool    // 是否已激活阶段B (利润锁定，启用追踪止损)
	buyATR            float64 // 买入时的基础 ATR 值 (用于自适应止损)
	halfSold          bool    // 是否已执行半仓卖出
	daysHeld          int     // 持仓天数（每日 +1）
	peakClose         float64 // 持仓期间最高收盘价（3D退出用）
	partialSells      []PartialSellEvent
	score             int // 信号评分
	totalQty          int // 当前持仓总股数（半仓卖出后减少）
}
