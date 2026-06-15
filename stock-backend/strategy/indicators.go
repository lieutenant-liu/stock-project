// ============================================================
// indicators.go —— 技术指标计算库
// 本文件包含所有量化策略共用的技术指标计算函数。
// 技术指标是从 K 线数据（开盘价、收盘价、最高价、最低价、成交量）
// 中提取的数学特征，用于判断股票的趋势、波动性、超买超卖等状态。
// ============================================================

package strategy

import (
	"math"  // 数学函数库，提供 Max、Min、Abs 等
	"sort"  // 排序库，提供 Float64s 切片排序
	"stock-backend/tushare" // 外部数据结构定义
)

// ----------------------------------------------------------
// CalcMA —— 计算简单移动平均线 (Simple Moving Average, SMA)
//
// 什么是均线？
//   均线是将过去 N 天的收盘价求平均值，得到一条平滑曲线。
//   MA5 = 最近5天收盘价的平均值，MA30 = 最近30天的平均值。
//   均线越长，趋势越平滑；均线越短，对价格变化越敏感。
//   当短期均线向上穿越长期均线时，称为"金叉"，是买入信号；
//   当短期均线向下穿越长期均线时，称为"死叉"，是卖出信号。
//
// 参数：
//   history - 日 K 线数据切片（slice），需要有足够的数据量
//   days    - 均线周期，如 5、10、30、60、120
// 返回值：
//   float64 - 均线值；数据不足时返回 0
// ----------------------------------------------------------
func CalcMA(history []tushare.DailyKLine, days int) float64 {
	// 数据不足时直接返回 0，防止数组越界
	if len(history) < days {
		return 0
	}
	total := 0.0
	// 从倒数第 days 天开始累加收盘价
	// 例如 days=5, len=10，则从 history[5] 累加到 history[9]
	startIndex := len(history) - days
	for i := startIndex; i < len(history); i++ {
		total += history[i].Close // Close = 收盘价
	}
	// 总和除以天数，得到平均值
	// float64(days) 是类型转换：Go 不允许 int 和 float64 直接运算
	return total / float64(days)
}

// ----------------------------------------------------------
// CalcMAFromData —— 计算指数日线数据的收盘价均线
// 功能与 CalcMA 相同，但输入是 IndexDaily 类型（用于大盘指数）。
// Go 函数不支持重载（同名不同参数），所以用不同的函数名。
// ----------------------------------------------------------
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

// ----------------------------------------------------------
// CalcVolMA —— 计算成交量移动平均线
//
// 什么是成交量均线？
//   与价格均线类似，但用成交量（Vol）代替收盘价。
//   用于判断当前成交量是否异常放大或萎缩。
//   当今日成交量 > 2 * MA20Vol 时，称为"放量"，可能预示重大变盘。
//   当今日成交量 < 0.5 * MA20Vol 时，称为"缩量"，市场观望情绪浓。
// ----------------------------------------------------------
func CalcVolMA(history []tushare.DailyKLine, days int) float64 {
	if len(history) < days {
		return 0
	}
	total := 0.0
	startIndex := len(history) - days
	for i := startIndex; i < len(history); i++ {
		total += history[i].Vol // Vol = 成交量（手）
	}
	return total / float64(days)
}

// ----------------------------------------------------------
// GetBox —— 计算 N 天箱体的最高价和最低价
//
// 什么是箱体（Box）？
//   箱体是一种价格震荡形态：股价在一段时间内反复在某个高点和低点之间波动，
//   形成一个"箱子"一样的区间。箱体上沿叫"阻力位"，下沿叫"支撑位"。
//   当股价突破箱体上沿时，可能开启上涨行情；跌破下沿时，可能继续下跌。
//
// 注意：计算时不包含最后一天（今天），因为今天可能是突破日。
// 返回值：(最高价, 最低价)
// ----------------------------------------------------------
func GetBox(history []tushare.DailyKLine, days int) (float64, float64) {
	if len(history) <= days {
		return 0, 0
	}
	high := 0.0
	low := math.MaxFloat64 // 初始化为 float64 的最大值，确保任何价格都能替换它
	// 不包含最后一天（今天）：startIndex 到 len-2
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		if history[i].High > high {
			high = history[i].High // High = 最高价
		}
		if history[i].Low < low {
			low = history[i].Low // Low = 最低价
		}
	}
	return high, low
}

