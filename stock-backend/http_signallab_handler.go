package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"stock-backend/db"
	"stock-backend/logger"
	"stock-backend/signallab"
)

// SignalLabRouter 统一路由 /api/signallab 及其子路径。
func SignalLabRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/signallab")
	path = strings.TrimPrefix(path, "/")

	// download 路由：只设 CORS，不设 Content-Type（由 handler 自行设置）
	if path == "download" && r.Method == http.MethodGet {
		setCORSHeaders(w)
		signalLabDownloadHandler(w, r)
		return
	}

	if prepareJSONWithCORS(w, r) {
		return
	}

	switch {
	case path == "run" && r.Method == http.MethodPost:
		signalLabRunHandler(w, r)
	case path == "jobs" && r.Method == http.MethodGet:
		signalLabListHandler(w, r)
	default:
		respondBadRequest(w, "未知端点: "+path)
	}
}

// signalLabRunHandler 处理 POST /api/signallab/run — 提交信号实验室任务。
func signalLabRunHandler(w http.ResponseWriter, r *http.Request) {
	var cfg signallab.SignalLabConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}

	cfg.StartDate = sanitizeDate(cfg.StartDate)
	cfg.EndDate = sanitizeDate(cfg.EndDate)
	if cfg.StartDate == "" || cfg.EndDate == "" {
		respondBadRequest(w, "日期格式无效，需要 YYYYMMDD")
		return
	}
	if cfg.Strategy == "" {
		cfg.Strategy = "ALL"
	}

	jobID, err := db.CreateLabJob(cfg.Strategy)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	go signallab.RunSignalLabJob(appCtx, jobID, cfg)

	respondOK(w, map[string]interface{}{
		"job_id": jobID,
		"status": "pending",
	})
}

// signalLabListHandler 处理 GET /api/signallab/jobs — 列出任务。
func signalLabListHandler(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	jobs, err := db.ListLabJobs(limit)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{
		"jobs": jobs,
	})
}

// signalLabDownloadHandler 处理 GET /api/signallab/download?job_id=X — 下载 CSV。
func signalLabDownloadHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 顶级防御：panic recovery
	defer func() {
		if err := recover(); err != nil {
			logger.Error("[LAB-API] 下载接口 Panic: %v", err)
			http.Error(w, "服务器内部崩溃", http.StatusInternalServerError)
		}
	}()

	idStr := r.URL.Query().Get("job_id")
	if idStr == "" {
		http.Error(w, "缺少 job_id 参数", http.StatusBadRequest)
		return
	}
	jobID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "job_id 格式无效", http.StatusBadRequest)
		return
	}

	// 2. 数据防御：严格校验 Job 记录
	job, err := db.GetLabJob(jobID)
	if err != nil || job == nil {
		http.Error(w, fmt.Sprintf("未找到任务 #%d 的记录", jobID), http.StatusNotFound)
		return
	}

	if job.Status != "completed" {
		http.Error(w, fmt.Sprintf("任务 #%d 状态为 %s，尚未完成", jobID, job.Status), http.StatusBadRequest)
		return
	}

	if job.FilePath == "" {
		http.Error(w, "任务完成但无文件路径", http.StatusInternalServerError)
		return
	}

	// 3. 物理防御：确认文件在磁盘上存在
	if _, err := os.Stat(job.FilePath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("文件在服务器上丢失: %s", job.FilePath), http.StatusNotFound)
		return
	}

	// 4. 协议防御：filename 必须用双引号包裹，防止策略名中的特殊字符破坏响应头
	filename := fmt.Sprintf("signal_lab_%s_%d.csv", job.Strategy, jobID)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "text/csv")

	http.ServeFile(w, r, job.FilePath)
}
