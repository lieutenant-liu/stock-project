// db_kline_stock.go 提供 K 线数据和股票基本信息的读写操作。
// 包含：K 线批量写入、K 线查询（原始/前复权）、股票代码查询、指数数据查询等。
package db

import (
	"database/sql"           // 数据库操作，这里用到 sql.NullString 处理可能为 NULL 的查询结果
	"log"                    // 日志输出
	"stock-backend/tushare"  // tushare 数据结构定义
	"time"                   // 时间处理，用于获取当前日期
)

// BatchInsertKLines 批量写入 K 线数据到 daily_klines 表。
// 使用事务（Transaction）和预编译语句（Prepared Statement）确保高效和安全。
// 参数：
//   tsCode - 股票代码（如 "000001.SZ"），所有 klines 属于同一只股票
//   klines - K 线数据切片，每个元素是一天的数据
// 返回值：实际插入/更新的记录数
//
// 技术要点：
//   1. 事务（tx）：保证所有插入要么全部成功，要么全部回滚，不会出现部分写入
//   2. 预编译（Prepare）：SQL 只解析一次，循环中多次执行，大幅提升批量写入性能
//   3. ON CONFLICT DO UPDATE：SQLite 的 upsert 语法——主键冲突时更新而非报错
//   4. WHERE excluded.trust_level >= daily_klines.trust_level：只在新数据可信度更高时才覆盖
func BatchInsertKLines(tsCode string, klines []tushare.DailyKLine) int {
	if len(klines) == 0 {
		return 0 // 空数据直接返回，不做无意义的数据库操作
	}

	// DB.Begin() 开启一个数据库事务。
	// 事务的作用：将多次操作打包成一个原子操作——要么全部成功，要么全部撤销。
	// 对于批量插入，事务还能大幅提升性能（不用每条记录都单独提交）。
	tx, err := DB.Begin()
	if err != nil {
		log.Println("开启事务失败:", err)
		return 0
	}

	// tx.Prepare() 预编译 SQL 语句。
	// 预编译的好处：SQL 语句只解析和编译一次，后续用 stmt.Exec() 执行时只传参数，性能远优于每次都拼接 SQL。
	// 这在循环中尤为重要——几千条数据只需编译一次 SQL。
	//
	// SQL 语义解析：
	// INSERT INTO ... VALUES ... - 插入新记录
	// ON CONFLICT(ts_code, trade_date) - 当主键冲突（同一股票同一日期已存在）时
	// DO UPDATE SET ... - 执行更新操作（SQLite 的 upsert 语法）
	// excluded.* 引用的是本次尝试插入的新值
	// WHERE excluded.trust_level >= daily_klines.trust_level - 只在新数据可信度 >= 旧数据时才更新
	//   这是数据质量控制：高可信度数据不会被低可信度数据覆盖
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
		log.Println("预编译带数据源的SQL失败:", err)
		_ = tx.Rollback() // Rollback 回滚事务，撤销所有未提交的操作
		return 0
	}
	defer stmt.Close() // defer 确保预编译语句在函数结束时被释放

	// 循环执行预编译语句，逐条插入 K 线数据
	insertCount := 0
	for _, k := range klines {
		// stmt.Exec() 执行预编译语句，传入参数替换 SQL 中的 ? 占位符
		res, err := stmt.Exec(
			tsCode, k.TradeDate, k.Open, k.High, k.Low, k.Close,
			k.PreClose, k.Change, k.PctChg, k.Vol, k.Amount,
			k.DataSource, k.TrustLevel,
		)
		if err == nil {
			// RowsAffected() 返回本次操作影响的行数
			// 对于 INSERT：成功插入返回 1；对于 DO UPDATE：实际更新返回 1，未更新（WHERE 不满足）返回 0
			rows, _ := res.RowsAffected()
			if rows > 0 {
				insertCount++
			}
		}
	}

	// tx.Commit() 提交事务，将所有插入操作持久化到数据库
	// 如果不调用 Commit，事务中的所有操作都会在 tx 被垃圾回收时自动回滚
	_ = tx.Commit()
	return insertCount
}

// GetKLinesFromDB 从数据库读取单只股票的原始 K 线数据（未复权）。
// 参数：
//   tsCode    - 股票代码
//   startDate - 开始日期（包含）
//   endDate   - 结束日期（包含）
// 返回值：按日期升序排列的 K 线切片
// 用途：策略计算和邮件报告模块使用
// 注意：这是原始价格，不含复权调整。如需前复权价格，请使用 GetKLinesWithAdj
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

