// =============================================================================
// http_routes.go - HTTP 路由注册文件
// =============================================================================
// 这个文件集中管理所有 HTTP API 路由的注册。
//
// 什么是路由？
//   路由就是 URL 路径到处理函数的映射关系。
//   例如：当用户访问 "/api/diagnose" 时，服务器会调用 api.DiagnoseHandler 函数来处理请求。
//
// 为什么要集中管理？
//   把所有路由注册放在一个文件中，可以一目了然地看到系统提供了哪些 API 接口，
//   避免分散在各处导致遗漏或重复注册。
//
// Go 的 HTTP 路由机制：
//   - http.HandleFunc(path, handler) 将 URL 路径绑定到处理函数
//   - handler 函数的签名必须是 func(http.ResponseWriter, *http.Request)
//   - http.ResponseWriter 用于写入响应数据
//   - *http.Request 包含请求的所有信息（URL 参数、请求体、Header 等）
// =============================================================================

// package main 表示这个文件属于主程序包。
// 同一个 package 内的所有文件可以互相访问未导出的函数和变量。
// 所以这里可以直接调用 triggerSyncKlineHandler 等在其他文件中定义的函数。
package main

import (
	"net/http"          // HTTP 功能包，提供 HandleFunc、Server 等
	"stock-backend/api" // 自定义的 API 处理器包，包含各种 HTTP Handler
)

