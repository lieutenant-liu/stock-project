package db

import "time"

// BacktestJob 回测任务记录。
type BacktestJob struct {
	ID         int64  `json:"id"`
	ConfigJSON string `json:"config_json"`
	Status     string `json:"status"`     // pending/running/completed/failed
	Progress   string `json:"progress"`   // "扫描中 45/120 只股票"
	ResultJSON string `json:"result_json"`
	ErrorMsg   string `json:"error_msg"`
	CSVPath    string `json:"csv_path"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

func nowRFC3339BT() string { return time.Now().Format(time.RFC3339) }

// CreateBacktestJob 创建回测任务，返回 job ID。
func CreateBacktestJob(configJSON string) (int64, error) {
	res, err := DB.Exec(
		`INSERT INTO backtest_jobs (config_json, status, created_at) VALUES (?, 'pending', ?)`,
		configJSON, nowRFC3339BT(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetBacktestJob 按 ID 查询回测任务。
func GetBacktestJob(id int64) (*BacktestJob, error) {
	row := DB.QueryRow(
		`SELECT id, config_json, status, progress, COALESCE(result_json,''), COALESCE(error_msg,''), COALESCE(csv_path,''), created_at, COALESCE(started_at,''), COALESCE(finished_at,'')
		 FROM backtest_jobs WHERE id = ?`, id,
	)
	j := &BacktestJob{}
	err := row.Scan(&j.ID, &j.ConfigJSON, &j.Status, &j.Progress, &j.ResultJSON, &j.ErrorMsg, &j.CSVPath, &j.CreatedAt, &j.StartedAt, &j.FinishedAt)
	if err != nil {
		return nil, err
	}
	return j, nil
}

// ListBacktestJobs 列出最近的回测任务（倒序）。
func ListBacktestJobs(limit int) ([]BacktestJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := DB.Query(
		`SELECT id, config_json, status, progress, COALESCE(result_json,''), COALESCE(error_msg,''), COALESCE(csv_path,''), created_at, COALESCE(started_at,''), COALESCE(finished_at,'')
		 FROM backtest_jobs ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []BacktestJob
	for rows.Next() {
		var j BacktestJob
		if err := rows.Scan(&j.ID, &j.ConfigJSON, &j.Status, &j.Progress, &j.ResultJSON, &j.ErrorMsg, &j.CSVPath, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateBacktestJobStatus 更新任务状态和进度。
func UpdateBacktestJobStatus(id int64, status, progress string) error {
	if status == "running" {
		_, err := DB.Exec(
			`UPDATE backtest_jobs SET status = ?, progress = ?, started_at = ? WHERE id = ?`,
			status, progress, nowRFC3339BT(), id,
		)
		return err
	}
	_, err := DB.Exec(
		`UPDATE backtest_jobs SET status = ?, progress = ? WHERE id = ?`,
		status, progress, id,
	)
	return err
}

// FinishBacktestJob 标记任务完成，写入结果 JSON 和 CSV 路径。
func FinishBacktestJob(id int64, resultJSON, csvPath string) error {
	_, err := DB.Exec(
		`UPDATE backtest_jobs SET status = 'completed', result_json = ?, csv_path = ?, finished_at = ? WHERE id = ?`,
		resultJSON, csvPath, nowRFC3339BT(), id,
	)
	return err
}

// FailBacktestJob 标记任务失败，写入错误信息。
func FailBacktestJob(id int64, errMsg string) error {
	_, err := DB.Exec(
		`UPDATE backtest_jobs SET status = 'failed', error_msg = ?, finished_at = ? WHERE id = ?`,
		errMsg, nowRFC3339BT(), id,
	)
	return err
}

// DeleteBacktestJob 删除回测任务。
func DeleteBacktestJob(id int64) error {
	_, err := DB.Exec(`DELETE FROM backtest_jobs WHERE id = ?`, id)
	return err
}
