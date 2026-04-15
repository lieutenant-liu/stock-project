# 项目目录结构

```text
├── api
│   └── handlers.go
├── db
│   ├── database.go
│   ├── ops.go
│   └── position.go
├── eastmoney
│   └── client.go
├── feeder
│   ├── engine.go
│   └── sync.go
├── go.mod
├── go.sum
├── main.go
├── stocks.db
├── stocks.db-shm
├── stocks.db-wal
├── strategy
│   ├── indicators.go
│   ├── models.go
│   ├── monitor.go
│   └── rules.go
└── tushare
    └── client.go
```

# 源代码

## File: api/handlers.go

```go
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

	var pos db.Position
	if err := json.NewDecoder(r.Body).Decode(&pos); err != nil {
		json.NewEncoder(w).Encode(APIResponse{Code: 400, Msg: "参数解析失败"})
		return
	}

	if err := db.AddPosition(pos); err != nil {
		json.NewEncoder(w).Encode(APIResponse{Code: 500, Msg: "录入失败"})
		return
	}
	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "持仓防线已部署"})
}

// GetPositionsHandler 供前端调用：展示当前持仓列表
func GetPositionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	positions, _ := db.GetAllPositions()
	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "success", Data: positions})
}

// DeletePositionHandler 供前端调用：手动移出持仓
func DeletePositionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var req struct {
		ID int `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	db.DeletePosition(req.ID)
	json.NewEncoder(w).Encode(APIResponse{Code: 200, Msg: "仓位已清除"})
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

