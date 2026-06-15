package main

// ============================================================
// http_common.go - HTTP 通用工具函数
// ============================================================
// 这个文件定义了所有 HTTP 处理器共用的工具函数。
// 包括：CORS 跨域处理、JSON 响应封装、参数清洗等。
//
// 【Go 语言知识点】
// - http.ResponseWriter: 用于写入 HTTP 响应（状态码、头部、Body）
// - *http.Request: 代表客户端发来的请求（方法、参数、Body）
// - w.Header().Set(): 设置响应头
// - json.NewEncoder(w).Encode(): 将 Go 数据结构编码为 JSON 写入响应
// ============================================================

import (
	"encoding/json" // Go 标准库：JSON 编解码
	"net/http"      // Go 标准库：HTTP 服务器和客户端
	"stock-backend/db" // 项目内部包：数据库操作
	"strings"       // Go 标准库：字符串处理
)

// ------------------------------------------------------------
// CORS 跨域处理
// ------------------------------------------------------------
// 【什么是 CORS？】
// 浏览器有同源策略限制：前端运行在 localhost:5173，后端在 localhost:8081
// 虽然都是 localhost，但端口不同，浏览器会阻止跨域请求。
// 解决方案：后端在响应头中声明"我允许跨域访问"。
//
// Access-Control-Allow-Origin: * 表示允许任何来源访问
// Access-Control-Allow-Methods: 允许的 HTTP 方法
// Access-Control-Allow-Headers: 允许的请求头
// ------------------------------------------------------------

// setCORSHeaders 为跨域请求写入统一响应头。
// 参数 w: HTTP 响应写入器，用于设置响应头
func setCORSHeaders(w http.ResponseWriter) {
	// 允许任何来源访问（生产环境应限制为具体域名）
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// 允许的 HTTP 方法
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	// 允许的请求头
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// handlePreflight 处理 OPTIONS 预检请求；返回 true 表示请求已结束。
// 【什么是预检请求？】
// 浏览器在发送跨域 POST/PUT/DELETE 请求前，会先发一个 OPTIONS 请求询问服务器：
// "我能不能用这个方法？" 服务器回答"可以"后，浏览器才会发送真正的请求。
func handlePreflight(w http.ResponseWriter, r *http.Request) bool {
	// 检查是否是 OPTIONS 预检请求
	if r.Method == http.MethodOptions {
		// 返回 204 No Content（成功但无内容）
		w.WriteHeader(http.StatusNoContent)
		return true // 告诉调用方：请求已处理完毕，不需要继续
	}
	return false // 不是预检请求，继续正常处理
}

// ------------------------------------------------------------
// JSON 响应封装
// ------------------------------------------------------------
// 统一的响应格式，方便前端解析。
// 所有接口都返回 JSON，格式为：{"code": 200, "data": ..., "msg": "..."}
// ------------------------------------------------------------

// prepareJSON 设置统一 JSON 响应类型。
// 告诉浏览器："我返回的是 JSON 格式，请按 JSON 解析"
func prepareJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

// preparePublicJSON 用于无需复杂 CORS 的公共只读接口。
// 这些接口只允许读取数据，不允许修改，所以 CORS 策略可以宽松一些。
func preparePublicJSON(w http.ResponseWriter) {
	prepareJSON(w) // 先设置 Content-Type
	w.Header().Set("Access-Control-Allow-Origin", "*") // 允许跨域
}

// prepareJSONWithCORS 用于需要 CORS + 预检处理的写操作接口。
// 写操作（POST/PUT/DELETE）需要更严格的 CORS 处理。
// 返回 true 表示是预检请求，已处理完毕，调用方应直接返回。
func prepareJSONWithCORS(w http.ResponseWriter, r *http.Request) bool {
	prepareJSON(w)      // 设置 Content-Type
	setCORSHeaders(w)   // 设置 CORS 头
	return handlePreflight(w, r) // 处理预检请求
}

// ------------------------------------------------------------
// 标准响应函数
// ------------------------------------------------------------
// 提供统一的响应格式，避免每个处理器重复写相同的代码。
// ------------------------------------------------------------

// respondJSON 统一输出标准 JSON 响应。
// 参数:
//   - w: 响应写入器
//   - status: HTTP 状态码（200=成功, 400=参数错误, 500=服务器错误）
//   - payload: 要返回的数据（会被编码为 JSON）
func respondJSON(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.WriteHeader(status) // 设置 HTTP 状态码
	_ = json.NewEncoder(w).Encode(payload) // 将 payload 编码为 JSON 并写入响应
	// 这里的 _ 表示忽略错误（写入响应时的错误通常无法处理）
}

// respondOK 输出 200 成功响应，并补齐默认 code 字段。
// 参数:
//   - w: 响应写入器
//   - payload: 要返回的数据，如果为 nil 则返回空对象
func respondOK(w http.ResponseWriter, payload map[string]interface{}) {
	// 如果 payload 为 nil，初始化为空对象
	if payload == nil {
		payload = map[string]interface{}{}
	}
	// 如果 payload 中没有 code 字段，自动添加 code=200
	if _, ok := payload["code"]; !ok {
		payload["code"] = 200
	}
	respondJSON(w, http.StatusOK, payload) // 输出 200 响应
}

// respondOKMsg 快速输出仅含消息的成功响应。
// 用于简单的成功提示，如 "同步已启动"
func respondOKMsg(w http.ResponseWriter, msg string) {
	respondOK(w, map[string]interface{}{"msg": msg})
}

// respondBadRequest 输出 400 参数错误。
// 当前端传来的参数不合法时使用。
func respondBadRequest(w http.ResponseWriter, msg string) {
	respondJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 400, "msg": msg})
}