// ----------------------------------------------------------
// CalcATR —— 计算平均真实波幅 (Average True Range)
//
// 什么是 ATR？
//   ATR 是衡量股票价格波动幅度的指标。
//   真实波幅 TR = max(当日最高-最低, |当日最高-昨收|, |当日最低-昨收|)
//   ATR 就是过去 N 天 TR 的平均值。
//   ATR 越大，说明股票波动越剧烈（妖股）；ATR 越小，波动越温和。
//   ATR 常用于动态止损：止损距离 = ATR * 倍数，波动大的股票止损更宽。
//
// 参数：
//   days - 计算周期（通常用 14 天 ATR）
// ----------------------------------------------------------
func CalcATR(history []tushare.DailyKLine, days int) float64 {
	if len(history) <= days {
		return 0
	}
	totalTR := 0.0
	startIndex := len(history) - days

	for i := startIndex; i < len(history); i++ {
		h := history[i].High  // 当日最高价
		l := history[i].Low   // 当日最低价
		var pc float64        // 前一日收盘价 (Previous Close)
		if i > 0 {
			pc = history[i-1].Close
		} else {
			pc = history[i].PreClose // 兜底：如果 i=0 没有前一天数据，用 PreClose 字段
		}

		// TR = 三者中的最大值：
		//   1. H - L          : 当日振幅（最高减最低）
		//   2. |H - Pc|       : 当日最高与昨收的差距（向上跳空）
		//   3. |L - Pc|       : 当日最低与昨收的差距（向下跳空）
		// 取最大值是为了捕捉跳空缺口带来的额外波动
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		totalTR += tr
	}
	// 除以天数得到平均值
	return totalTR / float64(days)
}

// ----------------------------------------------------------
// CheckConvergence —— 判定均线是否连续 T 天收敛
//
// 什么是均线收敛？
//   当 MA30、MA60、MA120 三条均线非常接近时，称为"收敛"或"纠缠"。
//   收敛意味着多空力量均衡，市场处于蓄势阶段。
//   收敛之后往往会选择方向突破：向上突破是买入信号，向下是卖出信号。
//
// 计算方式：
//   convergence = (最大均线 - 最小均线) / 最小均线
//   当 convergence < threshold（如 4%）时，认为均线收敛。
//   本函数要求连续 T 天都满足收敛条件，说明收敛是稳定的而非偶然。
//
// 参数：
//   days      - 要求连续收敛的天数
//   threshold - 收敛阈值（如 0.04 表示 4%）
// ----------------------------------------------------------
func CheckConvergence(history []tushare.DailyKLine, days int, threshold float64) bool {
	// 需要至少 120 天数据来计算 MA120，再加 days 天来检查连续性
	if len(history) < 120+days {
		return false
	}
	// 往前推 T 天，每天都必须满足收敛
	for i := len(history) - days; i < len(history); i++ {
		// history[:i+1] 是切片操作：截取从开头到第 i 天（含）的所有数据
		subHistory := history[:i+1]
		ma30 := CalcMA(subHistory, 30)
		ma60 := CalcMA(subHistory, 60)
		ma120 := CalcMA(subHistory, 120)

		// 找出三条均线中的最大值和最小值
		maxMA := math.Max(ma30, math.Max(ma60, ma120))
		minMA := math.Min(ma30, math.Min(ma60, ma120))

		if minMA <= 0 {
			return false // 防除零
		}

		// 计算收敛程度：(最大 - 最小) / 最小
		convergence := (maxMA - minMA) / minMA
		if convergence > threshold {
			return false // 只要有一天不满足，就说明没有持续纠缠
		}
	}
	return true // 连续 T 天全部满足收敛
}

