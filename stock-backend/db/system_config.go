package db

// SystemConfig 系统级配置（单行模式，id 固定为 1）。
type SystemConfig struct {
	EnableProData bool   `json:"enable_pro_data"`
	UpdatedAt     string `json:"updated_at"`
}

// EnsureSystemConfig 确保 system_config 表中存在默认行（幂等）。
func EnsureSystemConfig() error {
	now := nowRFC3339()
	_, err := DB.Exec(`
		INSERT INTO system_config(id, enable_pro_data, updated_at)
		VALUES (1, 0, ?)
		ON CONFLICT(id) DO NOTHING
	`, now)
	return err
}

// GetSystemConfig 读取系统配置。若不存在则先创建默认行。
func GetSystemConfig() (SystemConfig, error) {
	if err := EnsureSystemConfig(); err != nil {
		return SystemConfig{}, err
	}
	var cfg SystemConfig
	var proInt int
	err := DB.QueryRow(`
		SELECT enable_pro_data, updated_at
		FROM system_config WHERE id = 1
	`).Scan(&proInt, &cfg.UpdatedAt)
	if err != nil {
		return SystemConfig{}, err
	}
	cfg.EnableProData = proInt == 1
	return cfg, nil
}

// SaveSystemConfig 更新系统配置。
func SaveSystemConfig(cfg SystemConfig) error {
	proInt := 0
	if cfg.EnableProData {
		proInt = 1
	}
	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE system_config
		SET enable_pro_data = ?, updated_at = ?
		WHERE id = 1
	`, proInt, now)
	return err
}
