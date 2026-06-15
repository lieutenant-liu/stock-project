// package db 是数据库层（Data Access Layer）。
// 职责：初始化 SQLite 连接、创建全部数据表、提供 CRUD 函数。
// Go 语言中，同目录下的所有 .go 文件共享同一个 package 名，编译时会被合并。
package db

import (
	"database/sql" // Go 标准库：提供通用的数据库操作接口（连接、查询、事务等）
	"fmt"          // 格式化输出，用于打印日志信息
	"log"          // 日志库，Fatal 级别会打印后退出程序
	"os"           // 读取环境变量
	"stock-backend/sysmon" // 项目内部包：系统监控，检测是否运行在 Termux 环境

	// 下划线导入（blank import）：只执行包的 init() 函数来注册 SQLite 驱动，
	// 不直接使用包中的任何导出标识符。这是 Go 中数据库驱动的标准用法。
	_ "modernc.org/sqlite"
)

// DB 是全局数据库连接对象，整个项目通过 db.DB 来访问数据库。
// *sql.DB 是 Go 标准库的数据库句柄类型，它内部维护了一个连接池（connection pool），
// 并发安全，可以被多个 goroutine 同时使用。
// 注意：sql.DB 并不是一个实际的数据库连接，而是一个连接池的抽象。
var DB *sql.DB

