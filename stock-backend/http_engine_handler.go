package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"stock-backend/db"
)

// EngineConfigRouter 统一路由 /api/config/engine。
func EngineConfigRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/config/engine")
	path = strings.TrimPrefix(path, "/")

	if prepareJSONWithCORS(w, r) {
		return
	}

	switch {
	case path == "" && r.Method == http.MethodGet:
		engineConfigGetHandler(w, r)
	case path == "toggle" && r.Method == http.MethodPost:
		engineConfigToggleHandler(w, r)
	case path == "profile" && r.Method == http.MethodPost:
		engineConfigProfileHandler(w, r)
	default:
		respondBadRequest(w, "未知端点: "+path)
	}
}

func engineConfigGetHandler(w http.ResponseWriter, r *http.Request) {
	cfg, err := db.GetEngineConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondOK(w, map[string]interface{}{"config": cfg})
}

func engineConfigToggleHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
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
		"enable_regime_router":    true,
		"enable_signal_allocator": true,
		"enable_3d_exit":          true,
	}
	if !validKeys[req.Key] {
		respondBadRequest(w, "无效的配置项: "+req.Key)
		return
	}
	if err := db.UpdateEngineConfig(req.Key, req.Enabled); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKMsg(w, "引擎配置已更新")
}

func engineConfigProfileHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}
	if req.Profile != "scalp" && req.Profile != "trend" && req.Profile != "swing" && req.Profile != "guerrilla" {
		respondBadRequest(w, "无效的退出流派，仅支持 scalp / trend / swing / guerrilla")
		return
	}
	if err := db.UpdateExitProfile(req.Profile); err != nil {
		respondInternalError(w, err)
		return
	}
	respondOKMsg(w, "退出流派已更新为: "+req.Profile)
}
