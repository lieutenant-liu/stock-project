package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB() {
	var err error

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "stocks.db"
	}

	DB, err = sql.Open("sqlite", dbPath)
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

	// 11. API Token 池 (多凭证管理)
	createAPITokenTable := `
	CREATE TABLE IF NOT EXISTS api_tokens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		token TEXT NOT NULL,
		tier TEXT DEFAULT '',
		priority INTEGER NOT NULL DEFAULT 100,
		enabled INTEGER NOT NULL DEFAULT 1,
		is_active INTEGER NOT NULL DEFAULT 0,
		fail_count INTEGER NOT NULL DEFAULT 0,
		last_ok_at TEXT DEFAULT '',
		last_err_at TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_provider_token ON api_tokens(provider, token);
	CREATE INDEX IF NOT EXISTS idx_api_tokens_provider_active ON api_tokens(provider, is_active, enabled, priority);`

	// 12. 自动同步配置 (单行配置)
	createAutoSyncConfigTable := `
	CREATE TABLE IF NOT EXISTS auto_sync_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		enabled INTEGER NOT NULL DEFAULT 0,
		timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai',
		daily_run_time TEXT NOT NULL DEFAULT '19:00',
		lookback_days INTEGER NOT NULL DEFAULT 7,
		retry_limit INTEGER NOT NULL DEFAULT 6,
		retry_backoff_sec INTEGER NOT NULL DEFAULT 30,
		last_run_date TEXT DEFAULT '',
		updated_at TEXT NOT NULL
	);`

	// 13. 自动同步运行记录
	createAutoSyncRunsTable := `
	CREATE TABLE IF NOT EXISTS auto_sync_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_date TEXT NOT NULL,
		trigger_type TEXT NOT NULL,
		status TEXT NOT NULL,
		started_at TEXT NOT NULL,
		finished_at TEXT DEFAULT '',
		network_failures INTEGER NOT NULL DEFAULT 0,
		error_msg TEXT DEFAULT '',
		summary_json TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_auto_sync_runs_date ON auto_sync_runs(run_date);
	CREATE INDEX IF NOT EXISTS idx_auto_sync_runs_status ON auto_sync_runs(status);`

	// 14. 自动同步步骤级检查点
	createAutoSyncRunStepsTable := `
	CREATE TABLE IF NOT EXISTS auto_sync_run_steps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER NOT NULL,
		step_name TEXT NOT NULL,
		attempt INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL,
		targeted INTEGER NOT NULL DEFAULT 0,
		success INTEGER NOT NULL DEFAULT 0,
		failed INTEGER NOT NULL DEFAULT 0,
		skipped INTEGER NOT NULL DEFAULT 0,
		error_msg TEXT DEFAULT '',
		started_at TEXT NOT NULL,
		finished_at TEXT DEFAULT '',
		updated_at TEXT NOT NULL,
		FOREIGN KEY(run_id) REFERENCES auto_sync_runs(id)
	);
	CREATE INDEX IF NOT EXISTS idx_auto_sync_run_steps_run_id ON auto_sync_run_steps(run_id);
	CREATE INDEX IF NOT EXISTS idx_auto_sync_run_steps_status ON auto_sync_run_steps(status);`

	// 15. 邮件推送配置
	createEmailNotifyConfigTable := `
	CREATE TABLE IF NOT EXISTS email_notify_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		enabled INTEGER NOT NULL DEFAULT 0,
		auto_send_daily INTEGER NOT NULL DEFAULT 0,
		smtp_host TEXT NOT NULL DEFAULT '',
		smtp_port INTEGER NOT NULL DEFAULT 587,
		smtp_user TEXT NOT NULL DEFAULT '',
		smtp_pass TEXT NOT NULL DEFAULT '',
		smtp_from TEXT NOT NULL DEFAULT '',
		subject_prefix TEXT NOT NULL DEFAULT '[Stock-Strategy]',
		updated_at TEXT NOT NULL
	);`

	// 16. 收件人列表
	createEmailRecipientsTable := `
	CREATE TABLE IF NOT EXISTS email_recipients (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		label TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_email_recipients_enabled ON email_recipients(enabled);`

	// 💥 黎明扫荡：物理销毁旧时代的打卡本！
	DB.Exec(`DROP TABLE IF EXISTS sync_history;`)
	DB.Exec(`DROP TABLE IF EXISTS sync_history_fund;`)
	DB.Exec(`DROP TABLE IF EXISTS daily_limit_list;`) // 👈 炸毁旧的高阶表

	// 💥 3. 将 createLimitListTable 放回执行队列，去掉 createStkLimitTable
	tables := []string{
		createCalTable, createBasicTable, createKlineTable,
		createAdjTable, createFundTable, createFinaTable,
		createMoneyFlowTable, createStkLimitTable, createIndexTable, createPositionTable,
		createAPITokenTable, createAutoSyncConfigTable, createAutoSyncRunsTable,
		createAutoSyncRunStepsTable,
		createEmailNotifyConfigTable, createEmailRecipientsTable,
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
