package main

import (
	"encoding/json"
	"net/http"
	"stock-backend/db"
	"strconv"
	"strings"
)

type autoSyncConfigUpdateRequest struct {
	Enabled         *bool  `json:"enabled"`
	Timezone        string `json:"timezone"`
	DailyRunTime    string `json:"daily_run_time"`
	LookbackDays    int    `json:"lookback_days"`
	RetryLimit      int    `json:"retry_limit"`
	RetryBackoffSec int    `json:"retry_backoff_sec"`
}

func autoSyncConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := db.GetAutoSyncConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 500, "msg": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"data":    cfg,
			"running": autoSyncManager.IsRunning(),
		})
	case http.MethodPut:
		cfg, err := db.GetAutoSyncConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 500, "msg": err.Error()})
			return
		}

		var req autoSyncConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
			return
		}

		if req.Enabled != nil {
			cfg.Enabled = *req.Enabled
		}
		if strings.TrimSpace(req.Timezone) != "" {
			cfg.Timezone = strings.TrimSpace(req.Timezone)
		}
		if strings.TrimSpace(req.DailyRunTime) != "" {
			cfg.DailyRunTime = strings.TrimSpace(req.DailyRunTime)
		}
		if req.LookbackDays > 0 {
			cfg.LookbackDays = req.LookbackDays
		}
		if req.RetryLimit > 0 {
			cfg.RetryLimit = req.RetryLimit
		}
		if req.RetryBackoffSec > 0 {
			cfg.RetryBackoffSec = req.RetryBackoffSec
		}

		if err := db.SaveAutoSyncConfig(cfg); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		latest, _ := db.GetAutoSyncConfig()
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "自动任务配置已更新", "data": latest})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
	}
}

func autoSyncRunsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}

	limit := 20
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	runs, err := db.ListAutoSyncRuns(limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 500, "msg": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    200,
		"data":    runs,
		"running": autoSyncManager.IsRunning(),
	})
}

func autoSyncRunNowHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}

	if err := autoSyncManager.TriggerNow(); err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 409, "msg": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "自动任务已触发"})
}

func autoSyncRunStepsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}

	runIDStr := strings.TrimSpace(r.URL.Query().Get("run_id"))
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil || runID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "run_id 参数非法"})
		return
	}

	steps, err := db.ListAutoSyncRunSteps(runID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 500, "msg": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": steps})
}
