// ============================================================
// src/api/client.js - API 客户端
// ============================================================
// 这个文件是前端与后端通信的唯一出口。
// 所有 API 调用都通过这个文件封装，方便统一管理。
//
// 【为什么需要封装？】
// 1. 避免每个组件重复写 fetch 代码
// 2. 统一错误处理
// 3. 统一添加认证头（如 Token）
// 4. 方便切换 API 地址（开发/生产环境）
//
// 【JavaScript 知识点】
// - fetch: 浏览器内置的 HTTP 请求 API
// - async/await: 异步编程语法糖
// - URL: URL 解析对象
// - JSON.stringify: 将对象转为 JSON 字符串
// - template literal: 模板字符串 `hello ${name}`
// ============================================================

// API 基础地址
// 空字符串表示使用当前域名（前后端同域部署）
// 如果前后端分离部署，改为后端地址，如 "http://localhost:8081"
const API_BASE = '';

// ------------------------------------------------------------
// 内部工具函数
// ------------------------------------------------------------

// buildURL 构建完整的 API URL。
// 【功能】
// 将路径和查询参数拼接成完整的 URL。
// 例如：buildURL("/api/logs", {limit: 20}) → "/api/logs?limit=20"
//
// 【参数】
// - path: API 路径，如 "/api/logs"
// - params: 查询参数对象，如 {limit: 20, page: 1}
function buildURL(path, params = {}) {
  // 创建 URL 对象
  const url = new URL(`${API_BASE}${path}`, window.location.origin);

  // 遍历参数，添加到 URL
  Object.entries(params).forEach(([key, value]) => {
    // 跳过空值（undefined, null, ""）
    if (value !== undefined && value !== null && value !== "") {
      url.searchParams.set(key, String(value));
    }
  });

  return url.toString();
}

// request 统一的 HTTP 请求函数。
// 【功能】
// 封装 fetch API，提供统一的请求接口。
//
// 【参数】
// - path: API 路径，如 "/api/logs"
// - options: 请求选项
//   - method: HTTP 方法（默认 "GET"）
//   - params: 查询参数（GET 请求使用）
//   - body: 请求体（POST/PUT 请求使用）
//
// 【返回值】
// Promise<any>: 解析后的 JSON 响应
async function request(path, options = {}) {
  const { method = "GET", params, body } = options;

  // 发送请求
  const response = await fetch(buildURL(path, params), {
    method,
    // 如果有 body，设置 Content-Type 为 JSON
    headers: body ? { "Content-Type": "application/json" } : undefined,
    // 如果有 body，序列化为 JSON 字符串
    body: body ? JSON.stringify(body) : undefined,
  });

  // 解析响应为 JSON
  return response.json();
}

// ------------------------------------------------------------
// 领域 API 清单
// ------------------------------------------------------------
// 前端各模块只依赖这里，不直接写 fetch。
// 这样做的好处：
// 1. API 路径集中管理，修改一处即可
// 2. 参数类型清晰，IDE 可以提示
// 3. 方便 mock 测试
// ------------------------------------------------------------

