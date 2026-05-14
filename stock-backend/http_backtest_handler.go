package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"stock-backend/backtest"
	"stock-backend/db"
)

const exportsDir = "exports"

// BacktestRouter 统一路由 /api/backtest 及其子路径。
func BacktestRouter(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/backtest")
	path = strings.TrimPrefix(path, "/")

	// /api/backtest/plan/...
	if strings.HasPrefix(path, "plan") {
		planPath := strings.TrimPrefix(path, "plan")
		planPath = strings.TrimPrefix(planPath, "/")
		backtestPlanRouter(w, r, planPath)
		return
	}

	// /api/backtest/download?job_id=X
	if path == "download" {
		backtestDownloadHandler(w, r)
		return
	}

	// /api/backtest/list
	if path == "list" {
		backtestListHandler(w, r)
		return
	}

	// /api/backtest/{id}
	if path != "" {
		backtestJobHandler(w, r, path)
		return
	}

	// /api/backtest (root)
	switch r.Method {
	case http.MethodPost:
		backtestSubmitHandler(w, r)
	case http.MethodOptions:
		// preflight already handled
	default:
		respondMethodNotAllowed(w)
	}
}

// backtestSubmitHandler 处理 POST /api/backtest — 提交异步回测任务。
func backtestSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	var cfg backtest.BacktestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}

	cfg.StartDate = sanitizeDate(cfg.StartDate)
	cfg.EndDate = sanitizeDate(cfg.EndDate)
	if cfg.StartDate == "" || cfg.EndDate == "" {
		respondBadRequest(w, "起止日期不能为空，格式: YYYYMMDD 或 YYYY-MM-DD")
		return
	}

	configJSON, _ := json.Marshal(cfg)
	jobID, err := db.CreateBacktestJob(string(configJSON))
	if err != nil {
		respondInternalError(w, err)
		return
	}

	go runBacktestJob(jobID, cfg)

	respondOK(w, map[string]interface{}{
		"job_id": jobID,
		"status": "pending",
	})
}

// runBacktestJob 异步执行回测任务。
func runBacktestJob(jobID int64, cfg backtest.BacktestConfig) {
	// P0: 防止 panic 导致静默丢失任务
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Printf("[回测任务 #%d] PANIC: %v\n%s", jobID, r, stack)
			if err := db.FailBacktestJob(jobID, fmt.Sprintf("内部崩溃: %v", r)); err != nil {
				log.Printf("[回测任务 #%d] 标记失败也出错: %v", jobID, err)
			}
		}
	}()

	log.Printf("[回测任务 #%d] 开始执行", jobID)
	if err := db.UpdateBacktestJobStatus(jobID, "running", "初始化中..."); err != nil {
		log.Printf("[回测任务 #%d] 更新状态失败: %v", jobID, err)
	}

	result, err := backtest.RunV2(cfg)
	if err != nil {
		log.Printf("[回测任务 #%d] 失败: %v", jobID, err)
		if err := db.FailBacktestJob(jobID, err.Error()); err != nil {
			log.Printf("[回测任务 #%d] 标记失败出错: %v", jobID, err)
		}
		return
	}

	resultJSON, _ := json.Marshal(result)

	// 导出 CSV
	csvPath := ""
	if path, err := backtest.ExportCSV(result, exportsDir); err != nil {
		log.Printf("[回测任务 #%d] CSV导出失败: %v", jobID, err)
	} else {
		csvPath = path
		log.Printf("[回测任务 #%d] CSV已导出: %s", jobID, path)
	}

	if err := db.FinishBacktestJob(jobID, string(resultJSON), csvPath); err != nil {
		log.Printf("[回测任务 #%d] 写入结果失败: %v", jobID, err)
	} else {
		log.Printf("[回测任务 #%d] 完成", jobID)
	}
}

