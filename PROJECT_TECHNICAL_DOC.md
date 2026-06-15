# 股票量化研究工作台 - 技术文档

## 📋 目录

1. [项目概述](#项目概述)
2. [系统架构](#系统架构)
3. [后端结构详解](#后端结构详解)
4. [前端结构详解](#前端结构详解)
5. [数据库设计](#数据库设计)
6. [API 接口清单](#api-接口清单)
7. [核心模块解析](#核心模块解析)
8. [数据流向图](#数据流向图)
9. [Go 语言学习要点](#go-语言学习要点)

---

## 项目概述

这是一个 **A 股量化研究工作台**，采用前后端分离架构：

- **后端**: Go 语言实现，负责数据采集、策略计算、回测引擎
- **前端**: React + Vite 实现，提供可视化操作界面
- **数据库**: SQLite（嵌入式，无需额外安装）
- **数据源**: Tushare API（需要 Token）

### 项目目标

1. 自动同步 A 股行情数据（日线、基本面、资金流向等）
2. 运行量化策略扫描交易信号
3. 历史回测验证策略有效性
4. 持仓管理与风险控制

---

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      前端 (React + Vite)                     │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │ 数据管线 │ │ 策略扫描 │ │ 回测验证 │ │ 持仓风控 │  ...     │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘          │
│       │           │           │           │                │
│       └───────────┴───────────┴───────────┘                │
│                       HTTP API 调用                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    后端 (Go HTTP Server)                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │                 HTTP 路由层 (http_*.go)                │  │
│  └──────────────────────────────────────────────────────┘  │
│                            │                                │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐  │
│  │  feeder   │ │ strategy  │ │ backtest  │ │  signallab │  │
│  │ 数据采集  │ │ 策略引擎  │ │ 回测引擎  │ │ 信号实验  │  │
│  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘  │
│        │             │             │             │          │
│        └─────────────┴─────────────┴─────────────┘          │
│                            │                                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              SQLite 数据库 (db/*.go)                   │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   Tushare API (外部数据源)                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 后端结构详解

### 目录结构

```
stock-backend/
├── main.go                    # 程序入口，启动 HTTP 服务器
├── http_routes.go             # 路由注册（所有 API 路径映射）
├── http_*.go                  # 各模块的 HTTP 处理器
│
├── db/                        # 数据库层
│   ├── database.go            # 数据库初始化、表结构定义
│   ├── db_batch.go            # 批量查询（回测用）
│   ├── db_kline_stock.go      # K线数据 CRUD
│   ├── db_fundamental.go      # 基本面数据 CRUD
│   ├── token_pool.go          # Token 管理
│   ├── auto_sync.go           # 自动同步配置
│   ├── backtest_jobs.go       # 回测任务记录
│   ├── strategy_registry.go   # 策略注册表
│   └── ...
│
├── tushare/                   # Tushare API 客户端
│   └── client.go              # 封装所有 Tushare API 调用
│
├── feeder/                    # 数据采集引擎
│   ├── engine.go              # 全局调度引擎（限流、异步落盘）
│   ├── sync.go                # 同步逻辑
│   └── extended_sync.go       # 扩展数据同步
│
├── strategy/                  # 策略引擎
│   ├── models.go              # 核心数据结构定义
│   ├── rules.go               # 策略实现（MACB、CBBM、DSS、PBMA）
│   ├── indicators.go          # 技术指标计算（MA、ATR、布林带等）
│   ├── filters_exp.go         # 实验性过滤器
│   ├── composite_pullback.go  # 复合回踩策略
│   └── pbma.go                # 缩量回踩策略
│
├── backtest/                  # 回测引擎
│   ├── engine_v2.go           # V2 回测引擎（两阶段解耦）
│   ├── models.go              # 回测数据结构
│   ├── metrics.go             # 绩效指标计算
│   ├── execution.go           # 交易执行模拟
│   └── export.go              # 结果导出
│
├── signallab/                 # 信号实验室
│   ├── engine.go              # 信号评测引擎
│   └── models.go              # 信号模型
│
├── autosync/                  # 自动同步管理
│   └── manager.go             # 定时任务调度
│
├── logger/                    # 日志系统
│   └── logger.go              # 文件日志
│
├── sysmon/                    # 系统监控
│   └── manager.go             # 内存、CPU 监控
│
├── mailnotify/                # 邮件通知
│   └── email_report.go        # 邮件发送
│
├── stockutil/                 # 工具函数
│   └── board.go               # 板块判断
│
└── eastmoney/                 # 东方财富数据源（备用）
    └── client.go
```

### 启动流程 (main.go)

```go
func main() {
    // 1. 初始化日志系统
    logger.Init("logs/blackbox.log")

    // 2. 初始化数据库（创建表、设置 PRAGMA）
    db.InitDB()

    // 3. 清理僵尸任务
    db.MarkStaleRunningLabJobsFailed("服务重启，任务中断")

    // 4. 同步激活 Token
    syncActiveProviderToken("tushare")

    // 5. 启动自动同步管理器
    autoSyncManager.Start()

    // 6. 启动局域网广播（动态灯塔）
    startDynamicLighthouse()

    // 7. 初始化数据采集引擎（限流 800ms）
    feeder.InitGlobalEngine(800 * time.Millisecond)

    // 8. 注册 HTTP 路由
    registerRoutes()
    registerStaticRoutes()

    // 9. 启动 HTTP 服务器（默认端口 8081）
    server.ListenAndServe()
}
```

### Go 语言要点：程序入口

```go
// package main 表示这是一个可执行程序
package main

// import 导入依赖包
import (
    "net/http"      // Go 标准库：HTTP 服务器
    "stock-backend/db"  // 项目内部包：数据库层
)

// main() 函数是程序入口
func main() {
    // Go 使用 http.HandleFunc 注册路由
    http.HandleFunc("/api/logs", getLogsHandler)

    // 启动服务器，nil 表示使用默认路由
    http.ListenAndServe(":8081", nil)
}
```

---

## 前端结构详解

### 目录结构

```
stock-frontend/
├── index.html                 # HTML 入口
├── package.json               # 依赖配置
├── vite.config.js             # Vite 构建配置
│
├── src/
│   ├── main.jsx               # React 入口
│   ├── App.jsx                # 主组件（导航 + 路由）
│   ├── App.css                # 全局样式
│   ├── index.css              # 基础样式
│   │
│   ├── api/
│   │   └── client.js          # API 客户端（统一封装 fetch）
│   │
│   ├── components/
│   │   ├── OverviewSection.jsx    # 总览面板
│   │   └── DataAuditPanel.jsx     # 数据审计面板
│   │
│   ├── features/              # 功能模块（按特性组织）
│   │   ├── autosync/          # 自动同步
│   │   │   └── components/
│   │   │       ├── AutoSyncPanel.jsx
│   │   │       ├── AutoSyncConfigSection.jsx
│   │   │       └── ...
│   │   │
│   │   ├── backtest/          # 回测
│   │   │   ├── components/
│   │   │   │   ├── BacktestConfigPanel.jsx
│   │   │   │   ├── BacktestResultPanel.jsx
│   │   │   │   └── BacktestPlanEditor.jsx
│   │   │   └── charts/
│   │   │       └── backtestChartOption.js
│   │   │
│   │   ├── strategy/          # 策略扫描
│   │   │   ├── components/
│   │   │   │   ├── StrategyScanPanel.jsx
│   │   │   │   ├── StrategyResultCard.jsx
│   │   │   │   └── StrategySummarySection.jsx
│   │   │   └── charts/
│   │   │       └── strategyChartOption.js
│   │   │
│   │   ├── position/          # 持仓管理
│   │   │   ├── components/
│   │   │   │   ├── PositionFormSection.jsx
│   │   │   │   ├── PositionListSection.jsx
│   │   │   │   └── PositionRiskPanel.jsx
│   │   │   └── utils/
│   │   │       └── formatters.js
│   │   │
│   │   ├── token/             # Token 管理
│   │   │   ├── components/
│   │   │   └── hooks/
│   │   │
│   │   └── pipeline/          # 数据管线
│   │       └── components/
│   │
│   ├── workspaces/            # 工作区页面
│   │   ├── DataPipelineWorkspace.jsx
│   │   ├── StrategyWorkspace.jsx
│   │   ├── BacktestWorkspace.jsx
│   │   ├── SignalLabWorkspace.jsx
│   │   ├── PositionRiskWorkspace.jsx
│   │   ├── DataAuditWorkspace.jsx
│   │   └── StrategyManagerWorkspace.jsx
│   │
│   └── utils/
│       └── dateRange.js       # 日期工具
```

### 前端技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| React | 19.2 | UI 框架 |
| Vite | 7.3 | 构建工具 |
| ECharts | 6.0 | 图表库 |
| ESLint | 9.39 | 代码检查 |

### API 客户端 (src/api/client.js)

```javascript
// 统一的 API 客户端，所有前端模块都通过它调用后端
const api = {
  // 数据同步
  startSyncKline: (params) => request("/api/start_sync_kline", { params }),
  startSyncBasic: () => request("/api/start_sync_basic"),

  // Token 管理
  listTokens: (provider) => request("/api/tokens", { params: { provider } }),
  createToken: (payload) => request("/api/tokens", { method: "POST", body: payload }),

  // 回测
  runBacktest: (payload) => request("/api/backtest", { method: "POST", body: payload }),
  listBacktestJobs: (limit) => request("/api/backtest/list", { params: { limit } }),

  // 策略
  listStrategies: () => request("/api/strategies"),
  toggleStrategy: (id, isEnabled) => request("/api/strategies/toggle", { method: "POST", body: { id, is_enabled: isEnabled } }),

  // ... 更多接口
};

export default api;
```

### 组件组织模式

```
features/模块名/
├── components/        # UI 组件
│   ├── XxxPanel.jsx       # 主面板
│   ├── XxxSection.jsx     # 子区域
│   └── XxxForm.jsx        # 表单组件
├── hooks/             # 自定义 Hook
│   └── useXxx.js          # 状态管理 Hook
├── charts/            # 图表配置
│   └── xxxChartOption.js  # ECharts 配置
└── utils/             # 工具函数
    └── formatters.js      # 格式化函数
```

---

## 数据库设计

### 核心数据表 (共 23 张)

#### 1. 基础数据表

| 表名 | 用途 | 主键 |
|------|------|------|
| `trade_calendar` | 交易日历 | `cal_date` |
| `stock_basic` | 股票花名册 | `ts_code` |
| `daily_klines` | 日线行情 | `(ts_code, trade_date)` |
| `adj_factors` | 复权因子 | `(ts_code, trade_date)` |
| `daily_fundamentals` | 每日基本面 | `(ts_code, trade_date)` |
| `fina_indicators` | 季报财务 | `(ts_code, end_date)` |
| `daily_moneyflow` | 资金流向 | `(ts_code, trade_date)` |
| `daily_stk_limit` | 涨跌停价格 | `(trade_date, ts_code)` |
| `index_daily` | 大盘指数 | `(ts_code, trade_date)` |

#### 2. 业务数据表

| 表名 | 用途 | 主键 |
|------|------|------|
| `my_positions` | 持仓记录 | `id` (自增) |
| `api_tokens` | Token 池 | `id` (自增) |
| `backtest_jobs` | 回测任务 | `id` (自增) |
| `backtest_plans` | 回测计划 | `id` (自增) |
| `backtest_plan_tasks` | 回测子任务 | `id` (自增) |
| `signal_lab_jobs` | 信号实验任务 | `id` (自增) |
| `strategy_registry` | 策略注册表 | `id` |
| `engine_config` | 引擎配置 | `key` |

#### 3. 配置数据表

| 表名 | 用途 | 主键 |
|------|------|------|
| `auto_sync_config` | 自动同步配置 | `id` (固定为 1) |
| `auto_sync_runs` | 同步运行记录 | `id` (自增) |
| `auto_sync_run_steps` | 同步步骤记录 | `id` (自增) |
| `email_notify_config` | 邮件配置 | `id` (固定为 1) |
| `email_recipients` | 收件人列表 | `id` (自增) |
| `system_config` | 系统配置 | `id` (固定为 1) |

### 表结构示例 (daily_klines)

```sql
CREATE TABLE IF NOT EXISTS daily_klines (
    ts_code TEXT NOT NULL,          -- 股票代码，如 "000001.SZ"
    trade_date TEXT NOT NULL,       -- 交易日期，如 "20240101"
    open REAL,                      -- 开盘价
    close REAL,                     -- 收盘价
    high REAL,                      -- 最高价
    low REAL,                       -- 最低价
    vol REAL,                       -- 成交量
    amount REAL,                    -- 成交额
    pre_close REAL,                 -- 昨收价
    change REAL,                    -- 涨跌额
    pct_chg REAL,                   -- 涨跌幅 (%)
    data_source TEXT NOT NULL,      -- 数据源标记 (如 "TUSHARE")
    trust_level INTEGER NOT NULL,   -- 可信度权重 (如 100)
    PRIMARY KEY (ts_code, trade_date)
) WITHOUT ROWID;

-- 索引：加速按日期查询
CREATE INDEX IF NOT EXISTS idx_kline_date ON daily_klines(trade_date);
```

### Go 语言要点：数据库操作

```go
// db/database.go 中的初始化
var DB *sql.DB  // 全局数据库连接

func InitDB() {
    var err error
    // 打开 SQLite 数据库
    DB, err = sql.Open("sqlite", "stocks.db")

    // 设置 PRAGMA 优化
    DB.Exec("PRAGMA journal_mode = WAL;")    // 读写不阻塞
    DB.Exec("PRAGMA synchronous = NORMAL;")  // 提升写入速度

    // 创建表
    DB.Exec(`CREATE TABLE IF NOT EXISTS daily_klines (...)`)
}

// db/db_kline_stock.go 中的批量插入
func BatchInsertKLines(tsCode string, klines []tushare.DailyKLine) int {
    tx, _ := DB.Begin()  // 开启事务
    stmt, _ := tx.Prepare(`INSERT OR REPLACE INTO daily_klines ...`)

    for _, kline := range klines {
        stmt.Exec(kline.TSCode, kline.TradeDate, kline.Open, ...)
    }

    tx.Commit()  // 提交事务
    return len(klines)
}
```

---

## API 接口清单

### 数据同步类

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/start_sync_calendar` | 同步交易日历 |
| GET | `/api/start_sync_basic` | 同步股票花名册 |
| GET | `/api/start_sync_kline` | 同步日线行情 |
| GET | `/api/start_sync_fund` | 同步基本面数据 |
| GET | `/api/start_sync_adj` | 同步复权因子 |
| GET | `/api/start_sync_index` | 同步大盘指数 |
| GET | `/api/start_sync_moneyflow` | 同步资金流向 |
| GET | `/api/start_sync_fina` | 同步财务指标 |
| GET | `/api/start_sync_limit` | 同步涨跌停价格 |
| GET | `/api/start_sync_cyqperf` | 同步筹码分布 |
| GET | `/api/start_sync_stkfactorpro` | 同步技术因子 |

### Token 管理类

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/tokens` | 获取 Token 列表 |
| POST | `/api/tokens` | 创建 Token |
| PUT | `/api/tokens` | 更新 Token |
| DELETE | `/api/tokens` | 删除 Token |
| POST | `/api/tokens/activate` | 激活 Token |
| POST | `/api/tokens/test_permissions` | 测试 Token 权限 |

### 回测类

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/backtest` | 提交回测任务 |
| GET | `/api/backtest/{id}` | 获取回测结果 |
| GET | `/api/backtest/list` | 获取回测列表 |
| DELETE | `/api/backtest/{id}` | 删除回测任务 |
| POST | `/api/backtest/plan` | 提交回测计划 |
| GET | `/api/backtest/plan/{id}` | 获取计划详情 |

### 策略类

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/strategies` | 获取策略列表 |
| POST | `/api/strategies/toggle` | 启用/禁用策略 |
| GET | `/api/config/engine` | 获取引擎配置 |
| POST | `/api/config/engine/toggle` | 切换子策略开关 |

### 持仓管理类

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/position/list` | 获取持仓列表 |
| POST | `/api/position/add` | 添加持仓 |
| POST | `/api/position/delete` | 删除持仓 |
| GET | `/api/position/risk` | 获取风险分析 |

### 系统配置类

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/system/config` | 获取系统配置 |
| PUT | `/api/system/config` | 更新系统配置 |
| GET | `/api/auto_sync/config` | 获取自动同步配置 |
| PUT | `/api/auto_sync/config` | 更新自动同步配置 |
| POST | `/api/auto_sync/run_now` | 立即执行同步 |

### Go 语言要点：HTTP 路由

```go
// http_routes.go - 路由注册
func registerRoutes() {
    // 注册路由：路径 -> 处理函数
    http.HandleFunc("/api/logs", getLogsHandler)
    http.HandleFunc("/api/tokens", tokenCollectionHandler)
    http.HandleFunc("/api/backtest", BacktestRouter)
}

// http_common.go - HTTP 处理函数签名
func getLogsHandler(w http.ResponseWriter, r *http.Request) {
    // w: 写入响应
    // r: 读取请求

    // 读取查询参数
    limit := r.URL.Query().Get("limit")

    // 返回 JSON 响应
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}
```

---

## 核心模块解析

### 1. 数据采集引擎 (feeder/)

**职责**: 从 Tushare API 拉取数据，限流后异步写入数据库

```go
// feeder/engine.go 核心设计

// 全局调度引擎：限流 + 异步落盘
var (
    baseDelay time.Duration      // 基础延迟（如 800ms）
    sinkChan  chan SinkTask       // 异步写入通道
)

// SinkTask 定义落盘任务
type SinkTask struct {
    Type   string      // "kline", "fund", "adj" 等
    TSCode string      // 股票代码
    Data   interface{} // 数据负载
}

// WaitToken 仿生学限流：基础延迟 + 随机抖动
func WaitToken() {
    jitter := time.Duration(rand.Int63n(int64(baseDelay)))
    time.Sleep(baseDelay + jitter)
}

// PushToSink 将数据推入异步写入通道
func PushToSink(taskType, tsCode string, data interface{}) {
    sinkChan <- SinkTask{Type: taskType, TSCode: tsCode, Data: data}
}

// dataSinkWorker 单向落盘守护进程
func dataSinkWorker() {
    for task := range sinkChan {
        switch task.Type {
        case "kline":
            db.BatchInsertKLines(task.TSCode, task.Data.([]tushare.DailyKLine))
        case "fund":
            db.BatchInsertFundamentals(task.TSCode, task.Data.([]tushare.DailyFundamental))
        // ... 其他类型
        }
    }
}
```

**数据流**:
```
Tushare API → feeder.FetchXxx() → feeder.PushToSink() → sinkChan → dataSinkWorker() → db.BatchInsertXxx()
```

### 2. 策略引擎 (strategy/)

**职责**: 分析股票数据，生成交易信号

#### 核心接口

```go
// strategy/models.go

// Analyzer 策略接口（策略模式）
type Analyzer interface {
    Name() string                                    // 策略名称
    MarketTag() string                               // "right"(右侧) 或 "left"(左侧)
    RequiredData() []string                          // 需要的数据类型
    Analyze(ctx *SecurityContext) DiagnoseResult      // 分析买入信号
    EvaluateHold(pos *Position, today DailyKLine, ...) EvaluateHoldResult  // 评估持仓
}

// SecurityContext 策略上下文（一只股票的完整数据）
type SecurityContext struct {
    Code         string
    KLines       []tushare.DailyKLine          // K 线数据
    Fundamentals []tushare.DailyFundamental    // 基本面
    MoneyFlows   []tushare.DailyMoneyFlow      // 资金流向
    PEPercentile float64                       // PE 历史分位
    CyqPerf      *tushare.CyqPerf              // 筹码分布
    LatestFina   *tushare.FinaIndicator        // 最新财报
}

// DiagnoseResult 诊断结果
type DiagnoseResult struct {
    Code          string  `json:"code"`
    StrategyName  string  `json:"strategy_name"`
    Signal        string  `json:"signal"`          // "买入 🚀", "观望 💤", "卖出 🛑"
    BuyPrice      float64 `json:"buy_price"`
    SellPrice     float64 `json:"sell_price"`
    StopLossPrice float64 `json:"stop_loss_price"`
}
```

#### 已实现的策略

| 策略 ID | 名称 | 类型 | 说明 |
|---------|------|------|------|
| MACB | 均线收敛突破 | 右侧 | 均线收敛 + 放量突破 |
| CBBM | 中枢强势突破 | 右侧 | 箱体突破 + 涨停确认 |
| DSS | 深海动量 2.0 | 左侧 | 地量潜伏 + 箱体底部 (已下线) |
| PBMA | 缩量回踩狙击 | 左侧 | 回踩均线 + 缩量确认 |
| MACB-P | 均线突破回踩 | 复合 | MACB 信号后的回踩买点 |
| CBBM-P | 箱体突破回踩 | 复合 | CBBM 信号后的回踩买点 |

#### 策略执行流程

```
1. 获取全市场股票列表
2. 对每只股票：
   a. 加载 K 线、基本面、资金流向等数据
   b. 构建 SecurityContext
   c. 调用 analyzer.Analyze(ctx)
   d. 如果返回 "买入 🚀"，记录信号
3. 汇总所有买入信号
```

### 3. 回测引擎 (backtest/)

**职责**: 历史数据模拟交易，验证策略有效性

#### V2 引擎设计（两阶段解耦）

```go
// backtest/engine_v2.go

func RunV2(ctx context.Context, cfg BacktestConfig) (*BacktestResult, error) {
    // Phase 1: Signal Mining（按股票遍历）
    // - 每只股票仅 4 次 DB 查询
    // - 产出理论交易信号（含买入价、卖出价、持有期）
    signals := phase1SignalMining(ctx, cfg)

    // Phase 2: Portfolio Simulation（按日历推演）
    // - 0 次 DB 查询，纯内存计算
    // - 模拟资金分配、滑点、手续费
    result := phase2PortfolioSim(ctx, cfg, signals)

    return result, nil
}
```

#### Phase 1: 信号开采

```go
func mineStockSignals(code string, klines []DailyKLine, ...) []TheoreticalTrade {
    var pending *pendingSignal  // T+1 挂单
    var hold *holdState         // 当前持仓

    for i := 0; i < len(klines); i++ {
        today := klines[i]

        // A. 执行挂单买入（T+1：信号日 < 今天）
        if pending != nil && pending.signalDate < todayDate {
            buyPrice := today.Open * (1 + SlippageRate)  // 买入滑点
            hold = &holdState{buyPrice: buyPrice, ...}
            pending = nil
        }

        // B. 持仓评估：止盈止损
        if hold != nil {
            // 检查止损条件
            if shouldStopLoss(today, hold) {
                trades = append(trades, buildClosedTrade(...))
                hold = nil
            }
        }

        // C. 扫描买入信号（仅当未持仓时）
        if hold == nil && pending == nil {
            for _, analyzer := range analyzers {
                result := analyzer.Analyze(ctx)
                if result.Signal == "买入 🚀" {
                    pending = &pendingSignal{...}
                    break
                }
            }
        }
    }
}
```

#### Phase 2: 组合模拟

```go
func phase2PortfolioSim(cfg BacktestConfig, signals []TheoreticalTrade) *BacktestResult {
    cash := cfg.InitialCapital
    activePositions := make(map[string]*activePosition)

    // 按天循环
    for _, today := range tradingDays {
        // A. 卖出处理
        for code, ap := range activePositions {
            if ap.trade.SellDate == today {
                cash += sellAmount
                delete(activePositions, code)
            }
        }

        // B. 买入处理（按 score 降序优先）
        for _, signal := range todaySignals {
            shares := calculatePositionShares(totalEquity, cash, signal.BuyPrice)
            if shares >= 100 && cost <= cash {
                cash -= cost
                activePositions[signal.Code] = &activePosition{...}
            }
        }

        // C. 逐日盯市
        totalValue := cash + holdingValue
        dailyEquity = append(dailyEquity, DailyEquity{Date: today, Equity: totalValue})
    }
}
```

### 4. 信号实验室 (signallab/)

**职责**: 纯净信号评测，不考虑资金管理，只评估信号质量

```go
// signallab/engine.go

func RunSignalLab(ctx context.Context, strategy string) (*SignalLabResult, error) {
    // 1. 获取全市场股票
    // 2. 遍历每只股票，执行策略
    // 3. 记录所有买入信号
    // 4. 计算前向收益（买入后 N 天的收益）
    // 5. 导出 CSV 报告
}
```

---

## 数据流向图

### 数据同步流程

```
用户点击 "同步日线"
        │
        ▼
前端调用 api.startSyncKline({start_date, end_date})
        │
        ▼
后端 triggerSyncKlineHandler()
        │
        ▼
feeder.SyncKline(start, end)
        │
        ├─→ 遍历所有股票
        │       │
        │       ▼
        │   tushare.FetchStockHistory(code, start, end)
        │       │
        │       ▼
        │   feeder.PushToSink("kline", code, data)
        │       │
        │       ▼
        │   sinkChan (异步通道)
        │
        ▼
dataSinkWorker() 消费通道
        │
        ▼
db.BatchInsertKLines(code, klines)
        │
        ▼
SQLite 数据库
```

### 策略扫描流程

```
用户点击 "开始扫描"
        │
        ▼
前端调用 api.diagnose({code, strategy})
        │
        ▼
后端 DiagnoseHandler()
        │
        ▼
db.GetKLines(code) + db.GetFundamentals(code) + ...
        │
        ▼
构建 SecurityContext
        │
        ▼
analyzer.Analyze(ctx)
        │
        ├─→ MACBAnalyzer.Analyze()  → 检查均线收敛 + 突破
        ├─→ CBBMAnalyzer.Analyze()  → 检查箱体突破 + 涨停
        └─→ PBMAAnalyzer.Analyze()  → 检查缩量回踩
        │
        ▼
返回 DiagnoseResult {Signal: "买入 🚀", BuyPrice: ..., StopLoss: ...}
        │
        ▼
前端展示信号卡片
```

### 回测流程

```
用户配置回测参数，点击 "开始回测"
        │
        ▼
前端调用 api.runBacktest({strategy, start, end, capital})
        │
        ▼
后端创建 backtest_job (status=pending)
        │
        ▼
go backtest.RunV2(ctx, config)  // 异步执行
        │
        ├─→ Phase 1: Signal Mining
        │       │
        │       ├─→ 分块加载数据 (LoadChunk)
        │       ├─→ 并发执行策略 (MineChunkSignals)
        │       └─→ 收集所有买入信号
        │
        └─→ Phase 2: Portfolio Simulation
                │
                ├─→ 按日历推演
                ├─→ 模拟买卖执行
                ├─→ 计算净值曲线
                └─→ 生成绩效报告
        │
        ▼
更新 backtest_job (status=completed, result_json=...)
        │
        ▼
前端轮询获取结果，展示图表
```

---

## Go 语言学习要点

### 1. 包 (Package) 系统

```go
// 每个目录是一个包
package db  // db/database.go 属于 db 包

// 导入包
import (
    "database/sql"          // 标准库
    "stock-backend/tushare" // 项目内部包
)

// 导出规则：大写开头 = 可导出
var DB *sql.DB  // 可导出（其他包可用）
var db *sql.DB  // 不可导出（仅本包可用）
```

### 2. 结构体 (Struct)

```go
// 定义结构体
type DailyKLine struct {
    TSCode    string  `json:"ts_code"`    // JSON 标签
    TradeDate string  `json:"trade_date"`
    Open      float64 `json:"open"`
    Close     float64 `json:"close"`
}

// 方法绑定
func (k DailyKLine) IsYangLine() bool {
    return k.Close > k.Open
}
```

### 3. 接口 (Interface)

```go
// 定义接口
type Analyzer interface {
    Name() string
    Analyze(ctx *SecurityContext) DiagnoseResult
}

// 实现接口（无需显式声明）
type MACBAnalyzer struct{}
func (m *MACBAnalyzer) Name() string { return "MACB" }
func (m *MACBAnalyzer) Analyze(ctx *SecurityContext) DiagnoseResult { ... }
```

### 4. 并发 (Goroutine + Channel)

```go
// 启动协程
go func() {
    // 异步执行
}()

// 通道通信
ch := make(chan Task, 100)  // 带缓冲的通道

// 生产者
ch <- task

// 消费者
for task := range ch {
    process(task)
}
```

### 5. 错误处理

```go
// Go 使用多返回值处理错误
result, err := doSomething()
if err != nil {
    log.Printf("错误: %v", err)
    return err
}

// 错误包装
if err != nil {
    return fmt.Errorf("操作失败: %w", err)  // %w 包装原始错误
}
```

### 6. HTTP 处理

```go
// 处理函数签名
func handler(w http.ResponseWriter, r *http.Request) {
    // 读取请求
    method := r.Method
    params := r.URL.Query().Get("key")

    // 解析 JSON Body
    var req RequestStruct
    json.NewDecoder(r.Body).Decode(&req)

    // 返回 JSON 响应
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(responseData)
}
```

### 7. 数据库操作

```go
// 查询单行
var name string
err := db.QueryRow("SELECT name FROM stock_basic WHERE ts_code=?", code).Scan(&name)

// 查询多行
rows, err := db.Query("SELECT * FROM daily_klines WHERE ts_code=? ORDER BY trade_date", code)
defer rows.Close()

for rows.Next() {
    var kline DailyKLine
    rows.Scan(&kline.TSCode, &kline.TradeDate, &kline.Open, ...)
    klines = append(klines, kline)
}

// 事务
tx, _ := db.Begin()
stmt, _ := tx.Prepare("INSERT INTO ...")
stmt.Exec(...)
tx.Commit()
```

---

## 附录：快速上手指南

### 启动后端

```bash
cd stock-backend

# 设置 Tushare Token（可选，也可通过界面设置）
export TUSHARE_TOKEN="your_token_here"

# 运行
go run main.go

# 或编译后运行
go build -o stock-backend .
./stock-backend
```

### 启动前端

```bash
cd stock-frontend

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建生产版本
npm run build
```

### 访问地址

- 前端: http://localhost:5173 (开发模式)
- 后端: http://localhost:8081

---

*文档生成时间: 2026-06-15*
*项目版本: V2.0*
