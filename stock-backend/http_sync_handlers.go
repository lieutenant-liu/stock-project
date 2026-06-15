package main

// ============================================================
// http_sync_handlers.go - 数据同步 HTTP 处理器
// ============================================================
// 这个文件包含所有"数据同步"相关的 HTTP 处理函数。
// 每个处理器负责触发一种数据的同步任务。
//
// 【设计模式】
// 所有同步处理器都采用"异步执行"模式：
// 1. 收到请求后立即返回 "已启动" 响应
// 2. 在后台 goroutine 中执行真正的同步逻辑
// 3. 前端可以通过日志接口查看同步进度
//
// 【Go 语言知识点】
// - goroutine: go func() { ... }() 启动一个轻量级协程
// - 协程是并发执行的，不会阻塞主程序
// - 这样用户点击"同步"后不会卡住界面
// ============================================================

import (
	"fmt"           // Go 标准库：格式化输出
	"net/http"      // Go 标准库：HTTP 服务器
	"stock-backend/db"       // 项目内部包：数据库操作
	"stock-backend/feeder"   // 项目内部包：数据采集引擎
	"stock-backend/tushare"  // 项目内部包：Tushare API 客户端
	"strconv"       // Go 标准库：字符串转换（如 "123" → 123）
	"time"          // Go 标准库：时间处理
)

// ------------------------------------------------------------
// 数据源选择辅助函数
// ------------------------------------------------------------

// providerBySource 根据前端参数选择数据源实现。
// 【什么是 Provider？】
// 项目支持多种数据源（Tushare、东方财富等），通过接口统一调用方式。
// 前端传入 source 参数，后端返回对应的数据源实现。
func providerBySource(source string) feeder.DataProvider {
	if source == "tushare" {
		return &feeder.TushareProvider{} // 使用 Tushare 数据源
	}
	return &feeder.OpenSourceProvider{} // 默认使用开源数据源
}

// trustLevelBySource 与 provider 保持一致，用于"高权覆盖低权"策略。
// 【什么是信任等级？】
// 不同数据源的可靠性不同：
// - Tushare: 高质量数据，trust_level = 100
// - 开源数据: 质量较低，trust_level = 50
// 当同一数据有多个来源时，优先使用高信任等级的数据。
func trustLevelBySource(source string) int {
	if source == "tushare" {
		return 100 // Tushare 数据最可靠
	}
	return 50 // 开源数据质量较低
}

// ------------------------------------------------------------
// 同步股票花名册
// ------------------------------------------------------------

// triggerSyncBasicHandler 同步股票基础信息（花名册）。
// 【功能说明】
// 从 Tushare 拉取全市场股票的基本信息：
// - ts_code: 股票代码（如 "000001.SZ"）
// - name: 股票名称（如 "平安银行"）
// - industry: 所属行业（如 "银行"）
// - market: 市场类型（如 "主板"）
// - list_date: 上市日期
//
// 【触发方式】
// GET /api/start_sync_basic
func triggerSyncBasicHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w) // 设置 JSON 响应头 + CORS

	// 使用 goroutine 异步执行，不阻塞请求
	go func() {
		feeder.LogMsg("📜 [基建中心] 正在向 Tushare 索要 A 股最新花名册...")

		// 调用 Tushare API 获取股票花名册
		basics, err := tushare.FetchStockBasic()
		if err != nil {
			feeder.LogMsg("❌ [基建中心] 花名册拉取失败: %v", err)
			return // 拉取失败，直接返回
		}

		// 如果获取到数据，批量写入数据库
		if len(basics) > 0 {
			saved := db.BatchInsertStockBasic(basics)
			feeder.LogMsg("✅ [基建中心] 花名册更新完毕！共入库: %d 只股票", saved)
		}
	}()

	// 立即返回响应，告诉前端"已开始执行"
	respondOKMsg(w, "花名册同步指令已下发！")
}

