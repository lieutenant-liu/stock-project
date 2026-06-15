package main

// ============================================================
// http_backtest_handler.go - 回测 HTTP 处理器
// ============================================================
// 这个文件处理回测相关的 HTTP 请求，包括：
// 1. 单任务回测（/api/backtest）
// 2. 批量回测计划（/api/backtest/plan）
// 3. 结果查询、下载、删除
//
// 【Go 语言知识点】
// - context.Context: 用于控制 goroutine 的生命周期（取消、超时）
// - defer + recover: 捕获 panic，防止程序崩溃
// - debug.Stack(): 获取堆栈信息，用于调试
// - http.ServeFile: 提供文件下载
// ============================================================

import (
	"context"       // 上下文控制（取消、超时）
	"encoding/json" // JSON 编解码
	"fmt"           // 格式化输出
	"log"           // 日志
	"net/http"      // HTTP 服务器
	"os"            // 文件操作
	"path/filepath" // 文件路径处理
	"runtime/debug" // 堆栈调试信息
	"strconv"       // 字符串转数字
	"strings"       // 字符串处理

	"stock-backend/backtest" // 回测引擎
	"stock-backend/db"       // 数据库
)

// exportsDir CSV 导出目录
const exportsDir = "exports"

// ------------------------------------------------------------
// 回测路由分发器
// ------------------------------------------------------------

// BacktestRouter 统一路由 /api/backtest 及其子路径。
// 【功能说明】
// 这是一个路由分发器，根据 URL 路径将请求分发到不同的处理器：
// - /api/backtest          → 提交回测任务 (POST) 或查询单个任务 (GET)
// - /api/backtest/list     → 列出历史任务
// - /api/backtest/download → 下载 CSV 报表
// - /api/backtest/plan/*   → 回测计划相关操作
//
// 【Go 语言知识点：路径匹配】
// Go 的 http.HandleFunc 会匹配所有以指定前缀开头的路径。
// 例如 "/api/backtest" 会匹配 "/api/backtest/123"、"/api/backtest/list" 等。
// 所以需要手动解析路径，分发到对应的处理器。
func BacktestRouter(w http.ResponseWriter, r *http.Request) {
	// 处理 CORS 预检请求
	if prepareJSONWithCORS(w, r) {
		return
	}

	// 去掉前缀，获取子路径
	// 例如: "/api/backtest/plan/123" → "plan/123"
	path := strings.TrimPrefix(r.URL.Path, "/api/backtest")
	path = strings.TrimPrefix(path, "/")

	// ── /api/backtest/plan/... ──
	if strings.HasPrefix(path, "plan") {
		planPath := strings.TrimPrefix(path, "plan")
		planPath = strings.TrimPrefix(planPath, "/")
		backtestPlanRouter(w, r, planPath) // 分发到计划路由器
		return
	}

	// ── /api/backtest/download?job_id=X ──
	if path == "download" {
		backtestDownloadHandler(w, r)
		return
	}

	// ── /api/backtest/list ──
	if path == "list" {
		backtestListHandler(w, r)
		return
	}

	// ── /api/backtest/{id} ──
	if path != "" {
		backtestJobHandler(w, r, path) // 处理单个任务的查询/删除
		return
	}

	// ── /api/backtest (根路径) ──
	switch r.Method {
	case http.MethodPost:
		backtestSubmitHandler(w, r) // 提交新回测任务
	case http.MethodOptions:
		// 预检已在上面处理
	default:
		respondMethodNotAllowed(w)
	}
}

// ------------------------------------------------------------
// 提交回测任务
// ------------------------------------------------------------

// backtestSubmitHandler 处理 POST /api/backtest — 提交异步回测任务。
// 【功能说明】
// 接收前端提交的回测配置，创建数据库记录，然后在后台 goroutine 中执行回测。
//
// 【请求体示例】
// {
//   "strategy": "MACB",
//   "start_date": "20230101",
//   "end_date": "20241231",
//   "initial_capital": 100000,
//   "commission": 0.0003
// }
func backtestSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	// 解析请求体
	var cfg backtest.BacktestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}

	// 清洗日期参数
	cfg.StartDate = sanitizeDate(cfg.StartDate)
	cfg.EndDate = sanitizeDate(cfg.EndDate)
	if cfg.StartDate == "" || cfg.EndDate == "" {
		respondBadRequest(w, "起止日期不能为空，格式: YYYYMMDD 或 YYYY-MM-DD")
		return
	}

	// 将配置序列化为 JSON，存入数据库
	configJSON, _ := json.Marshal(cfg)
	jobID, err := db.CreateBacktestJob(string(configJSON))
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// 在后台 goroutine 中执行回测（异步）
	// appCtx 是全局上下文，服务关闭时会取消所有正在运行的任务
	go runBacktestJob(appCtx, jobID, cfg)

	// 立即返回任务 ID，前端可以轮询查询进度
	respondOK(w, map[string]interface{}{
		"job_id": jobID,
		"status": "pending",
	})
}