```

## File: db/database.go

```go
package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB() {
	var err error

	DB, err = sql.Open("sqlite", "stocks.db")
	if err != nil {
		log.Fatal("❌ 连接数据库失败: ", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("❌ 数据库通信失败: ", err)
	}
	fmt.Println("🗄️ [数据中心] SQLite 数据库连接成功！")

	// ========================================================
	// 💥 终极并发优化：Termux 防 OOM 与 WAL 极限调优
	// ========================================================
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",    // 读写不阻塞
		"PRAGMA synchronous = NORMAL;",  // 提升 WAL 写入速度
		"PRAGMA cache_size = -10000;",   // 强制限制缓存为 10MB，绝对防御 OOM
		"PRAGMA temp_store = FILE;",     // 放弃 MEMORY 临时表，用 I/O 换取内存安全
		"PRAGMA mmap_size = 268435456;", // 限制 mmap 最大为 256MB
		"PRAGMA busy_timeout = 5000;",   // 应对并发锁争用，超时等待 5 秒
	}

	for _, p := range pragmas {
		if _, err := DB.Exec(p); err != nil {
			log.Fatalf("⚠️ [警告] PRAGMA 注入失败 (%s): %v", p, err)
		}
	}
	fmt.Println("⚡ [数据中心] 防 OOM 屏障已激活，WAL 高并发读写模式就绪！")

	// ========================================================
	// 📊 V2.0 工业级 6 大核心表 (全部剔除自增 ID，启用 WITHOUT ROWID)
	// ========================================================

	// 1. 交易日历 (防空洞检测器)
	createCalTable := `
	CREATE TABLE IF NOT EXISTS trade_calendar (
		cal_date TEXT PRIMARY KEY,
		is_open INTEGER NOT NULL
	) WITHOUT ROWID;`

	// 2. 股票花名册
	createBasicTable := `
	CREATE TABLE IF NOT EXISTS stock_basic (
		ts_code TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		industry TEXT,
		market TEXT,
		list_date TEXT
	) WITHOUT ROWID;`

	// 3. 原始日线量价 (纯净版：剥离均线计算，交由内存或后续视图处理)
	// 3. 原始日线量价 (纯净版)
	createKlineTable := `
	CREATE TABLE IF NOT EXISTS daily_klines (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		open REAL,
		close REAL,
		high REAL,
		low REAL,
		vol REAL,
		amount REAL,
		pre_close REAL,              -- 💥 修复：补回昨收
		change REAL,                 -- 💥 修复：补回涨跌额
		pct_chg REAL,
		data_source TEXT NOT NULL,   -- 血缘标记
		trust_level INTEGER NOT NULL,-- 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_kline_date ON daily_klines(trade_date);`

	// 4. 复权因子表 (💥 V2.0 新核心：解决除权除息导致的断层)
	createAdjTable := `
	CREATE TABLE IF NOT EXISTS adj_factors (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		adj_factor REAL NOT NULL,
		data_source TEXT NOT NULL,   -- 💥 血缘追踪
		trust_level INTEGER NOT NULL,-- 💥 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 5. 每日基本面表 (情绪与估值)
	createFundTable := `
	CREATE TABLE IF NOT EXISTS daily_fundamentals (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		pe REAL,
		pb REAL,
		total_mv REAL,
		dv_ratio REAL,
		turnover_rate REAL,
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 6. 季报财务表 (第一层基本面漏斗)
	createFinaTable := `
	CREATE TABLE IF NOT EXISTS fina_indicators (
		ts_code TEXT NOT NULL,
		end_date TEXT NOT NULL,      
		ann_date TEXT NOT NULL,      
		update_flag TEXT,            
		roe REAL,                    
		netprofit_yoy REAL,          
		cfps REAL,                   
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, end_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_fina_ann_date ON fina_indicators(ann_date);`

	// 7. 大单资金流向表 (透视主力底牌)
	createMoneyFlowTable := `
	CREATE TABLE IF NOT EXISTS daily_moneyflow (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		buy_lg_vol REAL,
		sell_lg_vol REAL,
		buy_elg_vol REAL,
		sell_elg_vol REAL,
		net_mf_vol REAL,
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 8. 每日涨跌停价格表 (V4.0 降维特化版)
	createStkLimitTable := `
	CREATE TABLE IF NOT EXISTS daily_stk_limit (
		trade_date TEXT NOT NULL,
		ts_code TEXT NOT NULL,
		up_limit REAL,
		down_limit REAL,
		data_source TEXT NOT NULL,
		trust_level INTEGER NOT NULL,
		PRIMARY KEY (trade_date, ts_code)
	) WITHOUT ROWID;`

	// 9. 大盘指数表 (系统风控雷达)
	createIndexTable := `
	CREATE TABLE IF NOT EXISTS index_daily (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		close REAL NOT NULL,
		vol REAL NOT NULL,
		pct_chg REAL NOT NULL,
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`
	// 10. 专属持仓表 (极简设计)
	createPositionTable := `
	CREATE TABLE IF NOT EXISTS my_positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts_code TEXT NOT NULL,
		stock_name TEXT NOT NULL,
		hold_volume INTEGER NOT NULL,
		cost_price REAL NOT NULL,
		buy_date TEXT NOT NULL
	);`

	// 💥 黎明扫荡：物理销毁旧时代的打卡本！
	DB.Exec(`DROP TABLE IF EXISTS sync_history;`)
	DB.Exec(`DROP TABLE IF EXISTS sync_history_fund;`)
	DB.Exec(`DROP TABLE IF EXISTS daily_limit_list;`) // 👈 炸毁旧的高阶表

	// 💥 3. 将 createLimitListTable 放回执行队列，去掉 createStkLimitTable
	tables := []string{
		createCalTable, createBasicTable, createKlineTable,
		createAdjTable, createFundTable, createFinaTable,
		createMoneyFlowTable, createStkLimitTable, createIndexTable, createPositionTable, // 👈 替换为新表
	}
	for _, sqlStr := range tables {
		if _, err = DB.Exec(sqlStr); err != nil {
			log.Fatal("❌ 创建 V2 数据表失败: ", err, "\nSQL:", sqlStr)
		}
	}
	// ... 上面是原有的批量建表 for 循环 ...

	// 💥 修复：释放 WAL 读写并发能力
	DB.SetMaxOpenConns(4) // 允许 4 个并发连接 (1个给后台静默写入，3个给前端雷达查询)
	DB.SetMaxIdleConns(2) // 保持适度的长连接池，防止 Termux 频繁创建/销毁套接字资源耗尽

	fmt.Println("🔥 [黎明扫荡] 旧时代打卡本已被焚毁，进入精确对账时代！")
	fmt.Println("🗄️ [数据中心] 究极形态：V2.0 六大核心数据表部署完毕！")
}

```

## File: db/ops.go

```go
package db

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"stock-backend/tushare"
	"time"
)

// 💥 升级版存入：直接接收 tushare.DailyKLine 切片，存入 11 个字段
func BatchInsertKLines(tsCode string, klines []tushare.DailyKLine) int {
	if len(klines) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启事务失败:", err)
		return 0
	}

	// =========================================================================================
	// 🛡️ 智能拦截 SQL 释义：
	// 当尝试插入 (ts_code, trade_date) 发生冲突（即库里已经有这一天的数据时）：
	// DO UPDATE SET ... 触发更新。
	// 但是！最后有一个绝对护盾：WHERE excluded.trust_level >= daily_klines.trust_level
	// 这意味着：只有当你新塞进来的数据(excluded)的权重，大于等于库里老数据的权重时，覆盖才会真正执行！
	// =========================================================================================
	stmt, err := tx.Prepare(`
		INSERT INTO daily_klines 
		(ts_code, trade_date, open, high, low, close, pre_close, change, pct_chg, vol, amount, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET 
			open = excluded.open,
			high = excluded.high,
			low = excluded.low,
			close = excluded.close,
			pre_close = excluded.pre_close,
			change = excluded.change,
			pct_chg = excluded.pct_chg,
			vol = excluded.vol,
			amount = excluded.amount,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= daily_klines.trust_level
	`)

	if err != nil {
		log.Println("预编译带血缘的SQL失败:", err)
		tx.Rollback()
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, k := range klines {
		res, err := stmt.Exec(
			tsCode, k.TradeDate, k.Open, k.High, k.Low, k.Close,
			k.PreClose, k.Change, k.PctChg, k.Vol, k.Amount,
			k.DataSource, k.TrustLevel, // 💥 注入血缘与权重
		)
		if err == nil {
			rows, _ := res.RowsAffected()
			// RowsAffected 为 1 代表插入成功，为 2 代表更新成功，为 0 代表被 WHERE 条件拦截了！
			if rows > 0 {
				insertCount++
			}
		}
	}

	tx.Commit()
	return insertCount // 返回真正被插入或覆盖的有效行数
}

// 💥 升级版读取：把 11 个字段全部提出来给军师部和前端
func GetKLinesFromDB(tsCode string, startDate string, endDate string) []tushare.DailyKLine {
	var klines []tushare.DailyKLine
	query := `
		SELECT trade_date, open, high, low, close, pre_close, change, pct_chg, vol, amount
		FROM daily_klines 
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		log.Println("读取数据库失败:", err)
		return klines
	}
	defer rows.Close()

	for rows.Next() {
		var k tushare.DailyKLine
		k.TSCode = tsCode // 补齐代码
		err := rows.Scan(&k.TradeDate, &k.Open, &k.High, &k.Low, &k.Close, &k.PreClose, &k.Change, &k.PctChg, &k.Vol, &k.Amount)
		if err == nil {
			klines = append(klines, k)
		}
	}
	return klines
}

// BatchInsertStockBasic 批量保存全市场花名册 (已还原为安全版本)
func BatchInsertStockBasic(basics []tushare.StockBasicInfo) int {
	if len(basics) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启事务失败:", err)
		return 0
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO stock_basic (ts_code, name, industry, market, list_date) 
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Println("预编译花名册SQL失败:", err)
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, b := range basics {
		res, err := stmt.Exec(b.TSCode, b.Name, b.Industry, b.Market, b.ListDate)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}
	tx.Commit()
	return insertCount
}

// GetAllStockCodes 从数据库中提取所有股票代码，供抽水机使用
func GetAllStockCodes() []string {
	var codes []string

	query := `
			SELECT ts_code FROM stock_basic
			WHERE list_date <= strftime('%Y%m%d', date('now','-180 day'))
			ORDER BY ts_code ASC
		`
	// query := `
	// 		SELECT ts_code FROM stock_basic
	// 		WHERE  list_date < '20230101'
	// 		ORDER BY ts_code ASC
	// 	`
	rows, err := DB.Query(query)
	if err != nil {
		log.Println("提取股票代码失败:", err)
		return codes
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err == nil {
			codes = append(codes, code)
		}
	}
	return codes
}

// ==========================================
// 💎 基本面数据库操作
// ==========================================
// BatchInsertFundamentals 带有血缘权重的智能基本面入库
func BatchInsertFundamentals(tsCode string, fundamentals []tushare.DailyFundamental) int {
	if len(fundamentals) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启基本面事务失败:", err)
		return 0
	}

	// 💥 真正的基本面权重拦截器装载在这里
	stmt, err := tx.Prepare(`
		INSERT INTO daily_fundamentals 
		(ts_code, trade_date, pe, pb, total_mv, dv_ratio, turnover_rate, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET 
			pe = excluded.pe,
			pb = excluded.pb,
			total_mv = excluded.total_mv,
			dv_ratio = excluded.dv_ratio,
			turnover_rate = excluded.turnover_rate,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= daily_fundamentals.trust_level
	`)
	if err != nil {
		log.Println("预编译基本面带血缘SQL失败:", err)
		tx.Rollback()
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, f := range fundamentals {
		res, err := stmt.Exec(
			tsCode, f.TradeDate, f.PE, f.PB, f.TotalMV, f.DVRatio, f.TurnoverRate,
			f.DataSource, f.TrustLevel, // 💥 注入血缘参数
		)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}

	tx.Commit()
	return insertCount
}

// ==========================================
// 💥 V2.0 核心阵地：三大新兵种入库通道
// ==========================================

// BatchInsertTradeCalendar 批量灌注交易日历 (全局唯一真理钟)
func BatchInsertTradeCalendar(calendars []tushare.TradeCalendar) int {
	if len(calendars) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启日历事务失败:", err)
		return 0
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO trade_calendar (cal_date, is_open) VALUES (?, ?)`)
	if err != nil {
		log.Println("预编译日历SQL失败:", err)
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, c := range calendars {
		res, err := stmt.Exec(c.CalDate, c.IsOpen)
		if err == nil {
			rows, _ := res.RowsAffected()
			insertCount += int(rows)
		}
	}

	tx.Commit()
	return insertCount
}

// BatchInsertAdjFactors 批量灌注复权因子 (解决回测断层的核武器)
func BatchInsertAdjFactors(tsCode string, factors []tushare.AdjFactor) int {
	if len(factors) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启复权因子事务失败:", err)
		return 0
	}

	stmt, err := tx.Prepare(`
		INSERT INTO adj_factors (ts_code, trade_date, adj_factor, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET 
			adj_factor = excluded.adj_factor,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= adj_factors.trust_level
	`)
	if err != nil {
		log.Println("预编译复权因子SQL失败:", err)
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, f := range factors {
		res, err := stmt.Exec(f.TSCode, f.TradeDate, f.AdjFactor, f.DataSource, f.TrustLevel)
		if err == nil {
			rows, _ := res.RowsAffected()
			insertCount += int(rows)
		}
	}

	tx.Commit()
	return insertCount
}

// BatchInsertFinaIndicators 批量灌注季报财务 (第一层基本面漏斗弹药)
func BatchInsertFinaIndicators(tsCode string, indicators []tushare.FinaIndicator) int {
	if len(indicators) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	// 注意主键是 (ts_code, end_date)
	stmt, _ := tx.Prepare(`
		INSERT INTO fina_indicators (ts_code, end_date, ann_date, update_flag, roe, netprofit_yoy, cfps, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, end_date) DO UPDATE SET 
			ann_date = excluded.ann_date,
			update_flag = excluded.update_flag,
			roe = excluded.roe,
			netprofit_yoy = excluded.netprofit_yoy,
			cfps = excluded.cfps,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= fina_indicators.trust_level
	`)
	if err != nil {
		log.Println("预编译带血缘的SQL失败:", err)
		tx.Rollback()
		return 0
	}
	defer stmt.Close()
	insertCount := 0
	for _, ind := range indicators {
		res, err := stmt.Exec(ind.TSCode, ind.EndDate, ind.AnnDate, ind.UpdateFlag, ind.ROE, ind.NetProfitYOY, ind.CFPS, ind.DataSource, ind.TrustLevel)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}
	tx.Commit()
	return insertCount
}

// ==========================================
// 🏥 终极系统：多维数据血缘与对账中心 (Auditor V3.0)
// ==========================================

// TableHealth 单张表的健康报告
type TableHealth struct {
	DisplayName  string         `json:"display_name"`
	ExpectedDays int            `json:"expected_days"`
	ActualDays   int            `json:"actual_days"`
	Completeness float64        `json:"completeness"`
	MissingDates []string       `json:"missing_dates"`
	SourceCount  map[string]int `json:"source_count"`
}

// AuditReport 整体体检报告矩阵
type AuditReport struct {
	TSCode  string                 `json:"ts_code"`
	Reports map[string]TableHealth `json:"reports"`
}

// RunDataAudit 执行严格的多维数据对账
func RunDataAudit(tsCode, startDate, endDate string) AuditReport {
	report := AuditReport{
		TSCode:  tsCode,
		Reports: make(map[string]TableHealth),
	}

	// 1. 查绝对真理：预期开市天数 (全局共用)
	var expectedDays int
	DB.QueryRow(`
		SELECT COUNT(cal_date) FROM trade_calendar 
		WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?
	`, startDate, endDate).Scan(&expectedDays)

	// 💥 2. 定义需要联合透视的四大核心兵种表
	tables := map[string]string{
		"daily_klines":       "📈 日线量价",
		"daily_fundamentals": "💎 基本面估值",
		"adj_factors":        "🧬 复权因子",
		"daily_moneyflow":    "🌊 主力资金流",
	}

	// 3. 循环扫荡每张表
	for tableName, displayName := range tables {
		health := TableHealth{
			DisplayName:  displayName,
			ExpectedDays: expectedDays,
			MissingDates: []string{},
			SourceCount:  make(map[string]int),
		}

		// 查实际库存
		queryActual := fmt.Sprintf(`SELECT COUNT(trade_date) FROM %s WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?`, tableName)
		DB.QueryRow(queryActual, tsCode, startDate, endDate).Scan(&health.ActualDays)

		if expectedDays > 0 {
			health.Completeness = float64(health.ActualDays) / float64(expectedDays) * 100
		}

		// 查空洞 (限制最多返回10条，防止前端 UI 撑爆内存)
		queryMissing := fmt.Sprintf(`
			SELECT cal_date FROM trade_calendar 
			WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?
			AND cal_date NOT IN (
				SELECT trade_date FROM %s WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
			) ORDER BY cal_date ASC LIMIT 10
		`, tableName)

		rows, err := DB.Query(queryMissing, startDate, endDate, tsCode, startDate, endDate)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var missingDate string
				if rows.Scan(&missingDate) == nil {
					health.MissingDates = append(health.MissingDates, missingDate)
				}
			}
		}

		// 查血缘占比
		querySource := fmt.Sprintf(`SELECT data_source, COUNT(*) FROM %s WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ? GROUP BY data_source`, tableName)
		srcRows, err := DB.Query(querySource, tsCode, startDate, endDate)
		if err == nil {
			defer srcRows.Close()
			for srcRows.Next() {
				var source string
				var count int
				if srcRows.Scan(&source, &count) == nil {
					health.SourceCount[source] = count
				}
			}
		}

		report.Reports[tableName] = health
	}

	return report
}

// ==========================================
// 🎯 黎明扫荡：状态驱动的精确填缝算法
// ==========================================

// ==========================================
// 🎯 黎明扫荡：通用状态驱动的精确填缝与【权重升级】算法
// ==========================================

// GetDailySyncTaskRange 核心逻辑：传入表名，动态探测空洞与低权数据
func GetDailySyncTaskRange(tableName, tsCode, targetStart, targetEnd string, targetTrust int) (string, string, bool) {
	// ==========================================
	// 💥 史前空洞拦截器：坚决不查上市之前的数据！
	// ==========================================
	var listDate string
	errList := DB.QueryRow(`SELECT list_date FROM stock_basic WHERE ts_code = ?`, tsCode).Scan(&listDate)
	if errList == nil && listDate != "" {
		if targetStart < listDate {
			targetStart = listDate // 如果要求的起点早于上市日，强制把起点推延到上市那一天！
		}
		if targetStart > targetEnd {
			return "", "", false // 极端情况：连结束时间都没上市，直接跳过
		}
	}
	// ==========================================

	var expected int
	DB.QueryRow(`SELECT COUNT(cal_date) FROM trade_calendar WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?`, targetStart, targetEnd).Scan(&expected)

	if expected == 0 {
		return "", "", false
	}

	var minDate, maxDate sql.NullString
	// 泛型 SQL 动态注入表名
	query := fmt.Sprintf(`
		SELECT MIN(cal_date), MAX(cal_date) FROM trade_calendar 
		WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?
		AND cal_date NOT IN (
			SELECT trade_date FROM %s 
			WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ? AND trust_level >= ?
		)
	`, tableName)

	err := DB.QueryRow(query, targetStart, targetEnd, tsCode, targetStart, targetEnd, targetTrust).Scan(&minDate, &maxDate)

	if err == nil && minDate.Valid && maxDate.Valid {
		return minDate.String, maxDate.String, true // 发现真实空洞，返回精准作战区间！
	}

	return "", "", false
}

// ==========================================
// 💥 高阶围猎数据入库通道
// ==========================================

// BatchInsertMoneyFlow 资金流向入库 (带血缘护盾)
func BatchInsertMoneyFlow(tsCode string, flows []tushare.DailyMoneyFlow) int {
	if len(flows) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	stmt, _ := tx.Prepare(`
		INSERT INTO daily_moneyflow (ts_code, trade_date, buy_lg_vol, sell_lg_vol, buy_elg_vol, sell_elg_vol, net_mf_vol, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET 
			buy_lg_vol = excluded.buy_lg_vol,
			sell_lg_vol = excluded.sell_lg_vol,
			buy_elg_vol = excluded.buy_elg_vol,
			sell_elg_vol = excluded.sell_elg_vol,
			net_mf_vol = excluded.net_mf_vol,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= daily_moneyflow.trust_level
	`)
	if err != nil {
		log.Printf("⚠️ 致命错误: SQL 预编译失败 (请检查表结构是否已更新): %v\n", err)
		tx.Rollback()
		return 0 // 拦截！绝不往下执行导致 Panic
	}
	defer stmt.Close()
	insertCount := 0
	for _, f := range flows {
		res, err := stmt.Exec(f.TSCode, f.TradeDate, f.BuyLgVol, f.SellLgVol, f.BuyElgVol, f.SellElgVol, f.NetMfVol, f.DataSource, f.TrustLevel)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}
	tx.Commit()
	return insertCount
}

// BatchInsertIndexDaily 大盘指数入库 (带血缘护盾)
func BatchInsertIndexDaily(tsCode string, indices []tushare.IndexDaily) int {
	if len(indices) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	stmt, _ := tx.Prepare(`
		INSERT INTO index_daily (ts_code, trade_date, close, vol, pct_chg, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET 
			close = excluded.close,
			vol = excluded.vol,
			pct_chg = excluded.pct_chg,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= index_daily.trust_level
	`)
	if err != nil {
		log.Printf("⚠️ 致命错误: SQL 预编译失败 (请检查表结构是否已更新): %v\n", err)
		tx.Rollback()
		return 0 // 拦截！绝不往下执行导致 Panic
	}
	defer stmt.Close()
	insertCount := 0
	for _, idx := range indices {
		res, err := stmt.Exec(idx.TSCode, idx.TradeDate, idx.Close, idx.Vol, idx.PctChg, idx.DataSource, idx.TrustLevel)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}
	tx.Commit()
	return insertCount
}

// GetFundamentalsFromDB 从本地 SQLite 提取基本面数据供策略引擎使用
func GetFundamentalsFromDB(tsCode string, startDate string, endDate string) []tushare.DailyFundamental {
	var funds []tushare.DailyFundamental
	query := `
		SELECT trade_date, pe, pb, total_mv, dv_ratio, turnover_rate
		FROM daily_fundamentals 
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		return funds // 遇到错误直接返回空，让策略层自己决定是否降级
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.DailyFundamental
		f.TSCode = tsCode
		// 动态扫表赋值
		err := rows.Scan(&f.TradeDate, &f.PE, &f.PB, &f.TotalMV, &f.DVRatio, &f.TurnoverRate)
		if err == nil {
			funds = append(funds, f)
		}
	}
	return funds
}

// GetMoneyFlowFromDB 从本地 SQLite 提取主力资金流向数据
func GetMoneyFlowFromDB(tsCode string, startDate string, endDate string) []tushare.DailyMoneyFlow {
	var flows []tushare.DailyMoneyFlow
	query := `
		SELECT trade_date, buy_lg_vol, sell_lg_vol, buy_elg_vol, sell_elg_vol, net_mf_vol
		FROM daily_moneyflow 
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		return flows
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.DailyMoneyFlow
		f.TSCode = tsCode
		err := rows.Scan(&f.TradeDate, &f.BuyLgVol, &f.SellLgVol, &f.BuyElgVol, &f.SellElgVol, &f.NetMfVol)
		if err == nil {
			flows = append(flows, f)
		}
	}
	return flows
}

// GetIndexDailyFromDB 提取大盘指数数据 (用于全局风控)
func GetIndexDailyFromDB(tsCode string, startDate string, endDate string) []tushare.IndexDaily {
	var indices []tushare.IndexDaily
	query := `
		SELECT trade_date, close, vol, pct_chg 
		FROM index_daily 
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		return indices
	}
	defer rows.Close()

	for rows.Next() {
		var idx tushare.IndexDaily
		idx.TSCode = tsCode
		if err := rows.Scan(&idx.TradeDate, &idx.Close, &idx.Vol, &idx.PctChg); err == nil {
			indices = append(indices, idx)
		}
	}
	return indices
}

// GetAdjFactorsFromDB 从数据库提取复权因子
func GetAdjFactorsFromDB(tsCode string, startDate string, endDate string) []tushare.AdjFactor {
	var factors []tushare.AdjFactor
	query := `
		SELECT trade_date, adj_factor
		FROM adj_factors 
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		return factors
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.AdjFactor
		f.TSCode = tsCode
		if err := rows.Scan(&f.TradeDate, &f.AdjFactor); err == nil {
			factors = append(factors, f)
		}
	}
	return factors
}

// GetPEPercentile 💥 动态估值引擎：计算个股历史 PE 分位数 (0.0 ~ 1.0)
func GetPEPercentile(tsCode string, endDate string, lookbackDays int) float64 {
	// 提取过去 lookbackDays 天的有效 PE 数据 (剔除亏损的负PE)
	query := `
		SELECT pe 
		FROM daily_fundamentals 
		WHERE ts_code = ? AND trade_date <= ? AND pe > 0
		ORDER BY trade_date DESC LIMIT ?
	`
	rows, err := DB.Query(query, tsCode, endDate, lookbackDays)
	if err != nil {
		return 0.5 // 数据库异常时，默认返回 50% 中位数，防阻断
	}
	defer rows.Close()

	var peList []float64
	var currentPE float64
	isFirst := true

	for rows.Next() {
		var pe float64
		if err := rows.Scan(&pe); err == nil {
			if isFirst {
				currentPE = pe
				isFirst = false // 最新的一天作为当前比较基准
			}
			peList = append(peList, pe)
		}
	}

	if len(peList) < 100 {
		// 如果有效历史数据少于 100 天，说明是次新股或长期亏损刚扭亏
		return 0.5 // 样本不足，不提供极端参考
	}

	// 升序排序：从小到大
	sort.Float64s(peList)

	// 查找当前 PE 在历史中的排位
	rank := 0
	for i, pe := range peList {
		if currentPE <= pe {
			rank = i
			break
		}
	}

	// 返回排位百分比：比如 0.15 表示当前估值低于历史 85% 的时间，处于绝对低估区
	return float64(rank) / float64(len(peList))
}

// BatchInsertStkLimit 涨跌停绝对价格入库 (带血缘护盾，纯 Go SQLite 兼容)
func BatchInsertStkLimit(limits []tushare.StkLimit) int {
	if len(limits) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	stmt, _ := tx.Prepare(`
		INSERT INTO daily_stk_limit (trade_date, ts_code, up_limit, down_limit, data_source, trust_level) 
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(trade_date, ts_code) DO UPDATE SET 
			up_limit = excluded.up_limit,
			down_limit = excluded.down_limit,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= daily_stk_limit.trust_level
	`)
	if err != nil {
		return 0
	}
	defer stmt.Close()
	insertCount := 0
	for _, l := range limits {
		res, err := stmt.Exec(l.TradeDate, l.TSCode, l.UpLimit, l.DownLimit, l.DataSource, l.TrustLevel)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}
	tx.Commit()
	return insertCount
}

// GetLimitUpPremium 💥 情绪温度计 V2.0：通过 K 线收盘价与涨停价的联合透视计算溢价率
func GetLimitUpPremium(yesterday string, today string) (int, float64) {
	// 逻辑：昨天的 K线收盘价 (k1.close) 大于等于 昨天的涨停价 (l1.up_limit)，判定为昨日涨停。
	// 然后看它们今天 (k2) 的 pct_chg (涨跌幅) 的平均值。
	query := `
		SELECT COUNT(k1.ts_code), AVG(k2.pct_chg)
		FROM daily_klines k1
		JOIN daily_stk_limit l1 ON k1.ts_code = l1.ts_code AND k1.trade_date = l1.trade_date
		JOIN daily_klines k2 ON k1.ts_code = k2.ts_code
		WHERE k1.trade_date = ? 
		  AND l1.trade_date = ?
		  AND k2.trade_date = ? 
		  AND k1.close >= l1.up_limit
	`
	var count int
	var avgPremium sql.NullFloat64

	// 注意：这里需要传入 昨天、昨天、今天 三个日期参数
	err := DB.QueryRow(query, yesterday, yesterday, today).Scan(&count, &avgPremium)
	if err != nil || count == 0 || !avgPremium.Valid {
		return 0, 0.0 // 容错兜底
	}

	return count, avgPremium.Float64
}

// ==========================================
// 💥 全市场级数据空洞探测器 (适用于涨跌停榜、大盘指数等不区分特定股票的表)
// ==========================================

// GetMarketMissingDates 智能提取全市场级表的缺失日期列表
func GetMarketMissingDates(tableName, targetStart, targetEnd string) []string {
	// 1. 如果没传起点，不再 hardcode 19901219，而是从日历表里拿【A股开市第一天】
	if targetStart == "" {
		DB.QueryRow(`SELECT MIN(cal_date) FROM trade_calendar WHERE is_open = 1`).Scan(&targetStart)
	}
	// 2. 如果没传终点，默认为今天
	if targetEnd == "" {
		targetEnd = time.Now().Format("20060102")
	}

	// 3. 防呆安全锁
	if targetStart > targetEnd {
		targetStart, targetEnd = targetEnd, targetStart
	}

	// 4. 精确填缝：只找出这段时间里，日历显示开门，但目标表里没数据的日子
	query := fmt.Sprintf(`
		SELECT cal_date FROM trade_calendar 
		WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?
		AND cal_date NOT IN (
			SELECT DISTINCT trade_date FROM %s 
			WHERE trade_date >= ? AND trade_date <= ?
		) ORDER BY cal_date ASC
	`, tableName)

	var missingDates []string
	rows, err := DB.Query(query, targetStart, targetEnd, targetStart, targetEnd)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil {
				missingDates = append(missingDates, d)
			}
		}
	}
	return missingDates
}

```

## File: db/position.go

```go
package db

import (
	"log"
)

// Position 机构级持仓数据舱 (极简无状态版)
type Position struct {
	ID         int     `json:"id"`
	TSCode     string  `json:"ts_code"`
	StockName  string  `json:"stock_name"`
	HoldVolume int     `json:"hold_volume"` // 持仓股数
	CostPrice  float64 `json:"cost_price"`  // 建仓成本价
	BuyDate    string  `json:"buy_date"`    // 建仓日期 (YYYYMMDD)
}

// AddPosition 录入新兵 (前端买入后调用)
func AddPosition(pos Position) error {
	query := `
		INSERT INTO my_positions (ts_code, stock_name, hold_volume, cost_price, buy_date)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := DB.Exec(query, pos.TSCode, pos.StockName, pos.HoldVolume, pos.CostPrice, pos.BuyDate)
	if err != nil {
		log.Printf("❌ 写入持仓失败 [%s]: %v\n", pos.TSCode, err)
		return err
	}
	return nil
}

// GetAllPositions 提取全量持仓，交由【机械侧刀】审判
func GetAllPositions() ([]Position, error) {
	var positions []Position
	query := `SELECT id, ts_code, stock_name, hold_volume, cost_price, buy_date FROM my_positions ORDER BY id DESC`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.TSCode, &p.StockName, &p.HoldVolume, &p.CostPrice, &p.BuyDate); err == nil {
			positions = append(positions, p)
		}
	}
	return positions, nil
}