// respondInternalError 输出 500 服务器错误。
// 当后端发生意外错误时使用。
func respondInternalError(w http.ResponseWriter, err error) {
	msg := "internal error" // 默认错误消息
	if err != nil {
		msg = err.Error() // 如果有具体错误，使用错误消息
	}
	respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 500, "msg": msg})
}

// respondMethodNotAllowed 输出 405 方法不支持。
// 当使用了不支持的 HTTP 方法时使用（如用 GET 请求一个只支持 POST 的接口）。
func respondMethodNotAllowed(w http.ResponseWriter) {
	respondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
}

// respondConflict 输出 409 资源冲突。
// 当请求与当前状态冲突时使用（如重复创建已存在的资源）。
func respondConflict(w http.ResponseWriter, msg string) {
	respondJSON(w, http.StatusConflict, map[string]interface{}{"code": 409, "msg": msg})
}

// ------------------------------------------------------------
// 参数清洗工具
// ------------------------------------------------------------

// sanitizeDate 统一清洗日期参数（YYYYMMDD），不合法返回空串。
// 【为什么需要清洗？】
// 前端可能传入各种格式的日期："2024-01-01", "2024/01/01", "20240101"
// 后端需要统一转换为 "20240101" 格式才能使用。
func sanitizeDate(dateStr string) string {
	// 去掉横杠和斜杠
	clean := strings.ReplaceAll(dateStr, "-", "") // "2024-01-01" → "20240101"
	clean = strings.ReplaceAll(clean, "/", "")     // "2024/01/01" → "20240101"
	clean = strings.TrimSpace(clean)               // 去掉首尾空格

	// 检查长度是否为 8（YYYYMMDD）
	if len(clean) != 8 {
		return "" // 长度不对，返回空串表示无效
	}
	return clean
}

// parseTargetCodes 解析前端传来的指定股票代码，为空则回退全市场。
// 【参数格式】
// 前端可以传入逗号分隔的股票代码，如 "000001,600519,000858"
// 如果不传，则返回全市场所有股票代码。
//
// 【自动补全后缀】
// 用户可能只输入 "000001"，程序需要自动判断：
// - 6 开头 → 上海 ".SH"
// - 0 或 3 开头 → 深圳 ".SZ"
// - 4 或 8 开头 → 北交所 ".BJ"
func parseTargetCodes(rawCodes string) []string {
	var codesToSync []string

	if rawCodes != "" {
		// 按逗号分割
		parts := strings.Split(rawCodes, ",")
		for _, p := range parts {
			cleanCode := strings.TrimSpace(p) // 去掉空格
			if cleanCode == "" {
				continue // 跳过空串
			}

			// 如果没有后缀，自动补全
			if !strings.Contains(cleanCode, ".") {
				if strings.HasPrefix(cleanCode, "6") {
					cleanCode += ".SH" // 6 开头是上海交易所
				} else if strings.HasPrefix(cleanCode, "0") || strings.HasPrefix(cleanCode, "3") {
					cleanCode += ".SZ" // 0 或 3 开头是深圳交易所
				} else if strings.HasPrefix(cleanCode, "4") || strings.HasPrefix(cleanCode, "8") {
					cleanCode += ".BJ" // 4 或 8 开头是北交所
				}
			}
			codesToSync = append(codesToSync, cleanCode)
		}
	} else {
		// 没有指定代码，从数据库获取全市场股票代码
		codesToSync = db.GetAllStockCodes()
	}
	return codesToSync
}