// GetLatestOpenTradeDate 查询截止到指定日期的最近一个交易日（开市日）。
// 参数：
//   asOfDate - 截止日期（格式 "20240131"），为空时默认使用今天
// 返回值：最近交易日的日期字符串，无结果时返回空字符串
// 用途：确定"最新数据"的日期——周末/节假日时，最新数据是上一个交易日的
func GetLatestOpenTradeDate(asOfDate string) string {
	if asOfDate == "" {
		// time.Now().Format("20060102") 是 Go 的日期格式化方式
		// Go 使用固定的时间 "2006-01-02 15:04:05" 作为格式模板（这是 Go 语言的设计特色）
		// "20060102" 对应 2006年01月02日，即 年月日 紧凑格式
		asOfDate = time.Now().Format("20060102")
	}
	// sql.NullString 用于处理数据库中可能为 NULL 的字符串字段
	// 如果查询结果为空（无匹配记录），NullString.Valid 为 false
	var latest sql.NullString
	// QueryRow 只返回一行结果（与 Query 返回多行不同）
	// MAX(cal_date) 取满足条件的最大日期（即最近的交易日）
	err := DB.QueryRow(`
		SELECT MAX(cal_date)
		FROM trade_calendar
		WHERE is_open = 1 AND cal_date <= ?
	`, asOfDate).Scan(&latest)
	if err != nil || !latest.Valid {
		return "" // 查询失败或无结果返回空字符串
	}
	return latest.String
}

// BatchInsertStockBasic 批量写入股票基本信息到 stock_basic 表。
// 使用 INSERT OR REPLACE 语义：主键存在时替换整行记录（等价于先 DELETE 再 INSERT）。
// 参数：
//   basics - 股票基本信息切片，包含代码、名称、行业、市场、上市日期
// 返回值：实际写入的记录数
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

// GetAllKLinesForDate 查询全市场所有股票在指定日期的前复权 K 线数据。
// 返回 map[string]tushare.DailyKLine：key=股票代码，value=该股票当天的K线
// 用途：每日截面扫描——在同一天对所有股票进行策略信号计算
// 前复权公式：复权价 = 原始价 × 当日adj_factor / 最新adj_factor
func GetAllKLinesForDate(date string) map[string]tushare.DailyKLine {
	result := make(map[string]tushare.DailyKLine)
	// SQL 解析：
	// 1. 子查询 latest：获取截止到 date 的最新复权因子（所有股票共用同一个最新日期）
	// 2. JOIN adj_factors f：获取当日的复权因子
	// 3. 前复权公式调整 OHLC 价格
	// 4. WHERE k.trade_date = ?：只查询指定日期的数据
	// 注意：这里 latest 子查询不按 ts_code 分组，因为全市场复权因子的最新日期通常是同一天
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

// GetKLinesWithAdj 查询单只股票的前复权 K 线数据（带日期范围）。
// 与 GetAllKLinesForDate 不同：这个函数查询单只股票的一段时间范围。
// 参数：
//   tsCode    - 股票代码
//   startDate - 开始日期（包含）
//   endDate   - 结束日期（包含）
// 返回值：按日期升序排列的前复权 K 线切片
// 用途：持仓建仓时加载历史价格、候选股票分析时加载观察窗口
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

// GetIndexDailyForBacktest 查询大盘指数的日线数据，用于回测中的宏观风控。
// 参数：
//   tsCode    - 指数代码（如 "000001.SH" 上证指数，"000300.SH" 沪深300）
//   startDate - 开始日期
//   endDate   - 结束日期
// 返回值：按日期升序排列的指数日线切片
// 用途：回测时根据大盘走势判断市场环境，决定策略是否需要暂停（如大盘暴跌时减少买入）
func GetIndexDailyForBacktest(tsCode, startDate, endDate string) []tushare.IndexDaily {
	var indices []tushare.IndexDaily
	query := `
		SELECT trade_date, close, vol, pct_chg
		FROM index_daily
		WHERE ts_code = ? AND trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date ASC
	`
	rows, err := DB.Query(query, tsCode, startDate, endDate)
	if err != nil {
		log.Println("查询大盘指数失败:", err)
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

// GetAllStockCodes 从 stock_basic 表获取所有符合条件的股票代码。
// 筛选条件：上市满 180 天（排除新股，因为新股数据不稳定且历史数据不足）
// 返回值：股票代码切片（已按代码排序）
// 用途：策略扫描时获取全市场股票列表
func GetAllStockCodes() []string {
	var codes []string

	// SQL 解析：
	// strftime('%Y%m%d', date('now','-180 day')) - SQLite 内置函数，计算 180 天前的日期
	//   'now' 是当前 UTC 时间，'-180 day' 减去 180 天
	//   strftime 将结果格式化为 'YYYYMMDD' 格式
	// list_date <= ... - 只选择上市日期早于 180 天前的股票（排除次新股）
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