// DeletePosition 机械侧刀斩首后，将尸体移出持仓列表
func DeletePosition(id int) error {
	_, err := DB.Exec(`DELETE FROM my_positions WHERE id = ?`, id)
	return err
}

```

## File: eastmoney/client.go

```go
package eastmoney

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"stock-backend/tushare"
	"strconv"
	"strings"
	"time"
)

// FetchStockHistory 💥 责任链总控：东财(主力) -> 腾讯(灾备)
func FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	fmt.Printf("🕵️ [责任链] 节点 1: 尝试通过【东方财富 (Trust:50)】拉取 %s...\n", tsCode)
	klines, err := fetchFromEastMoney(tsCode, startDate, endDate)

	if err == nil && len(klines) > 0 {
		return klines, nil // 节点 1 成功，直接返回
	}

	fmt.Printf("⚠️ [责任链] 节点 1 阵亡 (%v)。触发降级，节点 2:【腾讯财经 (Trust:30)】接管...\n", err)
	return fetchFromTencent(tsCode, startDate, endDate)
}

// ==========================================
// 🛡️ 节点 1：东方财富 (全字段主力，TrustLevel: 50)
// ==========================================
func fetchFromEastMoney(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 强制走 HTTP 直连防代理 EOF
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=0&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true // 禁用 Keep-Alive，防服务端掐断
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	if err := json.Unmarshal(body, &emResp); err != nil {
		return nil, err
	}

	if emResp.Data.Klines == nil {
		return []tushare.DailyKLine{}, nil
	}

	var klines []tushare.DailyKLine
	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 11 {
			continue
		}

		open, _ := strconv.ParseFloat(fields[1], 64)
		closePrice, _ := strconv.ParseFloat(fields[2], 64)
		high, _ := strconv.ParseFloat(fields[3], 64)
		low, _ := strconv.ParseFloat(fields[4], 64)
		vol, _ := strconv.ParseFloat(fields[5], 64)
		amount, _ := strconv.ParseFloat(fields[6], 64)
		pctChg, _ := strconv.ParseFloat(fields[8], 64)
		change, _ := strconv.ParseFloat(fields[9], 64)

		klines = append(klines, tushare.DailyKLine{
			TSCode: tsCode, TradeDate: strings.ReplaceAll(fields[0], "-", ""),
			Open: open, Close: closePrice, High: high, Low: low,
			Vol: vol, Amount: amount / 1000.0,
			PctChg: pctChg, Change: change, PreClose: closePrice - change,

			// 💥 注入血缘标记
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}
	return klines, nil
}

// ==========================================
// 🛡️ 节点 2：腾讯财经 (海外节点不死鸟，TrustLevel: 30)
// ==========================================
func fetchFromTencent(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	tenCode := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		tenCode = strings.ToLower(parts[1]) + parts[0]
	}

	t_start := startDate[:4] + "-" + startDate[4:6] + "-" + startDate[6:8]
	t_end := endDate[:4] + "-" + endDate[4:6] + "-" + endDate[6:8]

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,day,%s,%s,500,", tenCode, t_start, t_end)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("节点 2 也阵亡了: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rawData map[string]interface{}
	json.Unmarshal(body, &rawData)

	dataObj, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return []tushare.DailyKLine{}, nil
	}
	stockData, ok := dataObj[tenCode].(map[string]interface{})
	if !ok {
		return []tushare.DailyKLine{}, nil
	}
	dayData, ok := stockData["day"].([]interface{})
	if !ok {
		return []tushare.DailyKLine{}, nil
	}

	var klines []tushare.DailyKLine
	for _, item := range dayData {
		kArr := item.([]interface{})
		if len(kArr) < 6 {
			continue
		}

		dateStr := strings.ReplaceAll(kArr[0].(string), "-", "")
		open, _ := strconv.ParseFloat(kArr[1].(string), 64)
		closePrice, _ := strconv.ParseFloat(kArr[2].(string), 64)
		high, _ := strconv.ParseFloat(kArr[3].(string), 64)
		low, _ := strconv.ParseFloat(kArr[4].(string), 64)
		vol, _ := strconv.ParseFloat(kArr[5].(string), 64)

		klines = append(klines, tushare.DailyKLine{
			TSCode: tsCode, TradeDate: dateStr,
			Open: open, Close: closePrice, High: high, Low: low,
			Vol: vol, Amount: 0, // 腾讯缺成交额

			// 💥 注入血缘标记 (底层兜底)
			DataSource: "TENCENT",
			TrustLevel: 30,
		})
	}
	return klines, nil
}

// ==========================================
// 💥 节点 1 扩编：高级战术情报 (指数与资金流向)
// ==========================================

// FetchIndexDaily 拉取大盘指数 (逻辑与 K 线完全一致，只是映射不同)
// 💥 FetchIndexDaily 责任链：东财(主力) -> 腾讯(灾备)
func FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	indices, err := fetchIndexFromEastMoney(tsCode, startDate, endDate)
	if err == nil && len(indices) > 0 {
		return indices, nil
	}
	// 东财阵亡，腾讯接管大盘指数！
	return fetchIndexFromTencent(tsCode, startDate, endDate)
}

// 内部函数：东财指数拉取
func fetchIndexFromEastMoney(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=0&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	json.Unmarshal(body, &emResp)

	var indices []tushare.IndexDaily
	if emResp.Data.Klines == nil {
		return indices, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 11 {
			continue
		}
		closePrice, _ := strconv.ParseFloat(fields[2], 64)
		vol, _ := strconv.ParseFloat(fields[5], 64)
		pctChg, _ := strconv.ParseFloat(fields[8], 64)

		indices = append(indices, tushare.IndexDaily{
			TSCode: tsCode, TradeDate: strings.ReplaceAll(fields[0], "-", ""),
			Close: closePrice, Vol: vol, PctChg: pctChg,
			// 💥 补全血缘：
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}
	return indices, nil
}

// 内部函数：腾讯指数拉取 (降级灾备)
func fetchIndexFromTencent(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	tenCode := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		tenCode = strings.ToLower(parts[1]) + parts[0]
	}

	t_start := startDate[:4] + "-" + startDate[4:6] + "-" + startDate[6:8]
	t_end := endDate[:4] + "-" + endDate[4:6] + "-" + endDate[6:8]

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,day,%s,%s,500,", tenCode, t_start, t_end)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rawData map[string]interface{}
	json.Unmarshal(body, &rawData)

	dataObj, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return []tushare.IndexDaily{}, nil
	}
	stockData, ok := dataObj[tenCode].(map[string]interface{})
	if !ok {
		return []tushare.IndexDaily{}, nil
	}
	dayData, ok := stockData["day"].([]interface{})
	if !ok {
		return []tushare.IndexDaily{}, nil
	}

	var indices []tushare.IndexDaily
	for _, item := range dayData {
		kArr := item.([]interface{})
		if len(kArr) < 6 {
			continue
		}

		dateStr := strings.ReplaceAll(kArr[0].(string), "-", "")
		closePrice, _ := strconv.ParseFloat(kArr[2].(string), 64)
		vol, _ := strconv.ParseFloat(kArr[5].(string), 64)

		indices = append(indices, tushare.IndexDaily{
			TSCode: tsCode, TradeDate: dateStr,
			Close: closePrice, Vol: vol, PctChg: 0, // 腾讯基础包不含涨跌幅，设为0兜底
			// 💥 补全血缘：
			DataSource: "TENCENT",
			TrustLevel: 30,
		})
	}
	return indices, nil
}