// backtestJobHandler 处理 /api/backtest/{id} — 查询/删除单个任务。
func backtestJobHandler(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondBadRequest(w, "无效的任务 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		job, err := db.GetBacktestJob(id)
		if err != nil {
			respondBadRequest(w, "任务不存在")
			return
		}
		resp := map[string]interface{}{
			"job_id":      job.ID,
			"status":      job.Status,
			"progress":    job.Progress,
			"created_at":  job.CreatedAt,
			"started_at":  job.StartedAt,
			"finished_at": job.FinishedAt,
		}
		if job.Status == "completed" && job.ResultJSON != "" {
			var result interface{}
			if json.Unmarshal([]byte(job.ResultJSON), &result) == nil {
				resp["data"] = result
			}
			resp["csv_path"] = job.CSVPath
		}
		if job.Status == "failed" {
			resp["error"] = job.ErrorMsg
		}
		respondOK(w, resp)

	case http.MethodDelete:
		if err := db.DeleteBacktestJob(id); err != nil {
			respondInternalError(w, err)
			return
		}
		respondOKMsg(w, "任务已删除")

	case http.MethodOptions:
		// preflight already handled

	default:
		respondMethodNotAllowed(w)
	}
}

// backtestListHandler 处理 GET /api/backtest/list — 列出历史任务。
func backtestListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	jobs, err := db.ListBacktestJobs(limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	var list []map[string]interface{}
	for _, j := range jobs {
		item := map[string]interface{}{
			"job_id":      j.ID,
			"status":      j.Status,
			"progress":    j.Progress,
			"created_at":  j.CreatedAt,
			"started_at":  j.StartedAt,
			"finished_at": j.FinishedAt,
		}
		// 从 config_json 提取策略名供前端展示
		var cfg struct {
			Strategy  string `json:"strategy"`
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
		}
		if json.Unmarshal([]byte(j.ConfigJSON), &cfg) == nil {
			item["strategy"] = cfg.Strategy
			item["start_date"] = cfg.StartDate
			item["end_date"] = cfg.EndDate
		}
		if j.Status == "failed" {
			item["error"] = j.ErrorMsg
		}
		list = append(list, item)
	}

	respondOK(w, map[string]interface{}{
		"data": list,
	})
}

// backtestDownloadHandler 处理 GET /api/backtest/download?job_id=X — CSV 下载。
func backtestDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	jobIDStr := r.URL.Query().Get("job_id")
	if jobIDStr == "" {
		respondBadRequest(w, "缺少 job_id 参数")
		return
	}

	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		respondBadRequest(w, "无效的 job_id")
		return
	}

	job, err := db.GetBacktestJob(jobID)
	if err != nil {
		respondBadRequest(w, "任务不存在")
		return
	}

	if job.CSVPath == "" {
		respondBadRequest(w, "该任务暂无 CSV 报表")
		return
	}

	if _, err := os.Stat(job.CSVPath); os.IsNotExist(err) {
		respondBadRequest(w, "报表文件已被清理，请重新执行回测")
		return
	}

	filename := filepath.Base(job.CSVPath)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, job.CSVPath)
}

// BacktestDownloadHandler 保留旧签名兼容路由注册（已废弃，使用 BacktestRouter）。
func BacktestDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}
	backtestDownloadHandler(w, r)
}

// BacktestHandler 保留旧签名兼容路由注册（已废弃，使用 BacktestRouter）。
func BacktestHandler(w http.ResponseWriter, r *http.Request) {
	BacktestRouter(w, r)
}

// ─── 回测计划 (Plan) 路由与处理器 ───

// backtestPlanRouter 路由 /api/backtest/plan 及其子路径。
func backtestPlanRouter(w http.ResponseWriter, r *http.Request, path string) {
	// /api/backtest/plan/download?task_id=X
	if path == "download" {
		planDownloadHandler(w, r)
		return
	}

	// /api/backtest/plan/list
	if path == "list" {
		planListHandler(w, r)
		return
	}

	// /api/backtest/plan/{id}
	if path != "" {
		planJobHandler(w, r, path)
		return
	}

	// /api/backtest/plan (root)
	switch r.Method {
	case http.MethodPost:
		planSubmitHandler(w, r)
	case http.MethodOptions:
		// preflight already handled
	default:
		respondMethodNotAllowed(w)
	}
}

