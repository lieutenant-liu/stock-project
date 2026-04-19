package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stock-backend/db"
	"stock-backend/strategy"
	"stock-backend/tushare"
)

type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// normalizeTSCode 支持前端输入纯数字代码并自动补齐交易所后缀。
func normalizeTSCode(rawCode string) string {
	cleanCode := strings.TrimSpace(rawCode)
	if cleanCode == "" {
		return ""
	}
	if strings.Contains(cleanCode, ".") {
		return cleanCode
	}
	if strings.HasPrefix(cleanCode, "6") {
		return cleanCode + ".SH"
	}
	if strings.HasPrefix(cleanCode, "0") || strings.HasPrefix(cleanCode, "3") {
		return cleanCode + ".SZ"
	}
	if strings.HasPrefix(cleanCode, "4") || strings.HasPrefix(cleanCode, "8") {
		return cleanCode + ".BJ"
	}
	return cleanCode
}

// resolveStockName 优先使用前端传值，缺失时回查 stock_basic 表做兜底。
func resolveStockName(tsCode, rawName string) string {
	name := strings.TrimSpace(rawName)
	if name != "" && name != "未知" {
		return name
	}

	var dbName string
	err := db.DB.QueryRow(`SELECT COALESCE(name, '') FROM stock_basic WHERE ts_code = ?`, tsCode).Scan(&dbName)
	if err == nil && strings.TrimSpace(dbName) != "" {
		return strings.TrimSpace(dbName)
	}

	return "未知"
}

