package main

import (
	"encoding/json"
	"net/http"
	"stock-backend/db"
	"strings"
)

// setCORSHeaders 为跨域请求写入统一响应头。
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// handlePreflight 处理 OPTIONS 预检；返回 true 表示请求已结束。
func handlePreflight(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// prepareJSON 设置统一 JSON 响应类型。
func prepareJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

// preparePublicJSON 用于无需复杂 CORS 的公共只读接口。
func preparePublicJSON(w http.ResponseWriter) {
	prepareJSON(w)
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// prepareJSONWithCORS 用于需要 CORS + 预检处理的写操作接口。
func prepareJSONWithCORS(w http.ResponseWriter, r *http.Request) bool {
	prepareJSON(w)
	setCORSHeaders(w)
	return handlePreflight(w, r)
}

// respondJSON 统一输出标准 JSON 响应。
func respondJSON(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// respondOK 输出 200 成功响应，并补齐默认 code 字段。
func respondOK(w http.ResponseWriter, payload map[string]interface{}) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if _, ok := payload["code"]; !ok {
		payload["code"] = 200
	}
	respondJSON(w, http.StatusOK, payload)
}

// respondOKMsg 快速输出仅含消息的成功响应。
func respondOKMsg(w http.ResponseWriter, msg string) {
	respondOK(w, map[string]interface{}{"msg": msg})
}

// respondBadRequest 输出 400 参数错误。
func respondBadRequest(w http.ResponseWriter, msg string) {
	respondJSON(w, http.StatusBadRequest, map[string]interface{}{"code": 400, "msg": msg})
}

// respondInternalError 输出 500 服务器错误。
func respondInternalError(w http.ResponseWriter, err error) {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"code": 500, "msg": msg})
}

// respondMethodNotAllowed 输出 405 方法不支持。
func respondMethodNotAllowed(w http.ResponseWriter) {
	respondJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
}

// respondConflict 输出 409 资源冲突。
func respondConflict(w http.ResponseWriter, msg string) {
	respondJSON(w, http.StatusConflict, map[string]interface{}{"code": 409, "msg": msg})
}

// sanitizeDate 统一清洗日期参数（YYYYMMDD），不合法返回空串。
func sanitizeDate(dateStr string) string {
	clean := strings.ReplaceAll(dateStr, "-", "")
	clean = strings.ReplaceAll(clean, "/", "")
	clean = strings.TrimSpace(clean)
	if len(clean) != 8 {
		return ""
	}
	return clean
}

// parseTargetCodes 解析前端传来的指定股票代码，为空则回退全市场。
func parseTargetCodes(rawCodes string) []string {
	var codesToSync []string
	if rawCodes != "" {
		parts := strings.Split(rawCodes, ",")
		for _, p := range parts {
			cleanCode := strings.TrimSpace(p)
			if cleanCode == "" {
				continue
			}
			if !strings.Contains(cleanCode, ".") {
				if strings.HasPrefix(cleanCode, "6") {
					cleanCode += ".SH"
				} else if strings.HasPrefix(cleanCode, "0") || strings.HasPrefix(cleanCode, "3") {
					cleanCode += ".SZ"
				} else if strings.HasPrefix(cleanCode, "4") || strings.HasPrefix(cleanCode, "8") {
					cleanCode += ".BJ"
				}
			}
			codesToSync = append(codesToSync, cleanCode)
		}
	} else {
		codesToSync = db.GetAllStockCodes()
	}
	return codesToSync
}
