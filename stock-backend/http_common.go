package main

import (
	"net/http"
	"stock-backend/db"
	"strings"
)

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func handlePreflight(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
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