// DiagnoseHandler 策略扫描核心 API (V3.0 多态调度架构)
func DiagnoseHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)

	rawCodes := r.URL.Query().Get("code")
	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	if endDate == "" {
		endDate = time.Now().Format("20060102")
	}
	if startDate == "" {
		startDate = time.Now().AddDate(-1, 0, 0).Format("20060102")
	}

	var codeList []string
	isAllMarket := false

	// 解析是“定向诊断”还是“全市场扫描”
	if strings.TrimSpace(rawCodes) == "" {
		codeList = db.GetAllStockCodes()
		isAllMarket = true
		fmt.Printf("🎯 [扫描中心] 收到全市场扫描指令，目标池: %d 只股票\n", len(codeList))
	} else {
		parts := strings.Split(rawCodes, ",")
		for _, p := range parts {
			cleanCode := normalizeTSCode(p)
			if cleanCode == "" {
				continue
			}
			codeList = append(codeList, cleanCode)
		}
	}

	var results []map[string]interface{}

	// 核心调度：唤醒已启用的策略分析器
	activeAnalyzers := strategy.GetActiveAnalyzers()
	// =========================================================
	// 🛡️ [机构级风控：大盘 Beta 与 情绪冰点 全局熔断前置拦截]
	// =========================================================
	shIndexData := db.GetIndexDailyFromDB("000001.SH", startDate, endDate)

	// 提取昨日和今日交易日期，计算涨停溢价率
	limitUpCount := 0
	avgPremium := 0.0
	if len(shIndexData) >= 2 {
		todayDate := shIndexData[len(shIndexData)-1].TradeDate
		yesterdayDate := shIndexData[len(shIndexData)-2].TradeDate
		limitUpCount, avgPremium = db.GetLimitUpPremium(yesterdayDate, todayDate)
	}

	// 将溢价率数据送入风控模块
	isMarketSafe, marketMsg := strategy.CheckMarketEnvironment(shIndexData, limitUpCount, avgPremium)

	fmt.Printf("🌐 [全局风控] %s\n", marketMsg)

	// 如果大盘处于风险区间，且用户正在执行“全市场扫描”，则直接拦截买入信号输出
	if !isMarketSafe && isAllMarket {
		writeAPIResponse(w, 200, marketMsg+"（系统已自动拦截今日全市场买入信号，建议等待环境改善）", []map[string]interface{}{})
		return
	}
	// =========================================================
	for _, code := range codeList {
		// 1. 过滤垃圾股，同时提取【所属行业】
		var stockName, industry string

		// 使用 SQL 的 COALESCE 自动处理 NULL 值，Go 端只接收 string
		err := db.DB.QueryRow(`SELECT COALESCE(name, '未知'), COALESCE(industry, '未知板块') FROM stock_basic WHERE ts_code = ?`, code).Scan(&stockName, &industry)
		if err != nil {
			stockName = "未知"
			industry = "未知板块"
		}

		if strings.Contains(stockName, "ST") {
			continue // 坚决不碰 ST
		}

		// 2. 提取底层数据：K 线量价数据
		historyData := db.GetKLinesFromDB(code, startDate, endDate)
		if len(historyData) == 0 {
			continue
		}

		// =========================================================
		// 数据治理：执行前复权清洗，减少 K 线断层影响
		// =========================================================
		adjFactors := db.GetAdjFactorsFromDB(code, startDate, endDate)
		if len(adjFactors) > 0 {
			historyData = strategy.ForwardAdjustKLines(historyData, adjFactors)
		}
		// =========================================================

		// 3. 动态数据注入机制
		var fundData []tushare.DailyFundamental
		var flowData []tushare.DailyMoneyFlow

		needFund, needFlow := false, false
		for _, analyzer := range activeAnalyzers {
			for _, req := range analyzer.RequiredData() {
				if req == "fundamentals" {
					needFund = true
				}
				if req == "moneyflow" {
					needFlow = true
				}
			}
		}

		if needFund {
			fundData = db.GetFundamentalsFromDB(code, startDate, endDate)
		}
		if needFlow {
			flowData = db.GetMoneyFlowFromDB(code, startDate, endDate)
		}

		pePercentile := db.GetPEPercentile(code, endDate, 750)

		// 直接使用纯净的 stockName
		ctx := &strategy.SecurityContext{
			Code:         code,
			StockName:    stockName,
			KLines:       historyData,
			Fundamentals: fundData,
			MoneyFlows:   flowData,
			PEPercentile: pePercentile,
		}

		// 4. 多策略并发/循环分析
		for _, analyzer := range activeAnalyzers {
			result := analyzer.Analyze(ctx)

			if result.Signal != "观望 💤" && result.Signal != "" {
				chartData := historyData
				if len(historyData) > 90 {
					chartData = historyData[len(historyData)-90:]
				}

				// 直接放入纯净的 stockName 和 industry
				frontItem := map[string]interface{}{
					"code":            result.Code,
					"name":            stockName,
					"industry":        industry,
					"strategy_name":   result.StrategyName,
					"signal":          result.Signal,
					"latest_price":    result.LatestPrice,
					"buy_price":       result.BuyPrice,
					"sell_price":      result.SellPrice,
					"stop_loss_price": result.StopLossPrice,
					"history":         chartData,
					"message":         result.Message,
				}

				if isAllMarket {
					if strings.Contains(result.Signal, "买入") || strings.Contains(result.Signal, "潜伏") {
						results = append(results, frontItem)
					}
				} else {
					results = append(results, frontItem)
				}
			}
		}
	}

	// =========================================================
	// [全局扫描] 行业共振统计与分层标签
	// =========================================================
	if len(results) > 0 {
		// 1. 统计今日各行业触发买点的股票总数
		industryCount := make(map[string]int)
		for _, item := range results {
			ind := item["industry"].(string)
			industryCount[ind]++
		}

		// 2. 二次遍历，打上不同权重的标签
		for i := range results {
			ind := results[i]["industry"].(string)
			count := industryCount[ind]

			var tagName string
			// 按行业聚集度分为三个梯队
			if count >= 3 {
				tagName = "[🌟 板块共振·高强度]" // 极高权重：产业级逻辑爆发
			} else if count == 2 {
				tagName = "[🔥 行业异动·中强度]" // 中高权重：资金关注提升
			} else {
				tagName = "[🐺 独立行情·低强度]" // 低权重：个股独立逻辑
			}

			// 将标签强行注入策略名称，前端会直接高亮显示
			origStrategy := results[i]["strategy_name"].(string)
			results[i]["strategy_name"] = fmt.Sprintf("%s %s", tagName, origStrategy)

			// 如果是板块共振，在结果说明中增加群聚提示
			if count > 1 {
				origMsg := results[i]["message"].(string)
				results[i]["message"] = fmt.Sprintf("【同板块今日共有 %d 只股票同时触发买点】\n%s", count, origMsg)
			}
		}
	}
	// =========================================================

	if isAllMarket {
		fmt.Printf("🎉 [扫描中心] 扫描完毕，策略引擎共发现 %d 个买点\n", len(results))
	}

	writeAPIResponse(w, 200, fmt.Sprintf("扫描完毕，共发现 %d 个符合策略的买点", len(results)), results)
}

