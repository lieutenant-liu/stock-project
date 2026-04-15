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

// DiagnoseHandler 策略扫描雷达的核心 API (V3.0 多态调度架构)
func DiagnoseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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
		fmt.Printf("🎯 [雷达中枢] 收到全市场扫描指令！目标池: %d 只股票...\n", len(codeList))
	} else {
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
			codeList = append(codeList, cleanCode)
		}
	}

	var results []map[string]interface{}

	// 💥 架构核心：唤醒所有军师（策略分析器）
	activeAnalyzers := strategy.GetActiveAnalyzers()
	// =========================================================
	// 🛡️ [机构级风控：大盘 Beta 与 情绪冰点 全局熔断前置拦截]
	// =========================================================
	shIndexData := db.GetIndexDailyFromDB("000001.SH", startDate, endDate)

	// 💥 提取昨日和今日的交易日期，计算打板溢价率
	limitUpCount := 0
	avgPremium := 0.0
	if len(shIndexData) >= 2 {
		todayDate := shIndexData[len(shIndexData)-1].TradeDate
		yesterdayDate := shIndexData[len(shIndexData)-2].TradeDate
		limitUpCount, avgPremium = db.GetLimitUpPremium(yesterdayDate, todayDate)
	}

	// 把溢价率数据喂给风控中心
	isMarketSafe, marketMsg := strategy.CheckMarketEnvironment(shIndexData, limitUpCount, avgPremium)

	fmt.Printf("🌐 [全局风控] %s\n", marketMsg)

	// 如果大盘处于暴跌或情绪退潮，且用户正在执行“全市场扫描”，则直接拔网线！
	if !isMarketSafe && isAllMarket {
		json.NewEncoder(w).Encode(APIResponse{
			Code: 200,
			Msg:  marketMsg + " (系统已自动拦截今日所有买入操作，耐心等待情绪反转！)",
			Data: []map[string]interface{}{},
		})
		return
	}
	// =========================================================
	for _, code := range codeList {
		// 1. 过滤垃圾股，同时提取【所属行业】
		var stockName, industry string

		// 💥 终极修复：使用 SQL 的 COALESCE 自动处理 NULL 值，Go 端只接收纯净的 string！
		err := db.DB.QueryRow(`SELECT COALESCE(name, '未知'), COALESCE(industry, '未知板块') FROM stock_basic WHERE ts_code = ?`, code).Scan(&stockName, &industry)
		if err != nil {
			stockName = "未知"
			industry = "未知板块"
		}

		if strings.Contains(stockName, "ST") {
			continue // 坚决不碰 ST
		}

		// 2. 提取底层公共弹药：K线量价数据
		historyData := db.GetKLinesFromDB(code, startDate, endDate)
		if len(historyData) == 0 {
			continue
		}

		// =========================================================
		// 💥 [数据治理]：强制前复权清洗，抹平 K 线断层！
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

		// 💥 直接使用纯净的 stockName，再也不会有 Object 报错！
		ctx := &strategy.SecurityContext{
			Code:         code,
			StockName:    stockName,
			KLines:       historyData,
			Fundamentals: fundData,
			MoneyFlows:   flowData,
			PEPercentile: pePercentile,
		}

		// 4. 兵分多路，多态策略并发/循环裁决
		for _, analyzer := range activeAnalyzers {
			result := analyzer.Analyze(ctx)

			if result.Signal != "观望 💤" && result.Signal != "" {
				chartData := historyData
				if len(historyData) > 90 {
					chartData = historyData[len(historyData)-90:]
				}

				// 💥 直接放入纯净的 stockName 和 industry！
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
	// 💥 [全局扫描] 行业共振雷达与阶梯打标引擎
	// =========================================================
	if len(results) > 0 {
		// 1. 统计今天各个行业触发买点的股票总数
		industryCount := make(map[string]int)
		for _, item := range results {
			ind := item["industry"].(string)
			industryCount[ind]++
		}

		// 2. 二次遍历，打上不同权重的战术标签
		for i := range results {
			ind := results[i]["industry"].(string)
			count := industryCount[ind]

			var tagName string
			// 按照资金攻击的强度分为三个梯队
			if count >= 3 {
				tagName = "[🌟 板块共振·主升浪]" // 极高权重：产业级逻辑爆发
			} else if count == 2 {
				tagName = "[🔥 行业异动·双龙戏珠]" // 中高权重：资金尝试攻击该方向
			} else {
				tagName = "[🐺 独立行情·孤狼]" // 低权重：个股独立逻辑 (不剔除，但提示风险)
			}

			// 将标签强行注入策略名称，前端会直接高亮显示
			origStrategy := results[i]["strategy_name"].(string)
			results[i]["strategy_name"] = fmt.Sprintf("%s %s", tagName, origStrategy)

			// 如果是板块共振，在战报最醒目的位置加上群聚提示
			if count > 1 {
				origMsg := results[i]["message"].(string)
				results[i]["message"] = fmt.Sprintf("【同板块今日共有 %d 只股票同时爆发买点！】\n%s", count, origMsg)
			}
		}
	}
	// =========================================================

	if isAllMarket {
		fmt.Printf("🎉 [雷达中枢] 扫描完毕！三大策略引擎共揪出 %d 个战术买点。\n", len(results))
	}

	json.NewEncoder(w).Encode(APIResponse{
		Code: 200,
		Msg:  fmt.Sprintf("扫描完毕，共发现 %d 个符合策略的买点", len(results)),
		Data: results,
	})
}

// AuditHandler 数据体检接口 (支持免后缀输入)
func AuditHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	code := r.URL.Query().Get("code")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	code = strings.TrimSpace(code)
	if code == "" {
		json.NewEncoder(w).Encode(APIResponse{Code: 400, Msg: "请提供股票代码"})
		return
	}

	// 💥 智能补齐后缀
	if !strings.Contains(code, ".") {
		if strings.HasPrefix(code, "6") {
			code += ".SH"
		} else if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
			code += ".SZ"
		} else if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
			code += ".BJ"
		}
	}

	report := db.RunDataAudit(code, start, end)
	json.NewEncoder(w).Encode(APIResponse{
		Code: 200,
		Msg:  "体检完成",
		Data: report,
	})
}

