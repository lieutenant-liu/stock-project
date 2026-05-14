package signallab

// SignalLabConfig 信号实验室任务配置。
type SignalLabConfig struct {
	Strategy  string `json:"strategy"`   // "MACB", "CBBM", "PBMA", "ALL"
	StartDate string `json:"start_date"` // YYYYMMDD
	EndDate   string `json:"end_date"`   // YYYYMMDD
}

// SignalEvaluation 单条信号的前向 20 日评测结果。
type SignalEvaluation struct {
	Symbol       string  `json:"symbol"`
	SignalDate   string  `json:"signal_date"`   // T日
	Strategy     string  `json:"strategy"`
	EntryDate    string  `json:"entry_date"`    // T+1日
	EntryOpen    float64 `json:"entry_open"`    // T+1 开盘价
	SignalClose  float64 `json:"signal_close"`  // T日收盘价
	GapPct       float64 `json:"gap_pct"`       // (EntryOpen/SignalClose)-1
	VolRatio     float64 `json:"vol_ratio"`     // T日Vol / 5日均Vol(T-5~T-1)
	MFE20        float64 `json:"mfe_20"`        // 最大有利偏移 (%)
	DaysToMFE    int     `json:"days_to_mfe"`
	MAE20        float64 `json:"mae_20"`        // 最大不利偏移 (%)
	DaysToMAE    int     `json:"days_to_mae"`
	MFEBeforeMAE bool    `json:"mfe_before_mae"` // MFE 是否先于 MAE 到达
	Ret1D        float64 `json:"ret_1d"`
	Ret3D        float64 `json:"ret_3d"`
	Ret5D        float64 `json:"ret_5d"`
	Ret10D       float64 `json:"ret_10d"`
	Ret20D       float64 `json:"ret_20d"`
}
