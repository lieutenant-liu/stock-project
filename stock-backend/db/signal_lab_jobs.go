package db

// SignalLabJob 信号实验室任务记录。
type SignalLabJob struct {
	ID         int64  `json:"id"`
	Strategy   string `json:"strategy"`
	Status     string `json:"status"`     // pending/running/completed/failed
	Progress   string `json:"progress"`   // "1500/3183"
	FilePath   string `json:"file_path"`
	ErrorMsg   string `json:"error_msg"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

// CreateLabJob 创建信号实验室任务，返回 job ID。
func CreateLabJob(strategy string) (int64, error) {
	res, err := DB.Exec(
		`INSERT INTO signal_lab_jobs (strategy, status, created_at) VALUES (?, 'pending', ?)`,
		strategy, nowRFC3339BT(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetLabJob 按 ID 查询信号实验室任务。
func GetLabJob(id int64) (*SignalLabJob, error) {
	row := DB.QueryRow(
		`SELECT id, strategy, status, progress, COALESCE(file_path,''), COALESCE(error_msg,''), created_at, COALESCE(started_at,''), COALESCE(finished_at,'')
		 FROM signal_lab_jobs WHERE id = ?`, id,
	)
	j := &SignalLabJob{}
	err := row.Scan(&j.ID, &j.Strategy, &j.Status, &j.Progress, &j.FilePath, &j.ErrorMsg, &j.CreatedAt, &j.StartedAt, &j.FinishedAt)
	if err != nil {
		return nil, err
	}
	return j, nil
}

// ListLabJobs 列出最近的信号实验室任务（倒序）。
func ListLabJobs(limit int) ([]SignalLabJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := DB.Query(
		`SELECT id, strategy, status, progress, COALESCE(file_path,''), COALESCE(error_msg,''), created_at, COALESCE(started_at,''), COALESCE(finished_at,'')
		 FROM signal_lab_jobs ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []SignalLabJob
	for rows.Next() {
		var j SignalLabJob
		if err := rows.Scan(&j.ID, &j.Strategy, &j.Status, &j.Progress, &j.FilePath, &j.ErrorMsg, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateLabJobStatus 更新任务状态和进度。
func UpdateLabJobStatus(id int64, status, progress string) error {
	if status == "running" {
		_, err := DB.Exec(
			`UPDATE signal_lab_jobs SET status = ?, progress = ?, started_at = ? WHERE id = ?`,
			status, progress, nowRFC3339BT(), id,
		)
		return err
	}
	_, err := DB.Exec(
		`UPDATE signal_lab_jobs SET status = ?, progress = ? WHERE id = ?`,
		status, progress, id,
	)
	return err
}

// FinishLabJob 标记任务完成，写入 CSV 文件路径。
func FinishLabJob(id int64, filePath string) error {
	_, err := DB.Exec(
		`UPDATE signal_lab_jobs SET status = 'completed', file_path = ?, finished_at = ? WHERE id = ?`,
		filePath, nowRFC3339BT(), id,
	)
	return err
}

// FailLabJob 标记任务失败，写入错误信息。
func FailLabJob(id int64, errMsg string) error {
	_, err := DB.Exec(
		`UPDATE signal_lab_jobs SET status = 'failed', error_msg = ?, finished_at = ? WHERE id = ?`,
		errMsg, nowRFC3339BT(), id,
	)
	return err
}

// MarkStaleRunningLabJobsFailed 将所有卡在 running/pending 状态的任务标记为失败。
// 用于服务启动时清理上次崩溃遗留的僵尸任务。
func MarkStaleRunningLabJobsFailed(reason string) error {
	if reason == "" {
		reason = "服务重启，任务中断"
	}
	now := nowRFC3339BT()
	_, err := DB.Exec(`
		UPDATE signal_lab_jobs
		SET status = 'failed',
			finished_at = ?,
			error_msg = CASE
				WHEN error_msg IS NULL OR error_msg = '' THEN ?
				ELSE error_msg || '; ' || ?
			END
		WHERE status IN ('running', 'pending')
	`, now, reason, reason)
	return err
}