// AddPositionHandler 供前端表单调用：手动录入持仓
func AddPositionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var pos db.Position
	if err := json.NewDecoder(r.Body).Decode(&pos); err != nil {
		json.NewEncoder(w).Encode(APIResponse{Code: 400, Msg: "参数解析失败"})
		return
	}

	// =========================================================
	// 💥 核心修复：智能补齐股票代码后缀 (防幽灵持仓)
	// =========================================================
	cleanCode := strings.TrimSpace(pos.TSCode)
	if cleanCode != "" && !strings.Contains(cleanCode, ".") {
		if strings.HasPrefix(cleanCode, "6") {
			cleanCode += ".SH"
		} else if strings.HasPrefix(cleanCode, "0") || strings.HasPrefix(cleanCode, "3") {
			cleanCode += ".SZ"
		} else if strings.HasPrefix(cleanCode, "4") || strings.HasPrefix(cleanCode, "8") {
			cleanCode += ".BJ"
		}
	}
	pos.TSCode = cleanCode // 将补全后的标准代码覆盖回去
	// =========================================================

	// 防呆拦截
	if pos.CostPrice <= 0 || pos.HoldVolume <= 0 {
		json.NewEncoder(w).Encode(APIResponse{Code: 400, Msg: "成本价和持仓量必须大于 0"})
		return
	}

	if err := db.AddPosition(pos); err != nil {
		json.NewEncoder(w).Encode(APIResponse{Code: 500, Msg: "录入失败: " + err.Error()})
		return
	}
	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "持仓防线已部署"})
}

// DeletePositionHandler 供前端调用：手动移出持仓
func DeletePositionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// 💥 同样补齐 CORS 跨域防弹衣
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(APIResponse{Code: 400, Msg: "参数解析失败"})
		return
	}
	db.DeletePosition(req.ID)
	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "仓位已清除"})
}

// GetPositionsHandler 供前端调用：展示当前持仓列表
func GetPositionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	positions, _ := db.GetAllPositions()
	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "success", Data: positions})
}

// ==========================================
// 🔪 机械侧刀核心引擎 (MonitorHandler)
// ==========================================
func MonitorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	positions, err := db.GetAllPositions()
	if err != nil || len(positions) == 0 {
		json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "当前空仓，侧刀静默。", Data: []map[string]interface{}{}})
		return
	}

	endDate := time.Now().Format("20060102")
	var reports []map[string]interface{}

	for _, pos := range positions {
		// 1. 动态获取建仓日以来的所有 K 线 (这是计算水位的关键)
		historyData := db.GetKLinesFromDB(pos.TSCode, pos.BuyDate, endDate)
		if len(historyData) == 0 {
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

		// 4. 战术指标计算
		profitPct := (currentPrice - pos.CostPrice) / pos.CostPrice * 100
		retracement := (highWatermark - currentPrice) / highWatermark * 100
		ma20 := strategy.CalcMA(historyData, 20)

		action := "🟢 安全持仓"
		reason := "防线稳固，明日继续持股。"

		// 🔪 侧刀 1：利润捍卫者 (8% 动态回撤)
		if retracement >= 8.0 {
			action = "🔴 执行斩首"
			reason = fmt.Sprintf("利润回撤达标(%.2f%%)！已跌破最高水位(%.2f)的 8%% 动态防线。", retracement, highWatermark)
		} else if currentPrice < ma20 && ma20 > 0 {
			// 🔪 侧刀 2：逻辑证伪器 (跌破 20 日生命线)
			action = "🔴 执行斩首"
			reason = fmt.Sprintf("破位警报！今日收盘(%.2f)已实质性击穿 20日生命线(%.2f)。", currentPrice, ma20)
		} else if retracement >= 6.0 {
			// 🟡 预警状态
			action = "🟡 警戒状态"
			reason = fmt.Sprintf("距高点已回撤 %.2f%%，逼近 8%% 斩首线，明日重点盯防！", retracement)
		}

		reports = append(reports, map[string]interface{}{
			"id":             pos.ID,
			"ts_code":        pos.TSCode,
			"name":           pos.StockName,
			"cost_price":     pos.CostPrice,
			"current_price":  currentPrice,
			"high_watermark": highWatermark,
			"profit_pct":     profitPct,
			"retracement":    retracement,
			"action":         action,
			"reason":         reason,
		})
	}

	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "盘后审判完毕", Data: reports})
}