// FetchMoneyFlow 拉取主力资金流向 (东财 L2 核心接口)
func FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 💥 东财专属 fflow 资金流历史接口
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get?secid=%s&klt=101&beg=%s&end=%s&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyFlowResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyFlowResp
	json.Unmarshal(body, &emResp)

	var flows []tushare.DailyMoneyFlow
	if emResp.Data.Klines == nil {
		return flows, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 10 {
			continue
		}

		dateStr := strings.ReplaceAll(fields[0], "-", "")

		// 💥 修复：人工时间过滤器，抛弃多余的历史数据
		if dateStr < startDate || dateStr > endDate {
			continue
		}

		mainNet, _ := strconv.ParseFloat(fields[2], 64)

		flows = append(flows, tushare.DailyMoneyFlow{
			TSCode: tsCode, TradeDate: dateStr,
			BuyLgVol: 0, SellLgVol: 0, BuyElgVol: 0, SellElgVol: 0,
			NetMfVol: mainNet / 10000.0,
			// 💥 补全血缘：
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}

	return flows, nil
}

// 💥 内部底层函数：带 fqt 参数的通用 K 线拉取器
func fetchEastMoneyRaw(tsCode, startDate, endDate, fqt string) ([]tushare.DailyKLine, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 注入 fqt 参数控制复权类型 (0:不复权, 1:前复权, 2:后复权)
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=%s&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, fqt, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	// req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	// 💥 注入全套高仿浏览器 Header，突破防爬墙
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "http://quote.eastmoney.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	json.Unmarshal(body, &emResp)

	var klines []tushare.DailyKLine
	if emResp.Data.Klines == nil {
		return klines, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 11 {
			continue
		}
		open, _ := strconv.ParseFloat(fields[1], 64)
		closePrice, _ := strconv.ParseFloat(fields[2], 64)
		high, _ := strconv.ParseFloat(fields[3], 64)
		low, _ := strconv.ParseFloat(fields[4], 64)
		vol, _ := strconv.ParseFloat(fields[5], 64)

		klines = append(klines, tushare.DailyKLine{
			TSCode: tsCode, TradeDate: strings.ReplaceAll(fields[0], "-", ""),
			Open: open, Close: closePrice, High: high, Low: low, Vol: vol,
		})
	}
	return klines, nil
}

func FetchDerivedAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	// 💥 拆除 WaitGroup 并发，改为串行请求，向东财防火墙妥协
	unadjusted, errUnadj := fetchEastMoneyRaw(tsCode, startDate, endDate, "0")
	if errUnadj != nil {
		return nil, fmt.Errorf("不复权拉取失败: %v", errUnadj)
	}

	// 💥 关键修复：第二枪也必须等待系统分配令牌，且增加随机性！
	// 注意：这里需要你手动引入 "math/rand" 和 "time" 包
	time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)

	adjusted, errAdj := fetchEastMoneyRaw(tsCode, startDate, endDate, "2")
	if errAdj != nil {
		return nil, fmt.Errorf("后复权拉取失败: %v", errAdj)
	}

	if len(unadjusted) == 0 || len(adjusted) == 0 {
		return []tushare.AdjFactor{}, nil
	}

	// 2. 构建内存哈希表，准备 O(1) 复杂度拉链对齐
	unadjMap := make(map[string]float64, len(unadjusted))
	for _, k := range unadjusted {
		unadjMap[k.TradeDate] = k.Close
	}

	// 3. 执行逆向除法推导
	var factors []tushare.AdjFactor
	for _, adjK := range adjusted {
		unadjClose, exists := unadjMap[adjK.TradeDate]
		if !exists || unadjClose <= 0 {
			continue // 防御除零异常或空洞数据
		}

		// 核心数学模型：后复权收盘价 / 不复权收盘价 = 复权因子
		factor := adjK.Close / unadjClose

		factors = append(factors, tushare.AdjFactor{
			TSCode:     tsCode,
			TradeDate:  adjK.TradeDate,
			AdjFactor:  factor,
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}

	return factors, nil
}

// ==========================================
// 💎 每日基本面提取器 (开源平替版)
// ==========================================

// FetchDailyBasic 榨取东财 K 线接口中的基本面衍生数据 (换手率)
func FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 复用 K 线接口，因为 fields2=f51~f61 中蕴含了换手率等衍生数据
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=0&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	if err := json.Unmarshal(body, &emResp); err != nil {
		return nil, err
	}

	var fundamentals []tushare.DailyFundamental
	if emResp.Data.Klines == nil {
		return fundamentals, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		// 东财的 K 线字符串至少有 11 个字段，索引 10 即为换手率
		if len(fields) < 11 {
			continue
		}

		dateStr := strings.ReplaceAll(fields[0], "-", "")
		turnover, _ := strconv.ParseFloat(fields[10], 64)

		fundamentals = append(fundamentals, tushare.DailyFundamental{
			TSCode:       tsCode,
			TradeDate:    dateStr,
			TurnoverRate: turnover,
			// 开源平替拿不到历史 PE/PB/市值，设为 0 作为占位符，等待 Tushare 高权限覆盖
			PE:      0,
			PB:      0,
			TotalMV: 0,
			DVRatio: 0,

			// 💥 注入血缘防线：标明这是东财的低权数据
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}

	return fundamentals, nil
}

```

## File: feeder/engine.go

```go
package feeder

import (
	"math/rand"
	"stock-backend/db"
	"stock-backend/tushare"
	"sync"
	"time"
)

// ==========================================
// 💥 全局调度引擎 (Global Engine - 动态变速版)
// ==========================================

var (
	baseDelay   time.Duration
	delayMutex  sync.RWMutex // 💥 新增：保护动态射速的并发锁
	limiterOnce sync.Once
	sinkChan    chan SinkTask
)

// SinkTask 定义异步落盘任务
type SinkTask struct {
	Type   string // "kline", "fund", "adj", "index", "moneyflow", "fina", "limitlist"
	TSCode string
	Data   interface{}
}

// InitGlobalEngine 初始化全局护盾
func InitGlobalEngine(delay time.Duration) {
	limiterOnce.Do(func() {
		SetBaseDelay(int(delay.Milliseconds())) // 初始化时设定初始速度
		sinkChan = make(chan SinkTask, 5000)
		go dataSinkWorker()

		// 初始化随机数种子
		rand.Seed(time.Now().UnixNano())
	})
}

// 💥 新增：动态设置基础延迟 (热重载变速箱)
func SetBaseDelay(delayMs int) {
	delayMutex.Lock()
	defer delayMutex.Unlock()
	baseDelay = time.Duration(delayMs) * time.Millisecond
	LogMsg("🛡️ [系统引擎] 仿生学漏桶射速已热更新为: %d 毫秒/发！", delayMs)
}

// WaitToken 仿生学阻塞：带并发安全锁的动态延迟
func WaitToken() {
	// 安全读取当前射速
	delayMutex.RLock()
	currentDelay := baseDelay
	delayMutex.RUnlock()

	if currentDelay > 0 {
		// 引入随机抖动：基准时间 + 0~100% 的随机抖动
		jitter := time.Duration(rand.Int63n(int64(currentDelay)))
		time.Sleep(currentDelay + jitter)
	}
}

// PushToSink 将清洗后的数据打入单向汇聚层
func PushToSink(taskType, tsCode string, data interface{}) {
	sinkChan <- SinkTask{
		Type:   taskType,
		TSCode: tsCode,
		Data:   data,
	}
}

// dataSinkWorker 单向落盘守护进程 (系统唯一的写入出口)
func dataSinkWorker() {
	for task := range sinkChan {
		saved := 0
		switch task.Type {
		case "kline":
			if klines, ok := task.Data.([]tushare.DailyKLine); ok {
				saved = db.BatchInsertKLines(task.TSCode, klines)
			}
		case "fund":
			if funds, ok := task.Data.([]tushare.DailyFundamental); ok {
				saved = db.BatchInsertFundamentals(task.TSCode, funds)
			}
		case "adj":
			if adjs, ok := task.Data.([]tushare.AdjFactor); ok {
				saved = db.BatchInsertAdjFactors(task.TSCode, adjs)
			}
		case "index":
			if indices, ok := task.Data.([]tushare.IndexDaily); ok {
				saved = db.BatchInsertIndexDaily(task.TSCode, indices)
			}
			// ... 之前的 kline, fund, adj, index 保持不变 ...
		case "moneyflow":
			if flows, ok := task.Data.([]tushare.DailyMoneyFlow); ok {
				saved = db.BatchInsertMoneyFlow(task.TSCode, flows)
			}
		case "fina":
			if finas, ok := task.Data.([]tushare.FinaIndicator); ok {
				saved = db.BatchInsertFinaIndicators(task.TSCode, finas)
			}
		case "stklimit":
			if limits, ok := task.Data.([]tushare.StkLimit); ok {
				saved = db.BatchInsertStkLimit(limits)
			}

		}

		if saved > 0 {
			LogMsg("💾 [Data Sink] %s [%s] 异步落盘成功: %d 条", task.Type, task.TSCode, saved)
		}
	}
}

```

## File: feeder/sync.go

```go
package feeder

import (
	"fmt"
	"stock-backend/db"
	"stock-backend/eastmoney"
	"stock-backend/tushare"
	"sync"
	"time"
)

// ==========================================
// 💥 数据源适配器接口 (Adapter Pattern)
// ==========================================

// DataProvider 定义了所有情报供应商必须遵守的协议
type DataProvider interface {
	FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error)
	FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error)
	FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) // 💥 新增
	FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error)
	FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error)
	FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) // 💥 挂载涨跌停榜武器
	GetName() string
}

// ------------------------------------------
// 驱动 A：Tushare 高级付费装甲兵 (全量历史基石)
type TushareProvider struct{}

func (t *TushareProvider) GetName() string { return "Tushare [高级]" }
func (t *TushareProvider) FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	return tushare.FetchStockHistory(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	return tushare.FetchAdjFactors(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	return tushare.FetchDailyBasic(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	return tushare.FetchIndexDaily(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	return tushare.FetchMoneyFlow(tsCode, startDate, endDate)
}
func (t *TushareProvider) FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) {
	return tushare.FetchStkLimit(tradeDate)
}

// ------------------------------------------
// 驱动 B：开源免费轻步兵 (东方财富)
// ------------------------------------------
type OpenSourceProvider struct{}

func (o *OpenSourceProvider) GetName() string {
	return "开源接口 (EastMoney)"
}

func (o *OpenSourceProvider) FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	return eastmoney.FetchStockHistory(tsCode, startDate, endDate)
}

// 修改 feeder/sync.go 中的 OpenSourceProvider 基本面方法
func (o *OpenSourceProvider) FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	// 💥 接入东财开源基本面榨取引擎
	return eastmoney.FetchDailyBasic(tsCode, startDate, endDate)
}

func (o *OpenSourceProvider) FetchAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	// 💥 启用东财逆向推导引擎替代 Tushare
	return eastmoney.FetchDerivedAdjFactors(tsCode, startDate, endDate)
}

// 💥 挂载新武器：大盘指数
func (o *OpenSourceProvider) FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	return eastmoney.FetchIndexDaily(tsCode, startDate, endDate)
}

// 💥 挂载新武器：资金流向
func (o *OpenSourceProvider) FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	return eastmoney.FetchMoneyFlow(tsCode, startDate, endDate)
}
func (o *OpenSourceProvider) FetchStkLimit(tradeDate string) ([]tushare.StkLimit, error) {
	// 开源降级版暂不支持，保护接口一致性
	return nil, fmt.Errorf("开源接口暂不支持历史涨跌停绝对价拉取")
}

// ==========================================

// ==========================================
// 💥 赛博控制台日志系统
// ==========================================
var (
	LogBuffer []string
	logMutex  sync.Mutex
)

// LogMsg 替代 fmt.Printf，同时输出到终端和前端内存环！
func LogMsg(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Print(msg) // 依然在终端输出一份保底

	logMutex.Lock()
	LogBuffer = append(LogBuffer, time.Now().Format("15:04:05")+" "+msg)
	// 永远只保留最新的 50 条日志，防止撑爆内存！
	if len(LogBuffer) > 50 {
		LogBuffer = LogBuffer[len(LogBuffer)-50:]
	}
	logMutex.Unlock()
}

// GetLogs 供前端读取最新日志
func GetLogs() []string {
	logMutex.Lock()
	defer logMutex.Unlock()
	res := make([]string, len(LogBuffer))
	copy(res, LogBuffer)
	return res
}

// ==========================================
// 💥 引擎 A：专属 K 线抽水机
// 💥 引擎 A：专属 K 线抽水机 (V3.0 动态射速版)
// 💥 引擎 A：专属 K 线抽水机 (黎明扫荡版：精确填缝)
func StartSyncKLine(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) {
	total := len(stockCodes)
	LogMsg("🚀 [抽水机A] K线引擎启动！当前源:[%s]\n", provider.GetName())
	// 💥 补上这段火力权重判定
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}
	for i, code := range stockCodes {
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_klines", code, targetStart, targetEnd, targetTrust)
		if !needSync {
			continue // 静默跳过，减少日志噪音
		}

		LogMsg("⏳ [K线 %d/%d] 发现空洞，准备拉取 %s...", i+1, total, code)

		var history []tushare.DailyKLine
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 💥 接入全局漏桶，等待开火指令
			history, err = provider.FetchStockHistory(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if err == nil && len(history) > 0 {
			// 💥 数据打入汇聚管线
			PushToSink("kline", code, history)
		} else if err == nil && len(history) == 0 {
			// =======================================================
			// 💥 终极修复：停牌股补漏机制！
			// 如果 API 请求成功，但确实没有数据，说明这天停牌了。
			// 塞入伪造的幽灵 K 线，权重设为 -1，防止以后天天重复拉取！
			// =======================================================
			LogMsg("⚠️ [K线] %s 在 %s~%s 期间无数据(停牌)，填入幽灵标记防止死循环...", code, actualStart, actualEnd)

			// 查出这期间所有的交易日
			query := `SELECT cal_date FROM trade_calendar WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?`
			rows, _ := db.DB.Query(query, actualStart, actualEnd)
			var ghostKLines []tushare.DailyKLine
			for rows.Next() {
				var d string
				if rows.Scan(&d) == nil {
					ghostKLines = append(ghostKLines, tushare.DailyKLine{
						TSCode: code, TradeDate: d,
						Open: 0, Close: 0, High: 0, Low: 0, Vol: 0, Amount: 0,
						DataSource: "SYSTEM_GHOST",
						TrustLevel: -1, // 极低权重，只要以后有了真实数据，立刻会被 UPSERT 覆盖！
					})
				}
			}
			rows.Close()
			if len(ghostKLines) > 0 {
				PushToSink("kline", code, ghostKLines)
			}
		} else {
			LogMsg("❌ [K线 %d/%d] %s 失败: %v", i+1, total, code, err)
		}
	}
	LogMsg("🎉 [抽水机A] K线填缝网络拉取阶段完成！(请等待 Sink 落盘)\n")
}

// 💥 引擎 B：专属基本面抽水机 (黎明扫荡版：异步单点汇聚)
func StartSyncFund(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) {
	total := len(stockCodes)
	LogMsg("💎 [抽水机B] 基本面引擎启动！\n")
	// 💥 补上这段火力权重判定
	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}
	for i, code := range stockCodes {
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("daily_fundamentals", code, targetStart, targetEnd, targetTrust)

		if !needSync {
			continue // 静默跳过，避免刷屏
		}

		LogMsg("⏳ [基本面 %d/%d] 发现空洞！定向拉取 %s (%s~%s)...\n", i+1, total, code, actualStart, actualEnd)

		var funds []tushare.DailyFundamental
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken() // 💥 接入全局漏桶防超速
			funds, err = provider.FetchDailyBasic(code, actualStart, actualEnd)
			if err == nil {
				break
			}
			LogMsg("⚠️ [基本面] %s 报错: %v。重试 %d/3...\n", code, err, retry+1)
			time.Sleep(5 * time.Second)
		}

		if err != nil {
			LogMsg("❌ [基本面 %d/%d] %s 彻底失败！交由对账员下次处理。\n", i+1, total, code)
			continue
		}

		if len(funds) > 0 {
			// 💥 正确做法：打入异步管线，Worker 立即转身去拉下一个股票！
			// 参数 1: 必须是 "fund" 字符串，让 Sink 路由知道调哪个表
			PushToSink("fund", code, funds)
		} else {
			LogMsg("⚠️ [基本面 %d/%d] %s 返回 0 条数据\n", i+1, total, code)
		}
	}
	LogMsg("🎉 [抽水机B] 基本面网络拉取填缝完成！(等待后台 Sink 落盘)\n")
}