// ----------------------------------------------------------
// IsLongUpperShadow —— 判定是否出现长上影线（派发/抛压信号）
//
// 什么是上影线？
//   K 线实体（开盘价到收盘价之间的矩形）上方的细线叫上影线。
//   上影线 = 最高价 - max(开盘价, 收盘价)
//   长上影线意味着股价冲高后被打回，上方抛压沉重。
//   如果上影线长度是实体的 N 倍以上，说明主力可能在高位派发（出货）。
//
// 参数：
//   k     - 单根 K 线数据
//   times - 上影线相对于实体的倍数阈值（如 1.5 表示 1.5 倍）
// ----------------------------------------------------------
func IsLongUpperShadow(k tushare.DailyKLine, times float64) bool {
	// 实体顶部 = max(开盘, 收盘)，实体底部 = min(开盘, 收盘)
	// 这样无论阳线还是阴线都能正确计算实体
	bodyTop := math.Max(k.Open, k.Close)
	bodyBottom := math.Min(k.Open, k.Close)

	upperShadow := k.High - bodyTop   // 上影线长度
	body := bodyTop - bodyBottom       // 实体长度

	// 防止一字板（开盘=收盘=最高=最低）时除数为零
	if body == 0 {
		return upperShadow > 0 // 一字板只要有上影线就算
	}
	// 上影线长度 > 实体长度 * 倍数阈值
	return upperShadow > (body * times)
}

// ----------------------------------------------------------
// GetRealBox —— 统计学方法寻找真实的筹码震荡箱体
//
// 与 GetBox 的区别：
//   GetBox 直接用最高/最低价，容易被极端影线（"毛刺"）误导。
//   GetRealBox 只取开盘价和收盘价（K 线实体部分），忽略上下影线，
//   然后用统计学方法砍掉顶部 10% 和底部 10% 的极端值，
//   取中间 80% 的价格区间作为"真实箱体"。
//   这样得到的箱体更能反映大多数筹码的交易区间。
//
// 算法步骤：
//   1. 收集过去 N 天所有 K 线的 Open 和 Close 价格
//   2. 将价格从小到大排序
//   3. 砍掉最低 10% 和最高 10%
//   4. 剩余的最低价 = 箱体下沿，最高价 = 箱体上沿
//
// 返回值：(箱体上沿, 箱体下沿)
// ----------------------------------------------------------
func GetRealBox(history []tushare.DailyKLine, days int) (float64, float64) {
	if len(history) <= days {
		return 0, 0
	}

	// 步骤 1：提取过去 N 天（不含今天）的开盘价和收盘价
	var prices []float64
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		// 只取实体部分(Open 和 Close)，抛弃容易骗线的上下影线(High 和 Low)
		prices = append(prices, history[i].Open)
		prices = append(prices, history[i].Close)
	}

	// 步骤 2：将价格从小到大排序
	sort.Float64s(prices) // Go 标准库提供的切片排序函数

	// 步骤 3：统计学剥离 —— 砍掉顶部 10% 和底部 10% 的极端值
	totalLen := len(prices)
	lowerIndex := int(float64(totalLen) * 0.10) // 10% 分位数索引
	upperIndex := int(float64(totalLen) * 0.90) // 90% 分位数索引

	// 防越界保护
	if lowerIndex < 0 {
		lowerIndex = 0
	}
	if upperIndex >= totalLen {
		upperIndex = totalLen - 1
	}

	// 步骤 4：取 80% 核心区间
	boxLower := prices[lowerIndex] // 箱体下沿（10% 分位价格）
	boxUpper := prices[upperIndex] // 箱体上沿（90% 分位价格）

	return boxUpper, boxLower
}

