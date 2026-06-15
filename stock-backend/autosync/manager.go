package autosync

// ============================================================
// autosync/manager.go - 自动同步任务管理器
// ============================================================
// 这个文件实现了定时自动同步功能。
//
// 【功能说明】
// - 每分钟检查一次是否到达配置的触发时间
// - 交易日自动同步所有数据类型
// - 支持失败重试和网络检测
// - 记录每次运行的详细步骤
//
// 【Go 语言知识点】
// - sync.Mutex: 互斥锁，保护共享状态
// - time.Ticker: 定时器，周期性触发
// - net.DialTimeout: 带超时的网络连接
// - goroutine: 异步执行任务
// ============================================================

import (
	"encoding/json" // JSON 编解码
	"errors"        // 错误处理
	"fmt"           // 格式化输出
	"net"           // 网络操作
	"stock-backend/db"       // 数据库操作
	"stock-backend/feeder"   // 数据采集引擎
	"stock-backend/tushare"  // Tushare API
	"strings"       // 字符串处理
	"sync"          // 同步原语
	"time"          // 时间处理
)

// Manager 自动同步任务管理器。
// 【状态说明】
// - started: 是否已启动（只允许启动一次）
// - running: 当前是否有任务在执行（防止重复执行）
type Manager struct {
	mu      sync.Mutex // 互斥锁，保护状态
	started bool       // 是否已启动
	running bool       // 是否正在运行
}

// NewManager 创建自动任务调度器实例。
func NewManager() *Manager {
	return &Manager{}
}

// Start 只允许启动一次；并在启动时做中断任务状态修复与恢复检查。
// 【Go 语言知识点：sync.Mutex】
// sync.Mutex 是互斥锁，保证同一时间只有一个 goroutine 能访问临界区。
// Lock() 加锁，Unlock() 解锁，defer Unlock() 保证函数结束时解锁。
func (m *Manager) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return // 已启动，跳过
	}
	m.started = true
	m.mu.Unlock()

	// 初始化：确保配置存在
	_ = db.EnsureAutoSyncConfig()

	// 修复：将上次崩溃遗留的"运行中"任务标记为失败
	_ = db.MarkStaleRunningRunsFailed("服务重启，自动任务中断")
	_ = db.MarkStaleRunningRunStepsFailed("服务重启，步骤中断")

	// 启动后优先做一次恢复检查：若当日有失败但无成功，立即补跑
	go m.tryResumeTodayOnBoot()

	// 启动定时检查循环
	go m.loop()
}

// tryResumeTodayOnBoot 启动时检查是否需要补跑。
// 【逻辑】
// 如果今天是交易日，且有失败记录但没有成功记录，立即补跑。
func (m *Manager) tryResumeTodayOnBoot() {
	// 获取配置
	cfg, err := db.GetAutoSyncConfig()
	if err != nil || !cfg.Enabled {
		return // 配置获取失败或未启用
	}

	// 加载时区
	loc, err := loadLocation(cfg.Timezone)
	if err != nil {
		return
	}

	// 获取当前日期
	runDate := time.Now().In(loc).Format("20060102")

	// 检查是否是交易日
	isOpenDay, err := db.IsTradeOpenDate(runDate)
	if err != nil || !isOpenDay {
		return // 不是交易日
	}

	// 检查是否已有成功记录
	successExists, err := db.ExistsSuccessfulAutoSyncRunByDate(runDate)
	if err != nil || successExists {
		return // 已有成功记录
	}

	// 检查是否有任何记录
	existsAny, err := db.ExistsAutoSyncRunByDate(runDate)
	if err != nil || !existsAny {
		return // 没有记录，不需要补跑
	}

	// 当日存在失败/中断记录且尚无成功记录时，启动补跑
	go m.run("boot_resume", runDate, cfg, loc)
}

// loop 每分钟检查一次是否命中定时触发窗口。
func (m *Manager) loop() {
	// 创建每分钟触发的定时器
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// 启动后先检查一次，避免错过窗口
	m.tryRunScheduled()

	// 定时检查
	for range ticker.C {
		m.tryRunScheduled()
	}
}

