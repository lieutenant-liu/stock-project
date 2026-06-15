package api

// ============================================================
// api/handlers.go - 核心 API 处理器
// ============================================================
// 这个文件包含项目最核心的业务逻辑处理器：
// 1. 策略扫描 (DiagnoseHandler)
// 2. 数据审计 (AuditHandler)
// 3. 持仓管理 (Add/Delete/Get/PositionRisk)
//
// 【Go 语言知识点】
// - 包级别函数: 不属于任何结构体，直接通过包名调用
// - 数据库查询: db.DB.QueryRow() 查询单行
// - 前复权: 调整历史价格，消除除权除息的影响
// ============================================================

import (
	"encoding/json" // JSON 编解码
	"fmt"           // 格式化输出
	"net/http"      // HTTP 服务器
	"strings"       // 字符串处理
	"time"          // 时间处理

	"stock-backend/db"        // 数据库操作
	"stock-backend/stockutil" // 板块判断
	"stock-backend/strategy"  // 策略引擎
	"stock-backend/tushare"   // Tushare 数据结构
)

// APIResponse 统一 API 响应结构。
type APIResponse struct {
	Code int         `json:"code"` // 状态码
	Msg  string      `json:"msg"`  // 消息
	Data interface{} `json:"data"` // 数据
}

// ------------------------------------------------------------
// 辅助函数
// ------------------------------------------------------------

// normalizeTSCode 支持前端输入纯数字代码并自动补齐交易所后缀。
// 【示例】
// "000001" → "000001.SZ"
// "600519" → "600519.SH"
// "000001.SZ" → "000001.SZ"（已包含后缀，不变）
func normalizeTSCode(rawCode string) string {
	cleanCode := strings.TrimSpace(rawCode)
	if cleanCode == "" {
		return ""
	}
	if strings.Contains(cleanCode, ".") {
		return cleanCode // 已包含后缀
	}
	if strings.HasPrefix(cleanCode, "6") {
		return cleanCode + ".SH" // 上海
	}
	if strings.HasPrefix(cleanCode, "0") || strings.HasPrefix(cleanCode, "3") {
		return cleanCode + ".SZ" // 深圳
	}
	if strings.HasPrefix(cleanCode, "4") || strings.HasPrefix(cleanCode, "8") {
		return cleanCode + ".BJ" // 北交所
	}
	return cleanCode
}

// resolveStockName 优先使用前端传值，缺失时回查 stock_basic 表做兜底。
// 【为什么需要这个？】
// 前端可能传入 "未知" 作为股票名称，此时需要从数据库查询真实名称。
func resolveStockName(tsCode, rawName string) string {
	name := strings.TrimSpace(rawName)
	if name != "" && name != "未知" {
		return name // 前端传了有效名称
	}

	// 从数据库查询
	var dbName string
	err := db.DB.QueryRow(`SELECT COALESCE(name, '') FROM stock_basic WHERE ts_code = ?`, tsCode).Scan(&dbName)
	if err == nil && strings.TrimSpace(dbName) != "" {
		return strings.TrimSpace(dbName)
	}

	return "未知" // 查不到
}

// ------------------------------------------------------------
// 策略扫描核心 API
// ------------------------------------------------------------

