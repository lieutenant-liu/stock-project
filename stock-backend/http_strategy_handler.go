package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"stock-backend/db"
)

// StrategyRouter 统一路由 /api/strategies 及其子路径。
func StrategyRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/strategies")
	path = strings.TrimPrefix(path, "/")

	if prepareJSONWithCORS(w, r) {
		return
	}

	switch {
	case path == "" && r.Method == http.MethodGet:
		strategyListHandler(w, r)
	case path == "active" && r.Method == http.MethodGet:
		strategyActiveHandler(w, r)
	case path == "toggle" && r.Method == http.MethodPost:
		strategyToggleHandler(w, r)
	default:
		respondBadRequest(w, "未知端点: "+path)
	}
}

// strategyListHandler GET /api/strategies — 返回全部策略元数据。
func strategyListHandler(w http.ResponseWriter, r *http.Request) {
	strategies, err := db.GetAllStrategies()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"strategies": strategies})
}

// strategyActiveHandler GET /api/strategies/active — 返回已启用策略。
func strategyActiveHandler(w http.ResponseWriter, r *http.Request) {
	strategies, err := db.GetEnabledStrategies()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"strategies": strategies})
}

// strategyToggleHandler POST /api/strategies/toggle — 更新策略开关。
func strategyToggleHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string `json:"id"`
		IsEnabled bool   `json:"is_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}
	if req.ID == "" {
		respondBadRequest(w, "缺少 id 参数")
		return
	}

	if err := db.ToggleStrategy(req.ID, req.IsEnabled); err != nil {
		respondInternalError(w, err)
		return
	}

	respondOKMsg(w, "策略状态已更新")
}