// IsRunning 返回当前是否有自动任务在执行。
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// beginRun 进入运行态；若已有运行中的任务则返回 false。
func (m *Manager) beginRun() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return false // 已有任务在运行
	}
	m.running = true
	return true
}

// endRun 退出运行态。
func (m *Manager) endRun() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}

// tryRunScheduled 在交易日 + 到达配置时间 + 当日未成功时触发任务。
// 【触发条件】
// 1. 自动同步已启用
// 2. 今天是交易日
// 3. 当前时间已超过配置的触发时间
// 4. 当日没有成功的运行记录
func (m *Manager) tryRunScheduled() {
	// 获取配置
	cfg, err := db.GetAutoSyncConfig()
	if err != nil || !cfg.Enabled {
		return
	}

	// 加载时区
	loc, err := loadLocation(cfg.Timezone)
	if err != nil {
		return
	}

	// 获取当前时间
	now := time.Now().In(loc)
	runDate := now.Format("20060102")

	// 检查是否是交易日
	isOpenDay, err := db.IsTradeOpenDate(runDate)
	if err != nil || !isOpenDay {
		return
	}

	// 检查是否到达触发时间
	triggerAt, err := parseTodayTrigger(now, cfg.DailyRunTime)
	if err != nil || now.Before(triggerAt) {
		return // 还没到时间
	}

	// 检查当日是否已有成功记录
	successExists, err := db.ExistsSuccessfulAutoSyncRunByDate(runDate)
	if err != nil || successExists {
		return // 已成功
	}

	// 检查是否有任何记录（决定是首次还是重试）
	existsAny, err := db.ExistsAutoSyncRunByDate(runDate)
	if err != nil {
		return
	}
	triggerType := "scheduled"
	if existsAny {
		triggerType = "scheduled_resume" // 有失败记录，属于重试
	}

	// 异步执行
	go m.run(triggerType, runDate, cfg, loc)
}

// TriggerNow 提供外部手动触发入口。
// 【触发方式】
// POST /api/auto_sync/run_now
func (m *Manager) TriggerNow() error {
	if m.IsRunning() {
		return errors.New("自动任务正在运行，请稍后再试")
	}

	cfg, err := db.GetAutoSyncConfig()
	if err != nil {
		return err
	}

	loc, err := loadLocation(cfg.Timezone)
	if err != nil {
		return err
	}

	runDate := time.Now().In(loc).Format("20060102")
	go m.run("manual", runDate, cfg, loc)
	return nil
}

// ------------------------------------------------------------
// 核心执行逻辑
// ------------------------------------------------------------

