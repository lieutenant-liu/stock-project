package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"stock-backend/sysmon"

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
		data_source TEXT NOT NULL,   -- 数据源标记
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
		data_source TEXT NOT NULL,   -- 💥 数据溯源
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
		data_source TEXT NOT NULL,   -- 💥 新增: 数据源标记
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
		data_source TEXT NOT NULL,   -- 💥 新增: 数据源标记
		trust_level INTEGER NOT NULL,-- 💥 新增: 权重等级
		PRIMARY KEY (ts_code, end_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_fina_ann_date ON fina_indicators(ann_date);`

	// 7. 大单资金流向表 (资金流向追踪)
	createMoneyFlowTable := `
	CREATE TABLE IF NOT EXISTS daily_moneyflow (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		buy_lg_vol REAL,
		sell_lg_vol REAL,
		buy_elg_vol REAL,
		sell_elg_vol REAL,
		net_mf_vol REAL,
		data_source TEXT NOT NULL,   -- 💥 新增: 数据源标记
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

	// 9. 大盘指数表 (系统风控数据源)
	createIndexTable := `
	CREATE TABLE IF NOT EXISTS index_daily (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		close REAL NOT NULL,
		vol REAL NOT NULL,
		pct_chg REAL NOT NULL,
		data_source TEXT NOT NULL,   -- 💥 新增: 数据源标记
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

	// 17. 筹码分布数据 (cyq_perf)
	createCyqPerfTable := `
	CREATE TABLE IF NOT EXISTS cyq_perf_data (
		ts_code     TEXT NOT NULL,
		trade_date  TEXT NOT NULL,
		profit_pct  REAL NOT NULL DEFAULT 0,
		winner_rate REAL NOT NULL DEFAULT 0,
		cost_5pct   REAL NOT NULL DEFAULT 0,
		cost_15pct  REAL NOT NULL DEFAULT 0,
		cost_50pct  REAL NOT NULL DEFAULT 0,
		cost_85pct  REAL NOT NULL DEFAULT 0,
		weight_avg  REAL NOT NULL DEFAULT 0,
		his_low     REAL NOT NULL DEFAULT 0,
		his_high    REAL NOT NULL DEFAULT 0,
		data_source TEXT NOT NULL DEFAULT '',
		trust_level INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_cyq_perf_trade_date ON cyq_perf_data(trade_date);`

	// 18. 技术因子专业版 (stk_factor_pro)
	createStkFactorProTable := `
	CREATE TABLE IF NOT EXISTS stk_factor_pro_data (
		ts_code     TEXT NOT NULL,
		trade_date  TEXT NOT NULL,
		macd        REAL NOT NULL DEFAULT 0,
		macd_signal REAL NOT NULL DEFAULT 0,
		macd_hist   REAL NOT NULL DEFAULT 0,
		rsi_6       REAL NOT NULL DEFAULT 0,
		rsi_12      REAL NOT NULL DEFAULT 0,
		kdj_k       REAL NOT NULL DEFAULT 0,
		kdj_d       REAL NOT NULL DEFAULT 0,
		kdj_j       REAL NOT NULL DEFAULT 0,
		boll_upper  REAL NOT NULL DEFAULT 0,
		boll_lower  REAL NOT NULL DEFAULT 0,
		data_source TEXT NOT NULL DEFAULT '',
		trust_level INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_stk_factor_pro_trade_date ON stk_factor_pro_data(trade_date);`

	// 19. 系统配置（单行模式）
	createSystemConfigTable := `
	CREATE TABLE IF NOT EXISTS system_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		enable_pro_data INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL
	);`

	createBacktestJobsTable := `
	CREATE TABLE IF NOT EXISTS backtest_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		config_json TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		progress TEXT DEFAULT '',
		result_json TEXT,
		error_msg TEXT DEFAULT '',
		csv_path TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		started_at TEXT DEFAULT '',
		finished_at TEXT DEFAULT ''
	);`

	// 21. 回测计划（一组相关回测任务的容器）
	createBacktestPlansTable := `
	CREATE TABLE IF NOT EXISTS backtest_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		progress TEXT DEFAULT '',
		task_count INTEGER NOT NULL DEFAULT 0,
		completed_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		started_at TEXT DEFAULT '',
		finished_at TEXT DEFAULT ''
	);`

	// 22. 信号实验室任务
	createSignalLabJobsTable := `
	CREATE TABLE IF NOT EXISTS signal_lab_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		strategy TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		progress TEXT DEFAULT '',
		file_path TEXT DEFAULT '',
		error_msg TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		started_at TEXT DEFAULT '',
		finished_at TEXT DEFAULT ''
	);`

	// 23. 回测计划中的单个任务
	createBacktestPlanTasksTable := `
	CREATE TABLE IF NOT EXISTS backtest_plan_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id INTEGER NOT NULL,
		config_json TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		result_json TEXT,
		error_msg TEXT DEFAULT '',
		csv_path TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		finished_at TEXT DEFAULT '',
		FOREIGN KEY(plan_id) REFERENCES backtest_plans(id)
	);
	CREATE INDEX IF NOT EXISTS idx_plan_tasks_plan_id ON backtest_plan_tasks(plan_id);`

	// 清理旧表：销毁过时的打卡本！
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
		createCyqPerfTable, createStkFactorProTable, createSystemConfigTable,
		createBacktestJobsTable, createBacktestPlansTable, createSignalLabJobsTable, createBacktestPlanTasksTable,
	}
	for _, sqlStr := range tables {
		if _, err = DB.Exec(sqlStr); err != nil {
			log.Fatal("❌ 创建 V2 数据表失败: ", err, "\nSQL:", sqlStr)
		}
	}

	// Migration: my_positions 新增 strategy 列（已存在的数据库自动补齐）
	DB.Exec(`ALTER TABLE my_positions ADD COLUMN strategy TEXT NOT NULL DEFAULT ''`)

	// 💥 修复：释放 WAL 读写并发能力
	DB.SetMaxOpenConns(4) // 允许 4 个并发连接 (1个给后台静默写入，3个给前端查询)
	DB.SetMaxIdleConns(2) // 保持适度的长连接池，防止 Termux 频繁创建/销毁套接字资源耗尽

	// Termux 下收紧连接池：与 MaxWorkers=2 对齐，消除 DB 锁排队
	if sysmon.GetEnv().IsTermux {
		DB.SetMaxOpenConns(2)
		DB.SetMaxIdleConns(2)
		DB.Exec("PRAGMA cache_size = -10000;") // 10MB 页缓存，消灭磁盘 Spill 开销
	}

	fmt.Println("[数据清理] 旧时代打卡本已被清除，进入精确对账时代！")
	fmt.Println("🗄️ [数据中心] 究极形态：V2.0 六大核心数据表部署完毕！")
}
