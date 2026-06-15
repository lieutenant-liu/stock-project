// db_batch.go 提供批量查询数据库的函数。
// 这些函数接受多个股票代码，一次性查询返回结果，避免 N+1 查询问题。
// 返回值统一为 map[string][]T 结构：key 是股票代码，value 是该股票的数据列表。
package db

import (
	"fmt"                    // 格式化字符串，用于拼接 SQL 语句
	"log"                    // 日志输出
	"stock-backend/tushare"  // tushare 数据结构定义（DailyKLine、StockBasicInfo 等）
	"strings"                // 字符串操作，用于 Join 拼接占位符
)

// buildPlaceholders 生成 n 个 SQL 占位符 "?,?,?" 的字符串。
// 参数 n：需要的占位符数量
// 返回值：如 n=3 时返回 "?,?,?"
// 用途：动态构建 IN (?, ?, ...) 子句，支持任意数量的股票代码查询
func buildPlaceholders(n int) string {
	if n <= 0 {
		return "" // 空数组返回空字符串，避免生成无效 SQL
	}
	// make([]string, n) 创建长度为 n 的字符串切片
	placeholders := make([]string, n)
	// range 遍历时只用索引（i），不取值，将每个位置填充为 "?"
	for i := range placeholders {
		placeholders[i] = "?"
	}
	// strings.Join 将切片用逗号连接：["?","?","?"] → "?,?,?"
	return strings.Join(placeholders, ",")
}