// run 执行完整的自动任务流水线，并写入 run/step 级运行记录。
// 【执行流程】
// 1. 创建运行记录
// 2. 按顺序执行各个同步步骤
// 3. 记录每个步骤的结果
// 4. 汇总最终状态
func (m *Manager) run(triggerType, runDate string, cfg db.AutoSyncConfig, loc *time.Location) {
	// 尝试进入运行态
	if !m.beginRun() {
		feeder.LogMsg("⚠️ [自动任务] 当前已有任务运行中，本次触发跳过")
		return
	}
	defer m.endRun() // 确保退出时释放状态

	// 创建运行记录
	runID, err := db.StartAutoSyncRun(runDate, triggerType)
	if err != nil {
		feeder.LogMsg("❌ [自动任务] 创建运行记录失败: %v", err)
		return
	}

	feeder.LogMsg("🕒 [自动任务] 开始执行 %s run_date=%s", triggerType, runDate)

	// 统计变量
	networkFailures := 0
	summary := map[string]feeder.SyncSummary{}
	var errList []string

	// 计算同步日期范围
	endDate := time.Now().In(loc).Format("20060102")
	startDate := time.Now().In(loc).AddDate(0, 0, -cfg.LookbackDays).Format("20060102")

	// 获取全市场股票代码
	codes := db.GetAllStockCodes()
	if len(codes) == 0 {
		errList = append(errList, "股票代码池为空，请先同步股票基础信息")
	}

	// 获取系统配置
	sysCfg, _ := db.GetSystemConfig()

	// 定义同步步骤
	steps := []struct {
		name string
		fn   func() feeder.SyncSummary
	}{
		{name: "calendar", fn: func() feeder.SyncSummary {
			// 同步交易日历（带重试）
			s := feeder.SyncSummary{Module: "calendar", Total: 1, Targeted: 1}
			var cals []tushare.TradeCalendar
			var e error
			for retry := 0; retry < 3; retry++ {
				feeder.WaitToken() // 限流
				cals, e = tushare.FetchTradeCalendar(startDate, endDate)
				if e == nil {
					break
				}
				time.Sleep(2 * time.Second) // 重试前等待
			}
			if e != nil {
				s.Failed = 1
				return s
			}
			if len(cals) > 0 {
				db.BatchInsertTradeCalendar(cals)
				s.Success = 1
			} else {
				s.Skipped = 1
			}
			return s
		}},
		{name: "basic", fn: func() feeder.SyncSummary {
			// 同步股票花名册（带重试）
			s := feeder.SyncSummary{Module: "basic", Total: 1, Targeted: 1}
			var basics []tushare.StockBasicInfo
			var e error
			for retry := 0; retry < 3; retry++ {
				feeder.WaitToken()
				basics, e = tushare.FetchStockBasic()
				if e == nil {
					break
				}
				time.Sleep(2 * time.Second)
			}
			if e != nil {
				s.Failed = 1
				return s
			}
			if len(basics) > 0 {
				db.BatchInsertStockBasic(basics)
				s.Success = 1
			} else {
				s.Skipped = 1
			}
			return s
		}},
		{name: "index", fn: func() feeder.SyncSummary {
			return feeder.StartSyncIndex(&feeder.TushareProvider{}, startDate, endDate)
		}},
		{name: "kline", fn: func() feeder.SyncSummary {
			return feeder.StartSyncKLine(&feeder.TushareProvider{}, codes, startDate, endDate)
		}},
		{name: "adj", fn: func() feeder.SyncSummary {
			return feeder.StartSyncAdjFactors(&feeder.TushareProvider{}, codes, startDate, endDate)
		}},
		{name: "fund", fn: func() feeder.SyncSummary {
			return feeder.StartSyncFund(&feeder.TushareProvider{}, codes, startDate, endDate)
		}},
		{name: "moneyflow", fn: func() feeder.SyncSummary {
			return feeder.StartSyncMoneyFlow(&feeder.TushareProvider{}, codes, startDate, endDate)
		}},
		{name: "stklimit", fn: func() feeder.SyncSummary {
			return feeder.StartSyncStkLimit(&feeder.TushareProvider{}, startDate, endDate)
		}},
		{name: "fina", fn: func() feeder.SyncSummary {
			return feeder.StartSyncFina(codes, startDate, endDate)
		}},
	}

	// 如果启用了高级数据，添加额外步骤
	if sysCfg.EnableProData {
		steps = append(steps, struct {
			name string
			fn   func() feeder.SyncSummary
		}{name: "cyqperf", fn: func() feeder.SyncSummary {
			return feeder.StartSyncCyqPerf(codes, startDate, endDate)
		}})
		steps = append(steps, struct {
			name string
			fn   func() feeder.SyncSummary
		}{name: "stkfactorpro", fn: func() feeder.SyncSummary {
			return feeder.StartSyncStkFactorPro(codes, startDate, endDate)
		}})
	}

	// 每个步骤都先写入 step 记录，再执行，最后回填统计结果
	for _, step := range steps {
		// 创建步骤记录
		stepID, stepErr := db.StartAutoSyncRunStep(runID, step.name, 1)
		if stepErr != nil {
			errList = append(errList, fmt.Sprintf("%s 步骤记录创建失败: %v", step.name, stepErr))
			break
		}

		// 等待网络可用
		if err := waitForNetwork(cfg.RetryLimit, cfg.RetryBackoffSec, &networkFailures); err != nil {
			errList = append(errList, fmt.Sprintf("%s 阶段网络不可用: %v", step.name, err))
			_ = db.FinishAutoSyncRunStep(stepID, "failed", 0, 0, 1, 0, err.Error())
			break
		}

		// 执行步骤
		s := step.fn()
		summary[step.name] = s

		// 记录步骤结果
		stepStatus := "success"
		stepErrMsg := ""
		if s.Failed > 0 {
			stepStatus = "failed"
			stepErrMsg = fmt.Sprintf("存在失败条目 %d", s.Failed)
			errList = append(errList, fmt.Sprintf("%s %s", step.name, stepErrMsg))
		}
		_ = db.FinishAutoSyncRunStep(stepID, stepStatus, s.Targeted, s.Success, s.Failed, s.Skipped, stepErrMsg)
	}

	// 任一步骤失败会把整次 run 标记为 failed
	status := "success"
	errorMsg := ""
	if len(errList) > 0 {
		status = "failed"
		errorMsg = strings.Join(errList, "; ")
	}

	// 序列化汇总信息
	summaryJSON := ""
	if b, err := json.Marshal(summary); err == nil {
		summaryJSON = string(b)
	}

	// 写入运行结果
	if err := db.FinishAutoSyncRun(runID, status, networkFailures, errorMsg, summaryJSON); err != nil {
		feeder.LogMsg("❌ [自动任务] 写入运行结果失败: %v", err)
	}

	// 更新最后运行日期
	if status == "success" {
		_ = db.SetAutoSyncLastRunDate(runDate)
		feeder.LogMsg("✅ [自动任务] 执行完成 run_date=%s", runDate)
	} else {
		feeder.LogMsg("❌ [自动任务] 执行失败 run_date=%s: %s", runDate, errorMsg)
	}
}

