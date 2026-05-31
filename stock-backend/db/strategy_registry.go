package db

import "log"

// StrategyMetadata 策略元数据（与前端 JSON 匹配）。
type StrategyMetadata struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Principle   string  `json:"principle"`
	WinRate     float64 `json:"win_rate"`
	IsEnabled   bool    `json:"is_enabled"`
}

// InitStrategyRegistry 初始化策略注册表（INSERT OR IGNORE，幂等）。
func InitStrategyRegistry() {
	strategies := []StrategyMetadata{
		{ID: "CBBM-P-EXP", Name: "箱体突破回踩-EXP", Category: "复合狙击", WinRate: 60.0, IsEnabled: true,
			Description: "以箱体突破为锚点，回踩阶段叠加 EXP 级微观过滤器的复合策略。",
			Principle:   "CBBM 箱体突破后等待缩量回踩至支撑线附近，叠加微幅高开陷阱拦截、天量诱多过滤和 K 线实体饱满度检测，确认回踩有效后入场。"},
		{ID: "MACB", Name: "均线收敛突破", Category: "右侧突破", WinRate: 49.6, IsEnabled: true,
			Description: "经典均线收敛后放量突破策略。",
			Principle:   "当 MA30/MA60/MA120 三条均线高度收敛（纠缠度 < 4%），且当日出现 >= 5% 大阳线放量（> 2x MA20）突破均线簇时触发买入。止损位为三均线最小值。"},
		{ID: "PBMA-EXP", Name: "缩量回踩狙击-实验", Category: "左侧回踩", WinRate: 49.0, IsEnabled: true,
			Description: "PBMA 实验版，叠加 EXP 微观过滤器。",
			Principle:   "在 PBMA 基础上增加微幅高开陷阱、天量诱多和 K 线实体过滤，拦截低质量回踩信号。"},
		{ID: "CBBM", Name: "中枢强势突破", Category: "右侧突破", WinRate: 47.4, IsEnabled: true,
			Description: "基本面共振的中枢箱体突破策略。",
			Principle:   "60 日箱体规律（振幅 <= 35%），涨停板放量突破箱体上沿，叠加基本面防雷护盾和资金流净流入确认。"},
		{ID: "CBBM-EXP-P-EXP", Name: "高质箱体突破回踩-EXP", Category: "复合狙击", WinRate: 50.0, IsEnabled: false,
			Description: "高质箱体突破（CBBM-EXP）后回踩，叠加 EXP 过滤器。",
			Principle:   "以 CBBM-EXP 实验版箱体突破为锚点，回踩端叠加 EXP 级过滤器的复合策略。"},
		{ID: "PBMA", Name: "缩量回踩狙击", Category: "左侧回踩", WinRate: 47.1, IsEnabled: false,
			Description: "放量大阳线启动后连续缩量回踩至 MA20 的狙击策略。",
			Principle:   "过去 15 天内出现 >= 5% 放量大阳线（> 1.5x MA20）作为锚点，随后回调期缩量至锚点日 50% 以下，今日最低价回踩 MA20 ±3% 并以阳线站稳。"},
		{ID: "CBBM-EXP", Name: "中枢强势突破-实验", Category: "右侧突破", WinRate: 46.5, IsEnabled: false,
			Description: "CBBM 实验版，叠加跳空陷阱、天量诱多和 K 线实体过滤。",
			Principle:   "在 CBBM 基础上增加微幅高开陷阱拦截、天量突破诱多过滤（> 3x MA20）和 K 线实体饱满度检测（> 60%）。"},
		{ID: "CBBM-P", Name: "箱体突破回踩", Category: "复合狙击", WinRate: 45.9, IsEnabled: false,
			Description: "箱体突破后回踩支撑线的复合策略。",
			Principle:   "CBBM 箱体突破后等待缩量回踩至上沿支撑线附近，验证缩量洗盘和支撑位确认后入场。"},
		{ID: "MACB-P", Name: "均线突破回踩", Category: "复合狙击", WinRate: 45.6, IsEnabled: false,
			Description: "均线收敛突破后回踩均线支撑的复合策略。",
			Principle:   "MACB 均线突破后等待缩量回踩至 max(MA30, MA60, MA120) 支撑线附近，验证回调缩量和支撑位确认后入场。"},
		{ID: "MACB-P-EXP", Name: "均线突破回踩-EXP", Category: "复合狙击", WinRate: 40.8, IsEnabled: false,
			Description: "均线突破回踩叠加 EXP 过滤器。",
			Principle:   "在 MACB-P 基础上回踩端叠加微幅高开陷阱和资金承接检测。"},
		{ID: "CBBM-EXP-P", Name: "高质箱体突破回踩", Category: "复合狙击", WinRate: 40.8, IsEnabled: false,
			Description: "高质箱体突破后回踩支撑线的复合策略。",
			Principle:   "以 CBBM-EXP 实验版箱体突破为锚点，回踩端验证缩量洗盘和支撑位确认后入场，不叠加 EXP 过滤器。"},
	}

	stmt, err := DB.Prepare(`INSERT OR IGNORE INTO strategy_registry (id, name, category, description, principle, win_rate, is_enabled) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatal("❌ Prepare strategy_registry insert failed: ", err)
	}
	defer stmt.Close()

	for _, s := range strategies {
		enabled := 0
		if s.IsEnabled {
			enabled = 1
		}
		if _, err := stmt.Exec(s.ID, s.Name, s.Category, s.Description, s.Principle, s.WinRate, enabled); err != nil {
			log.Printf("⚠️ [策略注册] 插入 %s 失败: %v", s.ID, err)
		}
	}
}

// GetAllStrategies 返回全部策略元数据。
func GetAllStrategies() ([]StrategyMetadata, error) {
	rows, err := DB.Query(`SELECT id, name, category, description, principle, win_rate, is_enabled FROM strategy_registry ORDER BY win_rate DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StrategyMetadata
	for rows.Next() {
		var s StrategyMetadata
		var enabled int
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description, &s.Principle, &s.WinRate, &enabled); err != nil {
			return nil, err
		}
		s.IsEnabled = enabled == 1
		result = append(result, s)
	}
	return result, nil
}

// GetEnabledStrategies 返回已启用的策略元数据。
func GetEnabledStrategies() ([]StrategyMetadata, error) {
	rows, err := DB.Query(`SELECT id, name, category, description, principle, win_rate, is_enabled FROM strategy_registry WHERE is_enabled = 1 ORDER BY win_rate DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StrategyMetadata
	for rows.Next() {
		var s StrategyMetadata
		var enabled int
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description, &s.Principle, &s.WinRate, &enabled); err != nil {
			return nil, err
		}
		s.IsEnabled = enabled == 1
		result = append(result, s)
	}
	return result, nil
}

// ToggleStrategy 更新策略启用状态。
func ToggleStrategy(id string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := DB.Exec(`UPDATE strategy_registry SET is_enabled = ? WHERE id = ?`, val, id)
	return err
}
