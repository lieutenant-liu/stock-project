package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB() {
	var err error

	DB, err = sql.Open("sqlite", "stocks.db")
	if err != nil {
		log.Fatal("❌ 连接数据库失败: ", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("❌ 数据库通信失败: ", err)
	}
	fmt.Println("🗄️ [数据中心] SQLite 数据库连接成功！")

	// ========================================================
	// 💥 终极并发优化：Termux 防 OOM 与 WAL 极限调优
	// ========================================================
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",    // 读写不阻塞
		"PRAGMA synchronous = NORMAL;",  // 提升 WAL 写入速度
		"PRAGMA cache_size = -10000;",   // 强制限制缓存为 10MB，绝对防御 OOM
		"PRAGMA temp_store = FILE;",     // 放弃 MEMORY 临时表，用 I/O 换取内存安全
		"PRAGMA mmap_size = 268435456;", // 限制 mmap 最大为 256MB
		"PRAGMA busy_timeout = 5000;",   // 应对并发锁争用，超时等待 5 秒
	}

	for _, p := range pragmas {
		if _, err := DB.Exec(p); err != nil {
			log.Fatalf("⚠️ [警告] PRAGMA 注入失败 (%s): %v", p, err)
		}
	}
	fmt.Println("⚡ [数据中心] 防 OOM 屏障已激活，WAL 高并发读写模式就绪！")

	// ========================================================
	// 📊 V2.0 工业级 6 大核心表 (全部剔除自增 ID，启用 WITHOUT ROWID)
	// ========================================================

	// 1. 交易日历 (防空洞检测器)
	createCalTable := `
	CREATE TABLE IF NOT EXISTS trade_calendar (
		cal_date TEXT PRIMARY KEY,
		is_open INTEGER NOT NULL
	) WITHOUT ROWID;`

	// 2. 股票花名册
	createBasicTable := `
	CREATE TABLE IF NOT EXISTS stock_basic (
		ts_code TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		industry TEXT,
		market TEXT,
		list_date TEXT
	) WITHOUT ROWID;`

	// 3. 原始日线量价 (纯净版：剥离均线计算，交由内存或后续视图处理)
	// 3. 原始日线量价 (纯净版)
	createKlineTable := `
	CREATE TABLE IF NOT EXISTS daily_klines (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		open REAL,
		close REAL,
		high REAL,
		low REAL,
		vol REAL,
		amount REAL,
		pre_close REAL,              -- 💥 修复：补回昨收
		change REAL,                 -- 💥 修复：补回涨跌额
		pct_chg REAL,
		data_source TEXT NOT NULL,   -- 血缘标记
		trust_level INTEGER NOT NULL,-- 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_kline_date ON daily_klines(trade_date);`

	// 4. 复权因子表 (💥 V2.0 新核心：解决除权除息导致的断层)
	createAdjTable := `
	CREATE TABLE IF NOT EXISTS adj_factors (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		adj_factor REAL NOT NULL,
		data_source TEXT NOT NULL,   -- 💥 血缘追踪
		trust_level INTEGER NOT NULL,-- 💥 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 5. 每日基本面表 (情绪与估值)
	createFundTable := `
	CREATE TABLE IF NOT EXISTS daily_fundamentals (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		pe REAL,
		pb REAL,
		total_mv REAL,
		dv_ratio REAL,
		turnover_rate REAL,
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 6. 季报财务表 (第一层基本面漏斗)
	createFinaTable := `
	CREATE TABLE IF NOT EXISTS fina_indicators (
		ts_code TEXT NOT NULL,
		end_date TEXT NOT NULL,      
		ann_date TEXT NOT NULL,      
		update_flag TEXT,            
		roe REAL,                    
		netprofit_yoy REAL,          
		cfps REAL,                   
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, end_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_fina_ann_date ON fina_indicators(ann_date);`

	// 7. 大单资金流向表 (透视主力底牌)
	createMoneyFlowTable := `
	CREATE TABLE IF NOT EXISTS daily_moneyflow (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		buy_lg_vol REAL,
		sell_lg_vol REAL,
		buy_elg_vol REAL,
		sell_elg_vol REAL,
		net_mf_vol REAL,
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 8. 每日涨跌停价格表 (V4.0 降维特化版)
	createStkLimitTable := `
	CREATE TABLE IF NOT EXISTS daily_stk_limit (
		trade_date TEXT NOT NULL,
		ts_code TEXT NOT NULL,
		up_limit REAL,
		down_limit REAL,
		data_source TEXT NOT NULL,
		trust_level INTEGER NOT NULL,
		PRIMARY KEY (trade_date, ts_code)
	) WITHOUT ROWID;`

	// 9. 大盘指数表 (系统风控雷达)
	createIndexTable := `
	CREATE TABLE IF NOT EXISTS index_daily (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		close REAL NOT NULL,
		vol REAL NOT NULL,
		pct_chg REAL NOT NULL,
		data_source TEXT NOT NULL,   -- 💥 新增: 血缘标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`
	// 10. 专属持仓表 (极简设计)
	createPositionTable := `
	CREATE TABLE IF NOT EXISTS my_positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts_code TEXT NOT NULL,
		stock_name TEXT NOT NULL,
		hold_volume INTEGER NOT NULL,
		cost_price REAL NOT NULL,
		buy_date TEXT NOT NULL
	);`

	// 💥 黎明扫荡：物理销毁旧时代的打卡本！
	DB.Exec(`DROP TABLE IF EXISTS sync_history;`)
	DB.Exec(`DROP TABLE IF EXISTS sync_history_fund;`)
	DB.Exec(`DROP TABLE IF EXISTS daily_limit_list;`) // 👈 炸毁旧的高阶表

	// 💥 3. 将 createLimitListTable 放回执行队列，去掉 createStkLimitTable
	tables := []string{
		createCalTable, createBasicTable, createKlineTable,
		createAdjTable, createFundTable, createFinaTable,
		createMoneyFlowTable, createStkLimitTable, createIndexTable, createPositionTable, // 👈 替换为新表
	}
	for _, sqlStr := range tables {
		if _, err = DB.Exec(sqlStr); err != nil {
			log.Fatal("❌ 创建 V2 数据表失败: ", err, "\nSQL:", sqlStr)
		}
	}
	// ... 上面是原有的批量建表 for 循环 ...

	// 💥 修复：释放 WAL 读写并发能力
	DB.SetMaxOpenConns(4) // 允许 4 个并发连接 (1个给后台静默写入，3个给前端雷达查询)
	DB.SetMaxIdleConns(2) // 保持适度的长连接池，防止 Termux 频繁创建/销毁套接字资源耗尽

	fmt.Println("🔥 [黎明扫荡] 旧时代打卡本已被焚毁，进入精确对账时代！")
	fmt.Println("🗄️ [数据中心] 究极形态：V2.0 六大核心数据表部署完毕！")
}
