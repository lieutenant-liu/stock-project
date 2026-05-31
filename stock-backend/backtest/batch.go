package backtest

import (
	"context"
	"fmt"
	"log"
	"sort"
	"stock-backend/db"
	"stock-backend/stockutil"
	"stock-backend/strategy"
	"stock-backend/sysmon"
	"strings"
	"time"
)

// PlanTaskInput 单个回测任务的输入。
type PlanTaskInput struct {
	TaskID int64
	Config BacktestConfig
}

// PlanTaskOutput 单个回测任务的输出。
type PlanTaskOutput struct {
	TaskID int64
	Result *BacktestResult
	Error  error
}

// dataSignature 用于对任务分组，相同签名的任务共享数据加载。
type dataSignature struct {
	poolKey   string // 排序后的股票池哈希，空=全市场
	startDate string // 组内最早 StartDate
	endDate   string // 组内最晚 EndDate
}

// RunPlan 执行回测计划。
// 核心优化：Chunk 为第一驱动轮，每个 Chunk 只查询一次 DB，组内所有任务共享。
func RunPlan(ctx context.Context, tasks []PlanTaskInput, progressFn func(completed, total int)) []PlanTaskOutput {
	sysmon.Init()
	if len(tasks) == 0 {
		return nil
	}

	// 1. 参数校验 + 板块过滤
	for i := range tasks {
		normalizeConfig(&tasks[i].Config)
	}

	// 2. 按数据签名分组
	groups := groupByDataSignature(tasks)
	log.Printf("[回测计划] %d 个任务 → %d 个数据组", len(tasks), len(groups))

	outputs := make([]PlanTaskOutput, len(tasks))
	taskIdxMap := make(map[int64]int, len(tasks)) // taskID → outputs index
	for i, t := range tasks {
		taskIdxMap[t.TaskID] = i
	}

	completed := 0

	// 3. 逐组执行
	for _, group := range groups {
		select {
		case <-ctx.Done():
			log.Printf("[回测计划] 收到取消指令，停止后续数据组")
			return outputs
		default:
		}
		runTaskGroup(ctx, group, taskIdxMap, outputs, func() {
			completed++
			if progressFn != nil {
				progressFn(completed, len(tasks))
			}
		})
	}

	return outputs
}

// taskGroup 一个数据组：共享相同数据签名的任务集合。
type taskGroup struct {
	sig   dataSignature
	tasks []PlanTaskInput
}

// groupByDataSignature 将任务按 (TargetPool, StartDate, EndDate) 分组。
func groupByDataSignature(tasks []PlanTaskInput) []taskGroup {
	type key struct {
		pool string
		s, e string // minStartDate, maxEndDate
	}

	// 先按 pool 分组，再合并日期区间
	poolGroups := make(map[string][]PlanTaskInput)
	for _, t := range tasks {
		poolKey := buildPoolKey(t.Config.TargetPool)
		poolGroups[poolKey] = append(poolGroups[poolKey], t)
	}

	var groups []taskGroup
	for poolKey, groupTasks := range poolGroups {
		// 在同一 pool 内，如果日期区间有重叠或包含关系，合并为一组
		// 简化处理：取所有任务的最宽日期区间作为统一区间
		minStart := groupTasks[0].Config.StartDate
		maxEnd := groupTasks[0].Config.EndDate
		for _, t := range groupTasks[1:] {
			if t.Config.StartDate < minStart {
				minStart = t.Config.StartDate
			}
			if t.Config.EndDate > maxEnd {
				maxEnd = t.Config.EndDate
			}
		}

		groups = append(groups, taskGroup{
			sig: dataSignature{
				poolKey:   poolKey,
				startDate: minStart,
				endDate:   maxEnd,
			},
			tasks: groupTasks,
		})
	}

	return groups
}