// runBacktestJob 异步执行回测任务。
// 【Go 语言知识点：defer + recover】
// defer: 函数结束时执行（无论正常返回还是 panic）
// recover: 捕获 panic，防止程序崩溃
// 组合使用可以优雅地处理运行时错误。
func runBacktestJob(ctx context.Context, jobID int64, cfg backtest.BacktestConfig) {
	// P0: 防止 panic 导致静默丢失任务
	// 如果回测过程中发生 panic（如数组越界），这里会捕获并记录错误
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack() // 获取堆栈信息
			log.Printf("[回测任务 #%d] PANIC: %v\n%s", jobID, r, stack)
			// 将任务标记为失败
			if err := db.FailBacktestJob(jobID, fmt.Sprintf("内部崩溃: %v", r)); err != nil {
				log.Printf("[回测任务 #%d] 标记失败也出错: %v", jobID, err)
			}
		}
	}()

	// 更新任务状态为 "running"
	log.Printf("[回测任务 #%d] 开始执行", jobID)
	if err := db.UpdateBacktestJobStatus(jobID, "running", "初始化中..."); err != nil {
		log.Printf("[回测任务 #%d] 更新状态失败: %v", jobID, err)
	}

	// 执行回测（这是核心逻辑，可能耗时很长）
	result, err := backtest.RunV2(ctx, cfg)
	if err != nil {
		// 回测失败
		log.Printf("[回测任务 #%d] 失败: %v", jobID, err)
		if err := db.FailBacktestJob(jobID, err.Error()); err != nil {
			log.Printf("[回测任务 #%d] 标记失败出错: %v", jobID, err)
		}
		return
	}

	// 回测成功，序列化结果
	resultJSON, _ := json.Marshal(result)

	// 导出 CSV 报表
	csvPath := ""
	if path, err := backtest.ExportCSV(result, exportsDir); err != nil {
		log.Printf("[回测任务 #%d] CSV导出失败: %v", jobID, err)
	} else {
		csvPath = path
		log.Printf("[回测任务 #%d] CSV已导出: %s", jobID, path)
	}

	// 将结果写入数据库
	if err := db.FinishBacktestJob(jobID, string(resultJSON), csvPath); err != nil {
		log.Printf("[回测任务 #%d] 写入结果失败: %v", jobID, err)
	} else {
		log.Printf("[回测任务 #%d] 完成", jobID)
	}
}

// ------------------------------------------------------------
// 查询/删除单个任务
// ------------------------------------------------------------

// backtestJobHandler 处理 /api/backtest/{id} — 查询/删除单个任务。
// 【触发方式】
// GET    /api/backtest/123  → 查询任务详情
// DELETE /api/backtest/123  → 删除任务
func backtestJobHandler(w http.ResponseWriter, r *http.Request, idStr string) {
	// 将字符串 ID 转为 int64
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondBadRequest(w, "无效的任务 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 从数据库获取任务
		job, err := db.GetBacktestJob(id)
		if err != nil {
			respondBadRequest(w, "任务不存在")
			return
		}

		// 构建响应
		resp := map[string]interface{}{
			"job_id":      job.ID,
			"status":      job.Status,
			"progress":    job.Progress,
			"created_at":  job.CreatedAt,
			"started_at":  job.StartedAt,
			"finished_at": job.FinishedAt,
		}

		// 如果任务完成，附加结果数据
		if job.Status == "completed" && job.ResultJSON != "" {
			var result interface{}
			if json.Unmarshal([]byte(job.ResultJSON), &result) == nil {
				resp["data"] = result
			}
			resp["csv_path"] = job.CSVPath
		}

		// 如果任务失败，附加错误信息
		if job.Status == "failed" {
			resp["error"] = job.ErrorMsg
		}
		respondOK(w, resp)

	case http.MethodDelete:
		// 删除任务
		if err := db.DeleteBacktestJob(id); err != nil {
			respondInternalError(w, err)
			return
		}
		respondOKMsg(w, "任务已删除")

	case http.MethodOptions:
		// 预检已在上面处理

	default:
		respondMethodNotAllowed(w)
	}
}

// ------------------------------------------------------------
// 列出历史任务
// ------------------------------------------------------------

