package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"stock-backend/db"
	"stock-backend/signallab"
)

// SignalLabRouter 统一路由 /api/signallab 及其子路径。
func SignalLabRouter(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/signallab")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "run" && r.Method == http.MethodPost:
		signalLabRunHandler(w, r)
	case path == "jobs" && r.Method == http.MethodGet:
		signalLabListHandler(w, r)
	case path == "download" && r.Method == http.MethodGet:
		signalLabDownloadHandler(w, r)
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

	go signallab.RunSignalLabJob(jobID, cfg)

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
	idStr := r.URL.Query().Get("job_id")
	if idStr == "" {
		respondBadRequest(w, "缺少 job_id 参数")
		return
	}
	jobID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondBadRequest(w, "job_id 格式无效")
		return
	}

	job, err := db.GetLabJob(jobID)
	if err != nil {
		respondBadRequest(w, fmt.Sprintf("任务 #%d 不存在", jobID))
		return
	}

	if job.Status != "completed" {
		respondBadRequest(w, fmt.Sprintf("任务 #%d 状态为 %s，尚未完成", jobID, job.Status))
		return
	}

	if job.FilePath == "" {
		respondBadRequest(w, "任务完成但无文件路径")
		return
	}

	f, err := os.Open(job.FilePath)
	if err != nil {
		respondInternalError(w, fmt.Errorf("打开文件失败: %w", err))
		return
	}
	defer f.Close()

	filename := fmt.Sprintf("signal_lab_%s_%d.csv", job.Strategy, jobID)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	io.Copy(w, f)
}