const api = {
  // ── 日志与诊断 ──
  getLogs: () => request("/api/logs"),                          // 获取系统日志
  diagnose: (params) => request("/api/diagnose", { params }),   // 策略诊断
  audit: (params) => request("/api/audit", { params }),         // 数据审计

  // ── Token 管理 ──
  setToken: (token) => request("/api/set_token", { params: { token } }),  // 设置 Token（旧接口）
  listTokens: (provider = "tushare") => request("/api/tokens", { params: { provider } }),  // 获取 Token 列表
  createToken: (payload) => request("/api/tokens", { method: "POST", body: payload }),     // 创建 Token
  updateTokenItem: (payload) => request("/api/tokens", { method: "PUT", body: payload }),  // 更新 Token
  deleteTokenItem: (id) => request("/api/tokens", { method: "DELETE", params: { id } }),   // 删除 Token
  activateToken: (payload) => request("/api/tokens/activate", { method: "POST", body: payload }),  // 激活 Token
  testTokenPermissions: () => request("/api/tokens/test_permissions", { method: "POST" }),  // 测试 Token 权限

  // ── 自动同步 ──
  getAutoSyncConfig: () => request("/api/auto_sync/config"),                    // 获取自动同步配置
  updateAutoSyncConfig: (payload) => request("/api/auto_sync/config", { method: "PUT", body: payload }),  // 更新配置
  listAutoSyncRuns: (limit = 20) => request("/api/auto_sync/runs", { params: { limit } }),  // 获取运行记录
  listAutoSyncRunSteps: (runId) => request("/api/auto_sync/run_steps", { params: { run_id: runId } }),  // 获取步骤记录
  triggerAutoSyncNow: () => request("/api/auto_sync/run_now", { method: "POST" }),  // 立即执行同步

  // ── 系统配置 ──
  getSystemConfig: () => request("/api/system/config"),                         // 获取系统配置
  updateSystemConfig: (payload) => request("/api/system/config", { method: "PUT", body: payload }),  // 更新系统配置

  // ── 邮件通知 ──
  getEmailNotifyConfig: () => request("/api/notify/email/config"),              // 获取邮件配置
  updateEmailNotifyConfig: (payload) => request("/api/notify/email/config", { method: "PUT", body: payload }),  // 更新邮件配置
  listEmailRecipients: () => request("/api/notify/email/recipients"),           // 获取收件人列表
  createEmailRecipient: (payload) => request("/api/notify/email/recipients", { method: "POST", body: payload }),  // 添加收件人
  updateEmailRecipient: (payload) => request("/api/notify/email/recipients", { method: "PUT", body: payload }),   // 更新收件人
  deleteEmailRecipient: (id) => request("/api/notify/email/recipients", { method: "DELETE", params: { id } }),    // 删除收件人
  sendStrategyScanReportEmail: (payload) => request("/api/notify/email/send_strategy_scan", { method: "POST", body: payload }),  // 发送策略扫描邮件

  // ── 数据同步 ──
  setSpeed: (speed) => request("/api/set_speed", { params: { speed } }),       // 设置请求速度
  startSyncCalendar: (source) => request("/api/start_sync_calendar", { params: { source } }),  // 同步交易日历
  startSyncBasic: () => request("/api/start_sync_basic"),                       // 同步股票花名册
  startSyncKline: (params) => request("/api/start_sync_kline", { params }),     // 同步 K 线
  startSyncFund: (params) => request("/api/start_sync_fund", { params }),       // 同步基本面
  startSyncAdj: (params) => request("/api/start_sync_adj", { params }),         // 同步复权因子
  startSyncIndex: (params) => request("/api/start_sync_index", { params }),     // 同步大盘指数
  startSyncMoneyFlow: (params) => request("/api/start_sync_moneyflow", { params }),  // 同步资金流向
  startSyncFina: (params) => request("/api/start_sync_fina", { params }),       // 同步财务指标
  startSyncLimit: (params) => request("/api/start_sync_limit", { params }),     // 同步涨跌停价格
  startSyncCyqPerf: (params) => request("/api/start_sync_cyqperf", { params }),  // 同步筹码分布
  startSyncStkFactorPro: (params) => request("/api/start_sync_stkfactorpro", { params }),  // 同步技术因子

  // ── 持仓管理 ──
  // 优先使用中性命名接口，旧接口保留兼容
  positionRisk: () => request("/api/position/risk"),       // 获取风险评估
  monitor: () => request("/api/monitor"),                  // 旧接口（兼容）
  listPositions: () => request("/api/position/list"),      // 获取持仓列表
  addPosition: (payload) => request("/api/position/add", { method: "POST", body: payload }),       // 添加持仓
  deletePosition: (payload) => request("/api/position/delete", { method: "POST", body: payload }), // 删除持仓

  // ── 回测 ──
  runBacktest: (payload) => request("/api/backtest", { method: "POST", body: payload }),  // 运行回测
  submitBacktest: (payload) => request("/api/backtest", { method: "POST", body: payload }),  // 提交回测（别名）
  getBacktestJob: (id) => request(`/api/backtest/${id}`),                                    // 获取回测任务
  listBacktestJobs: (limit = 20) => request("/api/backtest/list", { params: { limit } }),   // 获取回测列表
  deleteBacktestJob: (id) => request(`/api/backtest/${id}`, { method: "DELETE" }),           // 删除回测任务

  // 下载回测 CSV 报表
  downloadBacktestCSV: (jobId) => {
    const link = document.createElement('a')
    link.href = jobId ? `/api/backtest/download?job_id=${jobId}` : '/api/backtest/download'
    link.download = ''
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  },

  // ── 回测计划（批量多任务）──
  submitBacktestPlan: (payload) => request("/api/backtest/plan", { method: "POST", body: payload }),  // 提交计划
  getBacktestPlan: (id) => request(`/api/backtest/plan/${id}`),                                        // 获取计划详情
  listBacktestPlans: (limit = 20) => request("/api/backtest/plan/list", { params: { limit } }),       // 获取计划列表
  deleteBacktestPlan: (id) => request(`/api/backtest/plan/${id}`, { method: "DELETE" }),               // 删除计划

  // 下载计划任务 CSV
  downloadPlanTaskCSV: (taskId) => {
    const link = document.createElement('a')
    link.href = `/api/backtest/plan/download?task_id=${taskId}`
    link.download = ''
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  },

  // ── 信号实验室 ──
  runSignalLab: (payload) => request("/api/signallab/run", { method: "POST", body: payload }),  // 运行信号实验
  listSignalLabJobs: (limit = 20) => request("/api/signallab/jobs", { params: { limit } }),     // 获取任务列表

  // 下载信号实验室 CSV
  downloadSignalLabCSV: (jobId) => {
    // 强制拼接后端真实地址，绕过前端代理
    const backendUrl = `http://${window.location.hostname}:8081`;
    const downloadUrl = `${backendUrl}/api/signallab/download?job_id=${jobId}`;
    // Content-Disposition: attachment 会让浏览器拦截为文件下载，不会跳转页面
    window.location.href = downloadUrl;
  },

  // ── 策略管理 ──
  listStrategies: () => request("/api/strategies"),                // 获取策略列表
  listActiveStrategies: () => request("/api/strategies/active"),   // 获取已启用策略
  toggleStrategy: (id, isEnabled) => request("/api/strategies/toggle", { method: "POST", body: { id, is_enabled: isEnabled } }),  // 切换策略开关

  // ── 引擎子策略配置 ──
  getEngineConfig: () => request("/api/config/engine"),            // 获取引擎配置
  toggleEngineConfig: (key, enabled) => request("/api/config/engine/toggle", { method: "POST", body: { key, enabled } }),  // 切换子策略开关
  updateExitProfile: (profile) => request("/api/config/engine/profile", { method: "POST", body: { profile } }),            // 更新退出流派
};

export default api;
