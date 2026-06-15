package main

// ============================================================
// http_token_handlers.go - Token 管理 HTTP 处理器
// ============================================================
// 这个文件处理 API Token 的增删改查操作。
//
// 【什么是 Token？】
// Token 是访问 Tushare API 的凭证，类似于密码。
// 项目支持"Token 池"功能：可以存储多个 Token，按优先级轮换使用。
// 这样当一个 Token 达到调用限制时，可以自动切换到下一个。
//
// 【Go 语言知识点】
// - 结构体标签 (struct tag): `json:"provider"` 指定 JSON 序列化时的字段名
// - 指针类型 (*bool): 区分"未传值"和"传了 false"
// - json.NewDecoder: 从请求 Body 解析 JSON
// ============================================================

import (
	"encoding/json" // JSON 编解码
	"net/http"      // HTTP 服务器
	"stock-backend/db"       // 数据库操作
	"stock-backend/tushare"  // Tushare API 客户端
	"strconv"       // 字符串转数字
	"strings"       // 字符串处理
)

// updateTokenHandler 兼容旧入口：快速更新 tushare token 并同步 token 池。
// 【功能说明】
// 这是一个旧接口，用于快速设置 Tushare Token。
// 新接口使用 tokenCollectionHandler（支持完整的 CRUD 操作）。
//
// 【触发方式】
// GET /api/set_token?token=your_token_here
func updateTokenHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	// 从 URL 参数获取 token
	newToken := r.URL.Query().Get("token")
	if strings.TrimSpace(newToken) == "" {
		respondBadRequest(w, "Token 不能为空")
		return
	}

	// 更新运行时的 Tushare Token
	tushare.SetToken(newToken)

	// 检查 Token 是否已存在于数据库
	existedID, err := db.FindAPITokenID("tushare", newToken)
	if err == nil && existedID > 0 {
		// 已存在，直接激活
		_, _ = db.SetActiveAPIToken("tushare", existedID)
	} else if err == nil {
		// 不存在，添加到 Token 池并激活
		newID, addErr := db.AddAPIToken("tushare", newToken, "legacy", 100, true, "通过 /api/set_token 同步", true)
		if addErr == nil {
			_, _ = db.SetActiveAPIToken("tushare", newID)
		}
	}
	respondOKMsg(w, "Token 已更新并同步到 token 池")
}

// ------------------------------------------------------------
// 请求体结构定义
// ------------------------------------------------------------

// tokenCreateRequest 创建 Token 的请求体。
// 【Go 语言知识点：结构体标签】
// `json:"provider"` 表示 JSON 序列化时使用 "provider" 作为字段名。
// 如果没有标签，默认使用字段名的小写形式。
type tokenCreateRequest struct {
	Provider string `json:"provider"` // 数据源提供商（如 "tushare"）
	Token    string `json:"token"`    // Token 值
	Tier     string `json:"tier"`     // 层级（如 "legacy", "premium"）
	Priority int    `json:"priority"` // 优先级（数字越小越优先）
	Enabled  *bool  `json:"enabled"`  // 是否启用（指针类型，区分未传值和 false）
	Notes    string `json:"notes"`    // 备注
	Active   bool   `json:"active"`   // 是否立即激活
}

// tokenUpdateRequest 更新 Token 的请求体。
type tokenUpdateRequest struct {
	ID       int64  `json:"id"`       // Token ID
	Priority int    `json:"priority"` // 新优先级
	Enabled  bool   `json:"enabled"`  // 是否启用
	Tier     string `json:"tier"`     // 新层级
	Notes    string `json:"notes"`    // 新备注
}

// tokenActivateRequest 激活 Token 的请求体。
type tokenActivateRequest struct {
	Provider string `json:"provider"` // 数据源提供商
	ID       int64  `json:"id"`       // 要激活的 Token ID
}

// ------------------------------------------------------------
// Token 集合处理器（统一入口）
// ------------------------------------------------------------

