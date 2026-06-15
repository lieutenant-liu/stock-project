package main

// ============================================================
// http_system_handlers.go - 系统配置 HTTP 处理器
// ============================================================
// 这个文件处理系统级别的配置管理。
//
// 【什么是系统配置？】
// 系统配置是全局性的设置，影响整个系统的行为：
// - enable_pro_data: 是否启用高级数据功能（需要 Tushare 高积分）
//
// 【Go 语言知识点】
// - 指针类型 (*bool): 区分"未传值"和"传了 false"
// - HTTP 方法分发: GET 查询，PUT 更新
// ============================================================

import (
	"encoding/json" // JSON 编解码
	"net/http"      // HTTP 服务器
	"stock-backend/db" // 数据库操作
)

// systemConfigUpdateRequest 系统配置更新请求体。
// 【Go 语言知识点：指针类型】
// EnableProData 使用 *bool 而不是 bool，这样可以区分：
// - 未传值（nil）: 不修改该字段
// - 传了 false: 将字段设为 false
// - 传了 true: 将字段设为 true
type systemConfigUpdateRequest struct {
	EnableProData *bool `json:"enable_pro_data"` // 是否启用高级数据
}

// systemConfigHandler 处理 /api/system/config — 系统配置的查询和更新。
// 【触发方式】
// GET /api/system/config  → 查询当前配置
// PUT /api/system/config  → 更新配置
func systemConfigHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 CORS 预检请求
	if prepareJSONWithCORS(w, r) {
		return
	}

	switch r.Method {
	// ── GET: 查询当前配置 ──
	case http.MethodGet:
		cfg, err := db.GetSystemConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, map[string]interface{}{"data": cfg})

	// ── PUT: 更新配置 ──
	case http.MethodPut:
		// 先获取当前配置
		cur, err := db.GetSystemConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}

		// 解析请求体
		var req systemConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}

		// 只更新传入的字段（部分更新）
		if req.EnableProData != nil {
			cur.EnableProData = *req.EnableProData
		}

		// 保存到数据库
		if err := db.SaveSystemConfig(cur); err != nil {
			respondBadRequest(w, err.Error())
			return
		}

		// 返回更新后的配置
		latest, _ := db.GetSystemConfig()
		respondOK(w, map[string]interface{}{"msg": "系统配置已更新", "data": latest})

	default:
		respondMethodNotAllowed(w)
	}
}
