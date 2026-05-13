package db

import (
	"log"
	"stock-backend/tushare"
)

// BatchInsertCyqPerf 批量灌注筹码分布数据。
func BatchInsertCyqPerf(tsCode string, perfs []tushare.CyqPerf) int {
	if len(perfs) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启筹码分布事务失败:", err)
		return 0
	}

	stmt, err := tx.Prepare(`
		INSERT INTO cyq_perf_data
		(ts_code, trade_date, profit_pct, winner_rate, cost_5pct, cost_15pct, cost_50pct, cost_85pct, weight_avg, his_low, his_high, data_source, trust_level)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET
			profit_pct = excluded.profit_pct,
			winner_rate = excluded.winner_rate,
			cost_5pct = excluded.cost_5pct,
			cost_15pct = excluded.cost_15pct,
			cost_50pct = excluded.cost_50pct,
			cost_85pct = excluded.cost_85pct,
			weight_avg = excluded.weight_avg,
			his_low = excluded.his_low,
			his_high = excluded.his_high,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= cyq_perf_data.trust_level
	`)
	if err != nil {
		log.Println("预编译筹码分布SQL失败:", err)
		_ = tx.Rollback()
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, p := range perfs {
		res, err := stmt.Exec(
			tsCode, p.TradeDate, p.ProfitPct, p.WinnerRate,
			p.Cost5Pct, p.Cost15Pct, p.Cost50Pct, p.Cost85Pct,
			p.WeightAvg, p.HisLow, p.HisHigh,
			p.DataSource, p.TrustLevel,
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

// BatchInsertStkFactorPro 批量灌注技术因子专业版数据。
func BatchInsertStkFactorPro(tsCode string, factors []tushare.StkFactorPro) int {
	if len(factors) == 0 {
		return 0
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启技术因子事务失败:", err)
		return 0
	}

	stmt, err := tx.Prepare(`
		INSERT INTO stk_factor_pro_data
		(ts_code, trade_date, macd, macd_signal, macd_hist, rsi_6, rsi_12, kdj_k, kdj_d, kdj_j, boll_upper, boll_lower, data_source, trust_level)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts_code, trade_date) DO UPDATE SET
			macd = excluded.macd,
			macd_signal = excluded.macd_signal,
			macd_hist = excluded.macd_hist,
			rsi_6 = excluded.rsi_6,
			rsi_12 = excluded.rsi_12,
			kdj_k = excluded.kdj_k,
			kdj_d = excluded.kdj_d,
			kdj_j = excluded.kdj_j,
			boll_upper = excluded.boll_upper,
			boll_lower = excluded.boll_lower,
			data_source = excluded.data_source,
			trust_level = excluded.trust_level
		WHERE excluded.trust_level >= stk_factor_pro_data.trust_level
	`)
	if err != nil {
		log.Println("预编译技术因子SQL失败:", err)
		_ = tx.Rollback()
		return 0
	}
	defer stmt.Close()

	insertCount := 0
	for _, f := range factors {
		res, err := stmt.Exec(
			tsCode, f.TradeDate,
			f.MACD, f.MACDSignal, f.MACDHist,
			f.RSI6, f.RSI12,
			f.KDJ_K, f.KDJ_D, f.KDJ_J,
			f.BollUpper, f.BollLower,
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

// GetCyqPerfFromDB 查询单只股票某日的筹码分布数据（实盘用）。
func GetCyqPerfFromDB(tsCode, tradeDate string) *tushare.CyqPerf {
	row := DB.QueryRow(`
		SELECT ts_code, trade_date, profit_pct, winner_rate, cost_5pct, cost_15pct, cost_50pct, cost_85pct, weight_avg, his_low, his_high, data_source, trust_level
		FROM cyq_perf_data WHERE ts_code = ? AND trade_date = ?
	`, tsCode, tradeDate)

	var p tushare.CyqPerf
	err := row.Scan(
		&p.TSCode, &p.TradeDate, &p.ProfitPct, &p.WinnerRate,
		&p.Cost5Pct, &p.Cost15Pct, &p.Cost50Pct, &p.Cost85Pct,
		&p.WeightAvg, &p.HisLow, &p.HisHigh,
		&p.DataSource, &p.TrustLevel,
	)
	if err != nil {
		return nil
	}
	return &p
}

// BatchGetCyqPerf 批量查询多只股票在日期范围内的筹码分布数据（回测用）。
func BatchGetCyqPerf(codes []string, startDate, endDate string) map[string][]tushare.CyqPerf {
	result := make(map[string][]tushare.CyqPerf, len(codes))
	if len(codes) == 0 {
		return result
	}

	placeholders := ""
	args := make([]interface{}, 0, len(codes)+2)
	for i, code := range codes {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := DB.Query(`
		SELECT ts_code, trade_date, profit_pct, winner_rate, cost_5pct, cost_15pct, cost_50pct, cost_85pct, weight_avg, his_low, his_high, data_source, trust_level
		FROM cyq_perf_data
		WHERE ts_code IN (`+placeholders+`) AND trade_date >= ? AND trade_date <= ?
		ORDER BY ts_code, trade_date
	`, args...)
	if err != nil {
		log.Println("批量查询筹码分布失败:", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var p tushare.CyqPerf
		if err := rows.Scan(
			&p.TSCode, &p.TradeDate, &p.ProfitPct, &p.WinnerRate,
			&p.Cost5Pct, &p.Cost15Pct, &p.Cost50Pct, &p.Cost85Pct,
			&p.WeightAvg, &p.HisLow, &p.HisHigh,
			&p.DataSource, &p.TrustLevel,
		); err != nil {
			continue
		}
		result[p.TSCode] = append(result[p.TSCode], p)
	}

	return result
}
