package db

import (
	"database/sql"
	"fmt"
)

// TableHealth 单张表的健康报告。
type TableHealth struct {
	DisplayName  string         `json:"display_name"`
	ExpectedDays int            `json:"expected_days"`
	ActualDays   int            `json:"actual_days"`
	Completeness float64        `json:"completeness"`
	MissingDates []string       `json:"missing_dates"`
	SourceCount  map[string]int `json:"source_count"`
}

// AuditReport 整体体检报告矩阵。
type AuditReport struct {
	TSCode  string                 `json:"ts_code"`
	Reports map[string]TableHealth `json:"reports"`
}

// RunDataAudit 执行严格的多维数据对账。
func RunDataAudit(tsCode, startDate, endDate string) AuditReport {
	report := AuditReport{
		TSCode:  tsCode,
		Reports: make(map[string]TableHealth),
	}

	var expectedDays int
	_ = DB.QueryRow(`
		SELECT COUNT(cal_date) FROM trade_calendar
		WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?
	`, startDate, endDate).Scan(&expectedDays)

	tables := map[string]string{
		"daily_klines":       "📈 日线量价",
		"daily_fundamentals": "💎 基本面估值",
		"adj_factors":        "🧬 复权因子",
		"daily_moneyflow":    "🌊 资金流向",
	}

	for tableName, displayName := range tables {
		health := TableHealth{
			DisplayName:  displayName,
			ExpectedDays: expectedDays,
			MissingDates: []string{},
			SourceCount:  make(map[string]int),
		}

		queryActual := fmt.Sprintf(`SELECT COUNT(trade_date) FROM %s WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?`, tableName)
		_ = DB.QueryRow(queryActual, tsCode, startDate, endDate).Scan(&health.ActualDays)

		if expectedDays > 0 {
			health.Completeness = float64(health.ActualDays) / float64(expectedDays) * 100
		}

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

// GetDailySyncTaskRange 根据交易日历和信任等级计算需要补齐的数据区间。
func GetDailySyncTaskRange(tableName, tsCode, targetStart, targetEnd string, targetTrust int) (string, string, bool) {
	var listDate string
	errList := DB.QueryRow(`SELECT list_date FROM stock_basic WHERE ts_code = ?`, tsCode).Scan(&listDate)
	if errList == nil && listDate != "" {
		if targetStart < listDate {
			targetStart = listDate
		}
		if targetStart > targetEnd {
			return "", "", false
		}
	}

	var expected int
	_ = DB.QueryRow(`SELECT COUNT(cal_date) FROM trade_calendar WHERE is_open = 1 AND cal_date >= ? AND cal_date <= ?`, targetStart, targetEnd).Scan(&expected)
	if expected == 0 {
		return "", "", false
	}

	var minDate, maxDate sql.NullString
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
		return minDate.String, maxDate.String, true
	}

	return "", "", false
}
