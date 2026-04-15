package strategy

import (
	"fmt"
	"math"
	"stock-backend/tushare"
	"strings"
)

// ==========================================
// 🛡️ 全局公共基建：基本面防暴雷护盾 (升维版)
// ==========================================
func checkFundamentalShield(ctx *SecurityContext, strictMode bool) bool {
	funds := ctx.Fundamentals
	if len(funds) == 0 {
		return false
	}
	latestFund := funds[len(funds)-1]

	// 基础排雷线：亏损公司，或者历史估值分位处于 85% 以上的绝对泡沫区
	isGarbage := latestFund.PE <= 0 || ctx.PEPercentile > 0.85

	if strictMode {
		// 严格模式：PE 必须处于历史低/中水位 (<60%)，且不是亏损股
		if latestFund.PE <= 0 || ctx.PEPercentile > 0.60 {
			return false
		}
		return true
	}

	// 普通模式：只要不是亏损且极度泡沫即可
	if isGarbage && latestFund.DVRatio < 1.0 { // 除非股息率兜底
		return false
	}

	return true
}

// ==========================================
// 🛡️ 策略一：均线收敛突破 (MACB - 严谨重构版)
// ==========================================
type MACBAnalyzer struct{}

func (m *MACBAnalyzer) Name() string { return "均线收敛突破 (MACB)" }
func (m *MACBAnalyzer) RequiredData() []string {
	return []string{"klines", "fundamentals", "moneyflow"}
} // 💥 补充了 moneyflow