// =============================================================================
// registerRoutes - 注册所有 HTTP 路由
// =============================================================================
// 功能说明：
//   这个函数将所有 API 路由注册到 Go 的默认 HTTP 多路复用器（DefaultServeMux）。
//   当服务器收到请求时，会根据 URL 路径匹配对应的处理函数。
//
// 参数：无
// 返回值：无
//
// API 路由分类说明：
//   1. 系统诊断与日志
//   2. 数据同步触发接口
//   3. Token 管理接口
//   4. 回测与信号实验室
//   5. 自动同步管理
//   6. 邮件通知
//   7. 系统配置
//   8. 仓位管理
//   9. 策略管理
//  10. 引擎配置
// =============================================================================
func registerRoutes() {
	// =========================================================================
	// 1. 系统诊断与日志
	// =========================================================================

	// /api/diagnose - 系统健康检查接口
	// 前端可以调用此接口检查后端是否正常运行、数据库连接是否正常等
	http.HandleFunc("/api/diagnose", api.DiagnoseHandler)

	// /api/logs - 获取系统日志
	// 返回服务器的运行日志，用于前端展示和问题排查
	http.HandleFunc("/api/logs", getLogsHandler)

	// /api/audit - 审计日志接口
	// 记录和查看用户的操作历史，用于安全审计
	http.HandleFunc("/api/audit", api.AuditHandler)

	// /api/monitor - 系统监控接口
	// 返回服务器的运行状态指标（CPU、内存、goroutine 数量等）
	http.HandleFunc("/api/monitor", api.MonitorHandler)

	// =========================================================================
	// 2. 数据同步触发接口
	// =========================================================================
	// 这些接口用于手动触发各种股票数据的同步操作。
	// 每个接口对应一种数据类型，调用后会从数据源（如 Tushare）拉取数据并存入数据库。
	//
	// 常见的股票数据类型：
	//   - kline: K 线数据（日线、周线等，包含开盘价、收盘价、最高价、最低价）
	//   - fund: 基金数据
	//   - calendar: 交易日历（哪些天是交易日）
	//   - adj: 复权因子（用于计算前复权/后复权价格）
	//   - index: 指数数据（如上证指数、沪深300）
	//   - moneyflow: 资金流向数据（主力资金、散户资金的进出）
	//   - fina: 财务数据（营收、利润、资产负债表等）
	//   - limit: 涨跌停列表
	//   - cyqperf: 筹码分布性能数据
	//   - stkfactorpro: 股票因子数据（技术指标等）
	//   - basic: 股票基本信息（名称、行业、上市日期等）

	// K 线数据同步（日线、周线、月线等）
	http.HandleFunc("/api/start_sync_kline", triggerSyncKlineHandler)
	// 基金数据同步
	http.HandleFunc("/api/start_sync_fund", triggerSyncFundHandler)
	// 交易日历同步
	http.HandleFunc("/api/start_sync_calendar", triggerSyncCalendarHandler)
	// 复权因子同步（用于计算前复权/后复权价格）
	http.HandleFunc("/api/start_sync_adj", triggerSyncAdjHandler)
	// 指数数据同步（上证指数、沪深300 等）
	http.HandleFunc("/api/start_sync_index", triggerSyncIndexHandler)
	// 资金流向同步（主力资金、散户资金进出情况）
	http.HandleFunc("/api/start_sync_moneyflow", triggerSyncMoneyFlowHandler)
	// 财务数据同步（营收、利润、资产负债表等）
	http.HandleFunc("/api/start_sync_fina", triggerSyncFinaHandler)
	// 涨跌停列表同步
	http.HandleFunc("/api/start_sync_limit", triggerSyncLimitListHandler)
	// 筹码分布数据同步
	http.HandleFunc("/api/start_sync_cyqperf", triggerSyncCyqPerfHandler)
	// 股票因子数据同步（技术指标、量化因子等）
	http.HandleFunc("/api/start_sync_stkfactorpro", triggerSyncStkFactorProHandler)
	// 股票基本信息同步（股票名称、行业、上市日期等）
	http.HandleFunc("/api/start_sync_basic", triggerSyncBasicHandler)

	// =========================================================================
	// 3. Token 管理接口
	// =========================================================================
	// Tushare 等数据源需要 API Token 才能访问。
	// 这些接口用于管理 Token：添加、激活、测试权限等。

	// /api/set_token - 更新/添加 Token
	http.HandleFunc("/api/set_token", updateTokenHandler)
	// /api/tokens - Token 集合操作（GET 获取列表，POST 添加新 Token）
	// 注意：同一个路径可以处理不同的 HTTP 方法（GET/POST/PUT/DELETE）
	http.HandleFunc("/api/tokens", tokenCollectionHandler)
	// /api/tokens/activate - 激活指定 Token
	http.HandleFunc("/api/tokens/activate", tokenActivateHandler)
	// /api/tokens/test_permissions - 测试 Token 的 API 权限
	http.HandleFunc("/api/tokens/test_permissions", tokenPermissionTestHandler)

	// =========================================================================
	// 4. 回测与信号实验室
	// =========================================================================
	// 回测（Backtest）：用历史数据验证交易策略的表现
	// 信号实验室（SignalLab）：创建和管理交易信号/策略

	// /api/backtest 和 /api/backtest/ 都路由到 BacktestRouter
	// 带尾部斜杠和不带斜杠的都注册，是为了兼容不同的请求方式。
	// BacktestRouter 是一个子路由器，会根据完整路径进一步分发请求。
	http.HandleFunc("/api/backtest", BacktestRouter)
	http.HandleFunc("/api/backtest/", BacktestRouter)

	// 信号实验室路由（策略创建、编辑、运行等）
	http.HandleFunc("/api/signallab", SignalLabRouter)
	http.HandleFunc("/api/signallab/", SignalLabRouter)

	// =========================================================================
	// 5. 自动同步管理
	// =========================================================================
	// 自动同步功能允许用户配置定时任务，自动从数据源拉取最新数据。

	// /api/auto_sync/config - 获取/设置自动同步配置
	http.HandleFunc("/api/auto_sync/config", autoSyncConfigHandler)
	// /api/auto_sync/runs - 获取自动同步的运行记录列表
	http.HandleFunc("/api/auto_sync/runs", autoSyncRunsHandler)
	// /api/auto_sync/run_now - 立即触发一次自动同步
	http.HandleFunc("/api/auto_sync/run_now", autoSyncRunNowHandler)
	// /api/auto_sync/run_steps - 获取某次运行的详细步骤
	http.HandleFunc("/api/auto_sync/run_steps", autoSyncRunStepsHandler)

	// =========================================================================
	// 6. 邮件通知
	// =========================================================================
	// 这些接口用于配置邮件通知功能，当特定事件发生时发送邮件提醒。

	// /api/notify/email/config - 邮件服务器配置（SMTP 设置）
	http.HandleFunc("/api/notify/email/config", emailNotifyConfigHandler)
	// /api/notify/email/recipients - 管理邮件接收人列表
	http.HandleFunc("/api/notify/email/recipients", emailRecipientsHandler)
	// /api/notify/email/send_strategy_scan - 发送策略扫描结果邮件
	http.HandleFunc("/api/notify/email/send_strategy_scan", emailSendStrategyScanHandler)

	// =========================================================================
	// 7. 系统配置
	// =========================================================================

	// /api/system/config - 系统全局配置接口
	http.HandleFunc("/api/system/config", systemConfigHandler)

	// /api/set_speed - 设置数据同步速度（控制请求频率，避免被数据源限流）
	http.HandleFunc("/api/set_speed", updateSpeedHandler)

	// =========================================================================
	// 8. 仓位管理
	// =========================================================================
	// 仓位（Position）：用户持有的股票及其数量。
	// 这些接口用于管理用户的持仓信息和风险评估。

	// /api/position/risk - 仓位风险评估
	http.HandleFunc("/api/position/risk", api.PositionRiskHandler)
	// /api/position/add - 添加新仓位
	http.HandleFunc("/api/position/add", api.AddPositionHandler)
	// /api/position/list - 获取仓位列表
	http.HandleFunc("/api/position/list", api.GetPositionsHandler)
	// /api/position/delete - 删除仓位
	http.HandleFunc("/api/position/delete", api.DeletePositionHandler)

	// =========================================================================
	// 9. 策略管理
	// =========================================================================
	// 策略（Strategy）：用户的量化交易策略。

	// /api/strategies - 策略集合操作（GET 列表，POST 创建）
	http.HandleFunc("/api/strategies", StrategyRouter)
	http.HandleFunc("/api/strategies/", StrategyRouter)

	// =========================================================================
	// 10. 引擎配置
	// =========================================================================
	// 引擎配置：数据同步引擎的运行参数设置。

	// /api/config/engine - 引擎配置操作
	http.HandleFunc("/api/config/engine", EngineConfigRouter)
	http.HandleFunc("/api/config/engine/", EngineConfigRouter)
}
