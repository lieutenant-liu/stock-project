package db

import (
	"errors"
	"strings"
	"time"
)

type EmailNotifyConfig struct {
	Enabled       bool   `json:"enabled"`
	AutoSendDaily bool   `json:"auto_send_daily"`
	SMTPHost      string `json:"smtp_host"`
	SMTPPort      int    `json:"smtp_port"`
	SMTPUser      string `json:"smtp_user"`
	SMTPPass      string `json:"smtp_pass"`
	SMTPFrom      string `json:"smtp_from"`
	SubjectPrefix string `json:"subject_prefix"`
	UpdatedAt     string `json:"updated_at"`
}

type EmailRecipient struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Label     string `json:"label"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func EnsureEmailNotifyConfig() error {
	now := nowRFC3339()
	_, err := DB.Exec(`
		INSERT INTO email_notify_config(id, enabled, auto_send_daily, smtp_host, smtp_port, smtp_user, smtp_pass, smtp_from, subject_prefix, updated_at)
		VALUES (1, 0, 0, '', 587, '', '', '', '[Stock-AutoSync]', ?)
		ON CONFLICT(id) DO NOTHING
	`, now)
	return err
}

func GetEmailNotifyConfig() (EmailNotifyConfig, error) {
	if err := EnsureEmailNotifyConfig(); err != nil {
		return EmailNotifyConfig{}, err
	}
	var cfg EmailNotifyConfig
	var enabledInt, autoInt int
	err := DB.QueryRow(`
		SELECT enabled, auto_send_daily, smtp_host, smtp_port, smtp_user, smtp_pass, smtp_from, subject_prefix, updated_at
		FROM email_notify_config
		WHERE id = 1
	`).Scan(&enabledInt, &autoInt, &cfg.SMTPHost, &cfg.SMTPPort, &cfg.SMTPUser, &cfg.SMTPPass, &cfg.SMTPFrom, &cfg.SubjectPrefix, &cfg.UpdatedAt)
	if err != nil {
		return EmailNotifyConfig{}, err
	}
	cfg.Enabled = enabledInt == 1
	cfg.AutoSendDaily = autoInt == 1
	return cfg, nil
}

func SaveEmailNotifyConfig(cfg EmailNotifyConfig) error {
	if cfg.SMTPPort <= 0 {
		cfg.SMTPPort = 587
	}
	if strings.TrimSpace(cfg.SubjectPrefix) == "" {
		cfg.SubjectPrefix = "[Stock-AutoSync]"
	}
	enabledInt := 0
	if cfg.Enabled {
		enabledInt = 1
	}
	autoInt := 0
	if cfg.AutoSendDaily {
		autoInt = 1
	}
	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE email_notify_config
		SET enabled = ?, auto_send_daily = ?, smtp_host = ?, smtp_port = ?, smtp_user = ?, smtp_pass = ?, smtp_from = ?, subject_prefix = ?, updated_at = ?
		WHERE id = 1
	`, enabledInt, autoInt, strings.TrimSpace(cfg.SMTPHost), cfg.SMTPPort, strings.TrimSpace(cfg.SMTPUser), cfg.SMTPPass, strings.TrimSpace(cfg.SMTPFrom), strings.TrimSpace(cfg.SubjectPrefix), now)
	return err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ListEmailRecipients() ([]EmailRecipient, error) {
	rows, err := DB.Query(`
		SELECT id, email, label, enabled, created_at, updated_at
		FROM email_recipients
		ORDER BY enabled DESC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []EmailRecipient
	for rows.Next() {
		var item EmailRecipient
		var enabledInt int
		if err := rows.Scan(&item.ID, &item.Email, &item.Label, &enabledInt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabledInt == 1
		list = append(list, item)
	}
	return list, nil
}

func AddEmailRecipient(email, label string, enabled bool) (int64, error) {
	email = normalizeEmail(email)
	if email == "" {
		return 0, errors.New("邮箱不能为空")
	}
	now := time.Now().Format(time.RFC3339)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	res, err := DB.Exec(`
		INSERT INTO email_recipients(email, label, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, email, strings.TrimSpace(label), enabledInt, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateEmailRecipient(id int64, label string, enabled bool) error {
	if id <= 0 {
		return errors.New("id 非法")
	}
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := DB.Exec(`
		UPDATE email_recipients
		SET label = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, strings.TrimSpace(label), enabledInt, nowRFC3339(), id)
	return err
}

func DeleteEmailRecipient(id int64) error {
	if id <= 0 {
		return errors.New("id 非法")
	}
	_, err := DB.Exec(`DELETE FROM email_recipients WHERE id = ?`, id)
	return err
}

func GetEnabledRecipientEmailsByIDs(ids []int64) ([]string, error) {
	if len(ids) == 0 {
		rows, err := DB.Query(`SELECT email FROM email_recipients WHERE enabled = 1 ORDER BY id ASC`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var emails []string
		for rows.Next() {
			var e string
			if rows.Scan(&e) == nil && strings.TrimSpace(e) != "" {
				emails = append(emails, strings.TrimSpace(e))
			}
		}
		return emails, nil
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	args = append(args, 1)

	query := `SELECT email FROM email_recipients WHERE id IN (` + strings.Join(placeholders, ",") + `) AND enabled = ? ORDER BY id ASC`
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var e string
		if rows.Scan(&e) == nil && strings.TrimSpace(e) != "" {
			emails = append(emails, strings.TrimSpace(e))
		}
	}
	return emails, nil
}