// BatchGetKLinesWithAdj 批量获取多只股票的前复权 K 线数据。
// 前复权（Forward Adjusted）：将历史价格按复权因子调整，使价格序列连续可比。
// 参数：
//   codes     - 股票代码列表，如 ["000001.SZ", "600519.SH"]
//   startDate - 开始日期，格式 "20240101"
//   endDate   - 结束日期，格式 "20240131"
// 返回值：map[string][]tushare.DailyKLine，key=股票代码，value=该股票的日K线列表（按日期升序）
//
// 前复权公式：复权价 = 原始价 × 当日adj_factor / 最新adj_factor
// SQL 中使用子查询 latest 获取每只股票在 endDate 之前的最新复权因子作为分母
func BatchGetKLinesWithAdj(codes []string, startDate, endDate string) map[string][]tushare.DailyKLine {
	// make(map) 创建空 map，用于按股票代码分组存储结果
	result := make(map[string][]tushare.DailyKLine)
	if len(codes) == 0 {
		return result // 空输入直接返回，避免执行无效查询
	}

	// 动态生成与股票数量匹配的 SQL 占位符
	placeholders := buildPlaceholders(len(codes))

	// fmt.Sprintf 用 %s 将占位符字符串插入 SQL 模板
	// SQL 解析（从内到外）：
	// 1. 子查询 b：找出每只股票在 endDate 之前最新的复权日期
	// 2. 子查询 latest：根据最新日期获取对应的复权因子（latest_adj）
	// 3. JOIN adj_factors f：获取当日的复权因子
	// 4. 前复权公式：原始价 × 当日adj / 最新adj = 以最新价格为基准的调整价
	// 5. WHERE 过滤股票代码、日期范围、数据可信度（trust_level >= 0 表示有效数据）
	query := fmt.Sprintf(`
		SELECT
			k.ts_code, k.trade_date,
			k.open * f.adj_factor / latest.latest_adj,   -- 前复权开盘价
			k.high * f.adj_factor / latest.latest_adj,   -- 前复权最高价
			k.low * f.adj_factor / latest.latest_adj,    -- 前复权最低价
			k.close * f.adj_factor / latest.latest_adj,  -- 前复权收盘价
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

	// 构建参数列表：interface{} 是 Go 的空接口，可以持有任意类型的值
	// 参数顺序必须与 SQL 中 ? 的出现顺序完全一致
	var args []interface{}
	args = append(args, endDate)          // 第1个 ?：子查询 b 的 trade_date <= ?
	for _, code := range codes {
		args = append(args, code)         // 子查询 b 的 ts_code IN (?,?,...)
	}
	for _, code := range codes {
		args = append(args, code)         // 主查询的 ts_code IN (?,?,...)
	}
	args = append(args, startDate, endDate) // 主查询的日期范围

	// DB.Query 执行查询并返回结果集 rows
	// args... 是 Go 的可变参数展开语法，将切片展开为独立参数传入
	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("批量拉取 K 线失败: %v\n", err)
		return result
	}
	defer rows.Close() // defer 确保函数结束时关闭结果集，释放数据库连接

	// rows.Next() 逐行遍历结果集，返回 false 时结束循环
	for rows.Next() {
		var k tushare.DailyKLine
		// rows.Scan 将当前行的列值映射到结构体字段，参数顺序与 SELECT 列顺序一致
		err := rows.Scan(&k.TSCode, &k.TradeDate, &k.Open, &k.High, &k.Low, &k.Close, &k.Vol, &k.PctChg)
		if err == nil {
			// append 追加到对应股票代码的切片中（map 的零值是 nil，append 对 nil 切片也有效）
			result[k.TSCode] = append(result[k.TSCode], k)
		}
	}
	return result
}

// BatchGetFundamentals 批量获取多只股票的每日基本面数据（估值指标）。
// 参数与返回值结构与 BatchGetKLinesWithAdj 类似。
// 返回的字段：pe（市盈率）、pb（市净率）、total_mv（总市值）、
// dv_ratio（股息率）、turnover_rate（换手率）
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

// BatchGetMoneyFlow 批量获取多只股票的大单资金流向数据。
// 资金流向反映机构和大户的买卖行为，是判断市场主力意图的重要指标。
// 返回字段：buy_lg_vol（大单买入量）、sell_lg_vol（大单卖出量）、
// buy_elg_vol（特大单买入量）、sell_elg_vol（特大单卖出量）、net_mf_vol（净流入量）
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

// BatchGetStkLimit 批量获取多只股票的每日涨跌停价格数据。
// 涨停/跌停是 A 股特有的价格限制机制，到达限制后股票无法继续涨/跌。
// 返回字段：up_limit（涨停价）、down_limit（跌停价）
// 用途：策略中判断股票是否涨停/跌停，决定是否可以买入或卖出
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

// LoadStockNameMap 加载股票代码到名称的映射表。
// 返回 map[string]string：key=股票代码（如 "000001.SZ"），value=股票名称（如 "平安银行"）
// 用途：在前端展示或日志输出时，将代码转换为可读的名称
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

// BatchGetFinaIndicators 批量获取多只股票的季报财务指标。
// 注意：财务数据按 ann_date（公告日）而非 trade_date 筛选，因为财报发布有滞后。
// 返回字段：roe（净资产收益率）、netprofit_yoy（净利润同比增速）、cfps（每股现金流）
// 用途：基本面选股——筛选盈利能力强、增长快、现金流好的公司
func BatchGetFinaIndicators(codes []string, startDate, endDate string) map[string][]tushare.FinaIndicator {
	result := make(map[string][]tushare.FinaIndicator)
	if len(codes) == 0 {
		return result
	}

	placeholders := buildPlaceholders(len(codes))
	query := fmt.Sprintf(`
		SELECT ts_code, ann_date, end_date, roe, netprofit_yoy, cfps
		FROM fina_indicators
		WHERE ts_code IN (%s) AND ann_date >= ? AND ann_date <= ?
		ORDER BY ts_code, ann_date ASC
	`, placeholders)

	var args []interface{}
	for _, code := range codes {
		args = append(args, code)
	}
	args = append(args, startDate, endDate)

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("批量拉取财务指标失败: %v\n", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var f tushare.FinaIndicator
		if err := rows.Scan(&f.TSCode, &f.AnnDate, &f.EndDate, &f.ROE, &f.NetProfitYOY, &f.CFPS); err == nil {
			result[f.TSCode] = append(result[f.TSCode], f)
		}
	}
	return result
}
