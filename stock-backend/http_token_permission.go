package main

import (
	"fmt"
	"net/http"
	"stock-backend/tushare"
	"time"
)

// PermissionTestResult 单个 API 的权限测试结果。
type PermissionTestResult struct {
	APIClass       string `json:"api_class"`
	Category       string `json:"category"`
	APIDisplayName string `json:"api_display_name"`
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	PointsHint     string `json:"points_hint"`
}

// apiTestCase 定义一个最小化探测用例。
type apiTestCase struct {
	ClassName   string
	Category    string
	DisplayName string
	PointsHint  string
	Params      map[string]string
	Fields      string
}

// buildAllTestCases 返回全部 tushare 股票数据接口的探测用例。
func buildAllTestCases(testDate, lastWeek, lastMonth string) []apiTestCase {
	return []apiTestCase{
		// ── 基础数据 ──
		{"stock_basic", "基础数据", "股票列表", "120积分",
			map[string]string{"list_status": "L"}, "ts_code,name"},
		{"trade_cal", "基础数据", "交易日历", "120积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "cal_date,is_open"},
		{"stock_company", "基础数据", "上市公司信息", "120积分",
			map[string]string{"exchange": "SSE"}, "ts_code,chairman"},
		{"namechange", "基础数据", "股票曾用名", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,name,start_date,end_date"},
		{"hs_const", "基础数据", "沪深股通成分", "120积分",
			map[string]string{"hs_type": "SH"}, "ts_code,in_date"},
		{"stk_rewards", "基础数据", "管理层薪酬", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,name"},
		{"new_share", "基础数据", "IPO新股列表", "120积分",
			map[string]string{}, "ts_code,name"},

		// ── 行情数据 ──
		{"daily", "行情数据", "日线行情", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"weekly", "行情数据", "周线行情", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": lastWeek, "end_date": testDate}, "ts_code,trade_date,close"},
		{"monthly", "行情数据", "月线行情", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": lastMonth, "end_date": testDate}, "ts_code,trade_date,close"},
		{"adj_factor", "行情数据", "复权因子", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,adj_factor"},
		{"daily_basic", "行情数据", "每日指标", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,pe,pb"},
		{"suspend_d", "行情数据", "停复牌信息", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,suspend_type"},
		{"stk_limit", "行情数据", "涨跌停价格", "120积分",
			map[string]string{"trade_date": testDate}, "trade_date,ts_code,up_limit,down_limit"},
		{"hsgt_top10", "行情数据", "沪深港通十大成交", "120积分",
			map[string]string{"trade_date": testDate, "market_type": "1"}, "trade_date,ts_code,name"},
		{"ggt_top10", "行情数据", "港股通十大成交", "120积分",
			map[string]string{"trade_date": testDate}, "trade_date,ts_code,name"},
		{"moneyflow_hsgt", "行情数据", "沪深港通资金流向", "120积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "trade_date,north_money"},
		{"index_daily", "行情数据", "指数行情", "120积分",
			map[string]string{"ts_code": "000001.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},

		// ── 财务数据 ──
		{"income", "财务数据", "利润表", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,revenue"},
		{"balancesheet", "财务数据", "资产负债表", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,total_assets"},
		{"cashflow", "财务数据", "现金流量表", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,n_cashflow_act"},
		{"fina_indicator", "财务数据", "财务指标", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,roe"},
		{"fina_mainbz", "财务数据", "主营业务构成", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,main_type"},
		{"forecast", "财务数据", "业绩预告", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,type"},
		{"express", "财务数据", "业绩快报", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,revenue"},
		{"dividend", "财务数据", "分红送股", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,cash_div"},
		{"disclosure_date", "财务数据", "财报披露日期", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,pre_date"},
		{"fina_audit", "财务数据", "财务审计意见", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,audit_result"},

		// ── 参考数据 ──
		{"pledge_stat", "参考数据", "股权质押统计", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,pledge_ratio"},
		{"pledge_detail", "参考数据", "股权质押明细", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,holder_name"},
		{"repurchase", "参考数据", "股票回购", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,ann_date"},
		{"stk_holdernumber", "参考数据", "股东人数", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,holder_num"},
		{"stk_holdertrade", "参考数据", "股东增减持", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,holder_name"},
		{"top_list", "参考数据", "龙虎榜每日明细", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name,close"},
		{"top_inst", "参考数据", "龙虎榜机构明细", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,exalter"},
		{"block_trade", "参考数据", "大宗交易", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,trade_date"},

		// ── 资金流向 ──
		{"moneyflow", "资金流向", "个股资金流向", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,net_mf_vol"},
		{"moneyflow_dc", "资金流向", "大盘资金流向", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,buy_sm_amount"},
		{"moneyflow_ind_ths", "资金流向", "行业资金流向", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,buy_sm_amount"},
		{"moneyflow_ths", "资金流向", "同花顺个股资金流", "2000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},

		// ── 两融数据 ──
		{"margin", "两融数据", "融资融券汇总", "2000积分",
			map[string]string{"exchange_id": "SSE", "start_date": testDate, "end_date": testDate}, "trade_date,exchange_id,rzye"},
		{"margin_detail", "两融数据", "融资融券明细", "2000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,rzye"},

		// ── 特色数据 ──
		{"cyq_perf", "特色数据", "筹码及胜率", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "trade_date": testDate}, "ts_code,trade_date,his_low"},
		{"cyq_chips", "特色数据", "筹码分布", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "trade_date": testDate}, "ts_code,trade_date,price"},
		{"stk_factor", "特色数据", "技术面因子", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,macd"},
		{"ths_daily", "特色数据", "同花顺概念行情", "2000积分",
			map[string]string{"ts_code": "885947.TI", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"ths_member", "特色数据", "同花顺概念成分", "2000积分",
			map[string]string{"ts_code": "885947.TI"}, "ts_code,code,name"},
		{"concept", "特色数据", "概念股列表", "2000积分",
			map[string]string{}, "code,name"},
		{"concept_detail", "特色数据", "概念股详情", "2000积分",
			map[string]string{"id": "TS2"}, "ts_code,name"},

		// ── 打板专题 ──
		{"limit_list_d", "打板专题", "涨跌停和炸板", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
		{"limit_step", "打板专题", "连板统计", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
		{"ths_hot", "打板专题", "同花顺热榜", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
	}
}

// tokenPermissionTestHandler 处理 POST /api/tokens/test_permissions。
func tokenPermissionTestHandler(w http.ResponseWriter, r *http.Request) {
	if prepareJSONWithCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		respondMethodNotAllowed(w)
		return
	}

	token := tushare.GetToken()
	if token == "" {
		respondBadRequest(w, "当前无生效 token，请先激活一个 token")
		return
	}

	now := time.Now()
	testDate := now.AddDate(0, 0, -1).Format("20060102")
	lastWeek := now.AddDate(0, 0, -7).Format("20060102")
	lastMonth := now.AddDate(0, -1, 0).Format("20060102")

	testCases := buildAllTestCases(testDate, lastWeek, lastMonth)
	results := make([]PermissionTestResult, 0, len(testCases))

	for _, tc := range testCases {
		reqBody := tushare.TushareRequest{
			ApiName: tc.ClassName,
			Token:   token,
			Params:  tc.Params,
			Fields:  tc.Fields,
		}

		tsResp, err := tushare.ExecuteTestRequest(reqBody)

		result := PermissionTestResult{
			APIClass:       tc.ClassName,
			Category:       tc.Category,
			APIDisplayName: tc.DisplayName,
			PointsHint:     tc.PointsHint,
		}

		if err != nil {
			result.Success = false
			result.Message = err.Error()
		} else if tsResp.Code != 0 {
			result.Success = false
			result.Message = fmt.Sprintf("code=%d: %s", tsResp.Code, tsResp.Msg)
		} else {
			result.Success = true
			count := len(tsResp.Data.Items)
			if count > 0 {
				result.Message = fmt.Sprintf("%d 条数据", count)
			} else {
				result.Message = "权限可用（非交易日或无数据）"
			}
		}

		results = append(results, result)
	}

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	respondOK(w, map[string]interface{}{
		"data":          results,
		"total":         len(results),
		"success_count": successCount,
	})
}
