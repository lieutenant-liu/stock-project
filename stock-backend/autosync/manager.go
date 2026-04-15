package autosync

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"stock-backend/db"
	"stock-backend/feeder"
	"stock-backend/mailnotify"
	"stock-backend/tushare"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu      sync.Mutex
	started bool
	running bool
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	_ = db.EnsureAutoSyncConfig()
	_ = db.MarkStaleRunningRunsFailed("服务重启，自动任务中断")
	_ = db.MarkStaleRunningRunStepsFailed("服务重启，步骤中断")

	// 启动后优先做一次恢复检查：若当日有失败但无成功，立即补跑，不必等到下一次定时窗口
	go m.tryResumeTodayOnBoot()

	go m.loop()
}

func (m *Manager) tryResumeTodayOnBoot() {
	cfg, err := db.GetAutoSyncConfig()
	if err != nil || !cfg.Enabled {
		return
	}
	loc, err := loadLocation(cfg.Timezone)
	if err != nil {
		return
	}
	runDate := time.Now().In(loc).Format("20060102")

	isOpenDay, err := db.IsTradeOpenDate(runDate)
	if err != nil || !isOpenDay {
		return
	}

	successExists, err := db.ExistsSuccessfulAutoSyncRunByDate(runDate)
	if err != nil || successExists {
		return
	}
	existsAny, err := db.ExistsAutoSyncRunByDate(runDate)
	if err != nil || !existsAny {
		return
	}

	go m.run("boot_resume", runDate, cfg, loc)
}

func (m *Manager) loop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// 启动后先检查一次，避免错过窗口
	m.tryRunScheduled()

	for range ticker.C {
		m.tryRunScheduled()
	}
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Manager) beginRun() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return false
	}
	m.running = true
	return true
}

func (m *Manager) endRun() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}

func (m *Manager) tryRunScheduled() {
	cfg, err := db.GetAutoSyncConfig()
	if err != nil || !cfg.Enabled {
		return
	}

	loc, err := loadLocation(cfg.Timezone)
	if err != nil {
		return
	}

	now := time.Now().In(loc)
	runDate := now.Format("20060102")

	isOpenDay, err := db.IsTradeOpenDate(runDate)
	if err != nil || !isOpenDay {
		return
	}

	triggerAt, err := parseTodayTrigger(now, cfg.DailyRunTime)
	if err != nil || now.Before(triggerAt) {
		return
	}

	successExists, err := db.ExistsSuccessfulAutoSyncRunByDate(runDate)
	if err != nil || successExists {
		return
	}

	existsAny, err := db.ExistsAutoSyncRunByDate(runDate)
	if err != nil {
		return
	}
	triggerType := "scheduled"
	if existsAny {
		triggerType = "scheduled_resume"
	}

	go m.run(triggerType, runDate, cfg, loc)
}

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

