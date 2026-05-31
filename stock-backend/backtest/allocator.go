package backtest

import (
	"math"
	"strings"
)

// SlippageRate 滑点比率（千分之二），买入加价、卖出减价。
const SlippageRate = 0.002

// CalculatePositionShares 计算单笔交易的动态头寸股数。
//
// 参数:
//   - totalEquity: 当前总资产（cash + 持仓市值）
//   - currentCash: 当前可用现金
//   - price:       买入价格
//   - commission:  手续费率
//   - strategy:    策略全名（用于高胜率加成判断）
//   - score:       信号评分
//   - basePct:     基础仓位比例（来自 BacktestConfig.PositionSizePct）
//
// 返回: 可买入股数（100 的整数倍），0 表示资金不足一手
func CalculatePositionShares(totalEquity, currentCash, price, commission float64, strategy string, score int, basePct float64) int {
	if totalEquity <= 0 || price <= 0 {
		return 0
	}

	// 1. 基础仓位：总资产 * basePct
	targetAmount := totalEquity * basePct

	// 2. 动态置信度加成：高胜率策略或高分信号 → 重仓（最高 40%）
	if score > 5000 || strings.Contains(strategy, "CBBM-P-EXP") {
		boostedPct := math.Min(basePct*2, 0.40)
		targetAmount = totalEquity * boostedPct
	}

	// 3. 现金约束：不能超过当前可用现金
	if targetAmount > currentCash {
		targetAmount = currentCash
	}

	// 4. A 股物理限制：100 股为一手
	shares := math.Floor(targetAmount/(price*(1+commission))) / 100 * 100
	return int(shares)
}