// ------------------------------------------------------------
// 调整请求速度
// ------------------------------------------------------------

// updateSpeedHandler 动态调整数据采集的请求频率。
// 【为什么需要限流？】
// Tushare API 有调用频率限制，如果请求太快会被封 IP。
// 通过调整 delay（毫秒），控制每次请求之间的间隔。
//
// 【触发方式】
// GET /api/set_speed?speed=800
func updateSpeedHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	// 从 URL 参数获取 speed 值
	speedStr := r.URL.Query().Get("speed")

	// 将字符串转换为整数
	speedMs, err := strconv.Atoi(speedStr)
	if err != nil || speedMs < 0 {
		// 转换失败或值为负数，返回错误
		respondBadRequest(w, "请输入合法的毫秒数(大于等于0)")
		return
	}

	// 更新全局请求延迟
	feeder.SetBaseDelay(speedMs)
	respondOKMsg(w, fmt.Sprintf("引擎请求频率已更新为: %d 毫秒/发", speedMs))
}

// ------------------------------------------------------------
// 同步资金流向
// ------------------------------------------------------------

// triggerSyncMoneyFlowHandler 同步大单资金流向数据。
// 【功能说明】
// 资金流向数据反映主力资金的买卖情况：
// - buy_lg_vol: 大单买入量
// - sell_lg_vol: 大单卖出量
// - buy_elg_vol: 超大单买入量
// - sell_elg_vol: 超大单卖出量
// - net_mf_vol: 净流入量（正=流入，负=流出）
//
// 【触发方式】
// GET /api/start_sync_moneyflow?start=20240101&end=20241231&codes=000001,600519
func triggerSyncMoneyFlowHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	// 从 URL 参数获取日期范围和数据源
	start, end, source := r.URL.Query().Get("start"), r.URL.Query().Get("end"), r.URL.Query().Get("source")
	// 解析股票代码列表（为空则同步全市场）
	codesToSync := parseTargetCodes(r.URL.Query().Get("codes"))

	// 根据数据源类型选择对应的 Provider
	provider := providerBySource(source)
	targetTrust := trustLevelBySource(source)

	// 异步执行同步任务
	go func() {
		feeder.LogMsg("🌊 [采集器E] 资金流向引擎启动！当前源:[%s]", provider.GetName())

		// 遍历所有需要同步的股票
		for i, code := range codesToSync {
			// 智能增量同步：检查数据库中已有数据的范围，只同步缺失的部分
			// 例如：用户请求 2024-01-01 到 2024-12-31，但数据库已有到 2024-06-30
			// 则只同步 2024-07-01 到 2024-12-31
			actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_moneyflow", code, start, end, targetTrust)
			if !needSync {
				continue // 数据已完整，跳过
			}

			feeder.WaitToken() // 限流等待（避免请求过快）

			// 调用数据源 API 获取资金流向数据
			flows, err := provider.FetchMoneyFlow(code, actualStart, actualEnd)
			if err != nil {
				feeder.LogMsg("⚠️ [采集器E %d/%d] %s 报错: %v", i+1, len(codesToSync), code, err)
			} else if len(flows) > 0 {
				// 将数据推入异步写入通道，由 dataSinkWorker 负责写入数据库
				feeder.PushToSink("moneyflow", code, flows)
			}
		}
		feeder.LogMsg("🎉 [采集器E] 资金流向网络拉取完成！")
	}()

	// 立即返回响应
	respondOKMsg(w, "资金流向管线已启动！")
}

// ------------------------------------------------------------
// 同步季报财务指标
// ------------------------------------------------------------

