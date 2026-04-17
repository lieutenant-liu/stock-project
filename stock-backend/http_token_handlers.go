package main

import (
	"encoding/json"
	"net/http"
	"stock-backend/db"
	"stock-backend/tushare"
	"strconv"
	"strings"
)

// updateTokenHandler 兼容旧入口：快速更新 tushare token 并同步 token 池。
func updateTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	newToken := r.URL.Query().Get("token")
	if newToken != "" {
		tushare.SetToken(newToken)
		existedID, err := db.FindAPITokenID("tushare", newToken)
		if err == nil && existedID > 0 {
			_, _ = db.SetActiveAPIToken("tushare", existedID)
		} else if err == nil {
			newID, addErr := db.AddAPIToken("tushare", newToken, "legacy", 100, true, "通过 /api/set_token 同步", true)
			if addErr == nil {
				_, _ = db.SetActiveAPIToken("tushare", newID)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "Token 已更新并同步到 token 池"})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "Token 不能为空"})
	}
}

type tokenCreateRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
	Tier     string `json:"tier"`
	Priority int    `json:"priority"`
	Enabled  *bool  `json:"enabled"`
	Notes    string `json:"notes"`
	Active   bool   `json:"active"`
}

type tokenUpdateRequest struct {
	ID       int64  `json:"id"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
	Tier     string `json:"tier"`
	Notes    string `json:"notes"`
}

type tokenActivateRequest struct {
	Provider string `json:"provider"`
	ID       int64  `json:"id"`
}

func tokenCollectionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		provider := r.URL.Query().Get("provider")
		tokens, err := db.ListAPITokens(provider)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 500, "msg": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": tokens})
		return
	case http.MethodPost:
		var req tokenCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		id, err := db.AddAPIToken(req.Provider, req.Token, req.Tier, req.Priority, enabled, req.Notes, req.Active)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		if req.Active {
			token, activeErr := db.SetActiveAPIToken(req.Provider, id)
			if activeErr == nil && strings.EqualFold(strings.TrimSpace(req.Provider), "tushare") {
				tushare.SetToken(token)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "token 已添加", "id": id})
		return
	case http.MethodPut:
		var req tokenUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
			return
		}
		if req.ID <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "id 不能为空"})
			return
		}
		if err := db.UpdateAPIToken(req.ID, req.Tier, req.Priority, req.Enabled, req.Notes); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "token 已更新"})
		return
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "id 参数非法"})
			return
		}
		if err := db.DeleteAPIToken(id); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		active, activeErr := db.GetActiveAPIToken("tushare")
		if activeErr == nil && active != nil {
			tushare.SetToken(active.Token)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "token 已删除"})
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}
}

func tokenActivateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}

	var req tokenActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
		return
	}
	if req.ID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "id 不能为空"})
		return
	}

	rawToken, err := db.SetActiveAPIToken(req.Provider, req.ID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
		return
	}

	if strings.EqualFold(strings.TrimSpace(req.Provider), "tushare") || strings.TrimSpace(req.Provider) == "" {
		tushare.SetToken(rawToken)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "当前 token 已切换"})
}

func syncActiveProviderToken(provider string) {
	active, err := db.GetActiveAPIToken(provider)
	if err != nil || active == nil {
		return
	}
	if strings.EqualFold(provider, "tushare") {
		tushare.SetToken(active.Token)
	}
}