// 💥 引擎 C：专属复权因子抽水机
func StartSyncAdjFactors(provider DataProvider, stockCodes []string, targetStart string, targetEnd string) {
	total := len(stockCodes)
	LogMsg("🧬 [抽水机C] 复权因子引擎启动！当前源:[%s]\n", provider.GetName())

	targetTrust := 50
	if provider.GetName() == "Tushare [高级]" {
		targetTrust = 100
	}

	for i, code := range stockCodes {
		// 💥 接入天眼系统！
		actualStart, actualEnd, needSync := db.GetDailySyncTaskRange("adj_factors", code, targetStart, targetEnd, targetTrust)
		if !needSync {
			continue // 静默跳过，保护 Tushare 积分！
		}

		LogMsg("⏳ [复权 %d/%d] 发现断层！正在拉取并推导 %s (%s~%s)...", i+1, total, code, actualStart, actualEnd)

		var factors []tushare.AdjFactor
		var err error
		for retry := 0; retry < 3; retry++ {
			WaitToken()
			factors, err = provider.FetchAdjFactors(code, actualStart, actualEnd) // 使用精确区间
			if err == nil {
				break
			}
			time.Sleep(3 * time.Second)
		}

		if err != nil {
			LogMsg("❌ [复权 %d/%d] %s 彻底失败！", i+1, total, code)
			continue
		}

		if len(factors) > 0 {
			PushToSink("adj", code, factors)
		}
	}
	LogMsg("🎉 [抽水机C] 复权因子网络拉取完成！\n")
}
// 💥 引擎 G：专属涨跌榜抽水机 (通过日历智能推导区间)
func StartSyncStkLimit(provider DataProvider, targetStart string, targetEnd string) {
	LogMsg("🔥 [抽水机G] 涨跌停引擎启动！当前源:[%s]", provider.GetName())

	// 调用我们刚刚写好的日历雷达，直接锁定所有空洞日期！
	missingDates := db.GetMarketMissingDates("daily_stk_limit", targetStart, targetEnd)

	if len(missingDates) == 0 {
		LogMsg("✅ [抽水机G] 目标区间涨跌停数据严丝合缝，无需重复拉取！")
		return
	}

	LogMsg("⏳ [抽水机G] 发现 %d 个交易日缺失数据，开始逐日补齐...", len(missingDates))
	totalSaved := 0

	for i, d := range missingDates {
		WaitToken() // 依然经过限速漏桶

		limits, err := provider.FetchStkLimit(d)
		if err != nil {
			LogMsg("⚠️ [抽水机G] %s 报错: %v", d, err)
			time.Sleep(2 * time.Second) // 遇到错误冷静两秒
			continue
		}

		if len(limits) > 0 {
			PushToSink("stklimit", "ALL", limits)
			totalSaved += len(limits)
		} else {
			// 打上幽灵标记防死循环
			ghost := []tushare.StkLimit{{
				TradeDate:  d,
				TSCode:     "GHOST",
				DataSource: "SYSTEM_GHOST",
				TrustLevel: -1,
			}}
			PushToSink("stklimit", "ALL", ghost)
		}

		if (i+1)%50 == 0 || i == len(missingDates)-1 {
			LogMsg("🔄 [抽水机G] 进度汇报: 已处理 %d/%d 天，累计发现 %d 个标的...", i+1, len(missingDates), totalSaved)
		}
	}
	LogMsg("🎉 [抽水机G] 涨跌停底座历史扫荡完成！共推入真实数据: %d 条！", totalSaved)
}

```

## File: go.mod

```mod
module stock-backend

go 1.25.7

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/sys v0.37.0 // indirect
	modernc.org/libc v1.67.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.46.1 // indirect
)

```

## File: go.sum

```sum
github.com/dustin/go-humanize v1.0.1 h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=
github.com/dustin/go-humanize v1.0.1/go.mod h1:Mu1zIs6XwVuF/gI1OepvI0qD18qycQx+mFykh5fBlto=
github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=
github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/mattn/go-isatty v0.0.20 h1:xfD0iDuEKnDkl03q4limB+vH+GxLEtL/jb4xVJSWWEY=
github.com/mattn/go-isatty v0.0.20/go.mod h1:W+V8PltTTMOvKvAeJH7IuucS94S2C6jfK/D7dTCTo3Y=
github.com/ncruces/go-strftime v1.0.0 h1:HMFp8mLCTPp341M/ZnA4qaf7ZlsbTc+miZjCLOFAw7w=
github.com/ncruces/go-strftime v1.0.0/go.mod h1:Fwc5htZGVVkseilnfgOVb9mKy6w1naJmn9CehxcKcls=
github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec h1:W09IVJc94icq4NjY3clb7Lk8O1qJ8BdBEF8z0ibU0rE=
github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec/go.mod h1:qqbHyh8v60DhA7CoWK5oRCqLrMHRGoxYCSS9EjAz6Eo=
golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 h1:mgKeJMpvi0yx/sU5GsxQ7p6s2wtOnGAHZWCHUM4KGzY=
golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546/go.mod h1:j/pmGrbnkbPtQfxEe5D0VQhZC6qKbfKifgD0oM7sR70=
golang.org/x/sys v0.6.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.37.0 h1:fdNQudmxPjkdUTPnLn5mdQv7Zwvbvpaxqs831goi9kQ=
golang.org/x/sys v0.37.0/go.mod h1:OgkHotnGiDImocRcuBABYBEXf8A9a87e/uXjp9XT3ks=
modernc.org/libc v1.67.6 h1:eVOQvpModVLKOdT+LvBPjdQqfrZq+pC39BygcT+E7OI=
modernc.org/libc v1.67.6/go.mod h1:JAhxUVlolfYDErnwiqaLvUqc8nfb2r6S6slAgZOnaiE=
modernc.org/mathutil v1.7.1 h1:GCZVGXdaN8gTqB1Mf/usp1Y/hSqgI2vAGGP4jZMCxOU=
modernc.org/mathutil v1.7.1/go.mod h1:4p5IwJITfppl0G4sUEDtCr4DthTaT47/N3aT6MhfgJg=
modernc.org/memory v1.11.0 h1:o4QC8aMQzmcwCK3t3Ux/ZHmwFPzE6hf2Y5LbkRs+hbI=
modernc.org/memory v1.11.0/go.mod h1:/JP4VbVC+K5sU2wZi9bHoq2MAkCnrt2r98UGeSK7Mjw=
modernc.org/sqlite v1.46.1 h1:eFJ2ShBLIEnUWlLy12raN0Z1plqmFX9Qe3rjQTKt6sU=
modernc.org/sqlite v1.46.1/go.mod h1:CzbrU2lSB1DKUusvwGz7rqEKIq+NUd8GWuBBZDs9/nA=

```

## File: main.go

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"stock-backend/api"
	"stock-backend/db"
	"stock-backend/feeder"
	"stock-backend/tushare"
	"strconv"
	"strings"
	"time"
)

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

// parseTargetCodes 解析前端传来的指定股票代码，为空则全军出击
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
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "msg": "高阶 Token 动态装填成功！已解除火力封印。"})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 400, "msg": "Token 不能为空"})
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

			// 💥 修复静默吞没，暴露真实战况！
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

	// 挂载武器：默认启用 Tushare
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

	// 💥 武器挂载系统：根据前端指令选择弹药供应商
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
		"msg":  fmt.Sprintf("基本面抽水机已启动！当前火力源: %s", provider.GetName()),
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

	// 💥 武器挂载系统：根据前端指令选择弹药供应商
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
		"msg":  fmt.Sprintf("K线抽水机已启动！当前火力源: %s", provider.GetName()),
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
	// // ==========================================
	// // 💥 植入：上帝视角查账器 (每次启动时自动汇报数据库行数)
	// // ==========================================
	// go func() {
	// 	time.Sleep(2 * time.Second) // 等待日志安静下来再打印，比较显眼
	// 	fmt.Println("\n=========================================")
	// 	fmt.Println("📊 [终极查账] 当前本地数据库资产规模盘点")
	// 	tables := map[string]string{
	// 		"stock_basic":        "📜 花名册",
	// 		"trade_calendar":     "📅 交易日历",
	// 		"daily_klines":       "📈 日线量价",
	// 		"daily_fundamentals": "💎 基本面估值",
	// 		"adj_factors":        "🧬 复权因子",
	// 		"daily_moneyflow":    "🌊 主力资金流",
	// 		"fina_indicators":    "🏦 季报财务",
	// 		"daily_limit_list":   "🔥 涨跌停榜",
	// 	}
	// 	totalRows := 0
	// 	for table, name := range tables {
	// 		var count int
	// 		db.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	// 		fmt.Printf("   ├─ %s 表数据量: %d 行\n", name, count)
	// 		totalRows += count
	// 	}
	// 	fmt.Printf("   └─ 🏆 您的私有数据库总规模: %d 行！\n", totalRows)
	// 	fmt.Println("=========================================\n")
	// }()
	// ==========================================
	// 💥 挂载全局流量控制引擎 (200ms/发，相当于每秒并发 5 只股票的安全速度)
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
	http.HandleFunc("/api/start_sync_moneyflow", triggerSyncMoneyFlowHandler)
	http.HandleFunc("/api/start_sync_fina", triggerSyncFinaHandler)
	http.HandleFunc("/api/start_sync_limit", triggerSyncLimitListHandler)
	http.HandleFunc("/api/start_sync_basic", triggerSyncBasicHandler)
	http.HandleFunc("/api/set_speed", updateSpeedHandler) // 💥 注册变速接口
	http.HandleFunc("/api/monitor", api.MonitorHandler)   // 💥 侧刀入口
	fmt.Println("🟢 工业级全字段量化引擎启动完毕！监听端口: 8081")
	http.ListenAndServe(":8081", nil)
}

```

## File: strategy/indicators.go

