package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"stock-backend/api"
	"stock-backend/db"
	"stock-backend/feeder"
	"stock-backend/tushare"
	"strconv"
	"strings"
	"time"
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

// =========================================================
// 💥 终极动态灯塔：自动嗅探网卡并计算子网广播地址
// =========================================================
func startDynamicLighthouse() {
	go func() {
		for {
			// 1. 获取机器上所有的物理/虚拟网卡
			interfaces, err := net.Interfaces()
			if err != nil {
				time.Sleep(3 * time.Second)
				continue
			}

			// 2. 遍历所有网卡，计算各自的广播地址
			for _, iface := range interfaces {
				// 跳过处于 Down 状态的网卡和 Loopback (127.0.0.1)
				if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
					continue
				}

				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}

				for _, addr := range addrs {
					// 筛选出 IPv4 地址
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ip := ipnet.IP.To4()
							mask := ipnet.Mask

							// 3. 核心算法：IP | ^Mask 计算出动态定向广播地址
							bcast := make(net.IP, len(ip))
							for i := 0; i < len(ip); i++ {
								bcast[i] = ip[i] | ^mask[i]
							}

							// 4. 发射心跳包
							targetAddr := fmt.Sprintf("%s:8888", bcast.String())
							udpAddr, err := net.ResolveUDPAddr("udp", targetAddr)
							if err == nil {
								conn, err := net.DialUDP("udp", nil, udpAddr)
								if err == nil {
									conn.Write([]byte("QUANT_TERMINAL_ONLINE"))
									conn.Close()
								}
							}
						}
					}
				}
			}
			// 每 3 秒扫描并广播一次
			time.Sleep(3 * time.Second)
		}
	}()
	fmt.Println("📡 [动态灯塔] 局域网自适应广播已开启，无视动态 IP 漂移...")
}

// 💥 新增：强力时间清洗器，防御各种非法格式导致 Panic
func sanitizeDate(dateStr string) string {
	clean := strings.ReplaceAll(dateStr, "-", "")
	clean = strings.ReplaceAll(clean, "/", "")
	clean = strings.TrimSpace(clean)
	if len(clean) != 8 {
		return "" // 非法长度直接置空，让底层走默认逻辑
	}
	return clean
}

// parseTargetCodes 解析前端传来的指定股票代码，为空则扫描全部
func parseTargetCodes(rawCodes string) []string {
	var codesToSync []string
	if rawCodes != "" {
		parts := strings.Split(rawCodes, ",")
		for _, p := range parts {
			cleanCode := strings.TrimSpace(p)
			if cleanCode == "" {
				continue
			}
			// 自动补全后缀
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

// 💥 独立基建：全市场股票花名册同步
func triggerSyncBasicHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	go func() {
		feeder.LogMsg("📜 [基建中心] 正在向 Tushare 索要 A 股最新花名册...")

		// 务必确保这里的 token 是可用的，默认底火即可
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

// 💥 动态换弹夹接口
func updateTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	newToken := r.URL.Query().Get("token")
	if newToken != "" {
		tushare.SetToken(newToken)
		existedID, err := db.FindAPITokenID("tushare", newToken)
		if err == nil && existedID > 0 {
			_, _ = db.SetActiveAPIToken("tushare", existedID)
		} else if err == nil {
			newID, addErr := db.AddAPIToken("tushare", newToken, "legacy", 100, true, "通过 /api/set_token 同步", true)
			if addErr == nil {
				_, _ = db.SetActiveAPIToken("tushare", newID)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "Token 已更新并同步到 token 池"})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "Token 不能为空"})
	}
}

type tokenCreateRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
	Tier     string `json:"tier"`
	Priority int    `json:"priority"`
	Enabled  *bool  `json:"enabled"`
	Notes    string `json:"notes"`
	Active   bool   `json:"active"`
}

