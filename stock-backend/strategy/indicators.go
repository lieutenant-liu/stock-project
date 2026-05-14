package strategy

import (
	"math"
	"sort"
	"stock-backend/tushare"
)

// CalcMA 计算指定位置前 N 天的收盘价均线
func CalcMA(history []tushare.DailyKLine, days int) float64 {
	if len(history) < days {
		return 0
	}
	total := 0.0
	startIndex := len(history) - days
	for i := startIndex; i < len(history); i++ {
		total += history[i].Close
	}
	return total / float64(days)
}

// CalcMAFromData 计算 IndexDaily 序列最近 N 天的收盘价均线。
func CalcMAFromData(data []tushare.IndexDaily, days int) float64 {
	if len(data) < days {
		return 0
	}
	total := 0.0
	start := len(data) - days
	for i := start; i < len(data); i++ {
		total += data[i].Close
	}
	return total / float64(days)
}

// CalcVolMA 计算指定位置前 N 天的成交量均线
func CalcVolMA(history []tushare.DailyKLine, days int) float64 {
	if len(history) < days {
		return 0
	}
	total := 0.0
	startIndex := len(history) - days
	for i := startIndex; i < len(history); i++ {
		total += history[i].Vol
	}
	return total / float64(days)
}

// GetBox 计算前 N 天（不含今天）的箱体最高价和最低价
func GetBox(history []tushare.DailyKLine, days int) (float64, float64) {
	if len(history) <= days {
		return 0, 0
	}
	high := 0.0
	low := math.MaxFloat64
	// 注意：不包含最后一天（今天）
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		if history[i].High > high {
			high = history[i].High
		}
		if history[i].Low < low {
			low = history[i].Low
		}
	}
	return high, low
}

// CalcATR 计算过去 N 天的真实波幅 (Average True Range)
func CalcATR(history []tushare.DailyKLine, days int) float64 {
	if len(history) <= days {
		return 0
	}
	totalTR := 0.0
	startIndex := len(history) - days

	for i := startIndex; i < len(history); i++ {
		h := history[i].High
		l := history[i].Low
		var pc float64
		if i > 0 {
			pc = history[i-1].Close
		} else {
			pc = history[i].PreClose // 兜底
		}

		// TR = max(H-L, abs(H-Pc), abs(L-Pc))
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		totalTR += tr
	}
	return totalTR / float64(days)
}

// CheckConvergence 判定是否连续 T 天满足均线收敛
func CheckConvergence(history []tushare.DailyKLine, days int, threshold float64) bool {
	if len(history) < 120+days {
		return false
	}
	// 往前推 T 天，每天都必须满足收敛
	for i := len(history) - days; i < len(history); i++ {
		// 切片截取到当天的历史
		subHistory := history[:i+1]
		ma30 := CalcMA(subHistory, 30)
		ma60 := CalcMA(subHistory, 60)
		ma120 := CalcMA(subHistory, 120)

		maxMA := math.Max(ma30, math.Max(ma60, ma120))
		minMA := math.Min(ma30, math.Min(ma60, ma120))

		if minMA <= 0 {
			return false
		}

		convergence := (maxMA - minMA) / minMA
		if convergence > threshold {
			return false // 只要有一天不满足，就说明没有持续纠缠
		}
	}
	return true
}

// IsLongUpperShadow 判定是否出现高位长上影线 (派发信号)
// 上影线长度大于实体长度的 N 倍
func IsLongUpperShadow(k tushare.DailyKLine, times float64) bool {
	bodyTop := math.Max(k.Open, k.Close)
	bodyBottom := math.Min(k.Open, k.Close)

	upperShadow := k.High - bodyTop
	body := bodyTop - bodyBottom

	// 防止一字板除数为空
	if body == 0 {
		return upperShadow > 0
	}
	return upperShadow > (body * times)
}

// GetRealBox 方案B：统计学剥离毛刺法 (寻找真实的筹码震荡箱体)
func GetRealBox(history []tushare.DailyKLine, days int) (float64, float64) {
	if len(history) <= days {
		return 0, 0
	}

	// 1. 提取过去 N 天（不含今天）的开盘价和收盘价实体
	var prices []float64
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		// 我们只取实体部分(Open和Close)，直接抛弃极其容易骗线的上下影线(High和Low)
		prices = append(prices, history[i].Open)
		prices = append(prices, history[i].Close)
	}

	// 2. 将价格从小到大排序
	sort.Float64s(prices)

	// 3. 统计学剥离：砍掉顶部 10% 的诱多极值，和底部 10% 的诱空极值
	// 这样留下的，就是 80% 核心筹码沉淀的真实中枢
	totalLen := len(prices)
	lowerIndex := int(float64(totalLen) * 0.10) // 10% 分位数
	upperIndex := int(float64(totalLen) * 0.90) // 90% 分位数

	// 防越界保护
	if lowerIndex < 0 {
		lowerIndex = 0
	}
	if upperIndex >= totalLen {
		upperIndex = totalLen - 1
	}

	boxLower := prices[lowerIndex]
	boxUpper := prices[upperIndex]

	return boxUpper, boxLower
}

