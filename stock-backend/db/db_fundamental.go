package db

import (
	"log"
	"sort"
	"stock-backend/tushare"
)

// BatchInsertFundamentals 带有血缘权重的智能基本面入库。
func BatchInsertFundamentals(tsCode string, fundamentals []tushare.DailyFundamental) int {
	if len(fundamentals) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启基本面事务失败:", err)
		return 0
	}

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
		_ = tx.Rollback()
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, f := range fundamentals {
		res, err := stmt.Exec(
			tsCode, f.TradeDate, f.PE, f.PB, f.TotalMV, f.DVRatio, f.TurnoverRate,
			f.DataSource, f.TrustLevel,
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

// BatchInsertFinaIndicators 批量灌注季报财务。
func BatchInsertFinaIndicators(tsCode string, indicators []tushare.FinaIndicator) int {
	if len(indicators) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启季报事务失败:", err)
		return 0
	}

	stmt, err := tx.Prepare(`
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
		log.Println("预编译季报SQL失败:", err)
		_ = tx.Rollback()
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
	_ = tx.Commit()
	return insertCount
}

// GetFundamentalsFromDB 从本地 SQLite 提取基本面数据供策略引擎使用。
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
		return funds
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.DailyFundamental
		f.TSCode = tsCode
		err := rows.Scan(&f.TradeDate, &f.PE, &f.PB, &f.TotalMV, &f.DVRatio, &f.TurnoverRate)
		if err == nil {
			funds = append(funds, f)
		}
	}
	return funds
}

// GetPEPercentile 计算个股历史 PE 分位数 (0.0 ~ 1.0)。
func GetPEPercentile(tsCode string, endDate string, lookbackDays int) float64 {
	query := `
		SELECT pe
		FROM daily_fundamentals
		WHERE ts_code = ? AND trade_date <= ? AND pe > 0
		ORDER BY trade_date DESC LIMIT ?
	`
	rows, err := DB.Query(query, tsCode, endDate, lookbackDays)
	if err != nil {
		return 0.5
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
				isFirst = false
			}
			peList = append(peList, pe)
		}
	}

	if len(peList) < 100 {
		return 0.5
	}

	sort.Float64s(peList)

	rank := 0
	for i, pe := range peList {
		if currentPE <= pe {
			rank = i
			break
		}
	}

	return float64(rank) / float64(len(peList))
}
