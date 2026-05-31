const API_BASE = '';

function buildURL(path, params = {}) {
  // 统一 query 参数序列化，避免各模块重复拼接 URL。
  const url = new URL(`${API_BASE}${path}`, window.location.origin);
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      url.searchParams.set(key, String(value));
    }
  });
  return url.toString();
}

async function request(path, options = {}) {
  const { method = "GET", params, body } = options;
  // 所有接口都走同一出口，便于未来统一增加鉴权/重试/错误埋点。
  const response = await fetch(buildURL(path, params), {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  return response.json();
}

// 领域 API 清单：前端各模块只依赖这里，不直接写 fetch。
const api = {
  getLogs: () => request("/api/logs"),
  diagnose: (params) => request("/api/diagnose", { params }),
  audit: (params) => request("/api/audit", { params }),
  setToken: (token) => request("/api/set_token", { params: { token } }),
  listTokens: (provider = "tushare") => request("/api/tokens", { params: { provider } }),
  createToken: (payload) => request("/api/tokens", { method: "POST", body: payload }),
  updateTokenItem: (payload) => request("/api/tokens", { method: "PUT", body: payload }),
  deleteTokenItem: (id) => request("/api/tokens", { method: "DELETE", params: { id } }),
  activateToken: (payload) => request("/api/tokens/activate", { method: "POST", body: payload }),
  testTokenPermissions: () => request("/api/tokens/test_permissions", { method: "POST" }),
  getAutoSyncConfig: () => request("/api/auto_sync/config"),
  updateAutoSyncConfig: (payload) => request("/api/auto_sync/config", { method: "PUT", body: payload }),
  listAutoSyncRuns: (limit = 20) => request("/api/auto_sync/runs", { params: { limit } }),
  listAutoSyncRunSteps: (runId) => request("/api/auto_sync/run_steps", { params: { run_id: runId } }),
  triggerAutoSyncNow: () => request("/api/auto_sync/run_now", { method: "POST" }),
  getSystemConfig: () => request("/api/system/config"),
  updateSystemConfig: (payload) => request("/api/system/config", { method: "PUT", body: payload }),
  getEmailNotifyConfig: () => request("/api/notify/email/config"),
  updateEmailNotifyConfig: (payload) => request("/api/notify/email/config", { method: "PUT", body: payload }),
  listEmailRecipients: () => request("/api/notify/email/recipients"),
  createEmailRecipient: (payload) => request("/api/notify/email/recipients", { method: "POST", body: payload }),
  updateEmailRecipient: (payload) => request("/api/notify/email/recipients", { method: "PUT", body: payload }),
  deleteEmailRecipient: (id) => request("/api/notify/email/recipients", { method: "DELETE", params: { id } }),
  sendStrategyScanReportEmail: (payload) => request("/api/notify/email/send_strategy_scan", { method: "POST", body: payload }),
  setSpeed: (speed) => request("/api/set_speed", { params: { speed } }),
  startSyncCalendar: (source) => request("/api/start_sync_calendar", { params: { source } }),
  startSyncBasic: () => request("/api/start_sync_basic"),
  startSyncKline: (params) => request("/api/start_sync_kline", { params }),
  startSyncFund: (params) => request("/api/start_sync_fund", { params }),
  startSyncAdj: (params) => request("/api/start_sync_adj", { params }),
  startSyncIndex: (params) => request("/api/start_sync_index", { params }),
  startSyncMoneyFlow: (params) => request("/api/start_sync_moneyflow", { params }),
  startSyncFina: (params) => request("/api/start_sync_fina", { params }),
  startSyncLimit: (params) => request("/api/start_sync_limit", { params }),
  startSyncCyqPerf: (params) => request("/api/start_sync_cyqperf", { params }),
  startSyncStkFactorPro: (params) => request("/api/start_sync_stkfactorpro", { params }),
  // 优先使用中性命名接口，旧接口保留兼容
  positionRisk: () => request("/api/position/risk"),
  monitor: () => request("/api/monitor"),
  listPositions: () => request("/api/position/list"),
  addPosition: (payload) => request("/api/position/add", { method: "POST", body: payload }),
  deletePosition: (payload) => request("/api/position/delete", { method: "POST", body: payload }),
  runBacktest: (payload) => request("/api/backtest", { method: "POST", body: payload }),
  submitBacktest: (payload) => request("/api/backtest", { method: "POST", body: payload }),
  getBacktestJob: (id) => request(`/api/backtest/${id}`),
  listBacktestJobs: (limit = 20) => request("/api/backtest/list", { params: { limit } }),
  deleteBacktestJob: (id) => request(`/api/backtest/${id}`, { method: "DELETE" }),
  downloadBacktestCSV: (jobId) => {
    const link = document.createElement('a')
    link.href = jobId ? `/api/backtest/download?job_id=${jobId}` : '/api/backtest/download'
    link.download = ''
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  },
  // 回测计划（批量多任务）
  submitBacktestPlan: (payload) => request("/api/backtest/plan", { method: "POST", body: payload }),
  getBacktestPlan: (id) => request(`/api/backtest/plan/${id}`),
  listBacktestPlans: (limit = 20) => request("/api/backtest/plan/list", { params: { limit } }),
  deleteBacktestPlan: (id) => request(`/api/backtest/plan/${id}`, { method: "DELETE" }),
  downloadPlanTaskCSV: (taskId) => {
    const link = document.createElement('a')
    link.href = `/api/backtest/plan/download?task_id=${taskId}`
    link.download = ''
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  },
  // 信号实验室
  runSignalLab: (payload) => request("/api/signallab/run", { method: "POST", body: payload }),
  listSignalLabJobs: (limit = 20) => request("/api/signallab/jobs", { params: { limit } }),
  downloadSignalLabCSV: (jobId) => {
    // 强制拼接后端真实地址，绕过前端代理
    const backendUrl = `http://${window.location.hostname}:8081`;
    const downloadUrl = `${backendUrl}/api/signallab/download?job_id=${jobId}`;
    // Content-Disposition: attachment 会让浏览器拦截为文件下载，不会跳转页面
    window.location.href = downloadUrl;
  },
  // 策略管理
  listStrategies: () => request("/api/strategies"),
  listActiveStrategies: () => request("/api/strategies/active"),
  toggleStrategy: (id, isEnabled) => request("/api/strategies/toggle", { method: "POST", body: { id, is_enabled: isEnabled } }),
  // 引擎子策略配置
  getEngineConfig: () => request("/api/config/engine"),
  toggleEngineConfig: (key, enabled) => request("/api/config/engine/toggle", { method: "POST", body: { key, enabled } }),
  updateExitProfile: (profile) => request("/api/config/engine/profile", { method: "POST", body: { profile } }),
};

export default api;
