package mailnotify

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/smtp"
	"stock-backend/db"
	"strconv"
	"strings"
	"time"
)

var tinyGlyph = map[rune][]string{
	'0': {"111", "101", "101", "101", "111"},
	'1': {"010", "110", "010", "010", "111"},
	'2': {"111", "001", "111", "100", "111"},
	'3': {"111", "001", "111", "001", "111"},
	'4': {"101", "101", "111", "001", "001"},
	'5': {"111", "100", "111", "001", "111"},
	'6': {"111", "100", "111", "101", "111"},
	'7': {"111", "001", "001", "001", "001"},
	'8': {"111", "101", "111", "101", "111"},
	'9': {"111", "101", "111", "001", "111"},
	'.': {"000", "000", "000", "000", "010"},
	'K': {"101", "110", "100", "110", "101"},
	'M': {"101", "111", "111", "101", "101"},
	'B': {"110", "101", "110", "101", "110"},
	'-': {"000", "000", "111", "000", "000"},
	' ': {"000", "000", "000", "000", "000"},
}

type StrategyScanHistoryPoint struct {
	TradeDate string  `json:"trade_date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Vol       float64 `json:"vol"`
}

type StrategyScanItem struct {
	Code          string                     `json:"code"`
	Name          string                     `json:"name"`
	Industry      string                     `json:"industry"`
	StrategyName  string                     `json:"strategy_name"`
	Signal        string                     `json:"signal"`
	LatestPrice   float64                    `json:"latest_price"`
	BuyPrice      float64                    `json:"buy_price"`
	SellPrice     float64                    `json:"sell_price"`
	StopLossPrice float64                    `json:"stop_loss_price"`
	Message       string                     `json:"message"`
	History       []StrategyScanHistoryPoint `json:"history"`
}

type StrategyScanReportPayload struct {
	InputCode    string
	StartDate    string
	EndDate      string
	ScanMsg      string
	ScopeType    string
	ScopeCount   int
	ScopeDesc    string
	Results      []StrategyScanItem
	RecipientIDs []int64
}

func formatPrice(v float64) string {
	if v <= 0 {
		return "-"
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func parseTargetCodes(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func buildScopeDesc(payload StrategyScanReportPayload) string {
	if strings.TrimSpace(payload.ScopeDesc) != "" {
		return strings.TrimSpace(payload.ScopeDesc)
	}
	if strings.TrimSpace(payload.ScopeType) == "all_market" || strings.TrimSpace(payload.InputCode) == "" {
		return "全市场股票池（基于本地股票清单）"
	}
	codes := parseTargetCodes(payload.InputCode)
	if len(codes) == 0 {
		return "全市场股票池（基于本地股票清单）"
	}
	if len(codes) <= 12 {
		return fmt.Sprintf("定向股票池（%d只）: %s", len(codes), strings.Join(codes, ", "))
	}
	return fmt.Sprintf("定向股票池（%d只）: %s ...", len(codes), strings.Join(codes[:12], ", "))
}

func normalizedHistory(points []StrategyScanHistoryPoint) []StrategyScanHistoryPoint {
	out := make([]StrategyScanHistoryPoint, 0, len(points))
	for _, p := range points {
		if p.Close > 0 {
			if p.Open <= 0 {
				p.Open = p.Close
			}
			if p.High <= 0 {
				p.High = p.Close
			}
			if p.Low <= 0 {
				p.Low = p.Close
			}
			if p.High < p.Low {
				p.High, p.Low = p.Low, p.High
			}
			if p.Open > p.High {
				p.High = p.Open
			}
			if p.Open < p.Low {
				p.Low = p.Open
			}
			if p.Close > p.High {
				p.High = p.Close
			}
			if p.Close < p.Low {
				p.Low = p.Close
			}
			out = append(out, p)
		}
	}
	if len(out) > 24 {
		out = out[len(out)-24:]
	}
	return out
}

func buildSparkline(points []StrategyScanHistoryPoint) string {
	data := normalizedHistory(points)
	if len(data) == 0 {
		return "-"
	}
	levels := []rune("▁▂▃▄▅▆▇█")
	minV := data[0].Close
	maxV := data[0].Close
	for _, p := range data {
		if p.Close < minV {
			minV = p.Close
		}
		if p.Close > maxV {
			maxV = p.Close
		}
	}
	if maxV-minV < 1e-9 {
		return strings.Repeat("▅", len(data))
	}

	var b strings.Builder
	for _, p := range data {
		ratio := (p.Close - minV) / (maxV - minV)
		idx := int(math.Round(ratio * float64(len(levels)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(levels) {
			idx = len(levels) - 1
		}
		b.WriteRune(levels[idx])
	}
	return b.String()
}

func buildTrendText(points []StrategyScanHistoryPoint) string {
	data := normalizedHistory(points)
	if len(data) < 2 {
		return "趋势: 数据不足"
	}
	first := data[0]
	last := data[len(data)-1]
	changePct := (last.Close/first.Close - 1.0) * 100.0
	return fmt.Sprintf("趋势: %s -> %s | %.2f%%", formatPrice(first.Close), formatPrice(last.Close), changePct)
}

func drawVLine(img *image.RGBA, x, y1, y2 int, c color.Color) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X {
		return
	}
	if y1 < b.Min.Y {
		y1 = b.Min.Y
	}
	if y2 >= b.Max.Y {
		y2 = b.Max.Y - 1
	}
	for y := y1; y <= y2; y++ {
		img.Set(x, y, c)
	}
}

func drawTinyText(img *image.RGBA, x, y int, text string, scale int, c color.Color) {
	if scale < 1 {
		scale = 1
	}
	cursorX := x
	for _, ch := range text {
		pattern, ok := tinyGlyph[ch]
		if !ok {
			pattern = tinyGlyph[' ']
		}
		for row := 0; row < len(pattern); row++ {
			for col := 0; col < len(pattern[row]); col++ {
				if pattern[row][col] != '1' {
					continue
				}
				x1 := cursorX + col*scale
				y1 := y + row*scale
				fillRect(img, x1, y1, x1+scale-1, y1+scale-1, c)
			}
		}
		cursorX += (len(pattern[0]) + 1) * scale
	}
}

func formatVolumeAbbr(v float64) string {
	if v <= 0 {
		return "0"
	}
	if v >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", v/1_000_000_000.0)
	}
	if v >= 1_000_000 {
		return fmt.Sprintf("%.1fM", v/1_000_000.0)
	}
	if v >= 1_000 {
		return fmt.Sprintf("%.1fK", v/1_000.0)
	}
	return fmt.Sprintf("%.0f", v)
}

func calcAvgVolume(points []StrategyScanHistoryPoint) (float64, int) {
	data := normalizedHistory(points)
	if len(data) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, p := range data {
		if p.Vol > 0 {
			sum += p.Vol
		}
	}
	return sum / float64(len(data)), len(data)
}

func fillRect(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	b := img.Bounds()
	if x2 < b.Min.X || x1 >= b.Max.X || y2 < b.Min.Y || y1 >= b.Max.Y {
		return
	}
	if x1 < b.Min.X {
		x1 = b.Min.X
	}
	if y1 < b.Min.Y {
		y1 = b.Min.Y
	}
	if x2 >= b.Max.X {
		x2 = b.Max.X - 1
	}
	if y2 >= b.Max.Y {
		y2 = b.Max.Y - 1
	}
	r := image.Rect(x1, y1, x2+1, y2+1)
	draw.Draw(img, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func generateKlinePNG(points []StrategyScanHistoryPoint, width, height int) ([]byte, error) {
	data := normalizedHistory(points)
	if len(data) == 0 {
		return nil, fmt.Errorf("kline history empty")
	}
	if len(data) > 48 {
		data = data[len(data)-48:]
	}

	if width < 320 {
		width = 320
	}
	if height < 180 {
		height = 180
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := color.RGBA{R: 21, G: 27, B: 35, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	left, right, top, bottom := 20, 12, 10, 24
	plotW := width - left - right
	plotH := height - top - bottom
	if plotW <= 40 || plotH <= 40 {
		return nil, fmt.Errorf("chart size too small")
	}
	volH := int(math.Round(float64(plotH) * 0.26))
	if volH < 36 {
		volH = 36
	}
	if volH > plotH-40 {
		volH = plotH - 40
	}
	priceH := plotH - volH - 8
	if priceH < 40 {
		priceH = plotH - 36
		volH = plotH - priceH - 8
	}
	priceBottom := top + priceH
	volTop := priceBottom + 8
	volBottom := volTop + volH

	maxPrice := data[0].High
	minPrice := data[0].Low
	maxVol := data[0].Vol
	for _, p := range data {
		if p.High > maxPrice {
			maxPrice = p.High
		}
		if p.Low < minPrice {
			minPrice = p.Low
		}
		if p.Vol > maxVol {
			maxVol = p.Vol
		}
	}
	if maxPrice <= minPrice {
		maxPrice = minPrice + 1
	}
	if maxVol <= 0 {
		maxVol = 1
	}
	pad := (maxPrice - minPrice) * 0.08
	maxPrice += pad
	minPrice -= pad
	if maxPrice <= minPrice {
		maxPrice = minPrice + 1
	}

	toY := func(price float64) int {
		ratio := (maxPrice - price) / (maxPrice - minPrice)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		return top + int(math.Round(ratio*float64(priceH)))
	}

	toVolY := func(vol float64) int {
		ratio := vol / maxVol
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		return volBottom - int(math.Round(ratio*float64(volH)))
	}

	grid := color.RGBA{R: 53, G: 64, B: 79, A: 255}
	axis := color.RGBA{R: 105, G: 125, B: 149, A: 255}
	labelColor := color.RGBA{R: 173, G: 194, B: 215, A: 255}
	for i := 0; i <= 4; i++ {
		y := top + int(math.Round(float64(i)*float64(priceH)/4.0))
		fillRect(img, left, y, left+plotW, y, grid)
	}
	fillRect(img, left, priceBottom, left+plotW, priceBottom, axis)
	fillRect(img, left, volTop, left+plotW, volTop, grid)
	fillRect(img, left, volTop+volH/2, left+plotW, volTop+volH/2, grid)
	fillRect(img, left, volBottom, left+plotW, volBottom, axis)

	maxText := formatVolumeAbbr(maxVol)
	midText := formatVolumeAbbr(maxVol * 0.5)
	drawTinyText(img, 2, volTop-2, maxText, 2, labelColor)
	drawTinyText(img, 2, volTop+volH/2-5, midText, 2, labelColor)
	drawTinyText(img, 2, volBottom-10, "0", 2, labelColor)

	slot := float64(plotW) / float64(len(data))
	bodyW := int(math.Round(slot * 0.62))
	if bodyW < 3 {
		bodyW = 3
	}
	upColor := color.RGBA{R: 239, G: 35, B: 42, A: 255}
	downColor := color.RGBA{R: 20, G: 177, B: 67, A: 255}
	wickUp := color.RGBA{R: 255, G: 111, B: 97, A: 255}
	wickDown := color.RGBA{R: 80, G: 210, B: 120, A: 255}

	for i, p := range data {
		cx := left + int(math.Round((float64(i)+0.5)*slot))
		highY := toY(p.High)
		lowY := toY(p.Low)
		openY := toY(p.Open)
		closeY := toY(p.Close)
		if p.Close >= p.Open {
			drawVLine(img, cx, highY, lowY, wickUp)
			fillRect(img, cx-bodyW/2, closeY, cx+bodyW/2, openY, upColor)
			volY := toVolY(p.Vol)
			fillRect(img, cx-bodyW/2, volY, cx+bodyW/2, volBottom, wickUp)
		} else {
			drawVLine(img, cx, highY, lowY, wickDown)
			fillRect(img, cx-bodyW/2, openY, cx+bodyW/2, closeY, downColor)
			volY := toVolY(p.Vol)
			fillRect(img, cx-bodyW/2, volY, cx+bodyW/2, volBottom, wickDown)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildStrategyScanMail(payload StrategyScanReportPayload, subjectPrefix string) (string, string) {
	total := len(payload.Results)
	subject := fmt.Sprintf("%s 策略扫描结果 %s~%s (%d条)", subjectPrefix, payload.StartDate, payload.EndDate, total)

	buyCount := 0
	for _, item := range payload.Results {
		if strings.Contains(item.Signal, "买入") {
			buyCount++
		}
	}
	observeCount := total - buyCount

	const maxLines = 200
	var b strings.Builder
	b.WriteString("<html><body style=\"font-family:Arial,Helvetica,sans-serif;background:#0f1620;color:#e6edf3;\">")
	b.WriteString("<div style=\"max-width:920px;margin:0 auto;padding:16px;\">")
	b.WriteString("<h2 style=\"margin:0 0 12px 0;\">策略扫描结果报告</h2>")
	b.WriteString(fmt.Sprintf("<div><b>扫描范围:</b> %s</div>", html.EscapeString(buildScopeDesc(payload))))
	b.WriteString(fmt.Sprintf("<div><b>区间:</b> %s ~ %s</div>", html.EscapeString(payload.StartDate), html.EscapeString(payload.EndDate)))
	b.WriteString(fmt.Sprintf("<div><b>总结果数:</b> %d</div>", total))
	b.WriteString(fmt.Sprintf("<div><b>推荐买入:</b> %d</div>", buyCount))
	b.WriteString(fmt.Sprintf("<div><b>潜伏观察:</b> %d</div>", observeCount))
	if strings.TrimSpace(payload.ScanMsg) != "" {
		b.WriteString(fmt.Sprintf("<div style=\"margin-top:6px;\"><b>扫描提示:</b> %s</div>", html.EscapeString(strings.TrimSpace(payload.ScanMsg))))
	}
	if total == 0 {
		reason := strings.TrimSpace(payload.ScanMsg)
		if reason == "" {
			reason = "可能由于风控拦截、数据不足或当前区间无可触发信号。"
		}
		b.WriteString(fmt.Sprintf("<div style=\"margin-top:12px;padding:12px;background:#1d2938;border-radius:8px;\"><b>未选出股票原因:</b> %s</div>", html.EscapeString(reason)))
		b.WriteString(fmt.Sprintf("<div style=\"margin-top:12px;color:#9fb0c3;\">Generated At: %s</div>", html.EscapeString(time.Now().Format(time.RFC3339))))
		b.WriteString("</div></body></html>")
		return subject, b.String()
	}

	b.WriteString("<h3 style=\"margin:16px 0 10px 0;\">明细</h3>")
	for i, item := range payload.Results {
		if i >= maxLines {
			b.WriteString(fmt.Sprintf("<div style=\"margin:8px 0;color:#9fb0c3;\">其余 %d 条已省略，可在前端查看完整结果。</div>", total-maxLines))
			break
		}

		code := html.EscapeString(strings.TrimSpace(item.Code))
		name := html.EscapeString(strings.TrimSpace(item.Name))
		strategy := html.EscapeString(strings.TrimSpace(item.StrategyName))
		signal := html.EscapeString(strings.TrimSpace(item.Signal))
		industry := html.EscapeString(strings.TrimSpace(item.Industry))

		b.WriteString("<div style=\"margin:12px 0;padding:12px;background:#1b2633;border-radius:10px;border:1px solid #334155;\">")
		b.WriteString(fmt.Sprintf("<div style=\"font-weight:bold;\">%d) %s %s</div>", i+1, name, code))
		b.WriteString(fmt.Sprintf("<div style=\"margin-top:4px;\">策略=%s | 信号=%s | 最新=%s | 买入=%s | 止盈=%s | 止损=%s</div>",
			strategy, signal, formatPrice(item.LatestPrice), formatPrice(item.BuyPrice), formatPrice(item.SellPrice), formatPrice(item.StopLossPrice)))
		if strings.TrimSpace(item.Industry) != "" {
			b.WriteString(fmt.Sprintf("<div style=\"margin-top:3px;\">行业: %s</div>", industry))
		}
		if strings.TrimSpace(item.Message) != "" {
			msg := strings.ReplaceAll(strings.TrimSpace(item.Message), "\n", "<br/>")
			if len(msg) > 220 {
				msg = msg[:220] + "..."
			}
			b.WriteString(fmt.Sprintf("<div style=\"margin-top:6px;color:#c7d2de;\">说明: %s</div>", html.EscapeString(strings.ReplaceAll(msg, "<br/>", " "))))
		}
		avgVol, volDays := calcAvgVolume(item.History)
		if volDays > 0 {
			b.WriteString(fmt.Sprintf("<div style=\"margin-top:6px;color:#a9bdd3;\">图表: 最近%d日平均成交量 %s</div>", volDays, html.EscapeString(formatVolumeAbbr(avgVol))))
		}
		chartData, chartErr := generateKlinePNG(item.History, 680, 220)
		if chartErr == nil && len(chartData) > 0 {
			b64 := base64.StdEncoding.EncodeToString(chartData)
			b.WriteString(fmt.Sprintf("<div style=\"margin-top:8px;\"><img alt=\"kline\" style=\"width:100%%;max-width:680px;border-radius:6px;border:1px solid #3b4a5c;\" src=\"data:image/png;base64,%s\"/></div>", b64))
		} else {
			b.WriteString(fmt.Sprintf("<div style=\"margin-top:8px;color:#9fb0c3;\">K线截图生成失败，改为趋势图: %s</div>", buildSparkline(item.History)))
		}
		b.WriteString(fmt.Sprintf("<div style=\"margin-top:6px;color:#9fb0c3;\">%s</div>", html.EscapeString(buildTrendText(item.History))))
		b.WriteString("</div>")
	}
	b.WriteString(fmt.Sprintf("<div style=\"margin-top:14px;color:#9fb0c3;\">Generated At: %s</div>", html.EscapeString(time.Now().Format(time.RFC3339))))
	b.WriteString("</div></body></html>")
	return subject, b.String()
}

func sendMailSMTP(cfg db.EmailNotifyConfig, recipients []string, subject, body string) error {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return fmt.Errorf("SMTP Host 不能为空")
	}
	if cfg.SMTPPort <= 0 {
		return fmt.Errorf("SMTP Port 非法")
	}
	if strings.TrimSpace(cfg.SMTPUser) == "" {
		return fmt.Errorf("SMTP 用户名不能为空")
	}
	if strings.TrimSpace(cfg.SMTPPass) == "" {
		return fmt.Errorf("SMTP 密码不能为空")
	}
	if strings.TrimSpace(cfg.SMTPFrom) == "" {
		return fmt.Errorf("发件邮箱不能为空")
	}
	if len(recipients) == 0 {
		return fmt.Errorf("收件人为空")
	}

	host := strings.TrimSpace(cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", host, cfg.SMTPPort)
	auth := smtp.PlainAuth("", strings.TrimSpace(cfg.SMTPUser), cfg.SMTPPass, host)

	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.SMTPFrom))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(recipients, ",")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return smtp.SendMail(addr, auth, cfg.SMTPFrom, recipients, []byte(msg.String()))
}

func sendStrategyScanReportWithConfig(cfg db.EmailNotifyConfig, payload StrategyScanReportPayload) error {
	subjectPrefix := cfg.SubjectPrefix
	if strings.TrimSpace(subjectPrefix) == "" {
		subjectPrefix = "[Stock-Strategy]"
	}
	recipients, err := db.GetEnabledRecipientEmailsByIDs(payload.RecipientIDs)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return fmt.Errorf("没有可用收件人")
	}
	subject, body := buildStrategyScanMail(payload, subjectPrefix)
	return sendMailSMTP(cfg, recipients, subject, body)
}

func SendStrategyScanReport(payload StrategyScanReportPayload) error {
	cfg, err := db.GetEmailNotifyConfig()
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return fmt.Errorf("邮件推送未启用")
	}
	return sendStrategyScanReportWithConfig(cfg, payload)
}

func AutoSendStrategyScanReport(payload StrategyScanReportPayload) error {
	cfg, err := db.GetEmailNotifyConfig()
	if err != nil {
		return err
	}
	if !cfg.Enabled || !cfg.AutoSendDaily {
		return nil
	}
	return sendStrategyScanReportWithConfig(cfg, payload)
}
