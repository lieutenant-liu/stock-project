package signallab

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"stock-backend/backtest"
	"stock-backend/logger"
	"stock-backend/db"
	"stock-backend/stockutil"
	"stock-backend/strategy"
	"stock-backend/sysmon"
	"stock-backend/tushare"
	"sync"
	"sync/atomic"
	"time"
)

// csvWriteRequest 用于 producer-consumer 模式：chunk 处理 goroutine 将结果发送给 CSV writer goroutine。
type csvWriteRequest struct {
	evals []*SignalEvaluation
}

// csvWriterLoop 单一消费者 goroutine，顺序写入 CSV（非线程安全的 csv.Writer 的唯一使用者）。
func csvWriterLoop(writer *csv.Writer, resultsCh <-chan csvWriteRequest, done chan<- error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("[LAB] CSV Writer panic: %v\n%s", r, debug.Stack())
			done <- fmt.Errorf("csv writer panic: %v", r)
		}
	}()
	for req := range resultsCh {
		if len(req.evals) > 0 {
			if err := writeChunkToCSV(writer, req.evals); err != nil {
				done <- err
				return
			}
		}
	}
	writer.Flush()
	done <- writer.Error()
}

// RunSignalLabJob 执行信号实验室任务（异步 goroutine 入口）。
func RunSignalLabJob(ctx context.Context, jobID int64, cfg SignalLabConfig) {
	// panic recovery（主 goroutine）
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic: %v\n%s", r, debug.Stack())
			log.Printf("[信号实验室] 任务 #%d panic: %v", jobID, r)
			_ = db.FailLabJob(jobID, errMsg)
		}
	}()

	if err := db.UpdateLabJobStatus(jobID, "running", "初始化中..."); err != nil {
		log.Printf("[信号实验室] 更新状态失败: %v", err)
	}

	// 1. 创建 CSV 文件
	exportDir := "exports"
	os.MkdirAll(exportDir, 0o755)
	filePath := filepath.Join(exportDir, fmt.Sprintf("signal_lab_%d.csv", jobID))
	f, err := os.Create(filePath)
	if err != nil {
		_ = db.FailLabJob(jobID, "创建CSV文件失败: "+err.Error())
		return
	}
	defer f.Close()

	// UTF-8 BOM
	f.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(f)

	// CSV 表头
	header := []string{
		"Symbol", "SignalDate", "Strategy", "EntryDate", "EntryOpen", "SignalClose",
		"GapPct", "VolRatio", "MFE20", "DaysToMFE", "MAE20", "DaysToMAE", "MFEBeforeMAE",
		"Ret1D", "Ret3D", "Ret5D", "Ret10D", "Ret20D",
	}
	if err := writer.Write(header); err != nil {
		_ = db.FailLabJob(jobID, "写入CSV表头失败: "+err.Error())
		return
	}
	writer.Flush()

	// 2. 加载大盘指数
	lookbackStart := backtest.SubtractDays(cfg.StartDate, 365)
	indexData := db.GetIndexDailyForBacktest("000001.SH", lookbackStart, cfg.EndDate)
	isBullMarket := backtest.BuildMarketRegimeMap(indexData)
	isStrongMarket := backtest.BuildStrongMarketMap(indexData)
	log.Printf("[信号实验室] 大盘数据 %d 天", len(indexData))

	// 3. 获取股票池
	targetPool := db.GetAllStockCodes()
	var mainBoard []string
	for _, code := range targetPool {
		if stockutil.IsValidMainBoardCode(code) {
			mainBoard = append(mainBoard, code)
		}
	}
	targetPool = mainBoard
	if len(targetPool) == 0 {
		_ = db.FailLabJob(jobID, "股票池为空")
		return
	}

	// 4. 选择策略
	analyzers := backtest.SelectAnalyzers(cfg.Strategy)

	// 5. 自适应资源调度
	sysmon.Init()
	chunkSize := sysmon.GetChunkSize()
	maxWorkers := sysmon.GetMaxWorkers()
	log.Printf("[信号实验室] 股票池 %d 只, 策略 %s, ChunkSize=%d, Workers=%d",
		len(targetPool), cfg.Strategy, chunkSize, maxWorkers)

	// 6. 启动 CSV writer goroutine（producer-consumer 模式）
	resultsCh := make(chan csvWriteRequest) // 无缓冲：chunk 处理完即写，不囤积内存
	csvDone := make(chan error, 1)
	go csvWriterLoop(writer, resultsCh, csvDone)

	// 7. 并发 chunk 处理
	sem := make(chan struct{}, maxWorkers) // 信号量
	var wg sync.WaitGroup
	var processed int64
	var totalSignals int64
	var firstErr error
	var errOnce sync.Once

	total := len(targetPool)

	for i := 0; i < total; i += chunkSize {
		select {
		case <-ctx.Done():
			logger.Warn("[LAB] 任务 #%d 收到取消指令，停止后续分发 (已分发 %d/%d)", jobID, i, total)
			wg.Wait()
			close(resultsCh)
			_ = db.FailLabJob(jobID, "任务被取消")
			return
		default:
		}

		end := i + chunkSize
		if end > total {
			end = total
		}
		codes := targetPool[i:end]

		wg.Add(1)
		sem <- struct{}{} // 获取令牌（满时阻塞）

		go func(codes []string) {
			defer wg.Done()
			defer func() { <-sem }() // 释放令牌

			// 每个 goroutine 独立 panic recovery
			defer func() {
				if r := recover(); r != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("goroutine panic: %v\n%s", r, debug.Stack())
					})
				}
			}()

			// 内存泄压检查
			if sysmon.CheckMemoryBackpressure() {
				time.Sleep(2 * time.Second)
			}

			// DB 加载 + 纯内存信号扫描
			chunk := backtest.LoadChunk(codes, lookbackStart, cfg.EndDate)
			chunkEvals := scanChunkSignals(cfg, chunk, analyzers, isBullMarket, isStrongMarket)

			// 发送给 CSV writer（channel 有缓冲，非阻塞直到满）
			resultsCh <- csvWriteRequest{evals: chunkEvals}

			// 原子进度更新（throttled：每 3*chunkSize 只股票或到达终点时更新 DB）
			n := atomic.AddInt64(&processed, int64(len(codes)))
			s := atomic.AddInt64(&totalSignals, int64(len(chunkEvals)))
			if n%int64(3*chunkSize) == 0 || n >= int64(total) {
				progress := fmt.Sprintf("%d/%d (信号=%d)", n, total, s)
				_ = db.UpdateLabJobStatus(jobID, "running", progress)
				log.Printf("[信号实验室] %s", progress)
			}
		}(codes)
	}

	// 8. 等待所有 chunk 处理完成
	wg.Wait()
	close(resultsCh)

	// 9. 等待 CSV writer 完成
	csvErr := <-csvDone

	// 10. 错误处理与收尾
	if firstErr != nil {
		_ = db.FailLabJob(jobID, firstErr.Error())
		return
	}
	if csvErr != nil {
		_ = db.FailLabJob(jobID, "写入CSV失败: "+csvErr.Error())
		return
	}

	_ = db.FinishLabJob(jobID, filePath)
	log.Printf("[信号实验室] 任务 #%d 完成 | 信号=%d | 文件=%s", jobID, totalSignals, filePath)
}