// ==========================================
// V3.3 宏观调整 1：箱体规律性测算 (拒绝单边下跌途中的伪箱体)
// ==========================================
func CheckBoxRegularity(history []tushare.DailyKLine, days int) bool {
	if len(history) < days+30 {
		return false
	}
	// 提取箱体期间的 MA30 均线极值
	maxMA, minMA := 0.0, math.MaxFloat64
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		ma30 := CalcMA(history[:i+1], 30)
		if ma30 > maxMA {
			maxMA = ma30
		}
		if ma30 < minMA {
			minMA = ma30
		}
	}
	// 如果这 N 天内，30日均线的上下波动超过了 15%，说明根本不是横盘震荡，而是处于剧烈趋势中
	if minMA > 0 && (maxMA-minMA)/minMA > 0.15 {
		return false
	}
	return true
}

// ==========================================
// V3.3 宏观调整 2：资金试探检测 (寻找仙人指路或未遂涨停)
// ==========================================
func HasProbingAction(history []tushare.DailyKLine, boxUpper float64, lookbackDays int) bool {
	if len(history) <= lookbackDays {
		return false
	}
	startIndex := len(history) - 1 - lookbackDays
	for i := startIndex; i < len(history)-1; i++ {
		k := history[i]
		// 1. 曾经触及过涨停价附近 (涨幅 > 8%)
		// 2. 最高价曾极为逼近箱体上轨 (距离 < 3%)
		// 3. 但最终没有形成有效突破 (收盘价被打回)
		if k.PctChg > 8.0 && math.Abs(k.High-boxUpper)/boxUpper < 0.03 && k.Close <= boxUpper {
			return true // 发现明确的资金试探/洗盘痕迹！
		}
	}
	return false
}

// ==========================================
// V3.3 宏观调整 3：统一大级别压力位防雷网 (计算上方真空区)
// ==========================================
func GetOverheadRoom(history []tushare.DailyKLine, currentPrice float64, lookbackDays int) float64 {
	// P1: 数据不足时扫描全部可用历史，而非直接返回 1.0 绕过过滤
	actualLookback := lookbackDays
	if len(history) < lookbackDays {
		actualLookback = len(history)
	}
	if actualLookback <= 1 || currentPrice <= 0 {
		return 1.0
	}
	resistance := 0.0
	startIndex := len(history) - actualLookback
	for i := startIndex; i < len(history)-1; i++ {
		if history[i].High > resistance {
			resistance = history[i].High
		}
	}
	if resistance > currentPrice {
		return (resistance - currentPrice) / currentPrice
	}
	return 1.0 // 创出区间新高，上方无套牢盘
}

// ForwardAdjustKLines 前复权清洗器：修复除权除息导致的价格断层
func ForwardAdjustKLines(klines []tushare.DailyKLine, factors []tushare.AdjFactor) []tushare.DailyKLine {
	if len(klines) == 0 || len(factors) == 0 {
		return klines
	}

	// 1. 构建 O(1) 查询的哈希表
	factorMap := make(map[string]float64)
	for _, f := range factors {
		factorMap[f.TradeDate] = f.AdjFactor
	}

	// 2. 寻找基准锚点：最新交易日的复权因子
	latestDate := klines[len(klines)-1].TradeDate
	latestFactor, exists := factorMap[latestDate]
	if !exists {
		latestFactor = factors[len(factors)-1].AdjFactor // 降级兜底：取提供的因子列表最后一个
	}
	if latestFactor == 0 {
		latestFactor = 1.0 // 防除零崩溃
	}

	// 3. 执行前复权数学变换
	adjustedKLines := make([]tushare.DailyKLine, len(klines))
	for i, k := range klines {
		adjustedKLines[i] = k
		f, ok := factorMap[k.TradeDate]
		if !ok {
			f = latestFactor // 如果当天没因子数据，假设没有发生除权，继承最新因子
		}

		// 前复权乘数
		ratio := f / latestFactor

		// 价格等比缩小
		adjustedKLines[i].Open = k.Open * ratio
		adjustedKLines[i].Close = k.Close * ratio
		adjustedKLines[i].High = k.High * ratio
		adjustedKLines[i].Low = k.Low * ratio
		adjustedKLines[i].PreClose = k.PreClose * ratio
		adjustedKLines[i].Change = k.Change * ratio // 💥 补上这一行！

		// 💥 注意：价格缩小了，成交量必须等比放大，才能保证总成交额 (Amount) 不变，换手率计算才准确！
		if ratio > 0 {
			adjustedKLines[i].Vol = k.Vol / ratio
		}
	}

	return adjustedKLines
}