// planSubmitHandler 处理 POST /api/backtest/plan — 提交回测计划。
func planSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	var req struct {
		Name  string                `json:"name"`
		Tasks []backtest.BacktestConfig `json:"tasks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}

	if len(req.Tasks) == 0 {
		respondBadRequest(w, "任务列表不能为空")
		return
	}

	// 校验每个任务的日期
	for i := range req.Tasks {
		req.Tasks[i].StartDate = sanitizeDate(req.Tasks[i].StartDate)
		req.Tasks[i].EndDate = sanitizeDate(req.Tasks[i].EndDate)
		if req.Tasks[i].StartDate == "" || req.Tasks[i].EndDate == "" {
			respondBadRequest(w, fmt.Sprintf("任务 #%d 起止日期不能为空", i+1))
			return
		}
	}

	// 将 configs 转为 []interface{} 供 CreatePlan 使用
	configs := make([]interface{}, len(req.Tasks))
	for i := range req.Tasks {
		configs[i] = req.Tasks[i]
	}

	planID, err := db.CreatePlan(req.Name, configs)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// 加载刚创建的 tasks 获取 taskIDs
	tasks, _ := db.GetPlanTasks(planID)
	taskInputs := make([]backtest.PlanTaskInput, len(tasks))
	for i, t := range tasks {
		var cfg backtest.BacktestConfig
		json.Unmarshal([]byte(t.ConfigJSON), &cfg)
		taskInputs[i] = backtest.PlanTaskInput{TaskID: t.ID, Config: cfg}
	}

	go runPlanJob(planID, taskInputs)

	respondOK(w, map[string]interface{}{
		"plan_id":    planID,
		"task_count": len(req.Tasks),
		"status":     "pending",
	})
}

// runPlanJob 异步执行回测计划。
func runPlanJob(planID int64, tasks []backtest.PlanTaskInput) {
	// P0: 防止 panic 导致静默丢失计划
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Printf("[回测计划 #%d] PANIC: %v\n%s", planID, r, stack)
			if err := db.FailPlan(planID, fmt.Sprintf("内部崩溃: %v", r)); err != nil {
				log.Printf("[回测计划 #%d] 标记失败也出错: %v", planID, err)
			}
		}
	}()

	log.Printf("[回测计划 #%d] 开始执行 | %d 个任务", planID, len(tasks))
	if err := db.UpdatePlanStatus(planID, "running", "数据加载中..."); err != nil {
		log.Printf("[回测计划 #%d] 更新状态失败: %v", planID, err)
	}

	outputs := backtest.RunPlan(tasks, func(completed, total int) {
		if err := db.UpdatePlanProgress(planID, completed); err != nil {
			log.Printf("[回测计划 #%d] 更新进度失败: %v", planID, err)
		}
		if err := db.UpdatePlanStatus(planID, "running", fmt.Sprintf("%d/%d 任务完成", completed, total)); err != nil {
			log.Printf("[回测计划 #%d] 更新状态失败: %v", planID, err)
		}
	})

	completedCount := 0
	for _, out := range outputs {
		if out.Error != nil {
			if err := db.FailPlanTask(out.TaskID, out.Error.Error()); err != nil {
				log.Printf("[计划任务 #%d] 标记失败出错: %v", out.TaskID, err)
			}
		} else {
			resultJSON, _ := json.Marshal(out.Result)
			csvPath := ""
			if path, err := backtest.ExportCSV(out.Result, exportsDir); err == nil {
				csvPath = path
			}
			if err := db.FinishPlanTask(out.TaskID, string(resultJSON), csvPath); err != nil {
				log.Printf("[计划任务 #%d] 标记完成出错: %v", out.TaskID, err)
			}
			completedCount++
		}
	}

	if err := db.FinishPlan(planID); err != nil {
		log.Printf("[回测计划 #%d] 标记完成出错: %v", planID, err)
	}
	log.Printf("[回测计划 #%d] 执行完成 | %d/%d 成功", planID, completedCount, len(tasks))
}

// planJobHandler 处理 /api/backtest/plan/{id} — 查询/删除计划。
func planJobHandler(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondBadRequest(w, "无效的计划 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		plan, err := db.GetPlan(id)
		if err != nil {
			respondBadRequest(w, "计划不存在")
			return
		}
		tasks, _ := db.GetPlanTasks(id)

		var taskList []map[string]interface{}
		for _, t := range tasks {
			item := map[string]interface{}{
				"task_id":     t.ID,
				"status":      t.Status,
				"created_at":  t.CreatedAt,
				"finished_at": t.FinishedAt,
			}
			// 从 config_json 提取字段
			var cfg struct {
				Strategy       string  `json:"strategy"`
				StartDate      string  `json:"start_date"`
				EndDate        string  `json:"end_date"`
				InitialCapital float64 `json:"initial_capital"`
				Commission     float64 `json:"commission"`
			}
			if json.Unmarshal([]byte(t.ConfigJSON), &cfg) == nil {
				item["strategy"] = cfg.Strategy
				item["start_date"] = cfg.StartDate
				item["end_date"] = cfg.EndDate
				item["initial_capital"] = cfg.InitialCapital
				item["commission"] = cfg.Commission
			}
			if t.Status == "completed" && t.ResultJSON != "" {
				var result interface{}
				if json.Unmarshal([]byte(t.ResultJSON), &result) == nil {
					item["result"] = result
				}
				item["csv_path"] = t.CSVPath
			}
			if t.Status == "failed" {
				item["error"] = t.ErrorMsg
			}
			taskList = append(taskList, item)
		}

		respondOK(w, map[string]interface{}{
			"plan_id":         plan.ID,
			"name":            plan.Name,
			"status":          plan.Status,
			"progress":        plan.Progress,
			"task_count":      plan.TaskCount,
			"completed_count": plan.CompletedCount,
			"created_at":      plan.CreatedAt,
			"started_at":      plan.StartedAt,
			"finished_at":     plan.FinishedAt,
			"tasks":           taskList,
		})

	case http.MethodDelete:
		if err := db.DeletePlan(id); err != nil {
			respondInternalError(w, err)
			return
		}
		respondOKMsg(w, "计划已删除")

	case http.MethodOptions:
		// preflight already handled

	default:
		respondMethodNotAllowed(w)
	}
}

// planListHandler 处理 GET /api/backtest/plan/list — 列出历史计划。
func planListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	plans, err := db.ListPlans(limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	var list []map[string]interface{}
	for _, p := range plans {
		list = append(list, map[string]interface{}{
			"plan_id":         p.ID,
			"name":            p.Name,
			"status":          p.Status,
			"progress":        p.Progress,
			"task_count":      p.TaskCount,
			"completed_count": p.CompletedCount,
			"created_at":      p.CreatedAt,
			"started_at":      p.StartedAt,
			"finished_at":     p.FinishedAt,
		})
	}

	respondOK(w, map[string]interface{}{
		"data": list,
	})
}

// planDownloadHandler 处理 GET /api/backtest/plan/download?task_id=X — 计划任务 CSV 下载。
func planDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	taskIDStr := r.URL.Query().Get("task_id")
	if taskIDStr == "" {
		respondBadRequest(w, "缺少 task_id 参数")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		respondBadRequest(w, "无效的 task_id")
		return
	}

	task, err := db.GetPlanTask(taskID)
	if err != nil {
		respondBadRequest(w, "任务不存在")
		return
	}

	if task.CSVPath == "" {
		respondBadRequest(w, "该任务暂无 CSV 报表")
		return
	}

	if _, err := os.Stat(task.CSVPath); os.IsNotExist(err) {
		respondBadRequest(w, "报表文件已被清理，请重新执行回测")
		return
	}

	filename := filepath.Base(task.CSVPath)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, task.CSVPath)
}
