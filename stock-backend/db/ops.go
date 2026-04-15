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
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ? AND trust_level >= 0
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

// GetLatestOpenTradeDate 返回截止到 asOfDate 的最近一个开市日
func GetLatestOpenTradeDate(asOfDate string) string {
	if asOfDate == "" {
		asOfDate = time.Now().Format("20060102")
	}
	var latest sql.NullString
	err := DB.QueryRow(`
		SELECT MAX(cal_date)
		FROM trade_calendar
		WHERE is_open = 1 AND cal_date <= ?
	`, asOfDate).Scan(&latest)
	if err != nil || !latest.Valid {
		return ""
	}
	return latest.String
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
