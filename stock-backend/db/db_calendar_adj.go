package db

import (
	"log"
	"stock-backend/tushare"
)

// BatchInsertTradeCalendar 批量灌注交易日历。
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
		_ = tx.Rollback()
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

	_ = tx.Commit()
	return insertCount
}

// BatchInsertAdjFactors 批量灌注复权因子。
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
		_ = tx.Rollback()
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

	_ = tx.Commit()
	return insertCount
}

// GetAdjFactorsFromDB 从数据库提取复权因子。
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
