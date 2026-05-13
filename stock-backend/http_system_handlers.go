package main

import (
	"encoding/json"
	"net/http"
	"stock-backend/db"
)

type systemConfigUpdateRequest struct {
	EnableProData *bool `json:"enable_pro_data"`
}

func systemConfigHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := db.GetSystemConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, map[string]interface{}{"data": cfg})

	case http.MethodPut:
		cur, err := db.GetSystemConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}
		var req systemConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}
		if req.EnableProData != nil {
			cur.EnableProData = *req.EnableProData
		}
		if err := db.SaveSystemConfig(cur); err != nil {
			respondBadRequest(w, err.Error())
			return
		}
		latest, _ := db.GetSystemConfig()
		respondOK(w, map[string]interface{}{"msg": "系统配置已更新", "data": latest})

	default:
		respondMethodNotAllowed(w)
	}
}
