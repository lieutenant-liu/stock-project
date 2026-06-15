package main

// ============================================================
// http_strategy_handler.go - 策略管理 HTTP 处理器
// ============================================================
// 这个文件处理策略的查询和开关控制。
//
// 【什么是策略？】
// 策略是量化交易的核心，定义了"什么时候买入，什么时候卖出"。
// 项目内置了多个策略（MACB、CBBM、PBMA 等），用户可以通过接口启用/禁用。
//
// 【Go 语言知识点】
// - switch + case: 多条件分支
// - json.NewDecoder: 从请求 Body 解析 JSON
// ============================================================

import (
	"encoding/json" // JSON 编解码
	"net/http"      // HTTP 服务器
	"strings"       // 字符串处理

	"stock-backend/db" // 数据库操作
)

// StrategyRouter 统一路由 /api/strategies 及其子路径。
// 【路由分发】
// - GET  /api/strategies         → 获取全部策略列表
// - GET  /api/strategies/active  → 获取已启用的策略
// - POST /api/strategies/toggle  → 切换策略开关
func StrategyRouter(w http.ResponseWriter, r *http.Request) {
	// 去掉前缀，获取子路径
	path := strings.TrimPrefix(r.URL.Path, "/api/strategies")
	path = strings.TrimPrefix(path, "/")

	// 处理 CORS 预检请求
	if prepareJSONWithCORS(w, r) {
		return
	}

	// 根据路径和方法分发
	switch {
	case path == "" && r.Method == http.MethodGet:
		strategyListHandler(w, r) // 获取全部策略
	case path == "active" && r.Method == http.MethodGet:
		strategyActiveHandler(w, r) // 获取已启用策略
	case path == "toggle" && r.Method == http.MethodPost:
		strategyToggleHandler(w, r) // 切换策略开关
	default:
		respondBadRequest(w, "未知端点: "+path)
	}
}

// strategyListHandler GET /api/strategies — 返回全部策略元数据。
// 【功能说明】
// 返回所有已注册的策略信息，包括：
// - id: 策略 ID（如 "MACB"）
// - name: 策略名称（如 "均线收敛突破"）
// - category: 策略类别（如 "右侧突破"）
// - description: 策略描述
// - principle: 策略原理
// - win_rate: 历史胜率
// - is_enabled: 是否启用
func strategyListHandler(w http.ResponseWriter, r *http.Request) {
	strategies, err := db.GetAllStrategies()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"strategies": strategies})
}

// strategyActiveHandler GET /api/strategies/active — 返回已启用策略。
// 【功能说明】
// 只返回 is_enabled=true 的策略。
// 回测和策略扫描时使用此接口获取当前生效的策略列表。
func strategyActiveHandler(w http.ResponseWriter, r *http.Request) {
	strategies, err := db.GetEnabledStrategies()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"strategies": strategies})
}

// strategyToggleHandler POST /api/strategies/toggle — 更新策略开关。
// 【功能说明】
// 切换指定策略的启用/禁用状态。
//
// 【请求体示例】
// {
//   "id": "MACB",
//   "is_enabled": false
// }
func strategyToggleHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req struct {
		ID        string `json:"id"`         // 策略 ID
		IsEnabled bool   `json:"is_enabled"` // 是否启用
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}
	if req.ID == "" {
		respondBadRequest(w, "缺少 id 参数")
		return
	}

	// 更新数据库
	if err := db.ToggleStrategy(req.ID, req.IsEnabled); err != nil {
		respondInternalError(w, err)
		return
	}

	respondOKMsg(w, "策略状态已更新")
}
