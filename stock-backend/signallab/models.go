package signallab

// ============================================================
// signallab/models.go - 信号实验室数据模型
// ============================================================
// 这个文件定义了信号实验室使用的数据结构。
//
// 【什么是信号实验室？】
// 信号实验室是一个纯净的信号评测工具：
// - 不考虑资金管理、仓位控制
// - 只评估信号本身的准确性
// - 计算信号发出后 N 天的收益
// - 用于判断策略信号的质量
//
// 【Go 语言知识点】
// - 结构体标签 (struct tag): `json:"strategy"` 指定 JSON 字段名
// - 数据类型: string, float64, int, bool
// ============================================================

// SignalLabConfig 信号实验室任务配置。
// 【字段说明】
// - Strategy: 策略名称，如 "MACB", "CBBM", "PBMA", "ALL"
// - StartDate: 回测开始日期，格式 YYYYMMDD
// - EndDate: 回测结束日期，格式 YYYYMMDD
type SignalLabConfig struct {
	Strategy  string `json:"strategy"`   // 策略名称
	StartDate string `json:"start_date"` // 开始日期
	EndDate   string `json:"end_date"`   // 结束日期
}

// SignalEvaluation 单条信号的前向 20 日评测结果。
// 【什么是前向收益？】
// 前向收益是指信号发出后，未来 N 天的实际收益。
// 例如：信号日是 2024-01-15，那么：
// - Ret1D: 2024-01-16 的收益（1 天后）
// - Ret3D: 2024-01-18 的收益（3 天后）
// - Ret5D: 2024-01-22 的收益（5 天后）
// - Ret10D: 2024-01-29 的收益（10 天后）
// - Ret20D: 2024-02-12 的收益（20 天后）
type SignalEvaluation struct {
	Symbol     string `json:"symbol"`      // 股票代码
	SignalDate string `json:"signal_date"` // 信号日（T日）
	Strategy   string `json:"strategy"`    // 策略名称
	EntryDate  string `json:"entry_date"`  // 入场日（T+1日，因为 A 股 T+1 制度）

	// ── 价格信息 ──
	EntryOpen   float64 `json:"entry_open"`   // T+1 开盘价（实际入场价）
	SignalClose float64 `json:"signal_close"` // T 日收盘价（信号发出时的价格）

	// ── 跳空与量比 ──
	GapPct   float64 `json:"gap_pct"`   // 跳空幅度：(EntryOpen/SignalClose)-1
	VolRatio float64 `json:"vol_ratio"` // 量比：T日成交量 / 5日平均成交量

	// ── 最大有利/不利偏移 ──
	// 【什么是 MFE/MAE？】
	// MFE (Maximum Favorable Excursion): 信号后最大盈利幅度
	// MAE (Maximum Adverse Excursion): 信号后最大亏损幅度
	// 例如：买入后最高涨了 15%（MFE=15%），最低跌了 5%（MAE=-5%）
	MFE20        float64 `json:"mfe_20"`        // 20 日最大有利偏移（%）
	DaysToMFE    int     `json:"days_to_mfe"`   // 到达 MFE 的天数
	MAE20        float64 `json:"mae_20"`        // 20 日最大不利偏移（%）
	DaysToMAE    int     `json:"days_to_mae"`   // 到达 MAE 的天数
	MFEBeforeMAE bool    `json:"mfe_before_mae"` // MFE 是否先于 MAE 到达（盈利先来还是亏损先来）

	// ── 前向收益 ──
	// 【计算方式】
	// RetND = (第N天收盘价 - T+1开盘价) / T+1开盘价 * 100
	Ret1D  float64 `json:"ret_1d"`  // 1 日收益（%）
	Ret3D  float64 `json:"ret_3d"`  // 3 日收益（%）
	Ret5D  float64 `json:"ret_5d"`  // 5 日收益（%）
	Ret10D float64 `json:"ret_10d"` // 10 日收益（%）
	Ret20D float64 `json:"ret_20d"` // 20 日收益（%）
}
