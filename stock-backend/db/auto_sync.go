package db

import (
	"database/sql"
	"errors"
	"time"
)

type AutoSyncConfig struct {
	Enabled         bool   `json:"enabled"`
	Timezone        string `json:"timezone"`
	DailyRunTime    string `json:"daily_run_time"`
	LookbackDays    int    `json:"lookback_days"`
	RetryLimit      int    `json:"retry_limit"`
	RetryBackoffSec int    `json:"retry_backoff_sec"`
	LastRunDate     string `json:"last_run_date"`
	UpdatedAt       string `json:"updated_at"`
}

type AutoSyncRun struct {
	ID              int64  `json:"id"`
	RunDate         string `json:"run_date"`
	TriggerType     string `json:"trigger_type"`
	Status          string `json:"status"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	NetworkFailures int    `json:"network_failures"`
	ErrorMsg        string `json:"error_msg"`
	SummaryJSON     string `json:"summary_json"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type AutoSyncRunStep struct {
	ID         int64  `json:"id"`
	RunID      int64  `json:"run_id"`
	StepName   string `json:"step_name"`
	Attempt    int    `json:"attempt"`
	Status     string `json:"status"`
	Targeted   int    `json:"targeted"`
	Success    int    `json:"success"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
	ErrorMsg   string `json:"error_msg"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	UpdatedAt  string `json:"updated_at"`
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

func EnsureAutoSyncConfig() error {
	now := nowRFC3339()
	_, err := DB.Exec(`
		INSERT INTO auto_sync_config(id, enabled, timezone, daily_run_time, lookback_days, retry_limit, retry_backoff_sec, last_run_date, updated_at)
		VALUES (1, 0, 'Asia/Shanghai', '19:00', 7, 6, 30, '', ?)
		ON CONFLICT(id) DO NOTHING
	`, now)
	return err
}

func GetAutoSyncConfig() (AutoSyncConfig, error) {
	if err := EnsureAutoSyncConfig(); err != nil {
		return AutoSyncConfig{}, err
	}

	var cfg AutoSyncConfig
	var enabledInt int
	err := DB.QueryRow(`
		SELECT enabled, timezone, daily_run_time, lookback_days, retry_limit, retry_backoff_sec, last_run_date, updated_at
		FROM auto_sync_config
		WHERE id = 1
	`).Scan(
		&enabledInt, &cfg.Timezone, &cfg.DailyRunTime, &cfg.LookbackDays,
		&cfg.RetryLimit, &cfg.RetryBackoffSec, &cfg.LastRunDate, &cfg.UpdatedAt,
	)
	if err != nil {
		return AutoSyncConfig{}, err
	}
	cfg.Enabled = enabledInt == 1
	return cfg, nil
}

func SaveAutoSyncConfig(cfg AutoSyncConfig) error {
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Shanghai"
	}
	if cfg.DailyRunTime == "" {
		cfg.DailyRunTime = "19:00"
	}
	if cfg.LookbackDays <= 0 {
		cfg.LookbackDays = 7
	}
	if cfg.RetryLimit <= 0 {
		cfg.RetryLimit = 6
	}
	if cfg.RetryBackoffSec <= 0 {
		cfg.RetryBackoffSec = 30
	}

	enabledInt := 0
	if cfg.Enabled {
		enabledInt = 1
	}

	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE auto_sync_config
		SET enabled = ?, timezone = ?, daily_run_time = ?, lookback_days = ?, retry_limit = ?, retry_backoff_sec = ?, updated_at = ?
		WHERE id = 1
	`, enabledInt, cfg.Timezone, cfg.DailyRunTime, cfg.LookbackDays, cfg.RetryLimit, cfg.RetryBackoffSec, now)
	return err
}

func SetAutoSyncLastRunDate(runDate string) error {
	_, err := DB.Exec(`UPDATE auto_sync_config SET last_run_date = ?, updated_at = ? WHERE id = 1`, runDate, nowRFC3339())
	return err
}

func StartAutoSyncRun(runDate, triggerType string) (int64, error) {
	if runDate == "" {
		return 0, errors.New("run_date 不能为空")
	}
	if triggerType == "" {
		triggerType = "manual"
	}
	now := nowRFC3339()
	res, err := DB.Exec(`
		INSERT INTO auto_sync_runs(run_date, trigger_type, status, started_at, finished_at, network_failures, error_msg, summary_json, created_at, updated_at)
		VALUES (?, ?, 'running', ?, '', 0, '', '', ?, ?)
	`, runDate, triggerType, now, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func FinishAutoSyncRun(id int64, status string, networkFailures int, errorMsg, summaryJSON string) error {
	if id <= 0 {
		return errors.New("run id 非法")
	}
	if status == "" {
		status = "failed"
	}
	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE auto_sync_runs
		SET status = ?, finished_at = ?, network_failures = ?, error_msg = ?, summary_json = ?, updated_at = ?
		WHERE id = ?
	`, status, now, networkFailures, errorMsg, summaryJSON, now, id)
	return err
}

