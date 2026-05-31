package backtest

import (
	"fmt"
	"regexp"
	"stock-backend/tushare"
	"strings"
)

// ─────────────────────────────────────────────
// 信号评分系统
// ─────────────────────────────────────────────

// strategyPriority 策略优先级权重（仅 4 个终极策略）。
var strategyPriority = map[string]int{
	"CBBM-P-EXP": 4000,
	"MACB":       3000,
	"PBMA-EXP":   2000,
	"CBBM":       1000,
}

var idRegexp = regexp.MustCompile(`\(([^)]+)\)`)

// extractID 从策略全名中提取短码 ID，如 "均线收敛突破 (MACB)" → "MACB"。
func extractID(name string) string {
	m := idRegexp.FindStringSubmatch(name)
	if len(m) >= 2 {
		return m[1]
	}
	return name
}

// calcPctChg 计算 klines[idx] 相对于 klines[idx-n] 的涨幅百分比。
func calcPctChg(klines []tushare.DailyKLine, idx, n int) float64 {
	if idx < n || n <= 0 {
		return 0
	}
	base := klines[idx-n].Close
	if base <= 0 {
		return 0
	}
	return (klines[idx].Close - base) / base * 100
}

// signalScore 计算信号评分 = 策略权重*1000 + 20日涨幅动量。
func signalScore(strategyName string, klines []tushare.DailyKLine, idx int) int {
	id := extractID(strategyName)
	base := strategyPriority[id]
	pct20 := calcPctChg(klines, idx, 20)
	return base + int(pct20*100)
}

// ─────────────────────────────────────────────
// 3D 退出状态机
// ─────────────────────────────────────────────

// eval3DExit 按优先级依次检查 4 个退出维度。
// exitProfile: "scalp"（短线剥头皮）或 "trend"（趋势追踪），决定 D1~D3 的阈值。
// 返回 (sell, reason, partial): partial=true 表示半仓卖出。
func eval3DExit(today tushare.DailyKLine, hs *holdState, strategyID string, historyData []tushare.DailyKLine, exitProfile string) (bool, string, bool) {
	// ── 动态参数：根据 exitProfile 设定阈值 ──
	tpTarget := 0.10        // D1: 半仓止盈目标
	trailDrop := 0.05       // D2: 追踪止损回撤幅度
	limitDaysRight := 5     // D3: 右侧策略持仓上限
	limitDaysLeft := 8      // D3: 左侧策略持仓上限
	timeExitProfit := 0.03  // D3: 收益低于此值触发时间止损

	if exitProfile == "trend" {
		tpTarget = 0.25       // 25% 半仓止盈
		trailDrop = 0.12      // 12% 追踪止损
		limitDaysRight = 15   // 右侧 15 天
		limitDaysLeft = 20    // 左侧 20 天
		timeExitProfit = 0.00 // 水下才止损
	}

	if exitProfile == "swing" {
		tpTarget = 0.12       // 12% 全仓止盈（下调以应对 0.4% 滑点摩擦）
		trailDrop = 0.06      // 6% 追踪止损（收紧保护利润）
		limitDaysRight = 6    // 右侧 6 天（缩短提高资金周转率）
		limitDaysLeft = 8     // 左侧 8 天
		timeExitProfit = 0.00 // 水下才止损
	}

	if exitProfile == "guerrilla" {
		tpTarget = 0.10       // 10% 全仓止盈，绝不恋战
		trailDrop = 0.05      // 5% 追踪回撤
		limitDaysRight = 5    // 突破后右侧容忍 5 天
		limitDaysLeft = 6     // 左侧震荡容忍 6 天
		timeExitProfit = 0.02 // 到期收益不足 2% 立刻跑路
	}

	maxGainPct := (hs.peakClose - hs.buyPrice) / hs.buyPrice * 100

	// ── D1: 止盈（Take-Profit）──
	if !hs.halfSold && maxGainPct >= tpTarget*100 {
		if exitProfile == "swing" || exitProfile == "guerrilla" {
			return true, fmt.Sprintf("全仓止盈 +%.0f%%", tpTarget*100), false // swing/guerrilla: 全仓卖出
		}
		return true, fmt.Sprintf("半仓止盈 +%.0f%%", tpTarget*100), true // scalp/trend: 半仓卖出
	}

	// ── D2: 动态追踪止损（解耦 halfSold，独立于止盈块）──
	// 激活条件：最高浮盈达到 activationPct 后，从峰值回撤超过 trailDrop 即止损
	activationPct := 0.05 // 默认 5% 浮盈激活保护
	if exitProfile == "swing" {
		activationPct = 0.07 // 波段模式：涨到 7% 才激活回撤保护
	}
	if exitProfile == "guerrilla" {
		activationPct = 0.03 // 游击战：冲高 3% 即激活保护网
	}
	currentGainPct := (today.Close - hs.buyPrice) / hs.buyPrice * 100
	if maxGainPct >= activationPct*100 && (maxGainPct-currentGainPct) >= trailDrop*100 {
		isPartial := true
		if exitProfile == "swing" || exitProfile == "guerrilla" {
			isPartial = false // swing/guerrilla: 全仓卖出
		}
		return true, fmt.Sprintf("追踪止损 -%.0f%% from peak", trailDrop*100), isPartial
	}

	// ── D3: 时间止损（Time Exit）──
	limitDays := limitDaysRight
	if strings.Contains(strategyID, "PBMA") {
		limitDays = limitDaysLeft
	}
	if hs.daysHeld >= limitDays {
		profitPct := (today.Close - hs.buyPrice) / hs.buyPrice
		if profitPct < timeExitProfit {
			return true, fmt.Sprintf("持仓超%d天动能衰竭(收益<%.0f%%)，时间止损", limitDays, timeExitProfit*100), false
		}
	}

	// ── D4: 防洗盘硬止损（Washout Hard Stop）──
	// 参数化：不同流派使用不同底线
	hardStop := -0.08     // 默认盘中破位 -8%
	earlyDropLimit := -0.10 // 默认建仓初期收盘 -10%

	if exitProfile == "guerrilla" {
		hardStop = -0.06      // 游击战盘中底线 -6%
		earlyDropLimit = -0.08 // 游击战建仓初期底线也同步收紧
	}

	isPartialD4 := false
	if exitProfile == "scalp" || exitProfile == "trend" {
		isPartialD4 = true // scalp/trend 模式下硬止损也可能是半仓（保持与旧逻辑一致）
	}

	// 建仓前3天的收盘兜底保护
	if hs.daysHeld <= 3 && currentGainPct <= earlyDropLimit*100 {
		return true, fmt.Sprintf("建仓初期收盘跌破%.0f%%硬止损", earlyDropLimit*100), isPartialD4
	}

	// 常规盘中破位
	if currentGainPct <= hardStop*100 {
		return true, fmt.Sprintf("常规盘中破位%.0f%%硬止损", hardStop*100), isPartialD4
	}

	return false, "", false
}