// AuditHandler 数据体检接口 (支持免后缀输入)
func AuditHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)

	code := r.URL.Query().Get("code")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	code = normalizeTSCode(code)
	if code == "" {
		writeAPIResponse(w, 400, "请提供股票代码", nil)
		return
	}

	report := db.RunDataAudit(code, start, end)
	writeAPIResponse(w, 200, "体检完成", report)
}

// AddPositionHandler 供前端表单调用：手动录入持仓
func AddPositionHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSONForWriteOps(w)
	if handleAPIOptions(w, r) {
		return
	}

	var pos db.Position
	if err := json.NewDecoder(r.Body).Decode(&pos); err != nil {
		writeAPIResponse(w, 400, "参数解析失败", nil)
		return
	}

	pos.TSCode = normalizeTSCode(pos.TSCode)
	pos.StockName = resolveStockName(pos.TSCode, pos.StockName)

	// 防呆拦截
	if pos.CostPrice <= 0 || pos.HoldVolume <= 0 {
		writeAPIResponse(w, 400, "成本价和持仓量必须大于 0", nil)
		return
	}

	if err := db.AddPosition(pos); err != nil {
		writeAPIResponse(w, 500, "录入失败: "+err.Error(), nil)
		return
	}
	writeAPIResponse(w, 200, "持仓已添加", nil)
}

// DeletePositionHandler 供前端调用：手动移出持仓
func DeletePositionHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSONForWriteOps(w)
	if handleAPIOptions(w, r) {
		return
	}

	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIResponse(w, 400, "参数解析失败", nil)
		return
	}
	if err := db.DeletePosition(req.ID); err != nil {
		writeAPIResponse(w, 500, "移除失败: "+err.Error(), nil)
		return
	}
	writeAPIResponse(w, 200, "仓位已清除", nil)
}

// GetPositionsHandler 供前端调用：展示当前持仓列表
func GetPositionsHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)
	positions, _ := db.GetAllPositions()
	writeAPIResponse(w, 200, "success", positions)
}

