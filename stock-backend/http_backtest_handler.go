package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"stock-backend/backtest"
	"sync"
)

var (
	latestExportMu   sync.Mutex
	latestExportPath string
)

const exportsDir = "exports"

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

	result, err := backtest.RunV2(cfg)
	if err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	// 异步导出 CSV，不阻塞 HTTP 响应
	go func() {
		path, err := backtest.ExportCSV(result, exportsDir)
		if err != nil {
			log.Printf("[回测] CSV导出失败: %v", err)
			return
		}
		latestExportMu.Lock()
		latestExportPath = path
		latestExportMu.Unlock()
		log.Printf("[回测] CSV已导出: %s", path)
	}()

	respondOK(w, map[string]interface{}{
		"data": result,
		"msg":  result.Summary,
	})
}

// BacktestDownloadHandler 处理 GET /api/backtest/download，提供最新回测 CSV 下载。
func BacktestDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		respondMethodNotAllowed(w)
		return
	}

	latestExportMu.Lock()
	path := latestExportPath
	latestExportMu.Unlock()

	if path == "" {
		respondBadRequest(w, "暂无回测报表，请先执行一次回测")
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		respondBadRequest(w, "报表文件已被清理，请重新执行回测")
		return
	}

	filename := filepath.Base(path)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, path)
}