// DiagnoseHandler 策略扫描核心 API (V3.0 多态调度架构)
// 【功能说明】
// 这是整个项目最核心的接口：
// 1. 接收股票代码（单个或全市场）
// 2. 加载 K 线、基本面、资金流向等数据
// 3. 调用所有已启用的策略分析器
// 4. 返回买入/卖出信号
//
// 【触发方式】
// GET /api/diagnose?code=000001                    → 单只股票诊断
// GET /api/diagnose?code=000001,600519             → 多只股票诊断
// GET /api/diagnose                                 → 全市场扫描
// GET /api/diagnose?start=20240101&end=20241231    → 指定日期范围
func DiagnoseHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)

	// 获取参数
	rawCodes := r.URL.Query().Get("code")
	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	// 默认日期范围：最近一年
	if endDate == "" {
		endDate = time.Now().Format("20060102")
	}
	if startDate == "" {
		startDate = time.Now().AddDate(-1, 0, 0).Format("20060102")
	}

	// 解析是"定向诊断"还是"全市场扫描"
	var codeList []string
	isAllMarket := false

	if strings.TrimSpace(rawCodes) == "" {
		// 全市场扫描
		codeList = db.GetAllStockCodes()
		isAllMarket = true
		fmt.Printf("🎯 [扫描中心] 收到全市场扫描指令，目标池: %d 只股票\n", len(codeList))
	} else {
		// 定向诊断：解析逗号分隔的代码
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

	// ── 获取已启用的策略分析器 ──
	enabledStrategies, _ := db.GetEnabledStrategies()
	enabledSet := make(map[string]bool, len(enabledStrategies))
	for _, s := range enabledStrategies {
		enabledSet[s.ID] = true
	}
	activeAnalyzers := strategy.GetActiveAnalyzersFiltered(enabledSet)

	// =========================================================
	// [机构级风控：大盘环境与情绪冰点前置拦截]
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
	isMarketSafe, _, marketMsg := strategy.CheckMarketEnvironment(shIndexData, limitUpCount, avgPremium)
	fmt.Printf("🌐 [全局风控] %s\n", marketMsg)

	// 如果大盘处于风险区间，且用户正在执行"全市场扫描"，则直接拦截买入信号输出
	if !isMarketSafe && isAllMarket {
		writeAPIResponse(w, 200, marketMsg+"（系统已自动拦截今日全市场买入信号，建议等待环境改善）", []map[string]interface{}{})
		return
	}
	// =========================================================

	// 遍历每只股票
	for _, code := range codeList {
		// 0. 板块过滤：仅保留主板股票
		if !stockutil.IsValidMainBoardCode(code) {
			continue
		}

		// 1. 过滤垃圾股，同时提取【所属行业】
		var stockName, industry string
		err := db.DB.QueryRow(`SELECT COALESCE(name, '未知'), COALESCE(industry, '未知板块') FROM stock_basic WHERE ts_code = ?`, code).Scan(&stockName, &industry)
		if err != nil {
			stockName = "未知"
			industry = "未知板块"
		}

		// 坚决不碰 ST 股
		if strings.Contains(stockName, "ST") {
			continue
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
		// 根据策略需要的数据类型，按需加载
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

		// 计算 PE 历史分位
		pePercentile := db.GetPEPercentile(code, endDate, 750)

		// 获取筹码分布数据
		cyqPerf := db.GetCyqPerfFromDB(code, endDate)

		// 构建策略上下文
		ctx := &strategy.SecurityContext{
			Code:         code,
			StockName:    stockName,
			KLines:       historyData,
			Fundamentals: fundData,
			MoneyFlows:   flowData,
			PEPercentile: pePercentile,
			CyqPerf:      cyqPerf,
		}

		// 4. 多策略并发/循环分析
		for _, analyzer := range activeAnalyzers {
			result := analyzer.Analyze(ctx)

			// 如果不是"观望"信号，加入结果列表
			if result.Signal != "观望 💤" && result.Signal != "" {
				// 只保留最近 90 天的 K 线用于图表展示
				chartData := historyData
				if len(historyData) > 90 {
					chartData = historyData[len(historyData)-90:]
				}

				// 构建前端展示数据
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

				// 全市场扫描只返回买入信号
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
				tagName = "[📈 独立走势·低强度]" // 低权重：个股独立逻辑
			}

			// 将标签强行注入策略名称
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

// ------------------------------------------------------------
// 数据审计
// ------------------------------------------------------------

// AuditHandler 数据体检接口 (支持免后缀输入)
// 【功能说明】
// 检查指定股票的数据完整性：
// - K 线数据是否连续
// - 基本面数据是否缺失
// - 复权因子是否完整
//
// 【触发方式】
// GET /api/audit?code=000001&start=20240101&end=20241231
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

	// 执行数据审计
	report := db.RunDataAudit(code, start, end)
	writeAPIResponse(w, 200, "体检完成", report)
}

// ------------------------------------------------------------
// 持仓管理
// ------------------------------------------------------------

// AddPositionHandler 供前端表单调用：手动录入持仓
// 【触发方式】
// POST /api/position/add
// Body: {"ts_code": "000001.SZ", "stock_name": "平安银行", "hold_volume": 1000, "cost_price": 12.5, "buy_date": "20240115"}
func AddPositionHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSONForWriteOps(w)
	if handleAPIOptions(w, r) {
		return
	}

	// 解析请求体
	var pos db.Position
	if err := json.NewDecoder(r.Body).Decode(&pos); err != nil {
		writeAPIResponse(w, 400, "参数解析失败", nil)
		return
	}

	// 自动补齐股票代码后缀
	pos.TSCode = normalizeTSCode(pos.TSCode)
	// 解析股票名称
	pos.StockName = resolveStockName(pos.TSCode, pos.StockName)

	// 防呆拦截
	if pos.CostPrice <= 0 || pos.HoldVolume <= 0 {
		writeAPIResponse(w, 400, "成本价和持仓量必须大于 0", nil)
		return
	}

	// 写入数据库
	if err := db.AddPosition(pos); err != nil {
		writeAPIResponse(w, 500, "录入失败: "+err.Error(), nil)
		return
	}
	writeAPIResponse(w, 200, "持仓已添加", nil)
}

// DeletePositionHandler 供前端调用：手动移出持仓
// 【触发方式】
// POST /api/position/delete
// Body: {"id": 1}
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
// 【触发方式】
// GET /api/position/list
func GetPositionsHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)
	positions, _ := db.GetAllPositions()
	writeAPIResponse(w, 200, "success", positions)
}