```go
package strategy

import (
	"math"
	"sort"
	"stock-backend/tushare"
)

// CalcMA 计算指定位置前 N 天的收盘价均线
func CalcMA(history []tushare.DailyKLine, days int) float64 {
	if len(history) < days {
		return 0
	}
	total := 0.0
	startIndex := len(history) - days
	for i := startIndex; i < len(history); i++ {
		total += history[i].Close
	}
	return total / float64(days)
}

// CalcVolMA 计算指定位置前 N 天的成交量均线
func CalcVolMA(history []tushare.DailyKLine, days int) float64 {
	if len(history) < days {
		return 0
	}
	total := 0.0
	startIndex := len(history) - days
	for i := startIndex; i < len(history); i++ {
		total += history[i].Vol
	}
	return total / float64(days)
}

// GetBox 计算前 N 天（不含今天）的箱体最高价和最低价
func GetBox(history []tushare.DailyKLine, days int) (float64, float64) {
	if len(history) <= days {
		return 0, 0
	}
	high := 0.0
	low := math.MaxFloat64
	// 注意：不包含最后一天（今天）
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		if history[i].High > high {
			high = history[i].High
		}
		if history[i].Low < low {
			low = history[i].Low
		}
	}
	return high, low
}

// CalcATR 计算过去 N 天的真实波幅 (Average True Range)
func CalcATR(history []tushare.DailyKLine, days int) float64 {
	if len(history) <= days {
		return 0
	}
	totalTR := 0.0
	startIndex := len(history) - days

	for i := startIndex; i < len(history); i++ {
		h := history[i].High
		l := history[i].Low
		var pc float64
		if i > 0 {
			pc = history[i-1].Close
		} else {
			pc = history[i].PreClose // 兜底
		}

		// TR = max(H-L, abs(H-Pc), abs(L-Pc))
		tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		totalTR += tr
	}
	return totalTR / float64(days)
}

// CheckConvergence 判定是否连续 T 天满足均线收敛
func CheckConvergence(history []tushare.DailyKLine, days int, threshold float64) bool {
	if len(history) < 120+days {
		return false
	}
	// 往前推 T 天，每天都必须满足收敛
	for i := len(history) - days; i < len(history); i++ {
		// 切片截取到当天的历史
		subHistory := history[:i+1]
		ma30 := CalcMA(subHistory, 30)
		ma60 := CalcMA(subHistory, 60)
		ma120 := CalcMA(subHistory, 120)

		maxMA := math.Max(ma30, math.Max(ma60, ma120))
		minMA := math.Min(ma30, math.Min(ma60, ma120))

		if minMA <= 0 {
			return false
		}

		convergence := (maxMA - minMA) / minMA
		if convergence > threshold {
			return false // 只要有一天不满足，就说明没有持续纠缠
		}
	}
	return true
}

// IsLongUpperShadow 判定是否出现高位长上影线 (派发信号)
// 上影线长度大于实体长度的 N 倍
func IsLongUpperShadow(k tushare.DailyKLine, times float64) bool {
	bodyTop := math.Max(k.Open, k.Close)
	bodyBottom := math.Min(k.Open, k.Close)

	upperShadow := k.High - bodyTop
	body := bodyTop - bodyBottom

	// 防止一字板除数为空
	if body == 0 {
		return upperShadow > 0
	}
	return upperShadow > (body * times)
}

// GetRealBox 方案B：统计学剥离毛刺法 (寻找真实的筹码震荡箱体)
func GetRealBox(history []tushare.DailyKLine, days int) (float64, float64) {
	if len(history) <= days {
		return 0, 0
	}

	// 1. 提取过去 N 天（不含今天）的开盘价和收盘价实体
	var prices []float64
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		// 我们只取实体部分(Open和Close)，直接抛弃极其容易骗线的上下影线(High和Low)
		prices = append(prices, history[i].Open)
		prices = append(prices, history[i].Close)
	}

	// 2. 将价格从小到大排序
	sort.Float64s(prices)

	// 3. 统计学剥离：砍掉顶部 10% 的诱多极值，和底部 10% 的诱空极值
	// 这样留下的，就是 80% 核心筹码沉淀的真实中枢
	totalLen := len(prices)
	lowerIndex := int(float64(totalLen) * 0.10) // 10% 分位数
	upperIndex := int(float64(totalLen) * 0.90) // 90% 分位数

	// 防越界保护
	if lowerIndex < 0 {
		lowerIndex = 0
	}
	if upperIndex >= totalLen {
		upperIndex = totalLen - 1
	}

	boxLower := prices[lowerIndex]
	boxUpper := prices[upperIndex]

	return boxUpper, boxLower
}

// ==========================================
// V3.3 宏观调整 1：箱体规律性测算 (拒绝单边下跌途中的伪箱体)
// ==========================================
func CheckBoxRegularity(history []tushare.DailyKLine, days int) bool {
	if len(history) < days+30 {
		return false
	}
	// 提取箱体期间的 MA30 均线极值
	maxMA, minMA := 0.0, math.MaxFloat64
	startIndex := len(history) - 1 - days
	for i := startIndex; i < len(history)-1; i++ {
		ma30 := CalcMA(history[:i+1], 30)
		if ma30 > maxMA {
			maxMA = ma30
		}
		if ma30 < minMA {
			minMA = ma30
		}
	}
	// 如果这 N 天内，30日均线的上下波动超过了 15%，说明根本不是横盘震荡，而是处于剧烈趋势中
	if minMA > 0 && (maxMA-minMA)/minMA > 0.15 {
		return false
	}
	return true
}

// ==========================================
// V3.3 宏观调整 2：主力试盘雷达 (寻找仙人指路或未遂涨停)
// ==========================================
func HasProbingAction(history []tushare.DailyKLine, boxUpper float64, lookbackDays int) bool {
	if len(history) <= lookbackDays {
		return false
	}
	startIndex := len(history) - 1 - lookbackDays
	for i := startIndex; i < len(history)-1; i++ {
		k := history[i]
		// 1. 曾经触及过涨停价附近 (涨幅 > 8%)
		// 2. 最高价曾极为逼近箱体上轨 (距离 < 3%)
		// 3. 但最终没有形成有效突破 (收盘价被打回)
		if k.PctChg > 8.0 && math.Abs(k.High-boxUpper)/boxUpper < 0.03 && k.Close <= boxUpper {
			return true // 发现明确的主力试盘/洗盘痕迹！
		}
	}
	return false
}

// ==========================================
// V3.3 宏观调整 3：统一大级别压力位防雷网 (计算上方真空区)
// ==========================================
func GetOverheadRoom(history []tushare.DailyKLine, currentPrice float64, lookbackDays int) float64 {
	if len(history) < lookbackDays {
		return 1.0 // 数据不足，默认天空才是尽头 (不阻拦，交给基本面护盾)
	}
	resistance := 0.0
	startIndex := len(history) - lookbackDays
	for i := startIndex; i < len(history)-1; i++ {
		if history[i].High > resistance {
			resistance = history[i].High
		}
	}
	if resistance > currentPrice {
		return (resistance - currentPrice) / currentPrice
	}
	return 1.0 // 创出区间新高，上方无套牢盘
}

// ForwardAdjustKLines 前复权清洗器：修复除权除息导致的价格断层
func ForwardAdjustKLines(klines []tushare.DailyKLine, factors []tushare.AdjFactor) []tushare.DailyKLine {
	if len(klines) == 0 || len(factors) == 0 {
		return klines
	}

	// 1. 构建 O(1) 查询的哈希表
	factorMap := make(map[string]float64)
	for _, f := range factors {
		factorMap[f.TradeDate] = f.AdjFactor
	}

	// 2. 寻找基准锚点：最新交易日的复权因子
	latestDate := klines[len(klines)-1].TradeDate
	latestFactor, exists := factorMap[latestDate]
	if !exists {
		latestFactor = factors[len(factors)-1].AdjFactor // 降级兜底：取提供的因子列表最后一个
	}
	if latestFactor == 0 {
		latestFactor = 1.0 // 防除零崩溃
	}

	// 3. 执行前复权数学变换
	adjustedKLines := make([]tushare.DailyKLine, len(klines))
	for i, k := range klines {
		adjustedKLines[i] = k
		f, ok := factorMap[k.TradeDate]
		if !ok {
			f = latestFactor // 如果当天没因子数据，假设没有发生除权，继承最新因子
		}

		// 前复权乘数
		ratio := f / latestFactor

		// 价格等比缩小
		adjustedKLines[i].Open = k.Open * ratio
		adjustedKLines[i].Close = k.Close * ratio
		adjustedKLines[i].High = k.High * ratio
		adjustedKLines[i].Low = k.Low * ratio
		adjustedKLines[i].PreClose = k.PreClose * ratio
		adjustedKLines[i].Change = k.Change * ratio // 💥 补上这一行！

		// 💥 注意：价格缩小了，成交量必须等比放大，才能保证总成交额 (Amount) 不变，换手率计算才准确！
		if ratio > 0 {
			adjustedKLines[i].Vol = k.Vol / ratio
		}
	}

	return adjustedKLines
}

```

## File: strategy/models.go

```go
package strategy

import (
	"stock-backend/tushare"
)

// DiagnoseResult 工业级诊断报告（标准化输出）
type DiagnoseResult struct {
	Code          string  `json:"code"`
	StrategyName  string  `json:"strategy_name"`
	LatestPrice   float64 `json:"latest_price"`
	Signal        string  `json:"signal"`          // 信号："买入 🚀", "观望 💤", "卖出 🛑"
	Message       string  `json:"message"`         // 详细战报
	BuyPrice      float64 `json:"buy_price"`       // 🎯 建议买入价
	SellPrice     float64 `json:"sell_price"`      // 🎯 建议止盈价
	StopLossPrice float64 `json:"stop_loss_price"` // 🎯 硬性止损价
}

// 💥 架构升级：机构级多维数据上下文 (Security Context)
type SecurityContext struct {
	Code         string
	StockName    string
	KLines       []tushare.DailyKLine
	Fundamentals []tushare.DailyFundamental
	MoneyFlows   []tushare.DailyMoneyFlow // 💥 2000积分王牌：主力资金流向
	PEPercentile float64                  // 💥 新增：动态估值历史分位 (0.0 ~ 1.0)
	// 未来可直接在此处扩展 IndexDaily(大盘)、LimitList(涨停榜) 等，无需再改接口
}

// Analyzer 多态策略引擎接口定义 (Strategy Pattern)
type Analyzer interface {
	Name() string
	RequiredData() []string                      // 声明需要的数据："klines", "fundamentals", "moneyflow"
	Analyze(ctx *SecurityContext) DiagnoseResult // 💥 接口升维：接收上下文
}

```

## File: strategy/monitor.go

```go
package strategy

import (
	"fmt"
)

// PositionReport 侧刀输出的单只股票明日操作剧本
type PositionReport struct {
	TSCode        string  `json:"ts_code"`
	Name          string  `json:"name"`
	CurrentPrice  float64 `json:"current_price"`
	CostPrice     float64 `json:"cost_price"`
	HighWatermark float64 `json:"high_watermark"`
	ProfitPct     float64 `json:"profit_pct"`  // 当前盈亏比例
	Retracement   float64 `json:"retracement"` // 距最高点回撤
	Action        string  `json:"action"`      // 🟢 安全持仓 | 🟡 警戒状态 | 🔴 执行斩首
	Reason        string  `json:"reason"`      // 判决理由
}

// EvaluatePosition EOD 盘后持仓审判引擎
func EvaluatePosition(ctx *SecurityContext, costPrice float64, highWatermark float64) PositionReport {
	klines := ctx.KLines
	if len(klines) < 60 {
		return PositionReport{TSCode: ctx.Code, Action: "🟡 数据不足", Reason: "K线数据少于60天，无法进行有效防线测算"}
	}

	today := klines[len(klines)-1]
	currentPrice := today.Close
	profitPct := (currentPrice - costPrice) / costPrice * 100
	retracement := (highWatermark - currentPrice) / highWatermark * 100

	report := PositionReport{
		TSCode:        ctx.Code,
		Name:          ctx.StockName,
		CurrentPrice:  currentPrice,
		CostPrice:     costPrice,
		HighWatermark: highWatermark,
		ProfitPct:     profitPct,
		Retracement:   retracement,
		Action:        "🟢 安全持仓",
		Reason:        "未触及任何卖出防线，趋势良好，明日继续持股装死。",
	}

	// -----------------------------------------------------
	// 🔪 侧刀 1：动态最高水位防线 (利润捍卫者 - 8% 回撤死线)
	// -----------------------------------------------------
	if retracement >= 8.0 {
		report.Action = "🔴 执行斩首"
		report.Reason = fmt.Sprintf("利润回撤达标 (%.2f%%)！已跌破最高水位 (%.2f) 的 8%% 动态防线，趋势衰竭。", retracement, highWatermark)
		return report
	} else if retracement >= 6.0 {
		report.Action = "🟡 警戒状态"
		report.Reason = fmt.Sprintf("距高点已回撤 %.2f%%，逼近 8%% 斩首线，明日盘中必须重点盯防！", retracement)
	}

	// -----------------------------------------------------
	// 🔪 侧刀 2：核心防线实质性击穿 (趋势证伪器 - 破20日线)
	// -----------------------------------------------------
	ma20 := CalcMA(klines, 20)
	if currentPrice < ma20 {
		report.Action = "🔴 执行斩首"
		report.Reason = fmt.Sprintf("底层逻辑被证伪！今日收盘价 (%.2f) 已实质性跌破 20日生命线 (%.2f)，立刻离场。", currentPrice, ma20)
		return report
	} else if (currentPrice-ma20)/ma20 < 0.02 {
		if report.Action == "🟢 安全持仓" {
			report.Action = "🟡 警戒状态"
			report.Reason = fmt.Sprintf("现价已逼近 20日生命线 (%.2f)，防弹衣即将击穿，准备随时撤退！", ma20)
		}
	}

	// -----------------------------------------------------
	// 🔪 侧刀 3：情绪极度派发拦截 (主力出货探测器)
	// -----------------------------------------------------
	volMa20 := CalcVolMA(klines, 20)
	isVolumeSurge := today.Vol > (2.5 * volMa20)            // 爆出2.5倍以上平时天量
	hasUpperShadow := IsLongUpperShadow(today, 1.5)         // 带有长上影线
	isStagnant := today.PctChg < 3.0 && today.PctChg > -3.0 // 滞涨 (没涨停也没大跌，但疯狂换手)

	// 如果在高位（估值处于历史 70% 以上），爆出天量且带有长上影线滞涨
	if ctx.PEPercentile > 0.70 && isVolumeSurge && hasUpperShadow && isStagnant {
		report.Action = "🔴 执行斩首"
		report.Reason = fmt.Sprintf("高位天量派发警报！(估值分位:%.1f%%) 今日爆出 2.5 倍天量且留有长上影线，主力极大概率已在盘中倒货完毕，明日直接竞价抢跑！", ctx.PEPercentile*100)
		return report
	}

	return report
}

```

## File: strategy/rules.go

