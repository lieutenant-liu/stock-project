package backtest

import (
	"stock-backend/strategy"
	"stock-backend/tushare"
)

// checkSellConditions 评估持仓的全部卖出触发条件。
// 返回 (是否卖出, 卖出原因)。
// 优先级：跌停拦截 > MA120止损 > 箱体止损 > 止盈。
func checkSellConditions(
	today tushare.DailyKLine,
	window []tushare.DailyKLine,
	pos *position,
	cfg BacktestConfig,
	limitMap map[string]tushare.StkLimit,
) (bool, string) {

	// 跌停流动性检查：收盘价等于跌停价，卖不掉
	if limit, ok := limitMap[today.TradeDate]; ok {
		if today.Close <= limit.DownLimit && limit.DownLimit > 0 {
			return false, ""
		}
	}

	// MA120 止损
	if cfg.UseMA120Stop && len(window) >= 120 {
		ma120 := strategy.CalcMA(window, 120)
		if ma120 > 0 && today.Close < ma120 {
			return true, "跌破MA120均线止损"
		}
	}

	// 箱体支撑止损
	if cfg.UseBoxStop && len(window) >= 60 {
		_, boxLower := strategy.GetRealBox(window, 60)
		if boxLower > 0 && today.Close < boxLower {
			return true, "跌破箱体支撑位止损"
		}
	}

	// 止盈
	if cfg.ProfitTakePct > 0 {
		unrealizedPct := (today.Close - pos.BuyPrice) / pos.BuyPrice * 100
		if unrealizedPct >= cfg.ProfitTakePct {
			return true, "达到止盈目标"
		}
	}

	return false, ""
}