// backtestListHandler 处理 GET /api/backtest/list — 列出历史任务。
// 【触发方式】
// GET /api/backtest/list?limit=20
func backtestListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	// 获取 limit 参数（默认 20，最大 100）
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	// 从数据库查询任务列表
	jobs, err := db.ListBacktestJobs(limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// 构建响应列表
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

		// 如果任务失败，附加错误信息
		if j.Status == "failed" {
			item["error"] = j.ErrorMsg
		}
		list = append(list, item)
	}

	respondOK(w, map[string]interface{}{
		"data": list,
	})
}

// ------------------------------------------------------------
// 下载 CSV 报表
// ------------------------------------------------------------

// backtestDownloadHandler 处理 GET /api/backtest/download?job_id=X — CSV 下载。
// 【功能说明】
// 回测完成后会生成 CSV 报表（包含每笔交易的详细记录）。
// 前端通过此接口下载报表文件。
//
// 【Go 语言知识点：http.ServeFile】
// http.ServeFile 会自动设置 Content-Type 和 Content-Disposition 头，
// 浏览器收到后会触发文件下载。
func backtestDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	// 获取 job_id 参数
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

	// 从数据库获取任务
	job, err := db.GetBacktestJob(jobID)
	if err != nil {
		respondBadRequest(w, "任务不存在")
		return
	}

	// 检查是否有 CSV 文件
	if job.CSVPath == "" {
		respondBadRequest(w, "该任务暂无 CSV 报表")
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(job.CSVPath); os.IsNotExist(err) {
		respondBadRequest(w, "报表文件已被清理，请重新执行回测")
		return
	}

	// 设置响应头，触发文件下载
	filename := filepath.Base(job.CSVPath)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, job.CSVPath) // 提供文件下载
}

// ------------------------------------------------------------
// 兼容旧接口
// ------------------------------------------------------------

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

// ============================================================
// 回测计划 (Plan) 路由与处理器
// ============================================================
// 回测计划是一组相关回测任务的容器。
// 例如：用同一个策略在不同参数下回测，形成参数优化计划。
// ============================================================

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

	// /api/backtest/plan (根路径)
	switch r.Method {
	case http.MethodPost:
		planSubmitHandler(w, r) // 提交新计划
	case http.MethodOptions:
		// 预检已在上面处理
	default:
		respondMethodNotAllowed(w)
	}
}

// planSubmitHandler 处理 POST /api/backtest/plan — 提交回测计划。
// 【请求体示例】
// {
//   "name": "MACB 参数优化",
//   "tasks": [
//     {"strategy": "MACB", "start_date": "20230101", "end_date": "20241231", "initial_capital": 100000},
//     {"strategy": "MACB", "start_date": "20230101", "end_date": "20241231", "initial_capital": 200000}
//   ]
// }
func planSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	// 解析请求体
	var req struct {
		Name  string                    `json:"name"`  // 计划名称
		Tasks []backtest.BacktestConfig `json:"tasks"` // 任务列表
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

	// 创建计划记录
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

	// 在后台执行计划
	go runPlanJob(appCtx, planID, taskInputs)

	respondOK(w, map[string]interface{}{
		"plan_id":    planID,
		"task_count": len(req.Tasks),
		"status":     "pending",
	})
}

// runPlanJob 异步执行回测计划。
func runPlanJob(ctx context.Context, planID int64, tasks []backtest.PlanTaskInput) {
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

	// 执行计划（内部会并发执行多个任务）
	outputs := backtest.RunPlan(ctx, tasks, func(completed, total int) {
		// 进度回调：更新数据库中的进度
		if err := db.UpdatePlanProgress(planID, completed); err != nil {
			log.Printf("[回测计划 #%d] 更新进度失败: %v", planID, err)
		}
		if err := db.UpdatePlanStatus(planID, "running", fmt.Sprintf("%d/%d 任务完成", completed, total)); err != nil {
			log.Printf("[回测计划 #%d] 更新状态失败: %v", planID, err)
		}
	})

	// 处理每个任务的结果
	completedCount := 0
	for _, out := range outputs {
		if out.Error != nil {
			// 任务失败
			if err := db.FailPlanTask(out.TaskID, out.Error.Error()); err != nil {
				log.Printf("[计划任务 #%d] 标记失败出错: %v", out.TaskID, err)
			}
		} else {
			// 任务成功
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

	// 标记计划完成
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
		// 获取计划详情
		plan, err := db.GetPlan(id)
		if err != nil {
			respondBadRequest(w, "计划不存在")
			return
		}
		tasks, _ := db.GetPlanTasks(id)

		// 构建任务列表
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
		// 删除计划
		if err := db.DeletePlan(id); err != nil {
			respondInternalError(w, err)
			return
		}
		respondOKMsg(w, "计划已删除")

	case http.MethodOptions:
		// 预检已在上面处理

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
