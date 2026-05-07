package db

import (
	"database/sql"
	"log"
	"stock-backend/tushare"
	"time"
)

// BatchInsertKLines 直接接收 tushare.DailyKLine 切片，存入 11 个字段。
func BatchInsertKLines(tsCode string, klines []tushare.DailyKLine) int {
	if len(klines) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启事务失败:", err)
		return 0
	}

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
		_ = tx.Rollback()
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, k := range klines {
		res, err := stmt.Exec(
			tsCode, k.TradeDate, k.Open, k.High, k.Low, k.Close,
			k.PreClose, k.Change, k.PctChg, k.Vol, k.Amount,
			k.DataSource, k.TrustLevel,
		)
		if err == nil {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}

	_ = tx.Commit()
	return insertCount
}

// GetKLinesFromDB 读取 K 线数据供策略和邮件模块使用。
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
		k.TSCode = tsCode
		err := rows.Scan(&k.TradeDate, &k.Open, &k.High, &k.Low, &k.Close, &k.PreClose, &k.Change, &k.PctChg, &k.Vol, &k.Amount)
		if err == nil {
			klines = append(klines, k)
		}
	}
	return klines
}

// GetLatestOpenTradeDate 返回截止到 asOfDate 的最近一个开市日。
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

// BatchInsertStockBasic 批量保存全市场花名册。
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
		_ = tx.Rollback()
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
	_ = tx.Commit()
	return insertCount
}

// GetAllKLinesForDate 批量查询全市场某天的前复权K线。
// 返回 tsCode → DailyKLine 的 map，用于每日截面扫描。
func GetAllKLinesForDate(date string) map[string]tushare.DailyKLine {
	result := make(map[string]tushare.DailyKLine)
	query := `
		SELECT k.ts_code, k.trade_date,
			k.open * f.adj_factor / latest.latest_adj,
			k.high * f.adj_factor / latest.latest_adj,
			k.low * f.adj_factor / latest.latest_adj,
			k.close * f.adj_factor / latest.latest_adj,
			k.vol, k.pct_chg
		FROM daily_klines k
		JOIN adj_factors f ON k.ts_code = f.ts_code AND k.trade_date = f.trade_date
		JOIN (
			SELECT ts_code, adj_factor AS latest_adj
			FROM adj_factors
			WHERE trade_date = (SELECT MAX(trade_date) FROM adj_factors WHERE trade_date <= ?)
		) latest ON k.ts_code = latest.ts_code
		WHERE k.trade_date = ? AND k.trust_level >= 0
	`
	rows, err := DB.Query(query, date, date)
	if err != nil {
		log.Println("批量查询日K线失败:", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var k tushare.DailyKLine
		if err := rows.Scan(&k.TSCode, &k.TradeDate, &k.Open, &k.High, &k.Low, &k.Close, &k.Vol, &k.PctChg); err == nil {
			result[k.TSCode] = k
		}
	}
	return result
}

// GetKLinesWithAdj 查询单只股票的历史K线并前复权。
// 用于持仓建仓时加载历史、候选股票分析时加载窗口。
func GetKLinesWithAdj(tsCode string, startDate string, endDate string) []tushare.DailyKLine {
	var klines []tushare.DailyKLine
	query := `
		SELECT k.trade_date,
			k.open * f.adj_factor / latest.latest_adj,
			k.high * f.adj_factor / latest.latest_adj,
			k.low * f.adj_factor / latest.latest_adj,
			k.close * f.adj_factor / latest.latest_adj,
			k.vol, k.pct_chg
		FROM daily_klines k
		JOIN adj_factors f ON k.ts_code = f.ts_code AND k.trade_date = f.trade_date
		JOIN (
			SELECT ts_code, adj_factor AS latest_adj
			FROM adj_factors
			WHERE ts_code = ? AND trade_date = (SELECT MAX(trade_date) FROM adj_factors WHERE ts_code = ? AND trade_date <= ?)
		) latest ON k.ts_code = latest.ts_code
		WHERE k.ts_code = ? AND k.trade_date >= ? AND k.trade_date <= ? AND k.trust_level >= 0
		ORDER BY k.trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, tsCode, endDate, tsCode, startDate, endDate)
	if err != nil {
		log.Println("查询复权K线失败:", err)
		return klines
	}
	defer rows.Close()

	for rows.Next() {
		var k tushare.DailyKLine
		k.TSCode = tsCode
		if err := rows.Scan(&k.TradeDate, &k.Open, &k.High, &k.Low, &k.Close, &k.Vol, &k.PctChg); err == nil {
			klines = append(klines, k)
		}
	}
	return klines
}

// GetAllStockCodes 从数据库提取全市场股票代码。
func GetAllStockCodes() []string {
	var codes []string

	query := `
		SELECT ts_code FROM stock_basic
		WHERE list_date <= strftime('%Y%m%d', date('now','-180 day'))
		ORDER BY ts_code ASC
	`
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