// tokenCollectionHandler 提供 token 列表/新增/更新/删除的统一入口。
// 【功能说明】
// 这是一个 RESTful 风格的处理器，根据 HTTP 方法执行不同操作：
// - GET: 获取 Token 列表
// - POST: 创建新 Token
// - PUT: 更新 Token
// - DELETE: 删除 Token
//
// 【触发方式】
// GET    /api/tokens?provider=tushare        → 获取列表
// POST   /api/tokens                          → 创建 Token
// PUT    /api/tokens                          → 更新 Token
// DELETE /api/tokens?id=1                     → 删除 Token
func tokenCollectionHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 CORS 预检请求
	if prepareJSONWithCORS(w, r) {
		return // 是预检请求，已处理完毕
	}

	// 根据 HTTP 方法分发处理
	switch r.Method {

	// ── GET: 获取 Token 列表 ──
	case http.MethodGet:
		provider := r.URL.Query().Get("provider") // 可选：按数据源过滤
		tokens, err := db.ListAPITokens(provider)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, map[string]interface{}{"data": tokens})
		return

	// ── POST: 创建新 Token ──
	case http.MethodPost:
		// 解析请求体 JSON
		var req tokenCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}

		// 处理 enabled 字段（指针类型，区分未传值和 false）
		enabled := true // 默认启用
		if req.Enabled != nil {
			enabled = *req.Enabled // 使用传入的值
		}

		// 添加到数据库
		id, err := db.AddAPIToken(req.Provider, req.Token, req.Tier, req.Priority, enabled, req.Notes, req.Active)
		if err != nil {
			respondBadRequest(w, err.Error())
			return
		}

		// 如果需要立即激活
		if req.Active {
			token, activeErr := db.SetActiveAPIToken(req.Provider, id)
			// 如果是 Tushare Token，同步更新运行时
			if activeErr == nil && strings.EqualFold(strings.TrimSpace(req.Provider), "tushare") {
				tushare.SetToken(token)
			}
		}
		respondOK(w, map[string]interface{}{"msg": "token 已添加", "id": id})
		return

	// ── PUT: 更新 Token ──
	case http.MethodPut:
		var req tokenUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}
		if req.ID <= 0 {
			respondBadRequest(w, "id 不能为空")
			return
		}
		if err := db.UpdateAPIToken(req.ID, req.Tier, req.Priority, req.Enabled, req.Notes); err != nil {
			respondBadRequest(w, err.Error())
			return
		}
		respondOKMsg(w, "token 已更新")
		return

	// ── DELETE: 删除 Token ──
	case http.MethodDelete:
		// 从 URL 参数获取 ID
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64) // 字符串转 int64
		if err != nil || id <= 0 {
			respondBadRequest(w, "id 参数非法")
			return
		}
		if err := db.DeleteAPIToken(id); err != nil {
			respondBadRequest(w, err.Error())
			return
		}

		// 删除后，如果有其他激活的 Tushare Token，自动切换
		active, activeErr := db.GetActiveAPIToken("tushare")
		if activeErr == nil && active != nil {
			tushare.SetToken(active.Token)
		}
		respondOKMsg(w, "token 已删除")
		return

	default:
		respondMethodNotAllowed(w)
		return
	}
}

// ------------------------------------------------------------
// Token 激活处理器
// ------------------------------------------------------------

// tokenActivateHandler 切换指定 provider 的生效 token。
// 【功能说明】
// 当有多个 Token 时，通过此接口切换当前使用的 Token。
//
// 【触发方式】
// POST /api/tokens/activate
// Body: {"provider": "tushare", "id": 1}
func tokenActivateHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 CORS 预检请求
	if prepareJSONWithCORS(w, r) {
		return
	}

	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	// 解析请求体
	var req tokenActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "请求体格式错误")
		return
	}
	if req.ID <= 0 {
		respondBadRequest(w, "id 不能为空")
		return
	}

	// 在数据库中设置激活状态
	rawToken, err := db.SetActiveAPIToken(req.Provider, req.ID)
	if err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	// 如果是 Tushare Token，同步更新运行时
	if strings.EqualFold(strings.TrimSpace(req.Provider), "tushare") || strings.TrimSpace(req.Provider) == "" {
		tushare.SetToken(rawToken)
	}

	respondOKMsg(w, "当前 token 已切换")
}

// ------------------------------------------------------------
// 启动时同步 Token
// ------------------------------------------------------------

// syncActiveProviderToken 在服务启动时把数据库中"当前生效 token"装填到运行时。
// 【为什么需要这个？】
// 服务重启后，内存中的 Token 会丢失。
// 需要从数据库读取上次激活的 Token，恢复到运行时。
func syncActiveProviderToken(provider string) {
	// 从数据库获取当前激活的 Token
	active, err := db.GetActiveAPIToken(provider)
	if err != nil || active == nil {
		return // 没有激活的 Token，跳过
	}

	// 如果是 Tushare，更新运行时 Token
	if strings.EqualFold(provider, "tushare") {
		tushare.SetToken(active.Token)
	}
}