// buildPoolKey 构建股票池的唯一标识。
func buildPoolKey(pool []string) string {
	if len(pool) == 0 {
		return "" // 空 = 全市场
	}
	sorted := make([]string, len(pool))
	copy(sorted, pool)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// normalizeConfig 校验并填充默认值（复用 RunV2 的逻辑）。
func normalizeConfig(cfg *BacktestConfig) {
	if len(cfg.TargetPool) == 0 {
		cfg.TargetPool = db.GetAllStockCodes()
	}
	var mainBoard []string
	for _, code := range cfg.TargetPool {
		if stockutil.IsValidMainBoardCode(code) {
			mainBoard = append(mainBoard, code)
		}
	}
	cfg.TargetPool = mainBoard

	if cfg.InitialCapital <= 0 {
		cfg.InitialCapital = 100000
	}
	if cfg.Commission < 0 {
		cfg.Commission = 0
	}
	if cfg.Strategy == "" {
		cfg.Strategy = "ALL"
	}
	if cfg.PositionSizePct <= 0 || cfg.PositionSizePct > 1 {
		cfg.PositionSizePct = 0.20
	}
}

// runTaskGroup 执行一个数据组：Chunk 流式遍历 → 信号开采 → 组合推演。
func runTaskGroup(
	ctx context.Context,
	group taskGroup,
	taskIdxMap map[int64]int,
	outputs []PlanTaskOutput,
	onTaskComplete func(),
) {
	sig := group.sig
	lookbackStart := SubtractDays(sig.startDate, 365)
	batchChunkSize := sysmon.GetChunkSize()

	// 获取统一股票池
	var targetPool []string
	if sig.poolKey == "" {
		targetPool = db.GetAllStockCodes()
		var mainBoard []string
		for _, code := range targetPool {
			if stockutil.IsValidMainBoardCode(code) {
				mainBoard = append(mainBoard, code)
			}
		}
		targetPool = mainBoard
	} else {
		targetPool = strings.Split(sig.poolKey, ",")
	}

	if len(targetPool) == 0 {
		for _, t := range group.tasks {
			idx := taskIdxMap[t.TaskID]
			outputs[idx] = PlanTaskOutput{TaskID: t.TaskID, Error: fmt.Errorf("股票池为空")}
			onTaskComplete()
		}
		return
	}

	log.Printf("[回测计划] 数据组启动 | 池=%d只 区间=%s~%s lookback=%s | 共%d个任务",
		len(targetPool), sig.startDate, sig.endDate, lookbackStart, len(group.tasks))

	// 加载全局数据（1次）
	indexData := db.GetIndexDailyForBacktest("000001.SH", lookbackStart, sig.endDate)
	isBullMarket := BuildMarketRegimeMap(indexData)
	isStrongMarket := BuildStrongMarketMap(indexData)

	// 读取引擎子策略配置
	engineCfg, err := db.GetEngineConfig()
	if err != nil {
		log.Printf("[回测计划] 引擎配置读取失败，使用默认全开: %v", err)
		engineCfg = db.EngineConfig{EnableRegimeRouter: true, EnableSignalAllocator: true, Enable3DExit: true}
	}

	// 为每个 task 准备信号收集器和 analyzer
	type taskState struct {
		input     PlanTaskInput
		analyzers []strategy.Analyzer
		signals   []TheoreticalTrade
	}
	states := make([]taskState, len(group.tasks))
	for i, t := range group.tasks {
		states[i] = taskState{
			input:     t,
			analyzers: SelectAnalyzers(t.Config.Strategy),
		}
	}

	// ── Chunk 流式遍历：chunk 为第一驱动轮 ──
	total := len(targetPool)
	processed := 0
	maxGroupSignals := MaxGroupSignals(len(group.tasks)) // P3: 组级信号容量

	for i := 0; i < total; i += batchChunkSize {
		// P2: 优雅退出检查
		select {
		case <-ctx.Done():
			log.Printf("[回测计划] 收到取消指令，停止 Chunk 遍历 (已处理 %d/%d)", i, total)
			// 标记未完成的任务为取消
			for j := range states {
				idx := taskIdxMap[states[j].input.TaskID]
				if outputs[idx].Result == nil && outputs[idx].Error == nil {
					outputs[idx] = PlanTaskOutput{TaskID: states[j].input.TaskID, Error: ctx.Err()}
					onTaskComplete()
				}
			}
			return
		default:
		}

		end := i + batchChunkSize
		if end > total {
			end = total
		}
		codes := targetPool[i:end]

		// 加载当前 chunk（6条SQL，仅查这批股票）
		chunk := LoadChunk(codes, lookbackStart, sig.endDate)

		// P3: 检查组级信号总量是否已触及红线
		groupTotal := 0
		for j := range states {
			groupTotal += len(states[j].signals)
		}
		if groupTotal >= maxGroupSignals {
			log.Printf("[回测计划] 触及组级信号容量上限 %d（%d个任务），截断后续 Chunk", maxGroupSignals, len(states))
			break
		}

		// 对组内每个 task 执行信号开采
		for j := range states {
			taskCfg := states[j].input.Config
			chunkSignals := MineChunkSignals(taskCfg, chunk, states[j].analyzers, isBullMarket, isStrongMarket, engineCfg)
			// P1: 单任务信号上限
			if len(states[j].signals)+len(chunkSignals) > MaxSignalsPerTask {
				log.Printf("[回测计划] 任务 #%d 信号数已达上限 %d，截断", states[j].input.TaskID, MaxSignalsPerTask)
				continue
			}
			states[j].signals = append(states[j].signals, chunkSignals...)
		}

		// chunk 数据出作用域，GC 可回收
		processed += len(codes)
		log.Printf("[回测计划] Chunk %d/%d 完成 | 组内任务数=%d", processed, total, len(states))

		// P2: 内存背压
		if sysmon.CheckMemoryBackpressure() {
			time.Sleep(1 * time.Second)
		}
	}

	// ── Phase 2：每个 task 独立执行组合推演 ──
	for j := range states {
		// P2: 优雅退出检查
		select {
		case <-ctx.Done():
			idx := taskIdxMap[states[j].input.TaskID]
			outputs[idx] = PlanTaskOutput{TaskID: states[j].input.TaskID, Error: ctx.Err()}
			onTaskComplete()
			continue
		default:
		}

		t := states[j].input
		idx := taskIdxMap[t.TaskID]

		result, err := phase2PortfolioSim(ctx, t.Config, states[j].signals, engineCfg)
		if err != nil {
			outputs[idx] = PlanTaskOutput{TaskID: t.TaskID, Error: err}
		} else {
			outputs[idx] = PlanTaskOutput{TaskID: t.TaskID, Result: result}
		}
		onTaskComplete()
	}
}
