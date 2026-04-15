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
}

// 💥 架构升级：机构级多维数据上下文 (Security Context)
type SecurityContext struct {
	Code         string
	StockName    string
	KLines       []tushare.DailyKLine
	Fundamentals []tushare.DailyFundamental
	MoneyFlows   []tushare.DailyMoneyFlow // 💥 2000积分王牌：主力资金流向
	PEPercentile float64                  // 💥 新增：动态估值历史分位 (0.0 ~ 1.0)
	// 未来可直接在此处扩展 IndexDaily(大盘)、LimitList(涨停榜) 等，无需再改接口
}

// Analyzer 多态策略引擎接口定义 (Strategy Pattern)
type Analyzer interface {
	Name() string
	RequiredData() []string                      // 声明需要的数据："klines", "fundamentals", "moneyflow"
	Analyze(ctx *SecurityContext) DiagnoseResult // 💥 接口升维：接收上下文
}
