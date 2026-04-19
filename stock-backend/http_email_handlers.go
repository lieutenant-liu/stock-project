package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"stock-backend/db"
	"stock-backend/mailnotify"
)

type emailNotifyConfigUpdateRequest struct {
	Enabled       *bool  `json:"enabled"`
	AutoSendDaily *bool  `json:"auto_send_daily"`
	SMTPHost      string `json:"smtp_host"`
	SMTPPort      int    `json:"smtp_port"`
	SMTPUser      string `json:"smtp_user"`
	SMTPPass      string `json:"smtp_pass"`
	SMTPFrom      string `json:"smtp_from"`
	SubjectPrefix string `json:"subject_prefix"`
}

type emailRecipientCreateRequest struct {
	Email   string `json:"email"`
	Label   string `json:"label"`
	Enabled *bool  `json:"enabled"`
}

type emailRecipientUpdateRequest struct {
	ID      int64  `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

type strategyScanMailItem struct {
	Code          string                         `json:"code"`
	Name          string                         `json:"name"`
	Industry      string                         `json:"industry"`
	StrategyName  string                         `json:"strategy_name"`
	Signal        string                         `json:"signal"`
	LatestPrice   float64                        `json:"latest_price"`
	BuyPrice      float64                        `json:"buy_price"`
	SellPrice     float64                        `json:"sell_price"`
	StopLossPrice float64                        `json:"stop_loss_price"`
	Message       string                         `json:"message"`
	History       []strategyScanMailHistoryPoint `json:"history"`
}

type strategyScanMailHistoryPoint struct {
	TradeDate string  `json:"trade_date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Vol       float64 `json:"vol"`
}

type emailSendStrategyScanRequest struct {
	InputCode     string                 `json:"input_code"`
	StartDate     string                 `json:"start_date"`
	EndDate       string                 `json:"end_date"`
	ScanMsg       string                 `json:"scan_msg"`
	ScopeType     string                 `json:"scope_type"`
	ScopeCount    int                    `json:"scope_count"`
	ScopeDesc     string                 `json:"scope_desc"`
	Results       []strategyScanMailItem `json:"results"`
	RecipientIDs  []int64                `json:"recipient_ids"`
	AutoTriggered bool                   `json:"auto_triggered"`
}

func normalizeTradeDateCompact(raw string) string {
	clean := strings.ReplaceAll(strings.TrimSpace(raw), "-", "")
	clean = strings.ReplaceAll(clean, "/", "")
	if len(clean) != 8 {
		return ""
	}
	return clean
}

// hasPositiveVol 判断历史序列中是否已包含有效成交量数据。
func hasPositiveVol(points []mailnotify.StrategyScanHistoryPoint) bool {
	for _, p := range points {
		if p.Vol > 0 {
			return true
		}
	}
	return false
}

// enrichHistoryWithDB 用本地 K 线补齐邮件历史中的 OHLCV 缺口，确保图表可读性。
func enrichHistoryWithDB(code, startDate, endDate string, points []mailnotify.StrategyScanHistoryPoint) []mailnotify.StrategyScanHistoryPoint {
	start := sanitizeDate(startDate)
	end := sanitizeDate(endDate)
	dbHistory := db.GetKLinesFromDB(code, start, end)
	if len(dbHistory) == 0 {
		return points
	}

	type dbPoint struct {
		open  float64
		high  float64
		low   float64
		close float64
		vol   float64
	}
	byDate := make(map[string]dbPoint, len(dbHistory))
	for _, k := range dbHistory {
		byDate[k.TradeDate] = dbPoint{
			open:  k.Open,
			high:  k.High,
			low:   k.Low,
			close: k.Close,
			vol:   k.Vol,
		}
	}

	for i := range points {
		d := normalizeTradeDateCompact(points[i].TradeDate)
		if d == "" {
			continue
		}
		points[i].TradeDate = d
		if p, ok := byDate[d]; ok {
			if points[i].Open <= 0 {
				points[i].Open = p.open
			}
			if points[i].High <= 0 {
				points[i].High = p.high
			}
			if points[i].Low <= 0 {
				points[i].Low = p.low
			}
			if points[i].Close <= 0 {
				points[i].Close = p.close
			}
			if points[i].Vol <= 0 {
				points[i].Vol = p.vol
			}
		}
	}

	if hasPositiveVol(points) {
		return points
	}

	fallback := make([]mailnotify.StrategyScanHistoryPoint, 0, len(dbHistory))
	for _, k := range dbHistory {
		fallback = append(fallback, mailnotify.StrategyScanHistoryPoint{
			TradeDate: k.TradeDate,
			Open:      k.Open,
			High:      k.High,
			Low:       k.Low,
			Close:     k.Close,
			Vol:       k.Vol,
		})
	}
	if len(fallback) > 90 {
		fallback = fallback[len(fallback)-90:]
	}
	return fallback
}

func emailNotifyConfigHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := db.GetEmailNotifyConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, map[string]interface{}{"data": cfg})
	case http.MethodPut:
		cur, err := db.GetEmailNotifyConfig()
		if err != nil {
			respondInternalError(w, err)
			return
		}

		var req emailNotifyConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}

		if req.Enabled != nil {
			cur.Enabled = *req.Enabled
		}
		if req.AutoSendDaily != nil {
			cur.AutoSendDaily = *req.AutoSendDaily
		}
		if strings.TrimSpace(req.SMTPHost) != "" {
			cur.SMTPHost = strings.TrimSpace(req.SMTPHost)
		}
		if req.SMTPPort > 0 {
			cur.SMTPPort = req.SMTPPort
		}
		if strings.TrimSpace(req.SMTPUser) != "" {
			cur.SMTPUser = strings.TrimSpace(req.SMTPUser)
		}
		if strings.TrimSpace(req.SMTPPass) != "" {
			cur.SMTPPass = strings.TrimSpace(req.SMTPPass)
		}
		if strings.TrimSpace(req.SMTPFrom) != "" {
			cur.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
		}
		if strings.TrimSpace(req.SubjectPrefix) != "" {
			cur.SubjectPrefix = strings.TrimSpace(req.SubjectPrefix)
		}

		if err := db.SaveEmailNotifyConfig(cur); err != nil {
			respondBadRequest(w, err.Error())
			return
		}
		latest, _ := db.GetEmailNotifyConfig()
		respondOK(w, map[string]interface{}{"msg": "邮件配置已保存", "data": latest})
	default:
		respondMethodNotAllowed(w)
	}
}

// emailRecipientsHandler 管理策略邮件收件人列表。
func emailRecipientsHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := db.ListEmailRecipients()
		if err != nil {
			respondInternalError(w, err)
			return
		}
		respondOK(w, map[string]interface{}{"data": items})
	case http.MethodPost:
		var req emailRecipientCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		id, err := db.AddEmailRecipient(req.Email, req.Label, enabled)
		if err != nil {
			respondBadRequest(w, err.Error())
			return
		}
		respondOK(w, map[string]interface{}{"msg": "收件人已添加", "id": id})
	case http.MethodPut:
		var req emailRecipientUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondBadRequest(w, "请求体格式错误")
			return
		}
		if req.ID <= 0 {
			respondBadRequest(w, "id 不能为空")
			return
		}
		if err := db.UpdateEmailRecipient(req.ID, req.Label, req.Enabled); err != nil {
			respondBadRequest(w, err.Error())
			return
		}
		respondOKMsg(w, "收件人已更新")
	case http.MethodDelete:
		idStr := strings.TrimSpace(r.URL.Query().Get("id"))
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			respondBadRequest(w, "id 参数非法")
			return
		}
		if err := db.DeleteEmailRecipient(id); err != nil {
			respondBadRequest(w, err.Error())
			return
		}
		respondOKMsg(w, "收件人已删除")
	default:
		respondMethodNotAllowed(w)
	}
}

func emailSendStrategyScanHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	var req emailSendStrategyScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "请求体格式错误")
		return
	}

	payload := mailnotify.StrategyScanReportPayload{
		InputCode:    strings.TrimSpace(req.InputCode),
		StartDate:    strings.TrimSpace(req.StartDate),
		EndDate:      strings.TrimSpace(req.EndDate),
		ScanMsg:      strings.TrimSpace(req.ScanMsg),
		ScopeType:    strings.TrimSpace(req.ScopeType),
		ScopeCount:   req.ScopeCount,
		ScopeDesc:    strings.TrimSpace(req.ScopeDesc),
		RecipientIDs: req.RecipientIDs,
	}
	for _, item := range req.Results {
		var history []mailnotify.StrategyScanHistoryPoint
		for _, point := range item.History {
			if point.Close <= 0 {
				continue
			}
			history = append(history, mailnotify.StrategyScanHistoryPoint{
				TradeDate: strings.TrimSpace(point.TradeDate),
				Open:      point.Open,
				High:      point.High,
				Low:       point.Low,
				Close:     point.Close,
				Vol:       point.Vol,
			})
		}
		history = enrichHistoryWithDB(strings.TrimSpace(item.Code), req.StartDate, req.EndDate, history)
		payload.Results = append(payload.Results, mailnotify.StrategyScanItem{
			Code:          strings.TrimSpace(item.Code),
			Name:          strings.TrimSpace(item.Name),
			Industry:      strings.TrimSpace(item.Industry),
			StrategyName:  strings.TrimSpace(item.StrategyName),
			Signal:        strings.TrimSpace(item.Signal),
			LatestPrice:   item.LatestPrice,
			BuyPrice:      item.BuyPrice,
			SellPrice:     item.SellPrice,
			StopLossPrice: item.StopLossPrice,
			Message:       strings.TrimSpace(item.Message),
			History:       history,
		})
	}

	sendFn := mailnotify.SendStrategyScanReport
	if req.AutoTriggered {
		sendFn = mailnotify.AutoSendStrategyScanReport
	}
	if err := sendFn(payload); err != nil {
		respondBadRequest(w, err.Error())
		return
	}
	respondOKMsg(w, "邮件发送成功")
}