func (m *Manager) run(triggerType, runDate string, cfg db.AutoSyncConfig, loc *time.Location) {
	if !m.beginRun() {
		feeder.LogMsg("⚠️ [自动任务] 当前已有任务运行中，本次触发跳过")
		return
	}
	defer m.endRun()

	runID, err := db.StartAutoSyncRun(runDate, triggerType)
	if err != nil {
		feeder.LogMsg("❌ [自动任务] 创建运行记录失败: %v", err)
		return
	}

	feeder.LogMsg("🕒 [自动任务] 开始执行 %s run_date=%s", triggerType, runDate)

	networkFailures := 0
	summary := map[string]feeder.SyncSummary{}
	var errList []string

	endDate := time.Now().In(loc).Format("20060102")
	startDate := time.Now().In(loc).AddDate(0, 0, -cfg.LookbackDays).Format("20060102")
	codes := db.GetAllStockCodes()
	if len(codes) == 0 {
		errList = append(errList, "股票代码池为空，请先同步股票基础信息")
	}

	steps := []struct {
		name string
		fn   func() feeder.SyncSummary
	}{
		{name: "calendar", fn: func() feeder.SyncSummary {
			s := feeder.SyncSummary{Module: "calendar", Total: 1, Targeted: 1}
			var cals []tushare.TradeCalendar
			var e error
			for retry := 0; retry < 3; retry++ {
				feeder.WaitToken()
				cals, e = tushare.FetchTradeCalendar(startDate, endDate)
				if e == nil {
					break
				}
				time.Sleep(2 * time.Second)
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
		{name: "index", fn: func() feeder.SyncSummary { return feeder.StartSyncIndex(&feeder.TushareProvider{}, startDate, endDate) }},
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
		{name: "fina", fn: func() feeder.SyncSummary { return feeder.StartSyncFina(codes, startDate, endDate) }},
	}

	for _, step := range steps {
		stepID, stepErr := db.StartAutoSyncRunStep(runID, step.name, 1)
		if stepErr != nil {
			errList = append(errList, fmt.Sprintf("%s 步骤记录创建失败: %v", step.name, stepErr))
			break
		}

		if err := waitForNetwork(cfg.RetryLimit, cfg.RetryBackoffSec, &networkFailures); err != nil {
			errList = append(errList, fmt.Sprintf("%s 阶段网络不可用: %v", step.name, err))
			_ = db.FinishAutoSyncRunStep(stepID, "failed", 0, 0, 1, 0, err.Error())
			break
		}

		s := step.fn()
		summary[step.name] = s
		stepStatus := "success"
		stepErrMsg := ""
		if s.Failed > 0 {
			stepStatus = "failed"
			stepErrMsg = fmt.Sprintf("存在失败条目 %d", s.Failed)
			errList = append(errList, fmt.Sprintf("%s %s", step.name, stepErrMsg))
		}
		_ = db.FinishAutoSyncRunStep(stepID, stepStatus, s.Targeted, s.Success, s.Failed, s.Skipped, stepErrMsg)
	}

	status := "success"
	errorMsg := ""
	if len(errList) > 0 {
		status = "failed"
		errorMsg = strings.Join(errList, "; ")
	}

	summaryJSON := ""
	if b, err := json.Marshal(summary); err == nil {
		summaryJSON = string(b)
	}

	if err := db.FinishAutoSyncRun(runID, status, networkFailures, errorMsg, summaryJSON); err != nil {
		feeder.LogMsg("❌ [自动任务] 写入运行结果失败: %v", err)
	}
	if mailErr := mailnotify.AutoSendRunReport(runID); mailErr != nil {
		feeder.LogMsg("⚠️ [自动任务] 自动邮件推送失败: %v", mailErr)
	}
	if status == "success" {
		_ = db.SetAutoSyncLastRunDate(runDate)
		feeder.LogMsg("✅ [自动任务] 执行完成 run_date=%s", runDate)
	} else {
		feeder.LogMsg("❌ [自动任务] 执行失败 run_date=%s: %s", runDate, errorMsg)
	}
}

func loadLocation(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return time.LoadLocation("Asia/Shanghai")
	}
	return time.LoadLocation(name)
}

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
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return time.Time{}, errors.New("daily_run_time 超出范围")
	}
	return time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location()), nil
}

func hasNetwork() bool {
	targets := []string{"api.tushare.pro:443", "8.8.8.8:53"}
	for _, t := range targets {
		conn, err := net.DialTimeout("tcp", t, 3*time.Second)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

func waitForNetwork(retryLimit int, backoffSec int, networkFailures *int) error {
	if retryLimit <= 0 {
		retryLimit = 6
	}
	if backoffSec <= 0 {
		backoffSec = 30
	}

	for i := 0; i < retryLimit; i++ {
		if hasNetwork() {
			return nil
		}
		if networkFailures != nil {
			(*networkFailures)++
		}
		sleepSec := backoffSec * (i + 1)
		feeder.LogMsg("🌐 [自动任务] 网络不可用，%ds 后重试 (%d/%d)", sleepSec, i+1, retryLimit)
		time.Sleep(time.Duration(sleepSec) * time.Second)
	}
	return errors.New("重试上限后网络仍不可用")
}