// scanChunkSignals 对一个 chunk 的所有股票执行信号扫描。
func scanChunkSignals(
	cfg SignalLabConfig,
	chunk backtest.ChunkData,
	analyzers []strategy.Analyzer,
	isBullMarket, isStrongMarket map[string]bool,
) []*SignalEvaluation {
	var results []*SignalEvaluation

	for idx, code := range chunk.Codes {
		klines := chunk.KLinesMap[code]
		if len(klines) < 30 {
			continue
		}
		funds := chunk.FundsMap[code]
		flows := chunk.FlowsMap[code]
		cyqPerfs := chunk.CyqPerfMap[code]

		logger.Info("[LAB] 准备处理股票: %s, 进度: %d/%d", code, idx+1, len(chunk.Codes))
		evals := scanStockSignals(code, klines, funds, flows, cyqPerfs, cfg, analyzers, isBullMarket, isStrongMarket)
		logger.Info("[LAB] 股票 %s 处理完成，生成信号: %d 笔", code, len(evals))
		results = append(results, evals...)
	}
	return results
}

// scanStockSignals 对单只股票扫描所有 bar 的信号。
// 不 break —— 实验室平行记录同一天触发的所有策略信号。
func scanStockSignals(
	code string,
	klines []tushare.DailyKLine,
	funds []tushare.DailyFundamental,
	flows []tushare.DailyMoneyFlow,
	cyqPerfs []tushare.CyqPerf,
	cfg SignalLabConfig,
	analyzers []strategy.Analyzer,
	isBullMarket, isStrongMarket map[string]bool,
) []*SignalEvaluation {
	var results []*SignalEvaluation

	fundMap := make(map[string]tushare.DailyFundamental)
	for _, f := range funds {
		fundMap[f.TradeDate] = f
	}
	flowMap := make(map[string]tushare.DailyMoneyFlow)
	for _, f := range flows {
		flowMap[f.TradeDate] = f
	}
	cyqMap := make(map[string]tushare.CyqPerf)
	for _, c := range cyqPerfs {
		cyqMap[c.TradeDate] = c
	}

	for i := 30; i < len(klines)-1; i++ { // -1: 需要 T+1
		today := klines[i]
		todayDate := today.TradeDate

		if todayDate < cfg.StartDate || todayDate > cfg.EndDate {
			continue
		}
		if !isBullMarket[todayDate] {
			continue
		}

		ctx := backtest.BuildStockContext(code, klines, funds, flows,
			fundMap, flowMap, cyqMap, i, todayDate)
		if ctx == nil || len(ctx.KLines) < 30 {
			continue
		}

		eligible := analyzers
		if !isStrongMarket[todayDate] {
			eligible = make([]strategy.Analyzer, 0, len(analyzers))
			for _, a := range analyzers {
				if a.MarketTag() == "left" {
					eligible = append(eligible, a)
				}
			}
		}

		for _, analyzer := range eligible {
			result := analyzer.Analyze(ctx)
			if backtest.ContainsBuySignal(result.Signal) {
				ev := evaluateSignal(klines, i, analyzer.Name(), code)
				if ev != nil {
					results = append(results, ev)
				}
				// 不 break —— 实验室平行记录同一天触发的所有策略信号
			}
		}
	}
	return results
}