// ----------------------------------------------------------
// CheckBoxRegularity —— 箱体规律性验证
//
// 目的：拒绝"假箱体"。
//   在单边下跌趋势中，股价也可能形成看似横盘的区间，
//   但其实 30 日均线一直在往下走，这不是真正的横盘震荡。
//   本函数通过检查 30 日均线在整个箱体期间的波动幅度来判断：
//   如果 MA30 波动超过 15%，说明处于剧烈趋势中，不是真正的箱体。
//
// 参数：
//   days - 箱体观察期天数
// ----------------------------------------------------------
func CheckBoxRegularity(history []tushare.DailyKLine, days int) bool {
	// 需要 days + 30 天数据（箱体期 + MA30 计算窗口）
	if len(history) < days+30 {
		return false
	}
	// 在箱体期间逐日计算 MA30，记录其最大值和最小值
	maxMA, minMA := 0.0, math.MaxFloat64
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		// history[:i+1] = 截取到第 i 天的全部历史，用于计算当天的 MA30
		ma30 := CalcMA(history[:i+1], 30)
		if ma30 > maxMA {
			maxMA = ma30
		}
		if ma30 < minMA {
			minMA = ma30
		}
	}
	// MA30 的波动幅度 = (最高 - 最低) / 最低
	// 如果超过 15%，说明不是横盘震荡，而是处于剧烈趋势中
	if minMA > 0 && (maxMA-minMA)/minMA > 0.15 {
		return false
	}
	return true // MA30 平稳，箱体有效
}

// ----------------------------------------------------------
// HasProbingAction —— 检测资金试探行为（仙人指路/未遂涨停）
//
// 什么是资金试探？
//   主力在正式拉升前，可能会先"试探"上方的抛压。
//   表现为：盘中一度接近涨停（涨幅 > 8%），最高价逼近箱体上轨，
//   但收盘时被打回箱体内（收盘 <= 箱体上沿）。
//   这种形态叫"仙人指路"或"试盘"，说明主力在测试上方压力，
//   如果后续突破，成功率更高。
//
// 参数：
//   boxUpper     - 箱体上沿价格
//   lookbackDays - 回看天数
// ----------------------------------------------------------
func HasProbingAction(history []tushare.DailyKLine, boxUpper float64, lookbackDays int) bool {
	if len(history) <= lookbackDays {
		return false
	}
	startIndex := len(history) - 1 - lookbackDays
	for i := startIndex; i < len(history)-1; i++ {
		k := history[i]
		// 三个条件同时满足：
		// 1. 涨幅 > 8%（接近涨停）
		// 2. 最高价距离箱体上沿 < 3%（曾逼近突破）
		// 3. 收盘价 <= 箱体上沿（最终被打回，未真正突破）
		if k.PctChg > 8.0 && math.Abs(k.High-boxUpper)/boxUpper < 0.03 && k.Close <= boxUpper {
			return true // 发现明确的资金试探/洗盘痕迹
		}
	}
	return false
}

// ----------------------------------------------------------
// GetOverheadRoom —— 计算上方真空区（套牢盘压力检测）
//
// 什么是上方真空区？
//   股价上方如果没有大量套牢盘，上涨阻力就小。
//   本函数计算当前价格与过去 N 天最高价之间的距离。
//   如果当前价已经是区间新高（上方无阻力），返回 1.0（100% 空间）。
//   如果上方有历史高点压制，返回距高点的百分比距离。
//   返回值越小，说明上方套牢盘越密集，突破越困难。
//
// 参数：
//   currentPrice  - 当前股价
//   lookbackDays  - 回看天数（如 120 天、250 天）
// ----------------------------------------------------------
func GetOverheadRoom(history []tushare.DailyKLine, currentPrice float64, lookbackDays int) float64 {
	// 数据不足时扫描全部可用历史
	actualLookback := lookbackDays
	if len(history) < lookbackDays {
		actualLookback = len(history)
	}
	if actualLookback <= 1 || currentPrice <= 0 {
		return 1.0 // 数据不足或价格异常，假设无压力
	}

	// 寻找区间内的历史最高价（阻力位）
	resistance := 0.0
	startIndex := len(history) - actualLookback
	for i := startIndex; i < len(history)-1; i++ {
		if history[i].High > resistance {
			resistance = history[i].High
		}
	}

	// 如果阻力位高于当前价，返回距离百分比
	if resistance > currentPrice {
		return (resistance - currentPrice) / currentPrice
	}
	// 当前价已是区间新高，上方无套牢盘
	return 1.0
}

