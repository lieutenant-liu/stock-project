package db

import "fmt"

// EngineConfig 引擎子策略开关配置。
type EngineConfig struct {
	EnableRegimeRouter     bool   `json:"enable_regime_router"`
	EnableSignalAllocator  bool   `json:"enable_signal_allocator"`
	Enable3DExit           bool   `json:"enable_3d_exit"`
	ExitProfile            string `json:"exit_profile"` // "scalp" (短线剥头皮) 或 "trend" (趋势追踪)
}

// InitEngineConfig 初始化引擎配置（INSERT OR IGNORE，仅首次写入默认值）。
func InitEngineConfig() {
	defaults := map[string]bool{
		"enable_regime_router":    true,
		"enable_signal_allocator": true,
		"enable_3d_exit":          true,
	}
	for key, val := range defaults {
		DB.Exec(`INSERT OR IGNORE INTO engine_config (key, value) VALUES (?, ?)`, key, boolToInt(val))
	}
	// exit_profile 使用字符串值，value 列存储 0=scalp, 1=trend
	DB.Exec(`INSERT OR IGNORE INTO engine_config (key, value) VALUES ('exit_profile', 0)`)
}

// GetEngineConfig 读取引擎配置。
func GetEngineConfig() (EngineConfig, error) {
	rows, err := DB.Query(`SELECT key, value FROM engine_config`)
	if err != nil {
		return EngineConfig{}, fmt.Errorf("读取引擎配置失败: %w", err)
	}
	defer rows.Close()

	m := make(map[string]int)
	for rows.Next() {
		var key string
		var val int
		if err := rows.Scan(&key, &val); err != nil {
			return EngineConfig{}, err
		}
		m[key] = val
	}

	profile := "scalp"
	switch m["exit_profile"] {
	case 1:
		profile = "trend"
	case 2:
		profile = "swing"
	case 3:
		profile = "guerrilla"
	}

	return EngineConfig{
		EnableRegimeRouter:    m["enable_regime_router"] == 1,
		EnableSignalAllocator: m["enable_signal_allocator"] == 1,
		Enable3DExit:          m["enable_3d_exit"] == 1,
		ExitProfile:           profile,
	}, nil
}

// UpdateEngineConfig 更新单个引擎配置项。
func UpdateEngineConfig(key string, enabled bool) error {
	res, err := DB.Exec(`UPDATE engine_config SET value = ? WHERE key = ?`, boolToInt(enabled), key)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("配置项 %q 不存在", key)
	}
	return nil
}

// UpdateExitProfile 更新退出流派配置。
func UpdateExitProfile(profile string) error {
	val := 0
	switch profile {
	case "trend":
		val = 1
	case "swing":
		val = 2
	case "guerrilla":
		val = 3
	}
	res, err := DB.Exec(`UPDATE engine_config SET value = ? WHERE key = 'exit_profile'`, val)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// 不存在则插入
		_, err = DB.Exec(`INSERT INTO engine_config (key, value) VALUES ('exit_profile', ?)`, val)
	}
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