// ------------------------------------------------------------
// 持仓风险评估
// ------------------------------------------------------------

// PositionRiskHandler 持仓风险评估核心接口。
// 【功能说明】
// 100% 复用策略引擎 EvaluateHold，与回测卖出逻辑镜像一致。
// 对每只持仓股票：
// 1. 加载历史 K 线数据
// 2. 执行前复权清洗
// 3. 调用策略的 EvaluateHold 评估卖出信号
// 4. 检查追踪止损和硬止损
//
// 【触发方式】
// GET /api/position/risk
func PositionRiskHandler(w http.ResponseWriter, r *http.Request) {
	prepareAPIJSON(w)

	// 获取所有持仓
	positions, err := db.GetAllPositions()
	if err != nil || len(positions) == 0 {
		writeAPIResponse(w, 200, "当前无持仓，风险评估为空。", []map[string]interface{}{})
		return
	}

	endDate := time.Now().Format("20060102")
	latestMarketDate := db.GetLatestOpenTradeDate(endDate)

	// 构建策略名→Analyzer 实例的映射表
	analyzers := strategy.GetActiveAnalyzers()
	analyzerMap := make(map[string]strategy.Analyzer, len(analyzers)+1)
	for _, a := range analyzers {
		analyzerMap[a.Name()] = a
	}
	// DSS 虽然禁用但持仓可能仍关联它
	analyzerMap[(&strategy.DSSAnalyzer{}).Name()] = &strategy.DSSAnalyzer{}

	var reports []map[string]interface{}
	missingCount := 0
	staleCount := 0

	// 遍历每只持仓
	for _, pos := range positions {
		displayName := resolveStockName(pos.TSCode, pos.StockName)

		// 1. 加载买入日之前的额外 K 线（策略需要 120+ 天历史计算均线/箱体）
		lookbackStart := subtractDays(pos.BuyDate, 300)
		historyData := db.GetKLinesFromDB(pos.TSCode, lookbackStart, endDate)
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
				"profit_pct":        nil,
				"action":            "⚪ 数据待补齐",
				"status":            "missing_data",
				"data_trade_date":   "",
				"latest_trade_date": latestMarketDate,
				"is_stale":          true,
				"reason":            fmt.Sprintf("未找到 %s 从 %s 到当前的有效日线数据。请先同步近期K线。", pos.TSCode, lookbackStart),
			})
			continue
		}

		// 2. 前复权清洗
		adjFactors := db.GetAdjFactorsFromDB(pos.TSCode, lookbackStart, endDate)
		if len(adjFactors) > 0 {
			historyData = strategy.ForwardAdjustKLines(historyData, adjFactors)
		}

		today := historyData[len(historyData)-1]
		currentPrice := today.Close
		dataTradeDate := today.TradeDate

		// 3. 计算盈亏指标
		profitPct := (currentPrice - pos.CostPrice) / pos.CostPrice * 100
		highWatermark := pos.CostPrice
		for _, k := range historyData {
			if k.Close > highWatermark {
				highWatermark = k.Close
			}
		}
		retracement := 0.0
		if highWatermark > 0 {
			retracement = (highWatermark - currentPrice) / highWatermark * 100
		}

		// 4. 定位买入日在 K 线数组中的位置
		buyIdx := -1
		for idx, k := range historyData {
			if k.TradeDate >= pos.BuyDate {
				buyIdx = idx
				break
			}
		}
		if buyIdx < 0 {
			buyIdx = 0
		}

		// 计算买入时的基础 ATR
		buyATR := 0.0
		if buyIdx >= 14 {
			buyATR = strategy.CalcATR(historyData[:buyIdx+1], 14)
		}

		// 5. 查找对应的策略 Analyzer
		analyzer, found := analyzerMap[pos.Strategy]
		if !found || pos.Strategy == "" {
			// 未关联策略的持仓：降级为通用展示
			action := "🟢 继续持有"
			reason := "未关联策略引擎，请手动评估。"

			// 追踪止损（即使未关联策略也生效）
			holdingHistory := historyData[buyIdx:]
			if triggered, _, trailReason := strategy.CheckTrailingStop(today, holdingHistory, buyATR, 2.5, 0.12); triggered {
				action = "🔴 触发追踪止损"
				reason = trailReason
			} else if strategy.IsTrailingStopTriggered(today, holdingHistory, buyATR, 2.5, 0.12) {
				action = "⚠️ 触发止损但跌停无法卖出"
				reason = "追踪止损条件已满足，但当前处于跌停状态无法成交，需等待跌停打开后立即卖出。"
			}
			if action == "🟢 继续持有" {
				if triggered, _, hardReason := strategy.CheckHardStop(today, pos.CostPrice, 0.10); triggered {
					action = "🔴 触发-10%硬止损"
					reason = hardReason
				}
			}
			reports = append(reports, buildPositionReport(pos, displayName, currentPrice, profitPct, retracement, highWatermark, action, reason, "ok", dataTradeDate, latestMarketDate))
			continue
		}

		// 6. 重建策略元数据
		meta := buildLiveStrategyMeta(pos.Strategy, historyData, buyIdx)

		// 7. 构建 Position 对象供 EvaluateHold 使用
		strategyPos := &strategy.Position{
			Code:      pos.TSCode,
			BuyDate:   pos.BuyDate,
			BuyPrice:  pos.CostPrice,
			Strategy:  pos.Strategy,
			BuyResult: strategy.DiagnoseResult{PEPercentile: 0.5},
		}

		// 8. 两阶段动态止损
		holdingHistory := historyData[buyIdx:]
		maxGainPct := (highWatermark - pos.CostPrice) / pos.CostPrice * 100
		stageBActive := maxGainPct >= 15.0

		var stopTriggered, stopConditionMet bool
		var stopReason string

		if stageBActive {
			stopTriggered, _, stopReason = strategy.CheckTrailingStop(today, holdingHistory, buyATR, 2.5, 0.12)
			stopConditionMet = !stopTriggered && strategy.IsTrailingStopTriggered(today, holdingHistory, buyATR, 2.5, 0.12)
		} else {
			stopTriggered, _, stopReason = strategy.CheckHardStop(today, pos.CostPrice, 0.10)
			stopConditionMet = false
		}

		// 9. 调用策略 EvaluateHold
		eval := analyzer.EvaluateHold(strategyPos, today, historyData, meta)

		// 10. 组装前端输出
		stageTag := "A"
		if stageBActive {
			stageTag = "B"
		}
		action := fmt.Sprintf("🟢 策略持仓中 [阶段%s]", stageTag)
		reason := fmt.Sprintf("策略 [%s] 持仓评估通过，当前价格 %.2f，最大浮盈 %.1f%%，ATR %.2f。", pos.Strategy, currentPrice, maxGainPct, buyATR)
		status := "ok"

		if stopTriggered {
			if stageBActive {
				action = "🔴 触发追踪止损"
			} else {
				action = "🔴 触发-10%硬止损"
			}
			reason = stopReason
			status = "sell_signal"
		} else if stopConditionMet {
			action = "⚠️ 触发止损但跌停无法卖出"
			reason = "止损条件已满足，但当前处于跌停状态无法成交，需等待跌停打开后立即卖出。"
			status = "sell_signal"
		} else if eval.Sell {
			action = "🔴 策略触发卖出"
			reason = eval.Reason
			status = "sell_signal"
		}

		isStale := latestMarketDate != "" && dataTradeDate < latestMarketDate
		if isStale {
			staleCount++
			if !stopTriggered && !stopConditionMet && !eval.Sell {
				action = "🟡 数据待更新"
				status = "stale_data"
			}
			reason += fmt.Sprintf(" (数据截至 %s，最新交易日 %s)", dataTradeDate, latestMarketDate)
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
			"max_gain_pct":      maxGainPct,
			"stage":             stageTag,
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

// buildPositionReport 辅助：构建持仓报告 map。
func buildPositionReport(pos db.Position, displayName string, currentPrice, profitPct, retracement, highWatermark float64, action, reason, status, dataTradeDate, latestMarketDate string) map[string]interface{} {
	isStale := latestMarketDate != "" && dataTradeDate < latestMarketDate
	return map[string]interface{}{
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
	}
}

// buildLiveStrategyMeta 从买入日的 K 线上下文重建 EvaluateHold 所需的策略元数据。
func buildLiveStrategyMeta(strategyName string, klines []tushare.DailyKLine, buyIdx int) map[string]float64 {
	meta := make(map[string]float64)

	switch {
	case strings.Contains(strategyName, "CBBM"):
		if buyIdx >= 60 {
			boxUpper, _ := strategy.GetRealBox(klines[:buyIdx+1], 60)
			meta["box_upper"] = boxUpper
		}

	case strings.Contains(strategyName, "DSS"):
		if buyIdx >= 60 {
			boxUpper, boxLower := strategy.GetRealBox(klines[:buyIdx+1], 60)
			meta["box_upper"] = boxUpper
			meta["box_lower"] = boxLower
			meta["atr14"] = strategy.CalcATR(klines[:buyIdx+1], 14)
		}

	case strings.Contains(strategyName, "PBMA"):
		if buyIdx >= 14 {
			meta["buy_atr"] = strategy.CalcATR(klines[:buyIdx+1], 14)
		}
	}
	return meta
}

// subtractDays 从 YYYYMMDD 字符串中减去 N 天。
func subtractDays(dateStr string, days int) string {
	t, err := time.Parse("20060102", dateStr)
	if err != nil {
		return dateStr
	}
	return t.AddDate(0, 0, -days).Format("20060102")
}

// MonitorHandler 兼容旧路由别名（后续可逐步下线）
func MonitorHandler(w http.ResponseWriter, r *http.Request) {
	PositionRiskHandler(w, r)
}