type tokenUpdateRequest struct {
	ID       int64  `json:"id"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
	Tier     string `json:"tier"`
	Notes    string `json:"notes"`
}

type tokenActivateRequest struct {
	Provider string `json:"provider"`
	ID       int64  `json:"id"`
}

func tokenCollectionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		provider := r.URL.Query().Get("provider")
		tokens, err := db.ListAPITokens(provider)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 500, "msg": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": tokens})
		return
	case http.MethodPost:
		var req tokenCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
			return
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		id, err := db.AddAPIToken(req.Provider, req.Token, req.Tier, req.Priority, enabled, req.Notes, req.Active)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		if req.Active {
			token, activeErr := db.SetActiveAPIToken(req.Provider, id)
			if activeErr == nil && strings.EqualFold(strings.TrimSpace(req.Provider), "tushare") {
				tushare.SetToken(token)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "token 已添加", "id": id})
		return
	case http.MethodPut:
		var req tokenUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
			return
		}
		if req.ID <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "id 不能为空"})
			return
		}
		if err := db.UpdateAPIToken(req.ID, req.Tier, req.Priority, req.Enabled, req.Notes); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "token 已更新"})
		return
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "id 参数非法"})
			return
		}
		if err := db.DeleteAPIToken(id); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
			return
		}
		active, activeErr := db.GetActiveAPIToken("tushare")
		if activeErr == nil && active != nil {
			tushare.SetToken(active.Token)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "token 已删除"})
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}
}

func tokenActivateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	setCORSHeaders(w)
	if handlePreflight(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 405, "msg": "不支持的请求方法"})
		return
	}

	var req tokenActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请求体格式错误"})
		return
	}
	if req.ID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "id 不能为空"})
		return
	}

	rawToken, err := db.SetActiveAPIToken(req.Provider, req.ID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": err.Error()})
		return
	}

	if strings.EqualFold(strings.TrimSpace(req.Provider), "tushare") || strings.TrimSpace(req.Provider) == "" {
		tushare.SetToken(rawToken)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "当前 token 已切换"})
}

func syncActiveProviderToken(provider string) {
	active, err := db.GetActiveAPIToken(provider)
	if err != nil || active == nil {
		return
	}
	if strings.EqualFold(provider, "tushare") {
		tushare.SetToken(active.Token)
	}
}

// 💥 动态调整射速接口 (变速箱)
func updateSpeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	speedStr := r.URL.Query().Get("speed")

	speedMs, err := strconv.Atoi(speedStr)
	if err != nil || speedMs < 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "请输入合法的毫秒数(大于等于0)"})
		return
	}

	// 呼叫底层引擎换挡
	feeder.SetBaseDelay(speedMs)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("引擎射速已更新为: %d 毫秒/发", speedMs),
	})
}

// 💥 高阶管线 E：主力资金流向 (MoneyFlow - 暴露错误版)
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

			// 修复静默吞错，输出真实错误信息
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

// 💥 高阶管线 F：季报财务 (Fina - 暴露错误版)
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

			// 💥 修复静默吞没！
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

// / 💥 高阶管线 G：每日涨跌停榜 (标准架构多态版)
func triggerSyncLimitListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source") // 依然预留 source 接口供前端调度

	// 默认启用 Tushare 数据源
	var provider feeder.DataProvider = &feeder.TushareProvider{}
	if source == "eastmoney" {
		provider = &feeder.OpenSourceProvider{}
	}

	// 彻底甩给后台的智能调度引擎
	go feeder.StartSyncStkLimit(provider, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  "涨跌停管线已启动！系统将自动提取日历空洞并执行后台灌注。",
	})
}

// 💥 基本面同步接口 (双擎升级版)
func triggerSyncFundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source") // 💥 新增：接收前端的数据源指令

	// 删掉原有的 codesToSync := db.GetAllStockCodes()
	// 替换为：
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	// 根据前端指令选择数据源
	var provider feeder.DataProvider
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	} else {
		provider = &feeder.OpenSourceProvider{}
	}

	// 注入对应的驱动引擎
	go feeder.StartSyncFund(provider, codesToSync, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("基本面同步已启动，当前数据源: %s", provider.GetName()),
	})
}

// 💥 K线同步接口 (双擎升级版)
func triggerSyncKlineHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source") // 💥 新增：接收前端的数据源指令
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	// 根据前端指令选择数据源
	var provider feeder.DataProvider
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	} else {
		provider = &feeder.OpenSourceProvider{}
	}

	// 注入对应的驱动引擎
	go feeder.StartSyncKLine(provider, codesToSync, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"msg":  fmt.Sprintf("K 线同步已启动，当前数据源: %s", provider.GetName()),
	})
}

// 赛博控制台日志输出接口 (保持不变)
func getLogsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200,
		"data": feeder.GetLogs(),
	})
}

// 💥 绝对真理钟 (双擎混动版)：支持 Tushare 官方日历与开源交叉缝合
func triggerSyncCalendarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	source := r.URL.Query().Get("source") // 💥 接收前端传来的数据源指令

	go func() {
		currentYear := 1990
		endYear := time.Now().Year() + 1 // 往后推延1年，为未来策略打地基
		totalSaved := 0

		// 💥 混合模式(hybrid)和 Tushare 模式下，日历优先使用官方高权接口
		if source == "tushare" || source == "hybrid" {
			feeder.LogMsg("📅 [真理钟] 启动【Tushare 官方日历】同步引擎...")
			for y := currentYear; y <= endYear; y++ {
				startStr := fmt.Sprintf("%04d0101", y)
				endStr := fmt.Sprintf("%04d1231", y)

				feeder.WaitToken() // 接入漏桶防超速
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
			// 💥 开源降级模式：使用三大指数 K 线交叉推导
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
					time.Sleep(150 * time.Millisecond) // 防东财拦截
				}

				if len(dailyMap) == 0 {
					continue
				}

				var calendars []tushare.TradeCalendar
				for date := range dailyMap {
					calendars = append(calendars, tushare.TradeCalendar{
						CalDate: date,
						IsOpen:  1,
					})
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

// 💥 引擎 C：复权因子同步接口 (双擎升级版)
func triggerSyncAdjHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source") // 💥 接收前端指令

	// 删掉原有的 codesToSync := db.GetAllStockCodes()
	// 替换为：
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	var provider feeder.DataProvider
	if source == "tushare" {
		provider = &feeder.TushareProvider{}
	} else {
		provider = &feeder.OpenSourceProvider{} // 💥 启用东财逆向推导引擎
	}

	go feeder.StartSyncAdjFactors(provider, codesToSync, startDate, endDate)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 200, "msg": fmt.Sprintf("复权因子抽水机已启动！当前火力源: %s", provider.GetName()),
	})
}

func triggerSyncIndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source") // 💥 接收前端指令

	go func() {
		feeder.LogMsg("📊 [引擎D] 大盘指数同步启动...")

		var provider feeder.DataProvider = &feeder.OpenSourceProvider{}
		targetTrust := 50
		if source == "tushare" {
			provider = &feeder.TushareProvider{}
			targetTrust = 100
		}

		// 💥 接入天眼：大盘指数一般只查 000001.SH
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

func main() {

	db.InitDB()
	syncActiveProviderToken("tushare")
	startDynamicLighthouse()
	feeder.InitGlobalEngine(800 * time.Millisecond)
	http.HandleFunc("/api/diagnose", api.DiagnoseHandler)
	http.HandleFunc("/api/start_sync_kline", triggerSyncKlineHandler)
	http.HandleFunc("/api/start_sync_fund", triggerSyncFundHandler)
	http.HandleFunc("/api/logs", getLogsHandler)
	http.HandleFunc("/api/audit", api.AuditHandler) // 👈 注册体检接口
	http.HandleFunc("/api/start_sync_calendar", triggerSyncCalendarHandler)
	http.HandleFunc("/api/start_sync_adj", triggerSyncAdjHandler)
	http.HandleFunc("/api/start_sync_index", triggerSyncIndexHandler)
	http.HandleFunc("/api/set_token", updateTokenHandler)
	http.HandleFunc("/api/tokens", tokenCollectionHandler)
	http.HandleFunc("/api/tokens/activate", tokenActivateHandler)
	http.HandleFunc("/api/start_sync_moneyflow", triggerSyncMoneyFlowHandler)
	http.HandleFunc("/api/start_sync_fina", triggerSyncFinaHandler)
	http.HandleFunc("/api/start_sync_limit", triggerSyncLimitListHandler)
	http.HandleFunc("/api/start_sync_basic", triggerSyncBasicHandler)
	http.HandleFunc("/api/set_speed", updateSpeedHandler) // 💥 注册变速接口
	http.HandleFunc("/api/monitor", api.MonitorHandler)   // 兼容旧入口
	http.HandleFunc("/api/position/risk", api.PositionRiskHandler)
	// 💥 补上缺失的持仓管理三大管线！
	http.HandleFunc("/api/position/add", api.AddPositionHandler)
	http.HandleFunc("/api/position/list", api.GetPositionsHandler)
	http.HandleFunc("/api/position/delete", api.DeletePositionHandler)
	fmt.Println("🟢 工业级全字段量化引擎启动完毕！监听端口: 8081")
	http.ListenAndServe(":8081", nil)
}