```go
package strategy

import (
	"fmt"
	"math"
	"stock-backend/tushare"
	"strings"
)

// ==========================================
// 🛡️ 全局公共基建：基本面防暴雷护盾 (升维版)
// ==========================================
func checkFundamentalShield(ctx *SecurityContext, strictMode bool) bool {
	funds := ctx.Fundamentals
	if len(funds) == 0 {
		return false
	}
	latestFund := funds[len(funds)-1]

	// 基础排雷线：亏损公司，或者历史估值分位处于 85% 以上的绝对泡沫区
	isGarbage := latestFund.PE <= 0 || ctx.PEPercentile > 0.85

	if strictMode {
		// 严格模式：PE 必须处于历史低/中水位 (<60%)，且不是亏损股
		if latestFund.PE <= 0 || ctx.PEPercentile > 0.60 {
			return false
		}
		return true
	}

	// 普通模式：只要不是亏损且极度泡沫即可
	if isGarbage && latestFund.DVRatio < 1.0 { // 除非股息率兜底
		return false
	}

	return true
}

// ==========================================
// 🛡️ 策略一：均线收敛突破 (MACB - 严谨重构版)
// ==========================================
type MACBAnalyzer struct{}

func (m *MACBAnalyzer) Name() string { return "均线收敛突破 (MACB)" }
func (m *MACBAnalyzer) RequiredData() []string {
	return []string{"klines", "fundamentals", "moneyflow"}
} // 💥 补充了 moneyflow

func (m *MACBAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	funds := ctx.Fundamentals
	if len(klines) < 130 { // 需要120天算均线，外加几天算收敛持续期
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// -----------------------------------------------------
	// 1. 基本面过滤引擎 (Fundamental Filter)
	// -----------------------------------------------------
	if len(funds) == 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	latestFund := funds[len(funds)-1]

	// 核心逻辑：买入前，估值必须处于历史合理区间 (分位数 < 80%)
	// 允许高绝对值 PE (成长股)，但如果是亏损(PE <= 0)或极度历史泡沫(分位 > 80%)，拒绝买入。
	if latestFund.PE <= 0 || ctx.PEPercentile > 0.80 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	today := klines[len(klines)-1]
	yesterday := klines[len(klines)-2]

	// -----------------------------------------------------
	// 2. 技术面买点引擎 (Buy Signal: Convergence + Breakout)
	// -----------------------------------------------------
	isConverged := CheckConvergence(klines, 5, 0.04)

	pctChgReal := (today.Close - yesterday.Close) / yesterday.Close * 100
	ma30 := CalcMA(klines, 30)
	ma60 := CalcMA(klines, 60)
	ma120 := CalcMA(klines, 120)
	maxMA := math.Max(ma30, math.Max(ma60, ma120))
	minMA := math.Min(ma30, math.Min(ma60, ma120))
	volMa20 := CalcVolMA(klines, 20)

	isYangLine := today.Close > today.Open              // 拒绝高开低走的假阴线
	hasUpperShadowRisk := IsLongUpperShadow(today, 1.5) // 拒绝避雷针

	isBreakout := pctChgReal >= 5.0 && today.Close > maxMA && isYangLine && !hasUpperShadowRisk
	isVolumeSurge := today.Vol > (2.0 * volMa20)

	// 💥 宏观调整 3：接入月线防雷网！就算均线收敛，头顶也不能有大山
	room := GetOverheadRoom(klines, today.Close, 250) // 均线突破看长一点，看250天(年线)压力
	if room < 0.20 {
		return DiagnoseResult{Signal: "观望 💤"} // 上方 20% 内有年线级别的套牢盘，不撞墙！
	}

	// -----------------------------------------------------
	// 3. 卖出信号锚定与【机构级风控护盾】
	// -----------------------------------------------------
	if isConverged && isBreakout && isVolumeSurge {
		// =====================================================
		// 💥 [机构级护盾：流动性、市值与资金底牌透视]
		// =====================================================
		if latestFund.TotalMV < 300000 {
			return DiagnoseResult{Signal: "观望 💤"} // 剔除30亿以下微盘股
		}
		if latestFund.TurnoverRate < 4.0 || latestFund.TurnoverRate > 25.0 {
			return DiagnoseResult{Signal: "观望 💤"} // 换手率异常过滤
		}

		flows := ctx.MoneyFlows
		if len(flows) == 0 {
			return DiagnoseResult{Signal: "观望 💤"} // 缺失资金流向数据
		}
		todayFlow := flows[len(flows)-1]
		if todayFlow.NetMfVol <= 0 {
			return DiagnoseResult{Signal: "观望 💤"} // 突破日主力在出逃！一票否决
		}
		netMfWan := todayFlow.NetMfVol * 10000 // 换算为万元
		// =====================================================

		var sellPrice, stopLossPrice float64
		var msg string

		// 💥 动态止盈止损：如果当前估值处于历史 60% 以上的高水位，说明属于偏右侧投机，收紧防线！
		if ctx.PEPercentile > 0.60 {
			msg = fmt.Sprintf("⚠️ [估值分位:%.1f%%] 历史水位偏高！但均线收敛且大阳线突破，主力净流入 %.0f 万。只能做短线，跌破半年线立即逃跑！",
				ctx.PEPercentile*100, netMfWan)
			stopLossPrice = ma120
			sellPrice = today.Close * 1.10
		} else {
			msg = fmt.Sprintf("🎯 [估值分位:%.1f%%|市值:%.0f亿] 完美买点！处于历史低估区，均线高度纠缠，主力暴力净买入 %.0f 万元！中线看涨。",
				ctx.PEPercentile*100, latestFund.TotalMV/10000, netMfWan)
			stopLossPrice = minMA
			sellPrice = 0
		}

		return DiagnoseResult{
			Code: code, StrategyName: m.Name(), LatestPrice: today.Close,
			Signal: "买入 🚀", Message: msg,
			BuyPrice: today.Close, SellPrice: sellPrice, StopLossPrice: stopLossPrice,
		}
	}

	return DiagnoseResult{Signal: "观望 💤"}
}

// ==========================================
// 🔥 策略二：基本面共振·中枢强势突破 (CBBM - 终极状态机版)
// 替代原有的粗暴打板流(BBLU)
// ==========================================
type CBBMAnalyzer struct{}

func (c *CBBMAnalyzer) Name() string { return "中枢强势突破 (CBBM)" }
func (c *CBBMAnalyzer) RequiredData() []string {
	return []string{"klines", "fundamentals", "moneyflow"}
} // 💥 补充了 moneyflow

func (c *CBBMAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	funds := ctx.Fundamentals
	if len(klines) < 120 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if strings.HasPrefix(code, "3") || strings.HasPrefix(code, "688") || strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if !checkFundamentalShield(ctx, false) {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	today := klines[len(klines)-1]

	// -----------------------------------------------------
	// [微观排雷网]
	// -----------------------------------------------------
	isLimitUp := today.PctChg >= 9.5
	isOneWordBoard := today.Open == today.Close && today.Close == today.High
	hasUpperShadow := IsLongUpperShadow(today, 1.5)

	volMa20 := CalcVolMA(klines, 20)
	isVolumeSurge := today.Vol >= (1.5 * volMa20)

	if !isLimitUp || isOneWordBoard || hasUpperShadow || !isVolumeSurge {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	if len(klines) >= 6 {
		startJumpPrice := klines[len(klines)-6].Close
		if (today.Close-startJumpPrice)/startJumpPrice > 0.15 {
			return DiagnoseResult{Signal: "观望 💤"}
		}
	}

	// -----------------------------------------------------
	// [宏观调整 1 & 2]：箱体规律与试盘基因
	// -----------------------------------------------------
	boxUpper, boxLower := GetRealBox(klines, 60)
	if boxLower <= 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	amplitude := (boxUpper - boxLower) / boxLower
	isBoxStable := amplitude <= 0.35
	isBreakout := today.Close > boxUpper
	isCloseToBox := (today.Close-boxUpper)/boxUpper <= 0.15
	isRegularBox := CheckBoxRegularity(klines, 60)

	if !isBoxStable || !isBreakout || !isCloseToBox || !isRegularBox {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	hasProbed := HasProbingAction(klines, boxUpper, 20)

	// -----------------------------------------------------
	// [宏观调整 3]：统一月线级别防雷网
	// -----------------------------------------------------
	room := GetOverheadRoom(klines, today.Close, 120)
	if room < 0.15 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// -----------------------------------------------------
	// [宏观调整 4]：交易剧本重构与【机构级护盾】
	// -----------------------------------------------------
	latestFund := funds[len(funds)-1]

	// =====================================================
	// 💥 [机构级护盾：流动性、市值与资金底牌透视]
	// =====================================================
	if latestFund.TotalMV < 300000 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	if latestFund.TurnoverRate < 4.0 || latestFund.TurnoverRate > 25.0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	flows := ctx.MoneyFlows
	if len(flows) == 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	todayFlow := flows[len(flows)-1]
	if todayFlow.NetMfVol <= 0 {
		return DiagnoseResult{Signal: "观望 💤"} // 无主力资金净流入，属于跟风或诱多
	}
	netMfWan := todayFlow.NetMfVol * 10000
	// =====================================================

	geneMsg := ""
	if hasProbed {
		geneMsg = "🎯 侦测到近期【主力试盘洗盘】动作，突破可信度极高！"
	}

	roomMsg := "已突破近半年高点"
	if room < 1.0 {
		roomMsg = fmt.Sprintf("上方真空区约 %.1f%%", room*100)
	}

	msg := fmt.Sprintf("🔥 [估值分位:%.1f%%|市值:%.0f亿] 放量真突破！主力大单净流入 %.0f 万元！箱体规律(振幅%.1f%%)。%s %s\n"+
		"【明日剧本】绝不盲目追高！\n"+
		"1. 若低开下杀，在支撑位(%.2f)附近企稳买入。\n"+
		"2. 若平开出小阳线，下午确认承接有力后轻仓打底。\n"+
		"3. 若大幅跳空高开，放弃买入防砸盘。",
		ctx.PEPercentile*100, latestFund.TotalMV/10000, netMfWan, amplitude*100, roomMsg, geneMsg, boxUpper)

	return DiagnoseResult{
		Code: code, StrategyName: c.Name(), LatestPrice: today.Close,
		Signal: "买入 🚀", Message: msg,
		BuyPrice:      boxUpper,
		SellPrice:     today.Close * (1.0 + room*0.8),
		StopLossPrice: boxUpper * 0.97,
	}
}

// ==========================================
// 🌊 策略三：深海狙击手 2.0 (DSS V2 - EOD 批处理模型)该算法存在重大问题，暂时不要使用
// ==========================================
type DSSAnalyzer struct{}

func (d *DSSAnalyzer) Name() string           { return "深海狙击手 2.0 (DSS)" }
func (d *DSSAnalyzer) RequiredData() []string { return []string{"klines", "fundamentals"} }

func (d *DSSAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult {
	code := ctx.Code
	klines := ctx.KLines
	funds := ctx.Fundamentals
	// 引擎一：基本面重力引擎 (大级别防雷需要至少 250 天数据)
	if len(klines) < 250 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 绝对安全池：调用严格模式，过滤亏损及历史估值高位的泡沫股
	if !checkFundamentalShield(ctx, true) {
		return DiagnoseResult{Signal: "观望 💤"}
	}
	latestFund := funds[len(funds)-1]
	today := klines[len(klines)-1]

	// 大级别防雷测算：寻找上方 250 天内的历史最高点作为超级阻力位
	resistance250 := 0.0
	scanStart := len(klines) - 250
	scanEnd := len(klines) - 1 // 不含今天
	for i := scanStart; i < scanEnd; i++ {
		if klines[i].High > resistance250 {
			resistance250 = klines[i].High
		}
	}

	// 计算真空区：要求上方至少有 30% 的无阻力空间，否则不参与底部的内卷
	room := (resistance250 - today.Close) / today.Close
	if resistance250 > today.Close && room < 0.30 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	// 引擎二：深海潜伏引擎 (形态、价格、流动性三维共振)
	boxUpper, boxLower := GetRealBox(klines, 60)
	if boxLower <= 0 {
		return DiagnoseResult{Signal: "观望 💤"}
	}

	amplitude := (boxUpper - boxLower) / boxLower
	volMa20 := CalcVolMA(klines, 20)
	volMa60 := CalcVolMA(klines, 60)
	atr14 := CalcATR(klines, 14)

	// 1. 形态收敛：限制上下波动的幅度 (<=30%)，证明主力处于控盘休眠期
	isSpaceCompressed := amplitude <= 0.30

	// 2. 价格极寒：当前价格处于箱体下方的 30% 区域，且不能跌破绝对底线 (未破位崩盘)
	limitPrice := boxLower + 0.30*(boxUpper-boxLower)
	isPriceAtBottom := today.Close <= limitPrice && today.Close > boxLower

	// 3. 流动性枯竭 (灵魂指标)：当天成交量既远小于 20日均量，也远小于 60日均量 (长期资金也睡着了)
	isLiquidityDry := today.Vol < (0.5*volMa20) && today.Vol < (0.5*volMa60)

	if isSpaceCompressed && isPriceAtBottom && isLiquidityDry {
		msg := fmt.Sprintf("⚓ 深海潜伏！当前地量(20日均量%.0f%%, 60日均量%.0f%%)，价格极寒逼近箱底。"+
			"[PE:%.2f] 且上方拥有 %.1f%% 真空区！明日可从容挂单，等待右侧资金抬轿。",
			(today.Vol/volMa20)*100, (today.Vol/volMa60)*100, latestFund.PE, room*100)

		// 引擎三：双轨逃生引擎 (执行路由器)
		return DiagnoseResult{
			Code: code, StrategyName: d.Name(), LatestPrice: today.Close,
			Signal: "买入 🚀", Message: msg,
			BuyPrice:      today.Close,          // 左侧潜伏：直接在 C_t 附近从容挂单
			SellPrice:     boxUpper * 0.99,      // 狂热派发：触及箱体顶部(H_60)回落 1% 卖出，倒给突破客
			StopLossPrice: boxLower - 1.5*atr14, // 防核按钮：L_60 减去 1.5 倍 ATR 动态防线，跌破无条件斩仓！
		}
	}

	return DiagnoseResult{Signal: "观望 💤"}
}

// ==========================================
// 🛡️ 全局风控中心：大盘 Beta 熔断检测
// ==========================================

// CheckMarketEnvironment 评估大盘环境与短线情绪，返回 (是否安全, 诊断报告)
func CheckMarketEnvironment(indices []tushare.IndexDaily, limitUpCount int, avgPremium float64) (bool, string) {
	if len(indices) < 25 {
		return true, "大盘数据不足，全局风控默认放行。"
	}

	today := indices[len(indices)-1]

	// 1. 暴跌熔断：大盘单日暴跌
	if today.PctChg <= -1.5 {
		return false, fmt.Sprintf("⚠️ 全局熔断：上证指数今日暴跌 %.2f%%！倾巢之下无完卵，严禁逆势开仓！", today.PctChg)
	}

	// 2. 趋势熔断：大盘跌破 20日线且向下拐头
	var sum20, sumPrev20 float64
	for i := len(indices) - 20; i < len(indices); i++ {
		sum20 += indices[i].Close
	}
	for i := len(indices) - 21; i < len(indices)-1; i++ {
		sumPrev20 += indices[i].Close
	}
	ma20 := sum20 / 20.0
	prevMa20 := sumPrev20 / 20.0

	if today.Close < ma20 && ma20 < prevMa20 {
		return false, "⚠️ 全局熔断：上证指数跌破 20日线 且趋势向下，处于单边空头区间，停止一切突破买入！"
	}

	// =========================================================
	// 💥 3. 情绪退潮熔断 (2000积分高阶风控)
	// =========================================================
	if limitUpCount > 0 {
		if avgPremium <= -2.0 {
			// 昨天打板的人今天平均亏 2% 以上，说明核按钮遍地，极端恶劣！
			return false, fmt.Sprintf("🧊 情绪冰点熔断：昨日 %d 只涨停股今日平均大跌 %.2f%%！核按钮遍地，短线接力极度恶劣，管住手！", limitUpCount, avgPremium)
		} else if avgPremium < 0 {
			// 负溢价，打板资金没赚钱，短线情绪退潮，假突破极多！
			return false, fmt.Sprintf("⚠️ 情绪退潮熔断：昨日 %d 只涨停股今日平均收益为 %.2f%%。接力资金在亏钱，市场大概率是骗炮行情，暂缓开仓！", limitUpCount, avgPremium)
		}
	}

	premiumMsg := "情绪数据暂缺"
	if limitUpCount > 0 {
		premiumMsg = fmt.Sprintf("昨日涨停股今日平均溢价(吃肉率)为 %.2f%%", avgPremium)
	}

	return true, fmt.Sprintf("✅ 大盘与情绪健康 (指数涨跌: %.2f%%，%s)，允许个股引擎开火。", today.PctChg, premiumMsg)
}

// ==========================================
// ⚙️ 军师联盟注册中心 (Registry - 更新挂载)
// ==========================================
func GetActiveAnalyzers() []Analyzer {
	return []Analyzer{
		&MACBAnalyzer{}, // 均线收敛 (策略一)
		&CBBMAnalyzer{}, // 中枢强势突破 (策略二，替换原 BBLU)
		// &DSSAnalyzer{},  // ⚠️ 深海狙击手 (策略三，存在重大问题，暂时下线)
	}
}

```

## File: tushare/client.go

