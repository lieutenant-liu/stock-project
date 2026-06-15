package main

// ============================================================
// http_engine_handler.go - 引擎配置 HTTP 处理器
// ============================================================
// 这个文件处理回测引擎的子策略配置。
//
// 【什么是引擎配置？】
// 回测引擎包含多个子策略模块，可以独立开关：
// - enable_regime_router: 大盘路由（根据大盘环境选择策略）
// - enable_signal_allocator: 信号裁决（按信号质量排序）
// - enable_3d_exit: 3D 退出状态机（新的退出逻辑）
//
// 【Go 语言知识点】
// - 白名单校验: 用 map 验证输入是否合法
// - 结构体嵌套: 请求体中的嵌套 JSON
// ============================================================

import (
	"encoding/json" // JSON 编解码
	"net/http"      // HTTP 服务器
	"strings"       // 字符串处理

	"stock-backend/db" // 数据库操作
)

// EngineConfigRouter 统一路由 /api/config/engine。
// 【路由分发】
// - GET  /api/config/engine           → 获取引擎配置
// - POST /api/config/engine/toggle    → 切换子策略开关
// - POST /api/config/engine/profile   → 更新退出流派
func EngineConfigRouter(w http.ResponseWriter, r *http.Request) {
	// 去掉前缀，获取子路径
	path := strings.TrimPrefix(r.URL.Path, "/api/config/engine")
	path = strings.TrimPrefix(path, "/")

	// 处理 CORS 预检请求
	if prepareJSONWithCORS(w, r) {
		return
	}

	// 根据路径和方法分发
	switch {
	case path == "" && r.Method == http.MethodGet:
		engineConfigGetHandler(w, r) // 获取配置
	case path == "toggle" && r.Method == http.MethodPost:
		engineConfigToggleHandler(w, r) // 切换开关
	case path == "profile" && r.Method == http.MethodPost:
		engineConfigProfileHandler(w, r) // 更新退出流派
	default:
		respondBadRequest(w, "未知端点: "+path)
	}
}

// engineConfigGetHandler GET /api/config/engine — 获取引擎配置。
// 【返回示例】
// {
//   "config": {
//     "enable_regime_router": true,
//     "enable_signal_allocator": true,
//     "enable_3d_exit": true,
//     "exit_profile": "trend"
//   }
// }
func engineConfigGetHandler(w http.ResponseWriter, r *http.Request) {
	cfg, err := db.GetEngineConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"config": cfg})
}

// engineConfigToggleHandler POST /api/config/engine/toggle — 切换子策略开关。
// 【功能说明】
// 切换指定子策略的启用/禁用状态。
//
// 【请求体示例】
// {
//   "key": "enable_3d_exit",
//   "enabled": false
// }
//
// 【Go 语言知识点：白名单校验】
// 为了安全，只允许修改预定义的配置项。
// 如果用户传入任意 key，可能导致数据库被恶意修改。
func engineConfigToggleHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req struct {
		Key     string `json:"key"`     // 配置项名称
		Enabled bool   `json:"enabled"` // 是否启用
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}
	if req.Key == "" {
		respondBadRequest(w, "缺少 key 参数")
		return
	}

	// 白名单校验，防止任意 key 写入
	validKeys := map[string]bool{
		"enable_regime_router":    true, // 大盘路由
		"enable_signal_allocator": true, // 信号裁决
		"enable_3d_exit":          true, // 3D 退出状态机
	}
	if !validKeys[req.Key] {
		respondBadRequest(w, "无效的配置项: "+req.Key)
		return
	}

	// 更新数据库
	if err := db.UpdateEngineConfig(req.Key, req.Enabled); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKMsg(w, "引擎配置已更新")
}

// engineConfigProfileHandler POST /api/config/engine/profile — 更新退出流派。
// 【什么是退出流派？】
// 退出流派定义了持仓卖出的策略风格：
// - scalp: 超短线（快进快出）
// - trend: 趋势跟踪（跟随趋势）
// - swing: 波段操作（中线持有）
// - guerrilla: 游击战（灵活应变）
//
// 【请求体示例】
// {
//   "profile": "trend"
// }
func engineConfigProfileHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req struct {
		Profile string `json:"profile"` // 退出流派名称
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}

	// 验证流派是否合法
	if req.Profile != "scalp" && req.Profile != "trend" && req.Profile != "swing" && req.Profile != "guerrilla" {
		respondBadRequest(w, "无效的退出流派，仅支持 scalp / trend / swing / guerrilla")
		return
	}

	// 更新数据库
	if err := db.UpdateExitProfile(req.Profile); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKMsg(w, "退出流派已更新为: "+req.Profile)
}
