const API_BASE = `http://${window.location.hostname}:8081`;

function buildURL(path, params = {}) {
  const url = new URL(`${API_BASE}${path}`);
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      url.searchParams.set(key, String(value));
    }
  });
  return url.toString();
}

async function request(path, options = {}) {
  const { method = "GET", params, body } = options;
  const response = await fetch(buildURL(path, params), {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  return response.json();
}

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
  getAutoSyncConfig: () => request("/api/auto_sync/config"),
  updateAutoSyncConfig: (payload) => request("/api/auto_sync/config", { method: "PUT", body: payload }),
  listAutoSyncRuns: (limit = 20) => request("/api/auto_sync/runs", { params: { limit } }),
  listAutoSyncRunSteps: (runId) => request("/api/auto_sync/run_steps", { params: { run_id: runId } }),
  triggerAutoSyncNow: () => request("/api/auto_sync/run_now", { method: "POST" }),
  getEmailNotifyConfig: () => request("/api/notify/email/config"),
  updateEmailNotifyConfig: (payload) => request("/api/notify/email/config", { method: "PUT", body: payload }),
  listEmailRecipients: () => request("/api/notify/email/recipients"),
  createEmailRecipient: (payload) => request("/api/notify/email/recipients", { method: "POST", body: payload }),
  updateEmailRecipient: (payload) => request("/api/notify/email/recipients", { method: "PUT", body: payload }),
  deleteEmailRecipient: (id) => request("/api/notify/email/recipients", { method: "DELETE", params: { id } }),
  sendRunReportEmail: (payload) => request("/api/notify/email/send_run", { method: "POST", body: payload }),
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
  // 优先使用中性命名接口，旧接口保留兼容
  positionRisk: () => request("/api/position/risk"),
  monitor: () => request("/api/monitor"),
  listPositions: () => request("/api/position/list"),
  addPosition: (payload) => request("/api/position/add", { method: "POST", body: payload }),
  deletePosition: (payload) => request("/api/position/delete", { method: "POST", body: payload }),
};

export default api;