```go
package tushare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// 💥 废弃 hardcode const，启用并发安全的动态 Token 弹夹
var (
	currentToken = "0badb823100d9849b359c2ef2b4effa7b32bd4e638e74e647e137679" // 默认底火
	tokenMutex   sync.RWMutex
)

const TUSHARE_URL = "https://api.tushare.pro"

// 💥 新增：时间切片分发器 (按年拆分，突破 Tushare 单次 5000 条限制)
func splitDateRange(start, end string, yearsPerChunk int) [][2]string {
	layout := "20060102"
	startTime, err := time.Parse(layout, start)
	if err != nil {
		return [][2]string{{start, end}} // 降级兜底
	}
	endTime, err := time.Parse(layout, end)
	if err != nil {
		return [][2]string{{start, end}}
	}

	var chunks [][2]string
	curr := startTime
	for curr.Before(endTime) || curr.Equal(endTime) {
		next := curr.AddDate(yearsPerChunk, 0, 0)
		if next.After(endTime) {
			next = endTime
		}
		chunks = append(chunks, [2]string{curr.Format(layout), next.Format(layout)})
		curr = next.AddDate(0, 0, 1) // 下一个切片的起点为当前切片终点+1天
	}
	return chunks
}

// SetToken 动态装填高权 Token
func SetToken(newToken string) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	currentToken = newToken
	fmt.Println("🔋 [情报部] Tushare 高阶 Token 已动态装填完毕！")
}

// GetToken 安全读取当前 Token
func GetToken() string {
	tokenMutex.RLock()
	defer tokenMutex.RUnlock()
	return currentToken
}

type TushareRequest struct {
	ApiName string            `json:"api_name"`
	Token   string            `json:"token"`
	Params  map[string]string `json:"params"`
	Fields  string            `json:"fields"`
}

type TushareResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items [][]interface{} `json:"items"`
	} `json:"data"`
}

// 💥 升级：11大金刚全字段
type DailyKLine struct {
	TSCode    string  `json:"ts_code"`
	TradeDate string  `json:"trade_date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	PreClose  float64 `json:"pre_close"`
	Change    float64 `json:"change"`
	PctChg    float64 `json:"pct_chg"`
	Vol       float64 `json:"vol"`
	Amount    float64 `json:"amount"`
	// ==========================================
	// 💥 V2.2 数据血缘与治理字段
	DataSource string `json:"data_source"` // 数据源标记 (如 TUSHARE, EASTMONEY)
	TrustLevel int    `json:"trust_level"` // 可信度权重 (如 100, 40)
	// ==========================================
}

// 极其硬核的类型防错转换器（应对 Tushare 偶尔返回 int 或 nil 的情况）
func parseFloat(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case float32:
		return float64(v)
	}
	return 0
}

// FetchStockHistory 升级版：带防截断时间切片的全量日线拉取
func FetchStockHistory(tsCode string, startDate string, endDate string) ([]DailyKLine, error) {
	// 💥 10年一个切片 (约2500个交易日，绝对不会触发 5000 条截断)
	chunks := splitDateRange(startDate, endDate, 10)
	var allKLines []DailyKLine

	for _, chunk := range chunks {
		reqBody := TushareRequest{
			ApiName: "daily",
			Token:   GetToken(),
			Params: map[string]string{
				"ts_code":    tsCode,
				"start_date": chunk[0], // 使用切片起点
				"end_date":   chunk[1], // 使用切片终点
			},
			Fields: "ts_code,trade_date,open,high,low,close,pre_close,change,pct_chg,vol,amount",
		}

		jsonData, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			return nil, fmt.Errorf("网络请求失败: %v", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var tsResp TushareResponse
		json.Unmarshal(body, &tsResp)

		if tsResp.Code != 0 {
			return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
		}

		for _, item := range tsResp.Data.Items {
			code, _ := item[0].(string)
			date, _ := item[1].(string)
			allKLines = append(allKLines, DailyKLine{
				TSCode:     code,
				TradeDate:  date,
				Open:       parseFloat(item[2]),
				High:       parseFloat(item[3]),
				Low:        parseFloat(item[4]),
				Close:      parseFloat(item[5]),
				PreClose:   parseFloat(item[6]),
				Change:     parseFloat(item[7]),
				PctChg:     parseFloat(item[8]),
				Vol:        parseFloat(item[9]),
				Amount:     parseFloat(item[10]),
				DataSource: "TUSHARE",
				TrustLevel: 100,
			})
		}
	}

	return allKLines, nil
}

// ... 保持上面的代码不动 ...

// StockBasicInfo 定义了公司档案的核心要素
type StockBasicInfo struct {
	TSCode   string
	Name     string
	Industry string
	Market   string
	ListDate string
}

// FetchStockBasic 拉取全市场股票花名册
func FetchStockBasic() ([]StockBasicInfo, error) {
	fmt.Println("🕵️ [情报部] 正在向 Tushare 索要 A 股全市场花名册...")

	reqBody := TushareRequest{
		ApiName: "stock_basic",
		Token:   GetToken(),
		Params: map[string]string{
			"list_status": "L", // 只获取正常上市的股票 (L=上市, D=退市, P=暂停上市)
		},
		// 我们需要：代码，名称，行业，市场类型，上市日期
		Fields: "ts_code,name,industry,market,list_date",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var basics []StockBasicInfo
	for _, item := range tsResp.Data.Items {
		// Tushare 返回的数据可能有 nil，需要做类型断言防错
		tsCode, _ := item[0].(string)
		name, _ := item[1].(string)
		industry, _ := item[2].(string)
		market, _ := item[3].(string)
		listDate, _ := item[4].(string)

		basics = append(basics, StockBasicInfo{
			TSCode:   tsCode,
			Name:     name,
			Industry: industry,
			Market:   market,
			ListDate: listDate,
		})
	}

	fmt.Printf("📦 [情报部] 成功获取 %d 只股票的档案信息！\n", len(basics))
	return basics, nil
}

// ==========================================
// 💎 基本面情报中心 (Daily Basic)
// ==========================================

// DailyFundamental 每日基本面核心指标
type DailyFundamental struct {
	TSCode       string  `json:"ts_code"`
	TradeDate    string  `json:"trade_date"`
	PE           float64 `json:"pe"`
	PB           float64 `json:"pb"`
	TotalMV      float64 `json:"total_mv"`      // 总市值 (万元)
	TurnoverRate float64 `json:"turnover_rate"` // 换手率 (%)
	DVRatio      float64 `json:"dv_ratio"`      // 股息率 (%)
	// 💥 V2.2 数据血缘与治理字段
	DataSource string `json:"data_source"`
	TrustLevel int    `json:"trust_level"`
}

// FetchDailyBasic 拉取指定区间的每日基本面指标
func FetchDailyBasic(tsCode string, startDate string, endDate string) ([]DailyFundamental, error) {
	reqBody := TushareRequest{
		ApiName: "daily_basic",
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		// 索要基本面核心大杀器
		Fields: "ts_code,trade_date,pe,pb,total_mv,turnover_rate,dv_ratio",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 基本面报错: %s", tsResp.Msg)
	}

	var fundamentals []DailyFundamental
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)

		fundamentals = append(fundamentals, DailyFundamental{
			TSCode:       code,
			TradeDate:    date,
			PE:           parseFloat(item[2]),
			PB:           parseFloat(item[3]),
			TotalMV:      parseFloat(item[4]),
			TurnoverRate: parseFloat(item[5]),
			DVRatio:      parseFloat(item[6]),
			DataSource:   "TUSHARE",
			TrustLevel:   100,
		})
	}

	return fundamentals, nil
}

// ==========================================
// 💥 V2.0 新增核心情报兵种
// ==========================================

// TradeCalendar 交易日历
type TradeCalendar struct {
	CalDate string  `json:"cal_date"`
	IsOpen  float64 `json:"is_open"` // Tushare 返回 1 或 0
}

func FetchTradeCalendar(startDate string, endDate string) ([]TradeCalendar, error) {
	reqBody := TushareRequest{
		ApiName: "trade_cal",
		Token:   GetToken(),
		Params: map[string]string{
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "cal_date,is_open",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var calendars []TradeCalendar
	for _, item := range tsResp.Data.Items {
		date, _ := item[0].(string)
		calendars = append(calendars, TradeCalendar{
			CalDate: date,
			IsOpen:  parseFloat(item[1]),
		})
	}
	return calendars, nil
}

// 升级复权因子结构体
type AdjFactor struct {
	TSCode     string  `json:"ts_code"`
	TradeDate  string  `json:"trade_date"`
	AdjFactor  float64 `json:"adj_factor"`
	DataSource string  `json:"data_source"` // 💥 新增血缘
	TrustLevel int     `json:"trust_level"` // 💥 新增权重
}

func FetchAdjFactors(tsCode string, startDate string, endDate string) ([]AdjFactor, error) {
	reqBody := TushareRequest{
		ApiName: "adj_factor",
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "ts_code,trade_date,adj_factor",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var factors []AdjFactor
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)
		factors = append(factors, AdjFactor{
			TSCode:     code,
			TradeDate:  date,
			AdjFactor:  parseFloat(item[2]),
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return factors, nil
}

// FinaIndicator 季报财务指标
type FinaIndicator struct {
	TSCode       string  `json:"ts_code"`
	AnnDate      string  `json:"ann_date"`
	EndDate      string  `json:"end_date"`
	UpdateFlag   string  `json:"update_flag"`
	ROE          float64 `json:"roe"`
	NetProfitYOY float64 `json:"netprofit_yoy"`
	CFPS         float64 `json:"cfps"`
	DataSource   string  `json:"data_source"` // 💥 新增血缘
	TrustLevel   int     `json:"trust_level"` // 💥 新增权重
}

func FetchFinaIndicators(tsCode string, startDate string, endDate string) ([]FinaIndicator, error) {
	reqBody := TushareRequest{
		ApiName: "fina_indicator",
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "ts_code,ann_date,end_date,update_flag,roe,netprofit_yoy,cfps",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var indicators []FinaIndicator
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		annDate, _ := item[1].(string)
		endDate, _ := item[2].(string)
		flag, _ := item[3].(string)

		indicators = append(indicators, FinaIndicator{
			TSCode:       code,
			AnnDate:      annDate,
			EndDate:      endDate,
			UpdateFlag:   flag,
			ROE:          parseFloat(item[4]),
			NetProfitYOY: parseFloat(item[5]),
			CFPS:         parseFloat(item[6]),
			DataSource:   "TUSHARE",
			TrustLevel:   100,
		})
	}
	return indicators, nil
}

// ==========================================
// 💥 V2.0 高阶围猎数据兵种 (需 Tushare 2000 积分)
// ==========================================

// 1. 大单资金流向 (MoneyFlow)
type DailyMoneyFlow struct {
	TSCode     string  `json:"ts_code"`
	TradeDate  string  `json:"trade_date"`
	BuyLgVol   float64 `json:"buy_lg_vol"`
	SellLgVol  float64 `json:"sell_lg_vol"`
	BuyElgVol  float64 `json:"buy_elg_vol"`
	SellElgVol float64 `json:"sell_elg_vol"`
	NetMfVol   float64 `json:"net_mf_vol"`
	DataSource string  `json:"data_source"` // 💥 新增血缘
	TrustLevel int     `json:"trust_level"` // 💥 新增权重
}

func FetchMoneyFlow(tsCode, startDate, endDate string) ([]DailyMoneyFlow, error) {
	reqBody := TushareRequest{
		ApiName: "moneyflow",
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "start_date": startDate, "end_date": endDate},
		Fields:  "ts_code,trade_date,buy_lg_vol,sell_lg_vol,buy_elg_vol,sell_elg_vol,net_mf_vol",
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(TUSHARE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)
	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare报错: %s", tsResp.Msg)
	}

	var flows []DailyMoneyFlow
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)
		flows = append(flows, DailyMoneyFlow{
			TSCode: code, TradeDate: date,
			BuyLgVol: parseFloat(item[2]), SellLgVol: parseFloat(item[3]),
			BuyElgVol: parseFloat(item[4]), SellElgVol: parseFloat(item[5]), NetMfVol: parseFloat(item[6]),
			// 💥 补全血缘：
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return flows, nil
}

// ==========================================
// 💥 降维打击：2000积分专属每日涨跌停价格 (StkLimit)
// ==========================================
type StkLimit struct {
	TradeDate  string  `json:"trade_date"`
	TSCode     string  `json:"ts_code"`
	UpLimit    float64 `json:"up_limit"`
	DownLimit  float64 `json:"down_limit"`
	DataSource string  `json:"data_source"`
	TrustLevel int     `json:"trust_level"`
}

// FetchStkLimit 拉取每日涨跌停绝对价格 (2000积分高射速版)
func FetchStkLimit(tradeDate string) ([]StkLimit, error) {
	reqBody := TushareRequest{
		ApiName: "stk_limit", // 💥 官方正宗 2000 积分接口
		Token:   GetToken(),
		Params:  map[string]string{"trade_date": tradeDate},
		Fields:  "trade_date,ts_code,up_limit,down_limit",
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(TUSHARE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)
	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare报错: %s", tsResp.Msg)
	}

	var limits []StkLimit
	for _, item := range tsResp.Data.Items {
		if len(item) < 4 {
			continue // 防脏数据装甲
		}

		date, _ := item[0].(string)
		code, _ := item[1].(string)

		limits = append(limits, StkLimit{
			TradeDate: date, TSCode: code,
			UpLimit: parseFloat(item[2]), DownLimit: parseFloat(item[3]),
			DataSource: "TUSHARE", TrustLevel: 100,
		})
	}
	return limits, nil
}

// 3. 大盘指数行情 (IndexDaily) - 比如上证指数 000001.SH
type IndexDaily struct {
	TSCode     string  `json:"ts_code"`
	TradeDate  string  `json:"trade_date"`
	Close      float64 `json:"close"`
	Vol        float64 `json:"vol"`
	PctChg     float64 `json:"pct_chg"`
	DataSource string  `json:"data_source"` // 💥 新增血缘
	TrustLevel int     `json:"trust_level"` // 💥 新增权重
}

func FetchIndexDaily(tsCode, startDate, endDate string) ([]IndexDaily, error) {
	reqBody := TushareRequest{
		ApiName: "index_daily",
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "start_date": startDate, "end_date": endDate},
		Fields:  "ts_code,trade_date,close,vol,pct_chg",
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(TUSHARE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	var indices []IndexDaily
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)
		indices = append(indices, IndexDaily{
			TSCode: code, TradeDate: date,
			Close: parseFloat(item[2]), Vol: parseFloat(item[3]), PctChg: parseFloat(item[4]),
			// 💥 补全血缘：
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return indices, nil
}

```