// evaluateSignal 对单条买入信号执行前向 20 日评测。
func evaluateSignal(klines []tushare.DailyKLine, signalIdx int, stratName string, code string) *SignalEvaluation {
	n := len(klines)
	// 越界守卫：T+1 必须存在
	if signalIdx+1 >= n {
		return nil
	}

	signalBar := klines[signalIdx]
	entryBar := klines[signalIdx+1]
	entryOpen := entryBar.Open

	if entryOpen <= 0 || signalBar.Close <= 0 {
		return nil
	}

	ev := &SignalEvaluation{
		Symbol:      code,
		SignalDate:  signalBar.TradeDate,
		Strategy:    stratName,
		EntryDate:   entryBar.TradeDate,
		EntryOpen:   entryOpen,
		SignalClose: signalBar.Close,
		GapPct:      (entryOpen/signalBar.Close - 1) * 100,
	}

	// 5日均量 (T-5 ~ T-1，排除 T 日爆发量污染分母)
	if signalIdx >= 1 {
		volStart := signalIdx - 5
		if volStart < 0 {
			volStart = 0
		}
		volSum := 0.0
		volCount := 0
		for j := volStart; j <= signalIdx-1; j++ {
			volSum += klines[j].Vol
			volCount++
		}
		if volCount > 0 {
			avgVol := volSum / float64(volCount)
			if avgVol > 0 {
				ev.VolRatio = signalBar.Vol / avgVol
			}
		}
	}

	// 前向 20 日扫描 (T+1 ~ T+20)
	// d=0 → T+1 买入日, d=19 → T+20
	ev.MFE20 = 0.0
	ev.MAE20 = 0.0
	ev.DaysToMFE = 0
	ev.DaysToMAE = 0

	for d := 0; d <= 19; d++ {
		idx := signalIdx + 1 + d // T+1 ~ T+20
		if idx >= n {
			break // 数据不足，停止扫描
		}
		bar := klines[idx]

		// MFE: 最高价相对于入场价的涨幅
		highPct := (bar.High / entryOpen - 1) * 100
		if highPct > ev.MFE20 {
			ev.MFE20 = highPct
			ev.DaysToMFE = d
		}

		// MAE: 最低价相对于入场价的跌幅
		lowPct := (bar.Low / entryOpen - 1) * 100
		if lowPct < ev.MAE20 {
			ev.MAE20 = lowPct
			ev.DaysToMAE = d
		}

		// 固态收益 (Close / EntryOpen - 1)
		retPct := (bar.Close / entryOpen - 1) * 100
		switch d {
		case 0:
			ev.Ret1D = retPct
		case 2:
			ev.Ret3D = retPct
		case 4:
			ev.Ret5D = retPct
		case 9:
			ev.Ret10D = retPct
		case 19:
			ev.Ret20D = retPct
		}
	}

	// 同日到达 MFE 和 MAE（长上影+长下影）→ 保守判定 false（假定先被洗出）
	ev.MFEBeforeMAE = ev.DaysToMFE < ev.DaysToMAE

	return ev
}

// writeChunkToCSV 将一个 chunk 的评测结果写入 CSV 并 flush。
func writeChunkToCSV(writer *csv.Writer, evals []*SignalEvaluation) error {
	for _, ev := range evals {
		row := []string{
			ev.Symbol, ev.SignalDate, ev.Strategy, ev.EntryDate,
			fmt.Sprintf("%.2f", ev.EntryOpen),
			fmt.Sprintf("%.2f", ev.SignalClose),
			fmt.Sprintf("%.4f", ev.GapPct),
			fmt.Sprintf("%.2f", ev.VolRatio),
			fmt.Sprintf("%.2f", ev.MFE20),
			fmt.Sprintf("%d", ev.DaysToMFE),
			fmt.Sprintf("%.2f", ev.MAE20),
			fmt.Sprintf("%d", ev.DaysToMAE),
			fmt.Sprintf("%t", ev.MFEBeforeMAE),
			fmt.Sprintf("%.2f", ev.Ret1D),
			fmt.Sprintf("%.2f", ev.Ret3D),
			fmt.Sprintf("%.2f", ev.Ret5D),
			fmt.Sprintf("%.2f", ev.Ret10D),
			fmt.Sprintf("%.2f", ev.Ret20D),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
