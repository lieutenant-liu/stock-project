package api

import (
	"encoding/json"
	"net/http"
)

// prepareAPIJSON 统一 api 包响应头，避免每个 handler 重复设置。
func prepareAPIJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// prepareAPIJSONForWriteOps 为写操作补充 CORS 允许的方法和头部。
func prepareAPIJSONForWriteOps(w http.ResponseWriter) {
	prepareAPIJSON(w)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// handleAPIOptions 处理写接口预检请求。
func handleAPIOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// writeAPIResponse 统一 APIResponse 输出格式。
func writeAPIResponse(w http.ResponseWriter, code int, msg string, data interface{}) {
	_ = json.NewEncoder(w).Encode(APIResponse{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}