// triggerSyncFinaHandler 同步季报财务指标数据。
// 【功能说明】
// 季报财务指标反映公司的盈利能力：
// - roe: 净资产收益率（越高越好）
// - netprofit_yoy: 净利润同比增长率（正=增长，负=下降）
// - cfps: 每股经营现金流
//
// 【触发方式】
// GET /api/start_sync_fina?start=20240101&end=20241231
func triggerSyncFinaHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)
	start, end := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	codesToSync := parseTargetCodes(r.URL.Query().Get("codes"))

	go func() {
		feeder.LogMsg("🏦 [采集器F] 季报财务引擎启动！[Tushare专属]")

		for i, code := range codesToSync {
			feeder.WaitToken() // 限流等待

			// 调用 Tushare API 获取财务指标
			finas, err := tushare.FetchFinaIndicators(code, start, end)
			if err != nil {
				feeder.LogMsg("⚠️ [采集器F %d/%d] %s 报错: %v", i+1, len(codesToSync), code, err)
			} else if len(finas) > 0 {
				// 推入异步写入通道
				feeder.PushToSink("fina", code, finas)
			}
		}
		feeder.LogMsg("🎉 [采集器F] 季报财务拉取完成！")
	}()

	respondOKMsg(w, "季报财务管线(高权)已启动！")
}

// ------------------------------------------------------------
// 同步涨跌停价格
// ------------------------------------------------------------

// triggerSyncLimitListHandler 同步每日涨跌停价格表。
// 【功能说明】
// 涨跌停价格是策略判断一字涨停/跌停的关键数据：
// - up_limit: 涨停价（当日最高可交易价格）
// - down_limit: 跌停价（当日最低可交易价格）
//
// 【触发方式】
// GET /api/start_sync_limit?start=20240101&end=20241231
func triggerSyncLimitListHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")

	// 根据数据源选择 Provider
	var provider feeder.DataProvider = &feeder.TushareProvider{}
	if source == "eastmoney" {
		provider = &feeder.OpenSourceProvider{}
	}

	// 异步执行同步任务
	go feeder.StartSyncStkLimit(provider, startDate, endDate)
	respondOKMsg(w, "涨跌停管线已启动！系统将自动提取日历空洞并执行后台灌注。")
}

// ------------------------------------------------------------
// 同步基本面数据
// ------------------------------------------------------------

// triggerSyncFundHandler 同步每日基本面数据。
// 【功能说明】
// 基本面数据是估值分析的核心：
// - pe: 市盈率（股价/每股收益）
// - pb: 市净率（股价/每股净资产）
// - total_mv: 总市值（万元）
// - turnover_rate: 换手率（%）
// - dv_ratio: 股息率（%）
//
// 【触发方式】
// GET /api/start_sync_fund?start=20240101&end=20241231&source=tushare
func triggerSyncFundHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	provider := providerBySource(source)
	go feeder.StartSyncFund(provider, codesToSync, startDate, endDate)

	respondOKMsg(w, fmt.Sprintf("基本面同步已启动，当前数据源: %s", provider.GetName()))
}

// ------------------------------------------------------------
// 同步日线行情
// ------------------------------------------------------------

// triggerSyncKlineHandler 同步日线 K 线数据。
// 【功能说明】
// K 线数据是技术分析的基础：
// - open: 开盘价
// - close: 收盘价
// - high: 最高价
// - low: 最低价
// - vol: 成交量
// - amount: 成交额
// - pct_chg: 涨跌幅（%）
//
// 【触发方式】
// GET /api/start_sync_kline?start=20240101&end=20241231&source=tushare
func triggerSyncKlineHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	provider := providerBySource(source)
	go feeder.StartSyncKLine(provider, codesToSync, startDate, endDate)

	respondOKMsg(w, fmt.Sprintf("K 线同步已启动，当前数据源: %s", provider.GetName()))
}

// ------------------------------------------------------------
// 获取日志
// ------------------------------------------------------------

// getLogsHandler 获取系统运行日志。
// 【功能说明】
// 前端通过轮询此接口获取后端的实时日志，展示在页面上。
//
// 【触发方式】
// GET /api/logs
func getLogsHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)
	// 返回日志数组
	respondOK(w, map[string]interface{}{"data": feeder.GetLogs()})
}

