package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"stock-backend/db"
	"stock-backend/feeder"
	"stock-backend/tushare"
	"strconv"
	"time"
)

func triggerSyncBasicHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	go func() {
		feeder.LogMsg("📜 [基建中心] 正在向 Tushare 索要 A 股最新花名册...")
		basics, err := tushare.FetchStockBasic()
		if err != nil {
			feeder.LogMsg("❌ [基建中心] 花名册拉取失败: %v", err)
			return
		}

		if len(basics) > 0 {
			saved := db.BatchInsertStockBasic(basics)
			feeder.LogMsg("✅ [基建中心] 花名册更新完毕！共入库: %d 只股票", saved)
		}
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "花名册同步指令已下发！"})
}

func updateSpeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	speedStr := r.URL.Query().Get("speed")

	speedMs, err := strconv.Atoi(speedStr)
	if err != nil || speedMs < 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请输入合法的毫秒数(大于等于0)"})
		return
	}

	feeder.SetBaseDelay(speedMs)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("引擎射速已更新为: %d 毫秒/发", speedMs),
	})
}

func triggerSyncMoneyFlowHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	start, end, source := r.URL.Query().Get("start"), r.URL.Query().Get("end"), r.URL.Query().Get("source")
	codesToSync := parseTargetCodes(r.URL.Query().Get("codes"))

	var provider feeder.DataProvider = &feeder.OpenSourceProvider{}
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	}

	go func() {
		feeder.LogMsg("🌊 [抽水机E] 资金流向引擎启动！当前源:[%s]", provider.GetName())

		targetTrust := 50
		if source == "tushare" {
			targetTrust = 100
		}

		for i, code := range codesToSync {
			actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_moneyflow", code, start, end, targetTrust)
			if !needSync {
				continue
			}

			feeder.WaitToken()
			flows, err := provider.FetchMoneyFlow(code, actualStart, actualEnd)

			if err != nil {
				feeder.LogMsg("⚠️ [抽水机E %d/%d] %s 报错: %v", i+1, len(codesToSync), code, err)
			} else if len(flows) > 0 {
				feeder.PushToSink("moneyflow", code, flows)
			}
		}
		feeder.LogMsg("🎉 [抽水机E] 资金流向网络拉取完成！")
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "资金流向管线已启动！"})
}

func triggerSyncFinaHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	start, end := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	codesToSync := parseTargetCodes(r.URL.Query().Get("codes"))

	go func() {
		feeder.LogMsg("🏦 [抽水机F] 季报财务引擎启动！[Tushare专属]")
		for i, code := range codesToSync {
			feeder.WaitToken()
			finas, err := tushare.FetchFinaIndicators(code, start, end)
			if err != nil {
				feeder.LogMsg("⚠️ [抽水机F %d/%d] %s 报错: %v", i+1, len(codesToSync), code, err)
			} else if len(finas) > 0 {
				feeder.PushToSink("fina", code, finas)
			}
		}
		feeder.LogMsg("🎉 [抽水机F] 季报财务拉取完成！")
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "季报财务管线(高权)已启动！"})
}

func triggerSyncLimitListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")

	var provider feeder.DataProvider = &feeder.TushareProvider{}
	if source == "eastmoney" {
		provider = &feeder.OpenSourceProvider{}
	}

	go feeder.StartSyncStkLimit(provider, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  "涨跌停管线已启动！系统将自动提取日历空洞并执行后台灌注。",
	})
}

func triggerSyncFundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	var provider feeder.DataProvider
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	} else {
		provider = &feeder.OpenSourceProvider{}
	}

	go feeder.StartSyncFund(provider, codesToSync, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("基本面同步已启动，当前数据源: %s", provider.GetName()),
	})
}

func triggerSyncKlineHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	var provider feeder.DataProvider
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	} else {
		provider = &feeder.OpenSourceProvider{}
	}

	go feeder.StartSyncKLine(provider, codesToSync, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("K 线同步已启动，当前数据源: %s", provider.GetName()),
	})
}

func getLogsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"data": feeder.GetLogs(),
	})
}

func triggerSyncCalendarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	source := r.URL.Query().Get("source")

	go func() {
		currentYear := 1990
		endYear := time.Now().Year() + 1
		totalSaved := 0

		if source == "tushare" || source == "hybrid" {
			feeder.LogMsg("📅 [真理钟] 启动【Tushare 官方日历】同步引擎...")
			for y := currentYear; y <= endYear; y++ {
				startStr := fmt.Sprintf("%04d0101", y)
				endStr := fmt.Sprintf("%04d1231", y)

				feeder.WaitToken()
				calendars, err := tushare.FetchTradeCalendar(startStr, endStr)
				if err != nil {
					feeder.LogMsg("⚠️ [真理钟] %d年 日历拉取失败: %v", y, err)
					continue
				}

				if len(calendars) > 0 {
					saved := db.BatchInsertTradeCalendar(calendars)
					totalSaved += saved
				}
			}
			feeder.LogMsg("✅ [真理钟] Tushare 官方日历同步完毕！共覆盖 %d 天记录！", totalSaved)
		} else {
			feeder.LogMsg("📅 [真理钟] 启动【开源多源交叉验证】日历推导...")
			provider := &feeder.OpenSourceProvider{}
			targetIndices := []string{"000001.SH", "399001.SZ", "399006.SZ"}

			for y := currentYear; y <= endYear; y++ {
				startStr := fmt.Sprintf("%04d0101", y)
				endStr := fmt.Sprintf("%04d1231", y)

				dailyMap := make(map[string]bool)
				for _, idxCode := range targetIndices {
					klines, err := provider.FetchIndexDaily(idxCode, startStr, endStr)
					if err == nil && len(klines) > 0 {
						for _, k := range klines {
							dailyMap[k.TradeDate] = true
						}
					}
					time.Sleep(150 * time.Millisecond)
				}

				if len(dailyMap) == 0 {
					continue
				}

				var calendars []tushare.TradeCalendar
				for date := range dailyMap {
					calendars = append(calendars, tushare.TradeCalendar{CalDate: date, IsOpen: 1})
				}
				saved := db.BatchInsertTradeCalendar(calendars)
				totalSaved += saved
			}
			feeder.LogMsg("✅ [真理钟] 开源交叉日历生成完毕！共拼凑出 %d 个交易日！", totalSaved)
		}
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  "日历同步指令已下达，引擎正在后台运转，请查看终端日志！",
	})
}

func triggerSyncAdjHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	var provider feeder.DataProvider
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	} else {
		provider = &feeder.OpenSourceProvider{}
	}

	go feeder.StartSyncAdjFactors(provider, codesToSync, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("复权因子抽水机已启动！当前火力源: %s", provider.GetName()),
	})
}

func triggerSyncIndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")

	go func() {
		feeder.LogMsg("📊 [引擎D] 大盘指数同步启动...")

		var provider feeder.DataProvider = &feeder.OpenSourceProvider{}
		targetTrust := 50
		if source == "tushare" {
			provider = &feeder.TushareProvider{}
			targetTrust = 100
		}

		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("index_daily", "000001.SH", startDate, endDate, targetTrust)

		if needSync {
			indices, err := provider.FetchIndexDaily("000001.SH", actualStart, actualEnd)
			if err == nil && len(indices) > 0 {
				saved := db.BatchInsertIndexDaily("000001.SH", indices)
				feeder.LogMsg("✅ [引擎D] 上证指数同步完毕，新增/覆盖: %d 条", saved)
			} else if err != nil {
				feeder.LogMsg("❌ [引擎D] 上证指数拉取失败: %v", err)
			}
		} else {
			feeder.LogMsg("✅ [引擎D] 上证指数严丝合缝，无需重复拉取。")
		}
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "大盘指数抽水机已启动！"})
}
