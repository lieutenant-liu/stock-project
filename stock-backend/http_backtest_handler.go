package main

import (
	"encoding/json"
	"net/http"
	"stock-backend/backtest"
)

// BacktestHandler 处理 POST /api/backtest。
func BacktestHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	var cfg backtest.BacktestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondBadRequest(w, "参数解析失败: "+err.Error())
		return
	}

	cfg.StartDate = sanitizeDate(cfg.StartDate)
	cfg.EndDate = sanitizeDate(cfg.EndDate)
	if cfg.StartDate == "" || cfg.EndDate == "" {
		respondBadRequest(w, "起止日期不能为空，格式: YYYYMMDD 或 YYYY-MM-DD")
		return
	}

	result, err := backtest.Run(cfg)
	if err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	respondOK(w, map[string]interface{}{
		"data": result,
		"msg":  result.Summary,
	})
}