// ------------------------------------------------------------
// 同步交易日历
// ------------------------------------------------------------

// triggerSyncCalendarHandler 同步交易日历。
// 【功能说明】
// 交易日历记录了哪些天是交易日（开市），哪些天休市。
// 后续所有同步任务都需要依赖日历判断是否需要拉取数据。
//
// 【支持两种模式】
// 1. tushare: 使用 Tushare 官方日历（最准确）
// 2. hybrid: 使用多指数交易日并集推导（开源方案）
//
// 【触发方式】
// GET /api/start_sync_calendar?source=tushare
func triggerSyncCalendarHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	source := r.URL.Query().Get("source")

	go func() {
		// 按年切片同步，降低单次请求失败对整体任务的影响
		// 例如：2024年日历拉取失败，不影响2023年日历的获取
		currentYear := 1990 // 从1990年开始
		endYear := time.Now().Year() + 1 // 到明年
		totalSaved := 0

		if source == "tushare" || source == "hybrid" {
			// 使用 Tushare 官方日历
			feeder.LogMsg("📅 [真理钟] 启动【Tushare 官方日历】同步引擎...")

			// 按年遍历
			for y := currentYear; y <= endYear; y++ {
				startStr := fmt.Sprintf("%04d0101", y) // 格式化为 "20240101"
				endStr := fmt.Sprintf("%04d1231", y)   // 格式化为 "20241231"

				feeder.WaitToken() // 限流等待

				// 调用 Tushare API 获取当年日历
				calendars, err := tushare.FetchTradeCalendar(startStr, endStr)
				if err != nil {
					feeder.LogMsg("⚠️ [真理钟] %d年 日历拉取失败: %v", y, err)
					continue // 跳过这一年，继续下一年
				}

				if len(calendars) > 0 {
					saved := db.BatchInsertTradeCalendar(calendars)
					totalSaved += saved
				}
			}
			feeder.LogMsg("✅ [真理钟] Tushare 官方日历同步完毕！共覆盖 %d 天记录！", totalSaved)
		} else {
			// 使用开源多源交叉验证日历推导
			feeder.LogMsg("📅 [真理钟] 启动【开源多源交叉验证】日历推导...")
			provider := &feeder.OpenSourceProvider{}

			// 用多指数交易日并集推导开市日
			// 如果上证指数、深证成指、创业板指在某天都有数据，说明那天是交易日
			targetIndices := []string{"000001.SH", "399001.SZ", "399006.SZ"}

			for y := currentYear; y <= endYear; y++ {
				startStr := fmt.Sprintf("%04d0101", y)
				endStr := fmt.Sprintf("%04d1231", y)

				// 用 map 记录哪些天有交易数据
				dailyMap := make(map[string]bool)
				for _, idxCode := range targetIndices {
					klines, err := provider.FetchIndexDaily(idxCode, startStr, endStr)
					if err == nil && len(klines) > 0 {
						for _, k := range klines {
							dailyMap[k.TradeDate] = true // 标记为交易日
						}
					}
					time.Sleep(150 * time.Millisecond) // 避免请求过快
				}

				if len(dailyMap) == 0 {
					continue // 该年无数据，跳过
				}

				// 将 map 中的日期转换为日历格式
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

	respondOKMsg(w, "日历同步指令已下达，引擎正在后台运转，请查看终端日志！")
}

// ------------------------------------------------------------
// 同步复权因子
// ------------------------------------------------------------

// triggerSyncAdjHandler 同步复权因子数据。
// 【功能说明】
// 复权因子用于解决除权除息导致的价格断层：
// - 股票分红后，价格会跳空下跌（除权）
// - 复权因子可以将历史价格调整为"如果没分红，价格应该是多少"
// - 这样技术指标计算才不会被除权干扰
//
// 【触发方式】
// GET /api/start_sync_adj?start=20240101&end=20241231
func triggerSyncAdjHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")
	rawCodes := r.URL.Query().Get("codes")
	codesToSync := parseTargetCodes(rawCodes)

	provider := providerBySource(source)
	go feeder.StartSyncAdjFactors(provider, codesToSync, startDate, endDate)

	respondOKMsg(w, fmt.Sprintf("复权因子采集器已启动！当前数据源: %s", provider.GetName()))
}

// ------------------------------------------------------------
// 同步筹码分布（高级功能）
// ------------------------------------------------------------

// triggerSyncCyqPerfHandler 同步筹码分布数据。
// 【功能说明】
// 筹码分布反映不同价位的持仓成本分布：
// - profit_pct: 获利盘比例（%）
// - cost_50pct: 50% 成本分位（中位数成本）
// - weight_avg: 加权平均成本
//
// 【权限要求】
// 需要 Tushare 5000 积分才能使用此接口。
//
// 【触发方式】
// GET /api/start_sync_cyqperf?start=20240101&end=20241231
func triggerSyncCyqPerfHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	// 检查是否启用了高级数据功能
	sysCfg, err := db.GetSystemConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !sysCfg.EnableProData {
		respondConflict(w, "高级数据功能已关闭，请在系统配置中启用后重试")
		return
	}

	start, end := sanitizeDate(r.URL.Query().Get("start")), sanitizeDate(r.URL.Query().Get("end"))
	codesToSync := parseTargetCodes(r.URL.Query().Get("codes"))

	go feeder.StartSyncCyqPerf(codesToSync, start, end)
	respondOKMsg(w, "筹码分布管线已启动！[Tushare 5000积分专属]")
}

