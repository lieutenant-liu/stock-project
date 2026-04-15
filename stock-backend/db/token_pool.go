package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type APIToken struct {
	ID        int64  `json:"id"`
	Provider  string `json:"provider"`
	Token     string `json:"token,omitempty"`
	TokenMask string `json:"token_mask"`
	Tier      string `json:"tier"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
	IsActive  bool   `json:"is_active"`
	FailCount int    `json:"fail_count"`
	LastOKAt  string `json:"last_ok_at"`
	LastErrAt string `json:"last_err_at"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func maskToken(raw string) string {
	token := strings.TrimSpace(raw)
	if len(token) <= 10 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func normalizeProvider(p string) string {
	provider := strings.ToLower(strings.TrimSpace(p))
	if provider == "" {
		return "tushare"
	}
	return provider
}

func ListAPITokens(provider string) ([]APIToken, error) {
	provider = normalizeProvider(provider)
	rows, err := DB.Query(`
		SELECT id, provider, token, tier, priority, enabled, is_active, fail_count, last_ok_at, last_err_at, notes, created_at, updated_at
		FROM api_tokens
		WHERE provider = ?
		ORDER BY is_active DESC, enabled DESC, priority DESC, id ASC
	`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		var rawToken string
		var enabled, active int
		if err := rows.Scan(&t.ID, &t.Provider, &rawToken, &t.Tier, &t.Priority, &enabled, &active, &t.FailCount, &t.LastOKAt, &t.LastErrAt, &t.Notes, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Enabled = enabled == 1
		t.IsActive = active == 1
		t.TokenMask = maskToken(rawToken)
		tokens = append(tokens, t)
	}

	return tokens, nil
}

func AddAPIToken(provider, token, tier string, priority int, enabled bool, notes string, makeActive bool) (int64, error) {
	provider = normalizeProvider(provider)
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, errors.New("token 不能为空")
	}
	if priority == 0 {
		priority = 100
	}

	now := time.Now().Format(time.RFC3339)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO api_tokens(provider, token, tier, priority, enabled, is_active, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
	`, provider, token, strings.TrimSpace(tier), priority, enabledInt, strings.TrimSpace(notes), now, now)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if makeActive && enabled {
		if _, err := tx.Exec(`UPDATE api_tokens SET is_active = 0, updated_at = ? WHERE provider = ?`, now, provider); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`UPDATE api_tokens SET is_active = 1, updated_at = ? WHERE id = ?`, now, id); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return id, nil
}

func UpdateAPIToken(id int64, tier string, priority int, enabled bool, notes string) error {
	now := time.Now().Format(time.RFC3339)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var provider string
	var wasActive int
	if err := tx.QueryRow(`SELECT provider, is_active FROM api_tokens WHERE id = ?`, id).Scan(&provider, &wasActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("token 不存在")
		}
		return err
	}

	_, err = tx.Exec(`
		UPDATE api_tokens
		SET tier = ?, priority = ?, enabled = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`, strings.TrimSpace(tier), priority, enabledInt, strings.TrimSpace(notes), now, id)
	if err != nil {
		return err
	}

	if enabledInt == 0 && wasActive == 1 {
		_, _ = tx.Exec(`UPDATE api_tokens SET is_active = 0, updated_at = ? WHERE id = ?`, now, id)
		_, _ = tx.Exec(`
			UPDATE api_tokens
			SET is_active = 1, updated_at = ?
			WHERE id = (
				SELECT id FROM api_tokens
				WHERE provider = ? AND enabled = 1
				ORDER BY priority DESC, id ASC
				LIMIT 1
			)
		`, now, provider)
	}

	return tx.Commit()
}

func DeleteAPIToken(id int64) error {
	now := time.Now().Format(time.RFC3339)
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var provider string
	var wasActive int
	if err := tx.QueryRow(`SELECT provider, is_active FROM api_tokens WHERE id = ?`, id).Scan(&provider, &wasActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if _, err := tx.Exec(`DELETE FROM api_tokens WHERE id = ?`, id); err != nil {
		return err
	}

	if wasActive == 1 {
		_, _ = tx.Exec(`
			UPDATE api_tokens
			SET is_active = 1, updated_at = ?
			WHERE id = (
				SELECT id FROM api_tokens
				WHERE provider = ? AND enabled = 1
				ORDER BY priority DESC, id ASC
				LIMIT 1
			)
		`, now, provider)
	}

	return tx.Commit()
}

func SetActiveAPIToken(provider string, id int64) (string, error) {
	provider = normalizeProvider(provider)
	now := time.Now().Format(time.RFC3339)

	tx, err := DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var token string
	var enabled int
	if err := tx.QueryRow(`SELECT token, enabled FROM api_tokens WHERE id = ? AND provider = ?`, id, provider).Scan(&token, &enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("未找到对应 token")
		}
		return "", err
	}
	if enabled != 1 {
		return "", errors.New("该 token 已禁用，无法设为当前")
	}

	if _, err := tx.Exec(`UPDATE api_tokens SET is_active = 0, updated_at = ? WHERE provider = ?`, now, provider); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`UPDATE api_tokens SET is_active = 1, updated_at = ? WHERE id = ?`, now, id); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return token, nil
}

func GetActiveAPIToken(provider string) (*APIToken, error) {
	provider = normalizeProvider(provider)
	var t APIToken
	var rawToken string
	var enabled, active int

	err := DB.QueryRow(`
		SELECT id, provider, token, tier, priority, enabled, is_active, fail_count, last_ok_at, last_err_at, notes, created_at, updated_at
		FROM api_tokens
		WHERE provider = ? AND enabled = 1 AND is_active = 1
		LIMIT 1
	`, provider).Scan(&t.ID, &t.Provider, &rawToken, &t.Tier, &t.Priority, &enabled, &active, &t.FailCount, &t.LastOKAt, &t.LastErrAt, &t.Notes, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	t.Enabled = enabled == 1
	t.IsActive = active == 1
	t.Token = rawToken
	t.TokenMask = maskToken(rawToken)
	return &t, nil
}

func TouchTokenResult(id int64, success bool) error {
	now := time.Now().Format(time.RFC3339)
	if success {
		_, err := DB.Exec(`
			UPDATE api_tokens
			SET fail_count = 0, last_ok_at = ?, updated_at = ?
			WHERE id = ?
		`, now, now, id)
		return err
	}
	_, err := DB.Exec(`
		UPDATE api_tokens
		SET fail_count = fail_count + 1, last_err_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, id)
	return err
}

func FindAPITokenID(provider, token string) (int64, error) {
	provider = normalizeProvider(provider)
	token = strings.TrimSpace(token)
	var id int64
	err := DB.QueryRow(`SELECT id FROM api_tokens WHERE provider = ? AND token = ? LIMIT 1`, provider, token).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}