// ------------------------------------------------------------
// 辅助函数
// ------------------------------------------------------------

// loadLocation 加载时区。
func loadLocation(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return time.LoadLocation("Asia/Shanghai") // 默认上海时区
	}
	return time.LoadLocation(name)
}

// parseTodayTrigger 把配置的 HH:MM 解析成当天的触发时间点。
func parseTodayTrigger(now time.Time, runAt string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(runAt), ":")
	if len(parts) != 2 {
		return time.Time{}, errors.New("daily_run_time 格式错误，应为 HH:MM")
	}

	hour := 0
	min := 0
	_, err := fmt.Sscanf(runAt, "%d:%d", &hour, &min)
	if err != nil {
		return time.Time{}, err
	}

	// 验证范围
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return time.Time{}, errors.New("daily_run_time 超出范围")
	}

	// 构造今天的触发时间
	return time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location()), nil
}

// hasNetwork 通过多目标探测判断当前网络是否可用。
// 【探测策略】
// 尝试连接多个目标，只要有一个成功就认为网络可用：
// - api.tushare.pro:443: Tushare API 服务器
// - 8.8.8.8:53: Google DNS 服务器
func hasNetwork() bool {
	targets := []string{"api.tushare.pro:443", "8.8.8.8:53"}
	for _, t := range targets {
		conn, err := net.DialTimeout("tcp", t, 3*time.Second) // 3秒超时
		if err == nil {
			_ = conn.Close()
			return true // 连接成功
		}
	}
	return false // 所有目标都失败
}

// waitForNetwork 带线性退避重试，避免弱网环境下任务立刻失败。
// 【重试策略】
// - 最多重试 retryLimit 次
// - 每次等待时间递增：backoffSec * (i+1)
// - 例如：30s, 60s, 90s, 120s, 150s, 180s
func waitForNetwork(retryLimit int, backoffSec int, networkFailures *int) error {
	if retryLimit <= 0 {
		retryLimit = 6 // 默认重试 6 次
	}
	if backoffSec <= 0 {
		backoffSec = 30 // 默认退避 30 秒
	}

	for i := 0; i < retryLimit; i++ {
		if hasNetwork() {
			return nil // 网络可用
		}

		if networkFailures != nil {
			(*networkFailures)++ // 记录网络失败次数
		}

		// 线性退避：等待时间递增
		sleepSec := backoffSec * (i + 1)
		feeder.LogMsg("🌐 [自动任务] 网络不可用，%ds 后重试 (%d/%d)", sleepSec, i+1, retryLimit)
		time.Sleep(time.Duration(sleepSec) * time.Second)
	}

	return errors.New("重试上限后网络仍不可用")
}
