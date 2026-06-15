// ============================================================
// models.go —— 策略引擎的数据模型定义
// 本文件定义了策略系统中所有的数据结构（struct）和接口（interface）。
// Go 语言中，struct 类似于其他语言的"类"，interface 定义了一组方法签名，
// 任何 struct 只要实现了接口中的所有方法，就自动满足该接口（隐式实现，无需声明 implements）。
// ============================================================

package strategy

import (
	"stock-backend/tushare" // 导入 tushare 包，提供 K 线、基本面等数据结构
)

// ----------------------------------------------------------
// DiagnoseResult —— 策略诊断结果（标准化输出）
// 每个策略执行完毕后，都会返回一个 DiagnoseResult，
// 包含信号类型、建议买卖价格、止损位等信息。
// struct 字段后面的 `json:"xxx"` 是结构体标签（struct tag），
// 用于控制 JSON 序列化时的字段名。
// ----------------------------------------------------------
type DiagnoseResult struct {
	Code          string  `json:"code"`            // 股票代码，如 "000001.SZ"
	StrategyName  string  `json:"strategy_name"`   // 产生该信号的策略名称
	LatestPrice   float64 `json:"latest_price"`    // 最新收盘价
	Signal        string  `json:"signal"`          // 信号类型："买入", "观望", "卖出"
	Message       string  `json:"message"`         // 策略给出的详细解读文本（战报）
	BuyPrice      float64 `json:"buy_price"`       // 建议买入价格
	SellPrice     float64 `json:"sell_price"`      // 建议止盈价格（0 表示不设固定止盈）
	StopLossPrice float64 `json:"stop_loss_price"` // 建议止损价格
	PEPercentile  float64 `json:"pe_percentile"`   // 买入时的 PE 估值历史分位（0.0~1.0）
}

// ----------------------------------------------------------
// SecurityContext —— 个股多维数据上下文（策略分析的输入）
// 将一只股票的所有相关数据打包在一起传给策略引擎，
// 避免函数参数过多（Go 中推荐用 struct 传递上下文）。
// 指针类型 *tushare.XXX 表示该字段可以为 nil（空指针），
// 在 Go 中，指针为 nil 时表示"没有数据"，策略中需要先判断是否为 nil。
// ----------------------------------------------------------
type SecurityContext struct {
	Code         string                      // 股票代码
	StockName    string                      // 股票名称
	KLines       []tushare.DailyKLine        // 日 K 线数据数组（切片/slice）
	Fundamentals []tushare.DailyFundamental  // 日级基本面数据（PE、市值等）
	MoneyFlows   []tushare.DailyMoneyFlow    // 资金流向数据（大单、小单净流入）
	PEPercentile float64                     // 动态估值历史分位（0.0~1.0，越高越贵）
	CyqPerf      *tushare.CyqPerf            // 筹码分布数据（获利盘比例等），可能为 nil
	LatestFina   *tushare.FinaIndicator      // 最新季报财务指标（ROE、净利润增速等），可能为 nil
}

// ----------------------------------------------------------
// Analyzer —— 策略引擎接口（核心抽象）
// 这是 Go 语言的"接口"（interface），定义了所有策略必须实现的方法。
// Go 的接口是隐式实现的：只要某个 struct 实现了以下所有方法，
// 它就自动满足 Analyzer 接口，无需显式声明。
// 这就是"策略模式"（Strategy Pattern）的 Go 实现。
// ----------------------------------------------------------
type Analyzer interface {
	// Name 返回策略的可读名称，如 "均线收敛突破 (MACB)"
	Name() string

	// MarketTag 返回策略类型标签：
	//   "right" = 右侧交易（趋势确认后买入，追涨型）
	//   "left"  = 左侧交易（抄底型，在下跌中提前埋伏）
	MarketTag() string

	// RequiredData 声明该策略需要哪些数据源，供调用方按需加载。
	// 返回的字符串切片中可能包含："klines", "fundamentals", "moneyflow" 等。
	RequiredData() []string

	// Analyze 执行策略分析，接收完整的数据上下文，返回诊断结果。
	// 这是每个策略的核心方法，包含所有的买入信号判断逻辑。
	Analyze(ctx *SecurityContext) DiagnoseResult

	// EvaluateHold 持仓评估：每个交易日收盘后检查是否应该卖出。
	// 参数说明：
	//   pos     = 当前持仓信息（包含买入价、买入时的诊断结果等）
	//   today   = 今天的 K 线数据
	//   history = 从买入日到今天的历史 K 线（用于计算均线等指标）
	//   meta    = 策略元数据（如箱体上沿、ATR 等，由回测框架传入）
	EvaluateHold(pos *Position, today tushare.DailyKLine, history []tushare.DailyKLine, meta map[string]float64) EvaluateHoldResult
}

// ----------------------------------------------------------
// Position —— 持仓信息（回测时使用）
// 记录一次买入操作的完整信息，供 EvaluateHold 方法使用。
// ----------------------------------------------------------
type Position struct {
	Code      string           // 股票代码
	BuyDate   string           // 买入日期，格式如 "20250101"
	BuyPrice  float64          // 买入价格
	Strategy  string           // 使用的策略 ID
	BuyResult DiagnoseResult   // 买入时的完整诊断结果（包含策略参数、止损位等）
}

// ----------------------------------------------------------
// EvaluateHoldResult —— 持仓评估结果
// EvaluateHold 方法的返回值，告诉回测框架今天是否要卖出。
// ----------------------------------------------------------
type EvaluateHoldResult struct {
	Sell   bool    // true = 触发卖出信号，false = 继续持有
	Price  float64 // 卖出成交价格（仅 Sell=true 时有意义）
	Reason string  // 卖出原因的文字描述（用于日志和回测报告）
}