func ListAutoSyncRuns(limit int) ([]AutoSyncRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := DB.Query(`
		SELECT id, run_date, trigger_type, status, started_at, finished_at, network_failures, error_msg, summary_json, created_at, updated_at
		FROM auto_sync_runs
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []AutoSyncRun
	for rows.Next() {
		var r AutoSyncRun
		if err := rows.Scan(&r.ID, &r.RunDate, &r.TriggerType, &r.Status, &r.StartedAt, &r.FinishedAt, &r.NetworkFailures, &r.ErrorMsg, &r.SummaryJSON, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func ExistsAutoSyncRunByDate(runDate string) (bool, error) {
	var count int
	err := DB.QueryRow(`SELECT COUNT(1) FROM auto_sync_runs WHERE run_date = ?`, runDate).Scan(&count)
	return count > 0, err
}

func ExistsSuccessfulAutoSyncRunByDate(runDate string) (bool, error) {
	var count int
	err := DB.QueryRow(`SELECT COUNT(1) FROM auto_sync_runs WHERE run_date = ? AND status = 'success'`, runDate).Scan(&count)
	return count > 0, err
}

func MarkStaleRunningRunsFailed(reason string) error {
	if reason == "" {
		reason = "服务重启，运行任务中断"
	}
	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE auto_sync_runs
		SET status = 'failed',
			finished_at = ?,
			error_msg = CASE
				WHEN error_msg IS NULL OR error_msg = '' THEN ?
				ELSE error_msg || '; ' || ?
			END,
			updated_at = ?
		WHERE status = 'running'
	`, now, reason, reason, now)
	return err
}

func MarkStaleRunningRunStepsFailed(reason string) error {
	if reason == "" {
		reason = "服务重启，步骤执行中断"
	}
	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE auto_sync_run_steps
		SET status = 'failed',
			finished_at = ?,
			error_msg = CASE
				WHEN error_msg IS NULL OR error_msg = '' THEN ?
				ELSE error_msg || '; ' || ?
			END,
			updated_at = ?
		WHERE status = 'running'
	`, now, reason, reason, now)
	return err
}

func GetLatestAutoSyncRun() (*AutoSyncRun, error) {
	row := DB.QueryRow(`
		SELECT id, run_date, trigger_type, status, started_at, finished_at, network_failures, error_msg, summary_json, created_at, updated_at
		FROM auto_sync_runs
		ORDER BY id DESC
		LIMIT 1
	`)
	var r AutoSyncRun
	err := row.Scan(&r.ID, &r.RunDate, &r.TriggerType, &r.Status, &r.StartedAt, &r.FinishedAt, &r.NetworkFailures, &r.ErrorMsg, &r.SummaryJSON, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func StartAutoSyncRunStep(runID int64, stepName string, attempt int) (int64, error) {
	if runID <= 0 {
		return 0, errors.New("run_id 非法")
	}
	if stepName == "" {
		return 0, errors.New("step_name 不能为空")
	}
	if attempt <= 0 {
		attempt = 1
	}
	now := nowRFC3339()
	res, err := DB.Exec(`
		INSERT INTO auto_sync_run_steps(run_id, step_name, attempt, status, targeted, success, failed, skipped, error_msg, started_at, finished_at, updated_at)
		VALUES (?, ?, ?, 'running', 0, 0, 0, 0, '', ?, '', ?)
	`, runID, stepName, attempt, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func FinishAutoSyncRunStep(stepID int64, status string, targeted, success, failed, skipped int, errorMsg string) error {
	if stepID <= 0 {
		return errors.New("step id 非法")
	}
	if status == "" {
		status = "failed"
	}
	now := nowRFC3339()
	_, err := DB.Exec(`
		UPDATE auto_sync_run_steps
		SET status = ?, targeted = ?, success = ?, failed = ?, skipped = ?, error_msg = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`, status, targeted, success, failed, skipped, errorMsg, now, now, stepID)
	return err
}

func ListAutoSyncRunSteps(runID int64) ([]AutoSyncRunStep, error) {
	if runID <= 0 {
		return nil, errors.New("run_id 非法")
	}
	rows, err := DB.Query(`
		SELECT id, run_id, step_name, attempt, status, targeted, success, failed, skipped, error_msg, started_at, finished_at, updated_at
		FROM auto_sync_run_steps
		WHERE run_id = ?
		ORDER BY id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []AutoSyncRunStep
	for rows.Next() {
		var s AutoSyncRunStep
		if err := rows.Scan(&s.ID, &s.RunID, &s.StepName, &s.Attempt, &s.Status, &s.Targeted, &s.Success, &s.Failed, &s.Skipped, &s.ErrorMsg, &s.StartedAt, &s.FinishedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		steps = append(steps, s)
	}
	return steps, nil
}
