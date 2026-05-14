package db

import (
	"encoding/json"
)

// BacktestPlan 回测计划记录。
type BacktestPlan struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`     // pending/running/completed/failed
	Progress       string `json:"progress"`   // "3/8 任务完成"
	TaskCount      int    `json:"task_count"`
	CompletedCount int    `json:"completed_count"`
	CreatedAt      string `json:"created_at"`
	StartedAt      string `json:"started_at"`
	FinishedAt     string `json:"finished_at"`
}

// BacktestPlanTask 回测计划中的单个任务。
type BacktestPlanTask struct {
	ID         int64  `json:"id"`
	PlanID     int64  `json:"plan_id"`
	ConfigJSON string `json:"config_json"`
	Status     string `json:"status"` // pending/running/completed/failed
	ResultJSON string `json:"result_json"`
	ErrorMsg   string `json:"error_msg"`
	CSVPath    string `json:"csv_path"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at"`
}

// CreatePlan 事务内创建计划及所有任务。
func CreatePlan(name string, configs []interface{}) (int64, error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := nowRFC3339BT()
	res, err := tx.Exec(
		`INSERT INTO backtest_plans (name, status, task_count, created_at) VALUES (?, 'pending', ?, ?)`,
		name, len(configs), now,
	)
	if err != nil {
		return 0, err
	}
	planID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, cfg := range configs {
		cfgJSON, _ := json.Marshal(cfg)
		_, err := tx.Exec(
			`INSERT INTO backtest_plan_tasks (plan_id, config_json, status, created_at) VALUES (?, ?, 'pending', ?)`,
			planID, string(cfgJSON), now,
		)
		if err != nil {
			return 0, err
		}
	}

	return planID, tx.Commit()
}

// GetPlan 按 ID 查询计划。
func GetPlan(id int64) (*BacktestPlan, error) {
	row := DB.QueryRow(
		`SELECT id, name, status, progress, task_count, completed_count,
		 created_at, COALESCE(started_at,''), COALESCE(finished_at,'')
		 FROM backtest_plans WHERE id = ?`, id,
	)
	p := &BacktestPlan{}
	err := row.Scan(&p.ID, &p.Name, &p.Status, &p.Progress, &p.TaskCount,
		&p.CompletedCount, &p.CreatedAt, &p.StartedAt, &p.FinishedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ListPlans 列出最近的计划（倒序）。
func ListPlans(limit int) ([]BacktestPlan, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := DB.Query(
		`SELECT id, name, status, progress, task_count, completed_count,
		 created_at, COALESCE(started_at,''), COALESCE(finished_at,'')
		 FROM backtest_plans ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []BacktestPlan
	for rows.Next() {
		var p BacktestPlan
		if err := rows.Scan(&p.ID, &p.Name, &p.Status, &p.Progress, &p.TaskCount,
			&p.CompletedCount, &p.CreatedAt, &p.StartedAt, &p.FinishedAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

// GetPlanTasks 查询计划下的所有任务。
func GetPlanTasks(planID int64) ([]BacktestPlanTask, error) {
	rows, err := DB.Query(
		`SELECT id, plan_id, config_json, status, COALESCE(result_json,''),
		 COALESCE(error_msg,''), COALESCE(csv_path,''), created_at, COALESCE(finished_at,'')
		 FROM backtest_plan_tasks WHERE plan_id = ? ORDER BY id`, planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []BacktestPlanTask
	for rows.Next() {
		var t BacktestPlanTask
		if err := rows.Scan(&t.ID, &t.PlanID, &t.ConfigJSON, &t.Status, &t.ResultJSON,
			&t.ErrorMsg, &t.CSVPath, &t.CreatedAt, &t.FinishedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetPlanTask 按任务 ID 查询单个计划任务。
func GetPlanTask(taskID int64) (*BacktestPlanTask, error) {
	row := DB.QueryRow(
		`SELECT id, plan_id, config_json, status, COALESCE(result_json,''),
		 COALESCE(error_msg,''), COALESCE(csv_path,''), created_at, COALESCE(finished_at,'')
		 FROM backtest_plan_tasks WHERE id = ?`, taskID,
	)
	t := &BacktestPlanTask{}
	err := row.Scan(&t.ID, &t.PlanID, &t.ConfigJSON, &t.Status, &t.ResultJSON,
		&t.ErrorMsg, &t.CSVPath, &t.CreatedAt, &t.FinishedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// UpdatePlanStatus 更新计划状态。
func UpdatePlanStatus(id int64, status, progress string) error {
	if status == "running" {
		_, err := DB.Exec(
			`UPDATE backtest_plans SET status = ?, progress = ?, started_at = ? WHERE id = ?`,
			status, progress, nowRFC3339BT(), id,
		)
		return err
	}
	_, err := DB.Exec(
		`UPDATE backtest_plans SET status = ?, progress = ? WHERE id = ?`,
		status, progress, id,
	)
	return err
}

// UpdatePlanProgress 更新计划完成计数和进度文本。
func UpdatePlanProgress(id int64, completed int) error {
	_, err := DB.Exec(
		`UPDATE backtest_plans SET completed_count = ? WHERE id = ?`,
		completed, id,
	)
	return err
}

// FinishPlanTask 标记任务完成。
func FinishPlanTask(taskID int64, resultJSON, csvPath string) error {
	_, err := DB.Exec(
		`UPDATE backtest_plan_tasks SET status = 'completed', result_json = ?, csv_path = ?, finished_at = ? WHERE id = ?`,
		resultJSON, csvPath, nowRFC3339BT(), taskID,
	)
	return err
}

// FailPlanTask 标记任务失败。
func FailPlanTask(taskID int64, errMsg string) error {
	_, err := DB.Exec(
		`UPDATE backtest_plan_tasks SET status = 'failed', error_msg = ?, finished_at = ? WHERE id = ?`,
		errMsg, nowRFC3339BT(), taskID,
	)
	return err
}

// FinishPlan 标记计划完成。
func FinishPlan(id int64) error {
	_, err := DB.Exec(
		`UPDATE backtest_plans SET status = 'completed', finished_at = ? WHERE id = ?`,
		nowRFC3339BT(), id,
	)
	return err
}

// FailPlan 标记计划失败。
func FailPlan(id int64, errMsg string) error {
	_, err := DB.Exec(
		`UPDATE backtest_plans SET status = 'failed', progress = ?, finished_at = ? WHERE id = ?`,
		errMsg, nowRFC3339BT(), id,
	)
	return err
}

// DeletePlan 事务内删除计划及所有任务。
func DeletePlan(id int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM backtest_plan_tasks WHERE plan_id = ?`, id)
	tx.Exec(`DELETE FROM backtest_plans WHERE id = ?`, id)
	return tx.Commit()
}
