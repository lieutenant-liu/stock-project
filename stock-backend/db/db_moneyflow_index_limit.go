package db

import (
	"database/sql"
	"fmt"
	"log"
	"stock-backend/tushare"
	"time"
)

// BatchInsertMoneyFlow 资金流向入库 (带数据源校验)。
func BatchInsertMoneyFlow(tsCode string, flows []tushare.DailyMoneyFlow) int {
	if len(flows) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启资金流事务失败:", err)
		return 0
	}
	stmt, err := tx.Prepare(`
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
		_ = tx.Rollback()
		return 0
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
	_ = tx.Commit()
	return insertCount
}

// BatchInsertIndexDaily 大盘指数入库 (带数据源校验)。
func BatchInsertIndexDaily(tsCode string, indices []tushare.IndexDaily) int {
	if len(indices) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启指数事务失败:", err)
		return 0
	}
	stmt, err := tx.Prepare(`
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
		_ = tx.Rollback()
		return 0
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
	_ = tx.Commit()
	return insertCount
}

// GetMoneyFlowFromDB 从本地 SQLite 提取资金流向数据。
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

// GetIndexDailyFromDB 提取大盘指数数据 (用于全局风控)。
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

// BatchInsertStkLimit 涨跌停绝对价格入库 (带数据源校验)。
func BatchInsertStkLimit(limits []tushare.StkLimit) int {
	if len(limits) == 0 {
		return 0
	}
	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启涨跌停事务失败:", err)
		return 0
	}
	stmt, err := tx.Prepare(`
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
		_ = tx.Rollback()
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
	_ = tx.Commit()
	return insertCount
}

// GetStkLimitFromDB 提取涨跌停价格数据（供回测引擎做流动性过滤）。
func GetStkLimitFromDB(tsCode string, startDate string, endDate string) []tushare.StkLimit {
	var limits []tushare.StkLimit
	query := `
		SELECT trade_date, ts_code, up_limit, down_limit
		FROM daily_stk_limit
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		return limits
	}
	defer rows.Close()

	for rows.Next() {
		var l tushare.StkLimit
		l.TSCode = tsCode
		if err := rows.Scan(&l.TradeDate, &l.TSCode, &l.UpLimit, &l.DownLimit); err == nil {
			limits = append(limits, l)
		}
	}
	return limits
}

// GetAllLimitsForDate 批量查询全市场某天的涨跌停价格。
func GetAllLimitsForDate(date string) map[string]tushare.StkLimit {
	result := make(map[string]tushare.StkLimit)
	query := `SELECT ts_code, up_limit, down_limit FROM daily_stk_limit WHERE trade_date = ?`
	rows, err := DB.Query(query, date)
	if err != nil {
		log.Println("批量查询涨跌停失败:", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var l tushare.StkLimit
		l.TradeDate = date
		if err := rows.Scan(&l.TSCode, &l.UpLimit, &l.DownLimit); err == nil {
			result[l.TSCode] = l
		}
	}
	return result
}

// GetLimitUpPremium 通过 K 线收盘价与涨停价联合透视计算溢价率。
func GetLimitUpPremium(yesterday string, today string) (int, float64) {
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

	err := DB.QueryRow(query, yesterday, yesterday, today).Scan(&count, &avgPremium)
	if err != nil || count == 0 || !avgPremium.Valid {
		return 0, 0.0
	}

	return count, avgPremium.Float64
}

// GetMarketMissingDates 提取全市场级表的缺失日期列表。
func GetMarketMissingDates(tableName, targetStart, targetEnd string) []string {
	if targetStart == "" {
		_ = DB.QueryRow(`SELECT MIN(cal_date) FROM trade_calendar WHERE is_open = 1`).Scan(&targetStart)
	}
	if targetEnd == "" {
		targetEnd = time.Now().Format("20060102")
	}

	if targetStart > targetEnd {
		targetStart, targetEnd = targetEnd, targetStart
	}

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

// GetStockMissingDates 查询某只股票在指定表中的缺失日期列表。
func GetStockMissingDates(tableName, tsCode, targetStart, targetEnd string) []string {
	if targetStart == "" || targetEnd == "" {
		return nil
	}

	query := fmt.Sprintf(`
		SELECT cal_date FROM trade_calendar
		WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?
		AND cal_date NOT IN (
			SELECT DISTINCT trade_date FROM %s WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		) ORDER BY cal_date ASC
	`, tableName)

	var missingDates []string
	rows, err := DB.Query(query, targetStart, targetEnd, tsCode, targetStart, targetEnd)
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