// ==========================================
// 持仓风险评估核心接口 (PositionRiskHandler)
// ==========================================
func PositionRiskHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)

	positions, err := db.GetAllPositions()
	if err != nil || len(positions) == 0 {
		writeAPIResponse(w, 200, "当前无持仓，风险评估为空。", []map[string]interface{}{})
		return
	}

	endDate := time.Now().Format("20060102")
	latestMarketDate := db.GetLatestOpenTradeDate(endDate)
	var reports []map[string]interface{}
	missingCount := 0
	staleCount := 0

	for _, pos := range positions {
		displayName := resolveStockName(pos.TSCode, pos.StockName)

		// 1. 动态获取建仓日以来的所有 K 线 (这是计算水位的关键)
		historyData := db.GetKLinesFromDB(pos.TSCode, pos.BuyDate, endDate)
		if len(historyData) == 0 {
			missingCount++
			reports = append(reports, map[string]interface{}{
				"id":                pos.ID,
				"ts_code":           pos.TSCode,
				"name":              displayName,
				"hold_volume":       pos.HoldVolume,
				"buy_date":          pos.BuyDate,
				"cost_price":        pos.CostPrice,
				"current_price":     nil,
				"high_watermark":    nil,
				"profit_pct":        nil,
				"retracement":       nil,
				"action":            "⚪ 数据待补齐",
				"status":            "missing_data",
				"data_trade_date":   "",
				"latest_trade_date": latestMarketDate,
				"is_stale":          true,
				"reason":            fmt.Sprintf("未找到 %s 从建仓日(%s)到当前的有效日线数据。请先同步近期K线后再执行风险评估。", pos.TSCode, pos.BuyDate),
			})
			continue
		}

		// 2. 前复权清洗 (消除除权断层)
		adjFactors := db.GetAdjFactorsFromDB(pos.TSCode, pos.BuyDate, endDate)
		if len(adjFactors) > 0 {
			historyData = strategy.ForwardAdjustKLines(historyData, adjFactors)
		}

		today := historyData[len(historyData)-1]
		currentPrice := today.Close

		// 💥 3. 动态推演最高水位 (极简算法：建仓以来的最高收盘价)
		highWatermark := pos.CostPrice
		for _, k := range historyData {
			if k.Close > highWatermark {
				highWatermark = k.Close
			}
		}

		// 4. 风险指标计算
		profitPct := (currentPrice - pos.CostPrice) / pos.CostPrice * 100
		retracement := (highWatermark - currentPrice) / highWatermark * 100
		ma20 := strategy.CalcMA(historyData, 20)
		dataTradeDate := today.TradeDate

		action := "🟢 继续持有"
		reason := "当前波动可控，可继续跟踪。"
		status := "ok"

		// 风控 1：利润保护 (8% 动态回撤)
		if retracement >= 8.0 {
			action = "🔴 建议减仓/止盈"
			reason = fmt.Sprintf("利润回撤达到 %.2f%%，已跌破最高价 %.2f 的 8%% 动态保护线。", retracement, highWatermark)
		} else if currentPrice < ma20 && ma20 > 0 {
			// 风控 2：趋势转弱 (跌破 20 日均线)
			action = "🔴 建议减仓"
			reason = fmt.Sprintf("趋势转弱：今日收盘 %.2f 已跌破 20 日均线 %.2f。", currentPrice, ma20)
		} else if retracement >= 6.0 {
			// 🟡 预警状态
			action = "🟡 风险预警"
			reason = fmt.Sprintf("距高点已回撤 %.2f%%，接近 8%% 动态保护线，请重点关注。", retracement)
		}

		isStale := latestMarketDate != "" && dataTradeDate < latestMarketDate
		if isStale {
			staleCount++
			action = "🟡 数据待更新"
			status = "stale_data"
			reason = fmt.Sprintf("当前评估基于 %s 的收盘数据，落后于最新交易日 %s。请先同步近期K线后再做交易决策。", dataTradeDate, latestMarketDate)
		}

		reports = append(reports, map[string]interface{}{
			"id":                pos.ID,
			"ts_code":           pos.TSCode,
			"name":              displayName,
			"hold_volume":       pos.HoldVolume,
			"buy_date":          pos.BuyDate,
			"cost_price":        pos.CostPrice,
			"current_price":     currentPrice,
			"high_watermark":    highWatermark,
			"profit_pct":        profitPct,
			"retracement":       retracement,
			"action":            action,
			"status":            status,
			"data_trade_date":   dataTradeDate,
			"latest_trade_date": latestMarketDate,
			"is_stale":          isStale,
			"reason":            reason,
		})
	}

	normalCount := len(reports) - missingCount - staleCount
	msg := fmt.Sprintf("持仓风险评估完成：正常 %d，缺失数据 %d，数据过期 %d。", normalCount, missingCount, staleCount)
	writeAPIResponse(w, 200, msg, reports)
}

// MonitorHandler 兼容旧路由别名（后续可逐步下线）
func MonitorHandler(w http.ResponseWriter, r *http.Request) {
	PositionRiskHandler(w, r)
}
