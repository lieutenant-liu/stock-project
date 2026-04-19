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

// autoSyncConfigHandler 管理自动任务配置读写。
func autoSyncConfigHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := db.GetAutoSyncConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, map[string]interface{}{
			"data":    cfg,
			"running": autoSyncManager.IsRunning(),
		})
	case http.MethodPut:
		cfg, err := db.GetAutoSyncConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}

		var req autoSyncConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
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
			respondBadRequest(w, err.Error())
			return
		}
		latest, _ := db.GetAutoSyncConfig()
		respondOK(w, map[string]interface{}{"msg": "自动任务配置已更新", "data": latest})
	default:
		respondMethodNotAllowed(w)
	}
}

// autoSyncRunsHandler 返回自动任务运行历史与当前运行状态。
func autoSyncRunsHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
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
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{
		"data":    runs,
		"running": autoSyncManager.IsRunning(),
	})
}

// autoSyncRunNowHandler 手动触发一次自动任务执行。
func autoSyncRunNowHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	if err := autoSyncManager.TriggerNow(); err != nil {
		respondConflict(w, err.Error())
		return
	}

	respondOKMsg(w, "自动任务已触发")
}

// autoSyncRunStepsHandler 返回指定 run 的步骤级执行详情。
func autoSyncRunStepsHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	runIDStr := strings.TrimSpace(r.URL.Query().Get("run_id"))
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil || runID <= 0 {
		respondBadRequest(w, "run_id 参数非法")
		return
	}

	steps, err := db.ListAutoSyncRunSteps(runID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{"data": steps})
}