func (m *MACBAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	funds := ctx.Fundamentals
	if len(klines) < 130 { // 需要120天算均线，外加几天算收敛持续期
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// -----------------------------------------------------
	// 1. 基本面过滤引擎 (Fundamental Filter)
	// -----------------------------------------------------
	if len(funds) == 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	latestFund := funds[len(funds)-1]

	// 核心逻辑：买入前，估值必须处于历史合理区间 (分位数 < 80%)
	// 允许高绝对值 PE (成长股)，但如果是亏损(PE <= 0)或极度历史泡沫(分位 > 80%)，拒绝买入。
	if latestFund.PE <= 0 || ctx.PEPercentile > 0.80 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	today := klines[len(klines)-1]
	yesterday := klines[len(klines)-2]

	// -----------------------------------------------------
	// 2. 技术面买点引擎 (Buy Signal: Convergence + Breakout)
	// -----------------------------------------------------
	isConverged := CheckConvergence(klines, 5, 0.04)

	pctChgReal := (today.Close - yesterday.Close) / yesterday.Close * 100
	ma30 := CalcMA(klines, 30)
	ma60 := CalcMA(klines, 60)
	ma120 := CalcMA(klines, 120)
	maxMA := math.Max(ma30, math.Max(ma60, ma120))
	minMA := math.Min(ma30, math.Min(ma60, ma120))
	volMa20 := CalcVolMA(klines, 20)

	isYangLine := today.Close > today.Open              // 拒绝高开低走的假阴线
	hasUpperShadowRisk := IsLongUpperShadow(today, 1.5) // 拒绝避雷针

	isBreakout := pctChgReal >= 5.0 && today.Close > maxMA && isYangLine && !hasUpperShadowRisk
	isVolumeSurge := today.Vol > (2.0 * volMa20)

	// 💥 宏观调整 3：接入月线防雷网！就算均线收敛，头顶也不能有大山
	room := GetOverheadRoom(klines, today.Close, 250) // 均线突破看长一点，看250天(年线)压力
	if room < 0.20 {
		return DiagnoseResult{Signal: "观望 💤"} // 上方 20% 内有年线级别的套牢盘，不撞墙！
	}

	// -----------------------------------------------------
	// 3. 卖出信号锚定与【机构级风控护盾】
	// -----------------------------------------------------
	if isConverged && isBreakout && isVolumeSurge {
		// =====================================================
		// 💥 [机构级护盾：流动性、市值与资金底牌透视]
		// =====================================================
		if latestFund.TotalMV < 300000 {
			return DiagnoseResult{Signal: "观望 💤"} // 剔除30亿以下微盘股
		}
		if latestFund.TurnoverRate < 4.0 || latestFund.TurnoverRate > 25.0 {
			return DiagnoseResult{Signal: "观望 💤"} // 换手率异常过滤
		}

		flows := ctx.MoneyFlows
		if len(flows) == 0 {
			return DiagnoseResult{Signal: "观望 💤"} // 缺失资金流向数据
		}
		todayFlow := flows[len(flows)-1]
		if todayFlow.NetMfVol <= 0 {
			return DiagnoseResult{Signal: "观望 💤"} // 突破日主力在出逃！一票否决
		}
		netMfWan := todayFlow.NetMfVol * 10000 // 换算为万元
		// =====================================================

		var sellPrice, stopLossPrice float64
		var msg string

		// 💥 动态止盈止损：如果当前估值处于历史 60% 以上的高水位，说明属于偏右侧投机，收紧防线！
		if ctx.PEPercentile > 0.60 {
			msg = fmt.Sprintf("⚠️ [估值分位:%.1f%%] 历史水位偏高！但均线收敛且大阳线突破，主力净流入 %.0f 万。只能做短线，跌破半年线立即逃跑！",
				ctx.PEPercentile*100, netMfWan)
			stopLossPrice = ma120
			sellPrice = today.Close * 1.10
		} else {
			msg = fmt.Sprintf("🎯 [估值分位:%.1f%%|市值:%.0f亿] 完美买点！处于历史低估区，均线高度纠缠，主力暴力净买入 %.0f 万元！中线看涨。",
				ctx.PEPercentile*100, latestFund.TotalMV/10000, netMfWan)
			stopLossPrice = minMA
			sellPrice = 0
		}

		return DiagnoseResult{
			Code: code, StrategyName: m.Name(), LatestPrice: today.Close,
			Signal: "买入 🚀", Message: msg,
			BuyPrice: today.Close, SellPrice: sellPrice, StopLossPrice: stopLossPrice,
		}
	}

	return DiagnoseResult{Signal: "观望 💤"}
}

// ==========================================
// 🔥 策略二：基本面共振·中枢强势突破 (CBBM - 终极状态机版)
// 替代原有的粗暴打板流(BBLU)
// ==========================================
type CBBMAnalyzer struct{}

func (c *CBBMAnalyzer) Name() string { return "中枢强势突破 (CBBM)" }
func (c *CBBMAnalyzer) RequiredData() []string {
	return []string{"klines", "fundamentals", "moneyflow"}
} // 💥 补充了 moneyflow

func (c *CBBMAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	funds := ctx.Fundamentals
	if len(klines) < 120 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if strings.HasPrefix(code, "3") || strings.HasPrefix(code, "688") || strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if !checkFundamentalShield(ctx, false) {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	today := klines[len(klines)-1]

	// -----------------------------------------------------
	// [微观排雷网]
	// -----------------------------------------------------
	isLimitUp := today.PctChg >= 9.5
	isOneWordBoard := today.Open == today.Close && today.Close == today.High
	hasUpperShadow := IsLongUpperShadow(today, 1.5)

	volMa20 := CalcVolMA(klines, 20)
	isVolumeSurge := today.Vol >= (1.5 * volMa20)

	if !isLimitUp || isOneWordBoard || hasUpperShadow || !isVolumeSurge {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	if len(klines) >= 6 {
		startJumpPrice := klines[len(klines)-6].Close
		if (today.Close-startJumpPrice)/startJumpPrice > 0.15 {
			return DiagnoseResult{Signal: "观望 💤"}
		}
	}

	// -----------------------------------------------------
	// [宏观调整 1 & 2]：箱体规律与试盘基因
	// -----------------------------------------------------
	boxUpper, boxLower := GetRealBox(klines, 60)
	if boxLower <= 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	amplitude := (boxUpper - boxLower) / boxLower
	isBoxStable := amplitude <= 0.35
	isBreakout := today.Close > boxUpper
	isCloseToBox := (today.Close-boxUpper)/boxUpper <= 0.15
	isRegularBox := CheckBoxRegularity(klines, 60)

	if !isBoxStable || !isBreakout || !isCloseToBox || !isRegularBox {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	hasProbed := HasProbingAction(klines, boxUpper, 20)

	// -----------------------------------------------------
	// [宏观调整 3]：统一月线级别防雷网
	// -----------------------------------------------------
	room := GetOverheadRoom(klines, today.Close, 120)
	if room < 0.15 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// -----------------------------------------------------
	// [宏观调整 4]：交易剧本重构与【机构级护盾】
	// -----------------------------------------------------
	latestFund := funds[len(funds)-1]

	// =====================================================
	// 💥 [机构级护盾：流动性、市值与资金底牌透视]
	// =====================================================
	if latestFund.TotalMV < 300000 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if latestFund.TurnoverRate < 4.0 || latestFund.TurnoverRate > 25.0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	flows := ctx.MoneyFlows
	if len(flows) == 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	todayFlow := flows[len(flows)-1]
	if todayFlow.NetMfVol <= 0 {
		return DiagnoseResult{Signal: "观望 💤"} // 无主力资金净流入，属于跟风或诱多
	}
	netMfWan := todayFlow.NetMfVol * 10000
	// =====================================================

	geneMsg := ""
	if hasProbed {
		geneMsg = "🎯 侦测到近期【主力试盘洗盘】动作，突破可信度极高！"
	}

	roomMsg := "已突破近半年高点"
	if room < 1.0 {
		roomMsg = fmt.Sprintf("上方真空区约 %.1f%%", room*100)
	}

	msg := fmt.Sprintf("🔥 [估值分位:%.1f%%|市值:%.0f亿] 放量真突破！主力大单净流入 %.0f 万元！箱体规律(振幅%.1f%%)。%s %s\n"+
		"【明日剧本】绝不盲目追高！\n"+
		"1. 若低开下杀，在支撑位(%.2f)附近企稳买入。\n"+
		"2. 若平开出小阳线，下午确认承接有力后轻仓打底。\n"+
		"3. 若大幅跳空高开，放弃买入防砸盘。",
		ctx.PEPercentile*100, latestFund.TotalMV/10000, netMfWan, amplitude*100, roomMsg, geneMsg, boxUpper)

	return DiagnoseResult{
		Code: code, StrategyName: c.Name(), LatestPrice: today.Close,
		Signal: "买入 🚀", Message: msg,
		BuyPrice:      boxUpper,
		SellPrice:     today.Close * (1.0 + room*0.8),
		StopLossPrice: boxUpper * 0.97,
	}
}

// ==========================================
// 🌊 策略三：深海狙击手 2.0 (DSS V2 - EOD 批处理模型)该算法存在重大问题，暂时不要使用
// ==========================================
type DSSAnalyzer struct{}

func (d *DSSAnalyzer) Name() string           { return "深海狙击手 2.0 (DSS)" }
func (d *DSSAnalyzer) RequiredData() []string { return []string{"klines", "fundamentals"} }

func (d *DSSAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	funds := ctx.Fundamentals
	// 引擎一：基本面重力引擎 (大级别防雷需要至少 250 天数据)
	if len(klines) < 250 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 绝对安全池：调用严格模式，过滤亏损及历史估值高位的泡沫股
	if !checkFundamentalShield(ctx, true) {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	latestFund := funds[len(funds)-1]
	today := klines[len(klines)-1]

	// 大级别防雷测算：寻找上方 250 天内的历史最高点作为超级阻力位
	resistance250 := 0.0
	scanStart := len(klines) - 250
	scanEnd := len(klines) - 1 // 不含今天
	for i := scanStart; i < scanEnd; i++ {
		if klines[i].High > resistance250 {
			resistance250 = klines[i].High
		}
	}

	// 计算真空区：要求上方至少有 30% 的无阻力空间，否则不参与底部的内卷
	room := (resistance250 - today.Close) / today.Close
	if resistance250 > today.Close && room < 0.30 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 引擎二：深海潜伏引擎 (形态、价格、流动性三维共振)
	boxUpper, boxLower := GetRealBox(klines, 60)
	if boxLower <= 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	amplitude := (boxUpper - boxLower) / boxLower
	volMa20 := CalcVolMA(klines, 20)
	volMa60 := CalcVolMA(klines, 60)
	atr14 := CalcATR(klines, 14)

	// 1. 形态收敛：限制上下波动的幅度 (<=30%)，证明主力处于控盘休眠期
	isSpaceCompressed := amplitude <= 0.30

	// 2. 价格极寒：当前价格处于箱体下方的 30% 区域，且不能跌破绝对底线 (未破位崩盘)
	limitPrice := boxLower + 0.30*(boxUpper-boxLower)
	isPriceAtBottom := today.Close <= limitPrice && today.Close > boxLower

	// 3. 流动性枯竭 (灵魂指标)：当天成交量既远小于 20日均量，也远小于 60日均量 (长期资金也睡着了)
	isLiquidityDry := today.Vol < (0.5*volMa20) && today.Vol < (0.5*volMa60)

	if isSpaceCompressed && isPriceAtBottom && isLiquidityDry {
		msg := fmt.Sprintf("⚓ 深海潜伏！当前地量(20日均量%.0f%%, 60日均量%.0f%%)，价格极寒逼近箱底。"+
			"[PE:%.2f] 且上方拥有 %.1f%% 真空区！明日可从容挂单，等待右侧资金抬轿。",
			(today.Vol/volMa20)*100, (today.Vol/volMa60)*100, latestFund.PE, room*100)

		// 引擎三：双轨逃生引擎 (执行路由器)
		return DiagnoseResult{
			Code: code, StrategyName: d.Name(), LatestPrice: today.Close,
			Signal: "买入 🚀", Message: msg,
			BuyPrice:      today.Close,          // 左侧潜伏：直接在 C_t 附近从容挂单
			SellPrice:     boxUpper * 0.99,      // 狂热派发：触及箱体顶部(H_60)回落 1% 卖出，倒给突破客
			StopLossPrice: boxLower - 1.5*atr14, // 防核按钮：L_60 减去 1.5 倍 ATR 动态防线，跌破无条件斩仓！
		}
	}

	return DiagnoseResult{Signal: "观望 💤"}
}

// ==========================================
// 🛡️ 全局风控中心：大盘 Beta 熔断检测
// ==========================================

// CheckMarketEnvironment 评估大盘环境与短线情绪，返回 (是否安全, 诊断报告)
func CheckMarketEnvironment(indices []tushare.IndexDaily, limitUpCount int, avgPremium float64) (bool, string) {
	if len(indices) < 25 {
		return true, "大盘数据不足，全局风控默认放行。"
	}

	today := indices[len(indices)-1]

	// 1. 暴跌熔断：大盘单日暴跌
	if today.PctChg <= -1.5 {
		return false, fmt.Sprintf("⚠️ 全局熔断：上证指数今日暴跌 %.2f%%！倾巢之下无完卵，严禁逆势开仓！", today.PctChg)
	}

	// 2. 趋势熔断：大盘跌破 20日线且向下拐头
	var sum20, sumPrev20 float64
	for i := len(indices) - 20; i < len(indices); i++ {
		sum20 += indices[i].Close
	}
	for i := len(indices) - 21; i < len(indices)-1; i++ {
		sumPrev20 += indices[i].Close
	}
	ma20 := sum20 / 20.0
	prevMa20 := sumPrev20 / 20.0

	if today.Close < ma20 && ma20 < prevMa20 {
		return false, "⚠️ 全局熔断：上证指数跌破 20日线 且趋势向下，处于单边空头区间，停止一切突破买入！"
	}

	// =========================================================
	// 💥 3. 情绪退潮熔断 (2000积分高阶风控)
	// =========================================================
	if limitUpCount > 0 {
		if avgPremium <= -2.0 {
			// 昨天打板的人今天平均亏 2% 以上，说明核按钮遍地，极端恶劣！
			return false, fmt.Sprintf("🧊 情绪冰点熔断：昨日 %d 只涨停股今日平均大跌 %.2f%%！核按钮遍地，短线接力极度恶劣，管住手！", limitUpCount, avgPremium)
		} else if avgPremium < 0 {
			// 负溢价，打板资金没赚钱，短线情绪退潮，假突破极多！
			return false, fmt.Sprintf("⚠️ 情绪退潮熔断：昨日 %d 只涨停股今日平均收益为 %.2f%%。接力资金在亏钱，市场大概率是骗炮行情，暂缓开仓！", limitUpCount, avgPremium)
		}
	}

	premiumMsg := "情绪数据暂缺"
	if limitUpCount > 0 {
		premiumMsg = fmt.Sprintf("昨日涨停股今日平均溢价(吃肉率)为 %.2f%%", avgPremium)
	}

	return true, fmt.Sprintf("✅ 大盘与情绪健康 (指数涨跌: %.2f%%，%s)，允许个股引擎开火。", today.PctChg, premiumMsg)
}

// ==========================================
// ⚙️ 军师联盟注册中心 (Registry - 更新挂载)
// ==========================================
func GetActiveAnalyzers() []Analyzer {
	return []Analyzer{
		&MACBAnalyzer{}, // 均线收敛 (策略一)
		&CBBMAnalyzer{}, // 中枢强势突破 (策略二，替换原 BBLU)
		// &DSSAnalyzer{},  // ⚠️ 深海狙击手 (策略三，存在重大问题，暂时下线)
	}
}