// InitDB 初始化数据库：打开连接、配置性能参数、创建全部数据表。
// 这是整个应用启动时必须最先调用的函数之一。
// 函数无参数、无返回值——所有状态通过全局变量 DB 和日志输出。
func InitDB() {
	var err error // Go 的变量声明必须使用，否则编译报错（后面会被赋值）

	// 从环境变量读取数据库文件路径，如果未设置则默认使用 "stocks.db"
	// os.Getenv 返回字符串，空字符串表示未设置该环境变量
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "stocks.db" // 默认值：当前目录下的 stocks.db 文件
	}

	// sql.Open("sqlite", dbPath) 创建数据库连接池。
	// 第一个参数是驱动名（必须与上面 _ import 的驱动注册名一致），
	// 第二个参数是数据源名称（DSN），对于 SQLite 就是文件路径。
	// 注意：sql.Open 不会立即建立连接，只是初始化连接池配置。
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("❌ 连接数据库失败: ", err) // Fatal 会打印日志后调用 os.Exit(1)
	}

	// Ping() 会真正尝试连接数据库，验证连接是否可用。
	// 如果数据库文件不存在或损坏，这里会报错。
	if err = DB.Ping(); err != nil {
		log.Fatal("❌ 数据库通信失败: ", err)
	}
	fmt.Println("🗄️ [数据中心] SQLite 数据库连接成功！")

	// ========================================================
	// SQLite 性能调优（PRAGMA 语句）
	// PRAGMA 是 SQLite 专有的配置指令，用于调整数据库引擎的行为。
	// 这些配置针对 Termux（Android 终端）环境做了特别优化，防止内存溢出（OOM）。
	// ========================================================
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",    // WAL（Write-Ahead Logging）模式：读写不阻塞，允许多个读者和一个写者同时工作
		"PRAGMA synchronous = NORMAL;",  // 同步级别设为 NORMAL：比默认的 FULL 更快，WAL 模式下数据安全性仍然足够
		"PRAGMA cache_size = -10000;",   // 负数表示以 KB 为单位，-10000 = 约 10MB 页缓存，限制内存使用防止 OOM
		"PRAGMA temp_store = FILE;",     // 临时表存储在磁盘文件而非内存，用 I/O 换取内存安全（默认是 MEMORY）
		"PRAGMA mmap_size = 268435456;", // 内存映射 I/O 最大 256MB（268435456 字节），防止 mmap 过度占用虚拟内存
		"PRAGMA busy_timeout = 5000;",   // 当数据库被锁时，等待 5000 毫秒再返回 SQLITE_BUSY 错误，应对并发锁争用
	}

	// 遍历并执行每一条 PRAGMA 配置
	// range 遍历切片时，第一个返回值是索引（这里用 _ 忽略），第二个是值
	for _, p := range pragmas {
		if _, err := DB.Exec(p); err != nil {
			// Fatalf 格式化打印日志后立即退出程序，配置失败是致命错误
			log.Fatalf("⚠️ [警告] PRAGMA 注入失败 (%s): %v", p, err)
		}
	}
	fmt.Println("⚡ [数据中心] 防 OOM 屏障已激活，WAL 高并发读写模式就绪！")

	// ========================================================
	// 数据表定义区
	// 以下是所有业务数据表的 CREATE TABLE 语句。
	// SQLite 中，CREATE TABLE IF NOT EXISTS 表示"如果表不存在才创建"，幂等安全。
	// WITHOUT ROWID 是 SQLite 的优化选项：表使用 PRIMARY KEY 作为聚簇索引，
	// 对于主键查询密集的场景性能更好，且节省存储空间。
	// 注意：带 WITHOUT ROWID 的表必须有显式的 PRIMARY KEY。
	// ========================================================

	// 1. 交易日历表：记录每个日期是否为交易日（开市=1，休市=0）
	// 用途：判断某天是否需要拉取数据，避免在节假日做无效查询
	createCalTable := `
	CREATE TABLE IF NOT EXISTS trade_calendar (
		cal_date TEXT PRIMARY KEY,
		is_open INTEGER NOT NULL
	) WITHOUT ROWID;`

	// 2. 股票基本信息表（花名册）：记录每只股票的代码、名称、行业、市场、上市日期
	// ts_code 是 tushare 的股票代码格式，如 "000001.SZ"（平安银行，深圳市场）
	createBasicTable := `
	CREATE TABLE IF NOT EXISTS stock_basic (
		ts_code TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		industry TEXT,
		market TEXT,
		list_date TEXT
	) WITHOUT ROWID;`

	// 3. 日线 K 线数据表：存储每只股票每个交易日的 OHLCV（开高低收量）数据
	// data_source 标记数据来源（如 "tushare_pro"），trust_level 标记数据可信度（数字越大越可信）
	// 复合主键 (ts_code, trade_date) 确保同一股票同一日期只有一条记录
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

	// 4. 复权因子表：解决股票除权除息（分红、送股、配股）导致的价格断层问题
	// 前复权公式：复权价 = 原始价 × 当日复权因子 / 最新复权因子
	// 使用前复权后，历史价格可以与当前价格直接比较，技术指标计算才有意义
	createAdjTable := `
	CREATE TABLE IF NOT EXISTS adj_factors (
		ts_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		adj_factor REAL NOT NULL,
		data_source TEXT NOT NULL,   -- 💥 数据溯源
		trust_level INTEGER NOT NULL,-- 💥 权重等级
		PRIMARY KEY (ts_code, trade_date)
	) WITHOUT ROWID;`

	// 5. 每日基本面数据表：存储估值和市场情绪指标
	// pe = 市盈率（Price/Earnings），pb = 市净率（Price/Book），total_mv = 总市值
	// dv_ratio = 股息率，turnover_rate = 换手率（反映交易活跃度）
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

	// 6. 季报财务指标表：存储每季度发布的公司财务数据
	// end_date = 财报截止日（如 "20250331" 表示一季报），ann_date = 实际公告日
	// roe = 净资产收益率（Return on Equity），衡量公司盈利能力
	// netprofit_yoy = 净利润同比增长率，cfps = 每股经营现金流
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

	// 7. 大单资金流向表：追踪机构和大户的资金进出
	// buy_lg_vol = 大单买入量，sell_lg_vol = 大单卖出量
	// buy_elg_vol = 特大单买入量，sell_elg_vol = 特大单卖出量
	// net_mf_vol = 净流入量（正数=净流入，负数=净流出）
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

	// 8. 每日涨跌停价格表：记录每只股票每天的涨停价和跌停价
	// 涨停 = 当日涨幅达到上限（主板 10%，创业板/科创板 20%），股票不能再涨
	// 跌停 = 当日跌幅达到下限，股票不能再跌
	// up_limit = 涨停价，down_limit = 跌停价
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

	// 9. 大盘指数日线表：存储大盘指数（如上证指数 000001.SH、沪深300 000300.SH）的数据
	// 用途：宏观风控——当大盘大幅下跌时，策略可能需要暂停买入以规避系统性风险
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
	// 10. 个人持仓表：记录用户的股票持仓信息
	// id 使用自增主键（AUTOINCREMENT），这里不用 WITHOUT ROWID 是因为需要自增 ID
	// hold_volume = 持仓数量（股），cost_price = 成本价，buy_date = 买入日期
	createPositionTable := `
	CREATE TABLE IF NOT EXISTS my_positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts_code TEXT NOT NULL,
		stock_name TEXT NOT NULL,
		hold_volume INTEGER NOT NULL,
		cost_price REAL NOT NULL,
		buy_date TEXT NOT NULL
	);`

	// 11. API Token 池表：管理多个数据源 API 凭证，支持轮换和故障转移
	// provider = 数据提供者（如 "tushare"），token = API 密钥
	// tier = 用户等级（影响 API 调用频率限制），priority = 优先级（数字越小越优先）
	// enabled = 是否启用，is_active = 当前是否在使用，fail_count = 连续失败次数
	// UNIQUE INDEX 确保同一 provider 下 token 不重复
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

	// 12. 自动同步配置表：存储定时自动拉取数据的配置（单行模式，id 固定为 1）
	// daily_run_time = 每天执行时间，lookback_days = 回溯天数（补充拉取最近几天的数据）
	// retry_limit = 失败重试次数，retry_backoff_sec = 重试间隔秒数（退避策略）
	// CHECK (id = 1) 确保表中永远只有一行配置记录
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

	// 13. 自动同步运行记录表：记录每次自动同步任务的执行情况
	// trigger_type = 触发类型（"auto" 定时触发 / "manual" 手动触发）
	// status = 执行状态（"running" / "success" / "failed"）
	// summary_json = 执行摘要的 JSON 字符串，error_msg = 错误信息
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

	// 14. 自动同步步骤记录表：记录每次同步中每个子步骤的执行详情
	// run_id 关联 auto_sync_runs 表的 id（外键），step_name = 步骤名称
	// targeted = 目标数量，success = 成功数量，failed = 失败数量，skipped = 跳过数量
	// FOREIGN KEY 建立与父表的关联，确保数据一致性
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

	// 15. 邮件推送配置表：存储 SMTP 邮件发送配置（单行模式）
	// smtp_host/smtp_port = SMTP 服务器地址和端口，smtp_user/smtp_pass = 登录凭证
	// smtp_from = 发件人地址，subject_prefix = 邮件主题前缀
	// auto_send_daily = 是否每天自动发送策略报告
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

	// 16. 邮件收件人列表：存储策略报告的接收者
	// email 必须唯一（UNIQUE），label = 备注名称，enabled = 是否启用
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

	// 17. 筹码分布数据表：分析不同价位区间的持仓成本分布
	// profit_pct = 获利比例，winner_rate = 获利盘占比
	// cost_5pct/15pct/50pct/85pct = 不同百分位的成本价
	// weight_avg = 加权平均成本，his_low/his_high = 历史最低/最高价
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

	// 18. 技术指标数据表：存储预计算的常用技术分析指标
	// MACD（趋势指标）：macd = DIF 线，macd_signal = DEA 线，macd_hist = 柱状图
	// RSI（超买超卖）：rsi_6 = 6日RSI，rsi_12 = 12日RSI
	// KDJ（随机指标）：kdj_k/d/j 三个值
	// BOLL（布林带）：boll_upper = 上轨，boll_lower = 下轨
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

	// 19. 系统配置表：全局开关配置（单行模式）
	// enable_pro_data = 是否启用专业版数据（如筹码分布、技术指标等，拉取成本更高）
	createSystemConfigTable := `
	CREATE TABLE IF NOT EXISTS system_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		enable_pro_data INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL
	);`

	// 20. 回测任务表：记录策略回测任务的执行状态和结果
	// config_json = 回测配置的 JSON 字符串，status = 任务状态（pending/running/done/error）
	// progress = 进度信息，result_json = 回测结果 JSON，csv_path = 结果 CSV 文件路径
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

	// 21. 回测计划表：将多个相关回测任务组织成一个计划（批量回测）
	// task_count = 总任务数，completed_count = 已完成数，用于计算整体进度
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

	// 22. 信号实验室任务表：运行策略信号扫描和验证的异步任务
	// strategy = 策略名称，file_path = 输出文件路径
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

	// 23. 回测计划子任务表：属于某个回测计划的单个回测任务
	// plan_id 关联回测计划表，每个子任务有独立的配置和结果
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

	// 24. 策略注册表：存储所有可用策略的元信息
	// id = 策略唯一标识，name = 策略名称，category = 分类
	// description = 描述，principle = 策略原理，win_rate = 历史胜率
	// is_enabled = 是否启用（可以临时禁用某个策略）
	createStrategyRegistryTable := `
	CREATE TABLE IF NOT EXISTS strategy_registry (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		category    TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		principle   TEXT NOT NULL DEFAULT '',
		win_rate    REAL NOT NULL DEFAULT 0,
		is_enabled  INTEGER NOT NULL DEFAULT 1
	);`

	// 25. 引擎配置表：存储策略引擎的子策略开关（键值对形式）
	// key = 配置项名称，value = 开关值（1=启用，0=禁用）
	createEngineConfigTable := `
	CREATE TABLE IF NOT EXISTS engine_config (
		key   TEXT PRIMARY KEY,
		value INTEGER NOT NULL DEFAULT 1
	);`

	// 清理旧表：删除已废弃的旧版本数据表，避免占用空间和造成混淆
	// DROP TABLE IF EXISTS 是安全的——表不存在时不会报错
	DB.Exec(`DROP TABLE IF EXISTS sync_history;`)
	DB.Exec(`DROP TABLE IF EXISTS sync_history_fund;`)
	DB.Exec(`DROP TABLE IF EXISTS daily_limit_list;`) // 👈 炸毁旧的高阶表

	// 将所有 CREATE TABLE 语句收集到切片中，统一执行
	tables := []string{
		createCalTable, createBasicTable, createKlineTable,
		createAdjTable, createFundTable, createFinaTable,
		createMoneyFlowTable, createStkLimitTable, createIndexTable, createPositionTable,
		createAPITokenTable, createAutoSyncConfigTable, createAutoSyncRunsTable,
		createAutoSyncRunStepsTable,
		createEmailNotifyConfigTable, createEmailRecipientsTable,
		createCyqPerfTable, createStkFactorProTable, createSystemConfigTable,
		createBacktestJobsTable, createBacktestPlansTable, createSignalLabJobsTable, createBacktestPlanTasksTable,
		createStrategyRegistryTable, createEngineConfigTable,
	}
	// 遍历执行所有建表语句
	// DB.Exec() 用于执行不需要返回结果集的 SQL（如 CREATE、INSERT、UPDATE、DELETE）
	for _, sqlStr := range tables {
		if _, err = DB.Exec(sqlStr); err != nil {
			log.Fatal("❌ 创建 V2 数据表失败: ", err, "\nSQL:", sqlStr)
		}
	}

	// 数据库迁移（Migration）：给已有的 my_positions 表添加 strategy 列
	// ALTER TABLE ... ADD COLUMN 用于给已有表添加新字段
	// 如果列已存在，SQLite 会报错，这里忽略错误（因为已经是幂等操作）
	DB.Exec(`ALTER TABLE my_positions ADD COLUMN strategy TEXT NOT NULL DEFAULT ''`)

	// 连接池配置：控制同时打开的最大连接数和空闲连接数
	// SetMaxOpenConns = 最大打开连接数（包含正在使用和空闲的连接）
	// SetMaxIdleConns = 最大空闲连接数（保留在池中等待复用的连接）
	DB.SetMaxOpenConns(4) // 允许 4 个并发连接（1个给后台写入，3个给前端查询）
	DB.SetMaxIdleConns(2) // 保持 2 个长连接，减少频繁创建/销毁连接的开销

	// Termux（Android 终端）环境特殊优化：
	// 连接数与后台 Worker 数对齐，消除数据库锁排队等待
	if sysmon.GetEnv().IsTermux {
		DB.SetMaxOpenConns(4) // 4 个 Worker 各持独立连接，零等待
		DB.SetMaxIdleConns(4) // 全部保持长连接，避免 Termux 频繁创建套接字
		DB.Exec("PRAGMA cache_size = -20000;") // 增大到 20MB 页缓存，支持 4 路并发查询
	}

	// 初始化策略注册表：从 strategy_registry 表加载策略元信息到内存
	InitStrategyRegistry()

	// 初始化引擎子策略配置：从 engine_config 表加载各子策略的开关状态
	InitEngineConfig()

	fmt.Println("[数据清理] 旧时代打卡本已被清除，进入精确对账时代！")
	fmt.Println("🗄️ [数据中心] 究极形态：V2.0 六大核心数据表部署完毕！")
}
