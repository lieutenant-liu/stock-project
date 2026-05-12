package db

import (
	"fmt"
	"log"
	"stock-backend/tushare"
	"strings"
)

func buildPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}

// BatchGetKLinesWithAdj 批量获取前复权 K 线 (防停牌退市丢失版)
func BatchGetKLinesWithAdj(codes []string, startDate, endDate string) map[string][]tushare.DailyKLine {
	result := make(map[string][]tushare.DailyKLine)
	if len(codes) == 0 {
		return result
	}

	placeholders := buildPlaceholders(len(codes))

	query := fmt.Sprintf(`
		SELECT
			k.ts_code, k.trade_date,
			k.open * f.adj_factor / latest.latest_adj,
			k.high * f.adj_factor / latest.latest_adj,
			k.low * f.adj_factor / latest.latest_adj,
			k.close * f.adj_factor / latest.latest_adj,
			k.vol, k.pct_chg
		FROM daily_klines k
		JOIN adj_factors f ON k.ts_code = f.ts_code AND k.trade_date = f.trade_date
		JOIN (
			SELECT a.ts_code, a.adj_factor AS latest_adj
			FROM adj_factors a
			INNER JOIN (
				SELECT ts_code, MAX(trade_date) as max_date
				FROM adj_factors
				WHERE trade_date <= ? AND ts_code IN (%s)
				GROUP BY ts_code
			) b ON a.ts_code = b.ts_code AND a.trade_date = b.max_date
		) latest ON k.ts_code = latest.ts_code
		WHERE k.ts_code IN (%s)
		AND k.trade_date >= ? AND k.trade_date <= ?
		AND k.trust_level >= 0
		ORDER BY k.ts_code, k.trade_date ASC
	`, placeholders, placeholders)

	var args []interface{}
	args = append(args, endDate)
	for _, code := range codes {
		args = append(args, code)
	}
	for _, code := range codes {
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("批量拉取 K 线失败: %v\n", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var k tushare.DailyKLine
		err := rows.Scan(&k.TSCode, &k.TradeDate, &k.Open, &k.High, &k.Low, &k.Close, &k.Vol, &k.PctChg)
		if err == nil {
			result[k.TSCode] = append(result[k.TSCode], k)
		}
	}
	return result
}

// BatchGetFundamentals 批量获取基本面数据。
func BatchGetFundamentals(codes []string, startDate, endDate string) map[string][]tushare.DailyFundamental {
	result := make(map[string][]tushare.DailyFundamental)
	if len(codes) == 0 {
		return result
	}

	placeholders := buildPlaceholders(len(codes))
	query := fmt.Sprintf(`
		SELECT ts_code, trade_date, pe, pb, total_mv, dv_ratio, turnover_rate
		FROM daily_fundamentals
		WHERE ts_code IN (%s) AND trade_date >= ? AND trade_date <= ?
		ORDER BY ts_code, trade_date ASC
	`, placeholders)

	var args []interface{}
	for _, code := range codes {
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("批量拉取基本面失败: %v\n", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.DailyFundamental
		if err := rows.Scan(&f.TSCode, &f.TradeDate, &f.PE, &f.PB, &f.TotalMV, &f.DVRatio, &f.TurnoverRate); err == nil {
			result[f.TSCode] = append(result[f.TSCode], f)
		}
	}
	return result
}

// BatchGetMoneyFlow 批量获取资金流向数据。
func BatchGetMoneyFlow(codes []string, startDate, endDate string) map[string][]tushare.DailyMoneyFlow {
	result := make(map[string][]tushare.DailyMoneyFlow)
	if len(codes) == 0 {
		return result
	}

	placeholders := buildPlaceholders(len(codes))
	query := fmt.Sprintf(`
		SELECT ts_code, trade_date, buy_lg_vol, sell_lg_vol, buy_elg_vol, sell_elg_vol, net_mf_vol
		FROM daily_moneyflow
		WHERE ts_code IN (%s) AND trade_date >= ? AND trade_date <= ?
		ORDER BY ts_code, trade_date ASC
	`, placeholders)

	var args []interface{}
	for _, code := range codes {
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("批量拉取资金流失败: %v\n", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.DailyMoneyFlow
		if err := rows.Scan(&f.TSCode, &f.TradeDate, &f.BuyLgVol, &f.SellLgVol, &f.BuyElgVol, &f.SellElgVol, &f.NetMfVol); err == nil {
			result[f.TSCode] = append(result[f.TSCode], f)
		}
	}
	return result
}

// BatchGetStkLimit 批量获取涨跌停数据。
func BatchGetStkLimit(codes []string, startDate, endDate string) map[string][]tushare.StkLimit {
	result := make(map[string][]tushare.StkLimit)
	if len(codes) == 0 {
		return result
	}

	placeholders := buildPlaceholders(len(codes))
	query := fmt.Sprintf(`
		SELECT trade_date, ts_code, up_limit, down_limit
		FROM daily_stk_limit
		WHERE ts_code IN (%s) AND trade_date >= ? AND trade_date <= ?
		ORDER BY ts_code, trade_date ASC
	`, placeholders)

	var args []interface{}
	for _, code := range codes {
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("批量拉取涨跌停失败: %v\n", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var l tushare.StkLimit
		if err := rows.Scan(&l.TradeDate, &l.TSCode, &l.UpLimit, &l.DownLimit); err == nil {
			result[l.TSCode] = append(result[l.TSCode], l)
		}
	}
	return result
}

// LoadStockNameMap 加载股票代码→名称映射。
func LoadStockNameMap() map[string]string {
	result := make(map[string]string)
	rows, err := DB.Query(`SELECT ts_code, name FROM stock_basic`)
	if err != nil {
		log.Printf("加载股票名称映射失败: %v\n", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err == nil {
			result[code] = name
		}
	}
	return result
}