// ----------------------------------------------------------
// ForwardAdjustKLines —— 前复权价格清洗器
//
// 什么是前复权（Forward Adjustment）？
//   股票每年会分红、送股、配股，这些操作会导致股价出现断层。
//   例如：10 元的股票，10 送 10 后变成 5 元，但实际持有者没亏。
//   如果不处理，技术指标（均线、MACD 等）会被这些断层严重干扰。
//   前复权就是用数学方法把历史价格"缩小"，让价格曲线连续平滑。
//
// 前复权公式：
//   调整后价格 = 原始价格 * (当日复权因子 / 最新复权因子)
//   调整后成交量 = 原始成交量 / 比值（保证成交额不变）
//
// 参数：
//   klines  - 原始 K 线数据
//   factors - 复权因子数据（Tushare 提供）
// ----------------------------------------------------------
func ForwardAdjustKLines(klines []tushare.DailyKLine, factors []tushare.AdjFactor) []tushare.DailyKLine {
	if len(klines) == 0 || len(factors) == 0 {
		return klines // 无数据则原样返回
	}

	// 步骤 1：构建日期 -> 复权因子的哈希表（map），实现 O(1) 快速查找
	// Go 中用 make(map[KeyType]ValueType) 创建哈希表
	factorMap := make(map[string]float64)
	for _, f := range factors {
		factorMap[f.TradeDate] = f.AdjFactor
	}

	// 步骤 2：确定基准锚点 —— 最新交易日的复权因子
	// 前复权以最新价格为基准，将历史价格向最新价格对齐
	latestDate := klines[len(klines)-1].TradeDate
	latestFactor, exists := factorMap[latestDate] // exists 是布尔值，表示 key 是否存在
	if !exists {
		// 如果最新日期没有因子数据，用因子列表的最后一个作为兜底
		latestFactor = factors[len(factors)-1].AdjFactor
	}
	if latestFactor == 0 {
		latestFactor = 1.0 // 防除零崩溃
	}

	// 步骤 3：逐日执行前复权数学变换
	// make([]T, n) 创建长度为 n 的切片
	adjustedKLines := make([]tushare.DailyKLine, len(klines))
	for i, k := range klines {
		// 先复制原始数据（Go 中 struct 是值类型，赋值会复制一份）
		adjustedKLines[i] = k

		// 查找当日的复权因子
		f, ok := factorMap[k.TradeDate] // ok=false 表示 key 不存在
		if !ok {
			f = latestFactor // 没有因子数据则假设无除权，继承最新因子
		}

		// 前复权乘数 = 当日因子 / 最新因子
		// 当日因子 < 最新因子 时，ratio < 1，历史价格缩小
		ratio := f / latestFactor

		// 价格等比缩放（Open, Close, High, Low, PreClose, Change）
		adjustedKLines[i].Open = k.Open * ratio
		adjustedKLines[i].Close = k.Close * ratio
		adjustedKLines[i].High = k.High * ratio
		adjustedKLines[i].Low = k.Low * ratio
		adjustedKLines[i].PreClose = k.PreClose * ratio
		adjustedKLines[i].Change = k.Change * ratio

		// 成交量反向缩放：价格缩小后，成交量必须放大，才能保证成交额不变。
		// 成交额 = 价格 * 成交量，价格 * ratio，所以成交量要 / ratio。
		if ratio > 0 {
			adjustedKLines[i].Vol = k.Vol / ratio
		}
	}

	return adjustedKLines
}