// ------------------------------------------------------------
// 同步技术因子（高级功能）
// ------------------------------------------------------------

// triggerSyncStkFactorProHandler 同步技术因子专业版数据。
// 【功能说明】
// 技术因子包含常用的量化指标：
// - macd, macd_signal, macd_hist: MACD 指标
// - rsi_6, rsi_12: RSI 指标
// - kdj_k, kdj_d, kdj_j: KDJ 指标
// - boll_upper, boll_lower: 布林带
//
// 【权限要求】
// 需要 Tushare 5000 积分才能使用此接口。
//
// 【触发方式】
// GET /api/start_sync_stkfactorpro?start=20240101&end=20241231
func triggerSyncStkFactorProHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	// 检查是否启用了高级数据功能
	sysCfg, err := db.GetSystemConfig()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if !sysCfg.EnableProData {
		respondConflict(w, "高级数据功能已关闭，请在系统配置中启用后重试")
		return
	}

	start, end := sanitizeDate(r.URL.Query().Get("start")), sanitizeDate(r.URL.Query().Get("end"))
	codesToSync := parseTargetCodes(r.URL.Query().Get("codes"))

	go feeder.StartSyncStkFactorPro(codesToSync, start, end)
	respondOKMsg(w, "技术因子专业版管线已启动！[Tushare 5000积分专属]")
}

// ------------------------------------------------------------
// 同步大盘指数
// ------------------------------------------------------------

// triggerSyncIndexHandler 同步大盘指数数据。
// 【功能说明】
// 大盘指数（如上证指数 000001.SH）用于：
// - 判断市场整体趋势（牛市/熊市）
// - 策略中的大盘风控（暴跌时暂停买入）
//
// 【触发方式】
// GET /api/start_sync_index?start=20240101&end=20241231
func triggerSyncIndexHandler(w http.ResponseWriter, r *http.Request) {
	preparePublicJSON(w)

	startDate := sanitizeDate(r.URL.Query().Get("start"))
	endDate := sanitizeDate(r.URL.Query().Get("end"))
	source := r.URL.Query().Get("source")

	go func() {
		feeder.LogMsg("📊 [引擎D] 大盘指数同步启动...")

		provider := providerBySource(source)
		targetTrust := trustLevelBySource(source)

		// 智能增量同步：只同步缺失的部分
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

	respondOKMsg(w, "大盘指数采集器已启动！")
}
