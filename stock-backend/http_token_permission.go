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

// buildAllTestCases 返回全部 tushare 数据接口的探测用例。
func buildAllTestCases(testDate, lastWeek, lastMonth string) []apiTestCase {
	return []apiTestCase{
		// ══════════════════════════════════════════════════════════════
		// 基础数据（13）
		// ══════════════════════════════════════════════════════════════
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
		{"stk_premarket", "基础数据", "每日股本", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": lastWeek, "end_date": testDate}, "ts_code,trade_date"},
		{"stock_st", "基础数据", "ST股票列表", "120积分",
			map[string]string{}, "ts_code,name"},
		{"stock_hsgt", "基础数据", "沪深港通股票列表", "120积分",
			map[string]string{"hs_type": "SH"}, "ts_code,name"},
		{"stk_managers", "基础数据", "上市公司管理层", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,name"},
		{"stk_rewards", "基础数据", "管理层薪酬和持股", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,name"},

		// ══════════════════════════════════════════════════════════════
		// 行情数据（22）
		// ══════════════════════════════════════════════════════════════
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
		{"stk_mins", "行情数据", "历史分钟行情", "120积分",
			map[string]string{"ts_code": "000001.SZ", "freq": "5min", "start_date": testDate + " 09:30:00", "end_date": testDate + " 10:00:00"}, "ts_code,trade_time,close"},
		{"rt_min", "行情数据", "实时分钟行情", "120积分",
			map[string]string{"ts_code": "000001.SZ", "freq": "5min"}, "ts_code,trade_time,close"},
		{"rt_k", "行情数据", "A股实时日线", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,close"},
		{"rt_min_daily", "行情数据", "A股实时分钟-日累计", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,close"},
		{"stk_week_month_adj", "行情数据", "周月线复权行情", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": lastMonth, "end_date": testDate}, "ts_code,trade_date,close"},

		// ══════════════════════════════════════════════════════════════
		// 财务数据（10）
		// ══════════════════════════════════════════════════════════════
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

		// ══════════════════════════════════════════════════════════════
		// 参考数据（16）
		// ══════════════════════════════════════════════════════════════
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
		{"block_trade", "参考数据", "大宗交易", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,trade_date"},
		{"top10_holders", "参考数据", "前十大股东", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,holder_name"},
		{"top10_floatholders", "参考数据", "前十大流通股东", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,end_date,holder_name"},
		{"share_float", "参考数据", "限售股解禁", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,float_date"},
		{"margin_secs", "参考数据", "融资融券标的", "2000积分",
			map[string]string{"exchange_id": "SSE"}, "ts_code,exchange_id"},
		{"stk_shock", "参考数据", "个股异常波动", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,trade_date"},
		{"stk_high_shock", "参考数据", "个股严重异常波动", "2000积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,trade_date"},
		{"stk_alert", "参考数据", "交易所重点提示", "2000积分",
			map[string]string{}, "ts_code,trade_date"},
		{"stk_account", "参考数据", "股票开户数据", "2000积分",
			map[string]string{"start_date": lastMonth, "end_date": testDate}, "trade_date,total_persons"},

		// ══════════════════════════════════════════════════════════════
		// 资金流向（8）
		// ══════════════════════════════════════════════════════════════
		{"moneyflow", "资金流向", "个股资金流向", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,net_mf_vol"},
		{"moneyflow_mkt_dc", "资金流向", "大盘资金流向(DC)", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,buy_sm_amount"},
		{"moneyflow_ind_dc", "资金流向", "板块资金流向(DC)", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,buy_sm_amount"},
		{"moneyflow_cnt_ths", "资金流向", "板块资金流向(THS)", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,buy_sm_amount"},
		{"moneyflow_ind_ths", "资金流向", "行业资金流向(THS)", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,buy_sm_amount"},
		{"moneyflow_ths", "资金流向", "同花顺个股资金流", "2000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},

		// ══════════════════════════════════════════════════════════════
		// 两融数据（6）
		// ══════════════════════════════════════════════════════════════
		{"margin", "两融数据", "融资融券汇总", "2000积分",
			map[string]string{"exchange_id": "SSE", "start_date": testDate, "end_date": testDate}, "trade_date,exchange_id,rzye"},
		{"margin_detail", "两融数据", "融资融券明细", "2000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,rzye"},
		{"slb_sec", "两融数据", "转融券交易汇总", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "trade_date,exchange"},
		{"slb_len", "两融数据", "转融资交易汇总", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "trade_date,exchange"},

		// ══════════════════════════════════════════════════════════════
		// 特色数据（15）
		// ══════════════════════════════════════════════════════════════
		{"cyq_perf", "特色数据", "筹码及胜率", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "trade_date": testDate}, "ts_code,trade_date,his_low"},
		{"cyq_chips", "特色数据", "筹码分布", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "trade_date": testDate}, "ts_code,trade_date,price"},
		{"stk_factor", "特色数据", "技术面因子", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,macd"},
		{"concept", "特色数据", "概念股列表", "2000积分",
			map[string]string{}, "code,name"},
		{"concept_detail", "特色数据", "概念股详情", "2000积分",
			map[string]string{"id": "TS2"}, "ts_code,name"},
		{"stk_factor_pro", "特色数据", "股票技术面因子(专业版)", "5000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,macd"},
		{"hk_hold", "特色数据", "中央结算系统持股统计", "2000积分",
			map[string]string{"code": "000001", "start_date": testDate, "end_date": testDate}, "code,trade_date"},
		{"ccass_hold_detail", "特色数据", "中央结算系统持股明细", "2000积分",
			map[string]string{"code": "000001", "trade_date": testDate}, "code,trade_date"},
		{"hk_hold", "特色数据", "沪深股通持股明细", "2000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},
		{"stk_auction_o", "特色数据", "开盘集合竞价数据", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,trade_date"},
		{"stk_auction_c", "特色数据", "收盘集合竞价数据", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,trade_date"},
		{"stk_nineturn", "特色数据", "神奇九转指标", "2000积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},
		{"stk_ah_comparison", "特色数据", "AH股比价", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,ts_code"},

		// ══════════════════════════════════════════════════════════════
		// 涨停专题（18）
		// ══════════════════════════════════════════════════════════════
		{"limit_list_d", "涨停专题", "涨跌停和炸板", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
		{"limit_step", "涨停专题", "连板统计", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
		{"ths_hot", "涨停专题", "同花顺热榜", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
		{"dc_hot", "涨停专题", "东方财富热榜", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name"},
		{"ths_index", "涨停专题", "同花顺行业概念板块", "2000积分",
			map[string]string{"exchange": "A", "type": "N"}, "ts_code,name"},
		{"ths_daily", "涨停专题", "同花顺概念和行业指数行情", "2000积分",
			map[string]string{"ts_code": "885947.TI", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"ths_member", "涨停专题", "同花顺行业概念成分", "2000积分",
			map[string]string{"ts_code": "885947.TI"}, "ts_code,code,name"},
		{"dc_index", "涨停专题", "东方财富概念板块", "2000积分",
			map[string]string{}, "ts_code,name"},
		{"dc_daily", "涨停专题", "东财概念和行业指数行情", "2000积分",
			map[string]string{"ts_code": "885947.TI", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"dc_member", "涨停专题", "东方财富概念成分", "2000积分",
			map[string]string{"ts_code": "885947.TI"}, "ts_code,code,name"},
		{"tdx_index", "涨停专题", "通达信板块信息", "2000积分",
			map[string]string{}, "ts_code,name"},
		{"tdx_member", "涨停专题", "通达信板块成分", "2000积分",
			map[string]string{"ts_code": "BK0475"}, "ts_code,code,name"},
		{"tdx_daily", "涨停专题", "通达信板块行情", "2000积分",
			map[string]string{"ts_code": "BK0475", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"top_list", "涨停专题", "龙虎榜每日统计", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,name,close"},
		{"top_inst", "涨停专题", "龙虎榜机构交易", "2000积分",
			map[string]string{"trade_date": testDate}, "ts_code,exalter"},
		{"limit_cpt_list", "涨停专题", "涨停最强板块统计", "2000积分",
			map[string]string{"trade_date": testDate}, "trade_date,limit_count"},

		// ══════════════════════════════════════════════════════════════
		// ETF专题（11）
		// ══════════════════════════════════════════════════════════════
		{"fund_basic", "ETF专题", "ETF基本信息", "120积分",
			map[string]string{"market": "E"}, "ts_code,name"},
		{"fund_daily", "ETF专题", "ETF日线行情", "120积分",
			map[string]string{"ts_code": "510300.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"fund_adj", "ETF专题", "ETF复权因子", "120积分",
			map[string]string{"ts_code": "510300.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,adj_factor"},
		{"etf_share_size", "ETF专题", "ETF份额规模", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,trade_date"},
		{"rt_etf_sz_iopv", "ETF专题", "ETF实时参考", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,trade_date"},
		{"etf_basic", "ETF专题", "ETF基本信息(新)", "120积分",
			map[string]string{}, "ts_code,name"},
		{"etf_index", "ETF专题", "ETF基准指数", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,ts_name"},
		{"mkt_idx_bmk", "ETF专题", "ETF业绩比较基准", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,benchmark"},
		{"rt_min", "ETF专题", "ETF实时分钟", "120积分",
			map[string]string{"ts_code": "510300.SH", "freq": "5min"}, "ts_code,trade_time,close"},
		{"stk_mins", "ETF专题", "ETF历史分钟", "120积分",
			map[string]string{"ts_code": "510300.SH", "start_date": testDate + " 09:30:00", "end_date": testDate + " 10:00:00"}, "ts_code,trade_time,close"},

		// ══════════════════════════════════════════════════════════════
		// 指数专题（16）
		// ══════════════════════════════════════════════════════════════
		{"index_basic", "指数专题", "指数基本信息", "120积分",
			map[string]string{"market": "SSE"}, "ts_code,name"},
		{"index_daily", "指数专题", "指数日线行情", "120积分",
			map[string]string{"ts_code": "000001.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"index_weekly", "指数专题", "指数周线行情", "120积分",
			map[string]string{"ts_code": "000001.SH", "start_date": lastMonth, "end_date": testDate}, "ts_code,trade_date,close"},
		{"index_monthly", "指数专题", "指数月线行情", "120积分",
			map[string]string{"ts_code": "000001.SH", "start_date": lastMonth, "end_date": testDate}, "ts_code,trade_date,close"},
		{"index_weight", "指数专题", "指数成分和权重", "120积分",
			map[string]string{"index_code": "000001.SH", "start_date": testDate, "end_date": testDate}, "index_code,con_code"},
		{"index_dailybasic", "指数专题", "大盘指数每日指标", "120积分",
			map[string]string{"trade_date": testDate}, "ts_code,trade_date,total_mv"},
		{"sw_daily", "指数专题", "申万行业指数日行情", "120积分",
			map[string]string{"ts_code": "801010.SI", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"index_classify", "指数专题", "申万行业分类", "120积分",
			map[string]string{"l1_name": "非银金融"}, "index_code,industry_name"},
		{"rt_sw_k", "指数专题", "申万实时行情", "120积分",
			map[string]string{"ts_code": "801010.SI"}, "ts_code,close"},
		{"ci_daily", "指数专题", "中信行业指数日行情", "120积分",
			map[string]string{"ts_code": "CI005001.WI", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"ci_index_member", "指数专题", "中信行业成分", "120积分",
			map[string]string{"ts_code": "CI005001.WI"}, "ts_code,con_code"},
		{"index_global", "指数专题", "国际主要指数", "120积分",
			map[string]string{"ts_code": "SPX"}, "ts_code,trade_date,close"},
		{"idx_factor_pro", "指数专题", "指数技术面因子(专业版)", "5000积分",
			map[string]string{"ts_code": "000001.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,macd"},
		{"daily_info", "指数专题", "沪深市场每日交易统计", "120积分",
			map[string]string{"trade_date": testDate}, "trade_date,ts_code"},
		{"sz_daily_info", "指数专题", "深圳市场每日交易情况", "120积分",
			map[string]string{"trade_date": testDate}, "trade_date,ts_code"},

		// ══════════════════════════════════════════════════════════════
		// 公募基金（8）
		// ══════════════════════════════════════════════════════════════
		{"fund_basic", "公募基金", "基金列表", "120积分",
			map[string]string{"market": "E"}, "ts_code,name"},
		{"fund_company", "公募基金", "基金管理人", "120积分",
			map[string]string{}, "name,shortname"},
		{"fund_manager", "公募基金", "基金经理", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,name"},
		{"fund_share", "公募基金", "基金规模", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,end_date"},
		{"fund_nav", "公募基金", "基金净值", "120积分",
			map[string]string{"ts_code": "510300.SH", "start_date": testDate, "end_date": testDate}, "ts_code,ann_date"},
		{"fund_div", "公募基金", "基金分红", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,ann_date"},
		{"fund_portfolio", "公募基金", "基金持仓", "120积分",
			map[string]string{"ts_code": "510300.SH"}, "ts_code,end_date"},
		{"fund_factor_pro", "公募基金", "基金技术面因子(专业版)", "5000积分",
			map[string]string{"ts_code": "510300.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},

		// ══════════════════════════════════════════════════════════════
		// 期货数据（14）
		// ══════════════════════════════════════════════════════════════
		{"fut_basic", "期货数据", "合约信息", "2000积分",
			map[string]string{"exchange": "DCE"}, "ts_code,name"},
		{"trade_cal", "期货数据", "期货交易日历", "2000积分",
			map[string]string{"exchange": "DCE", "start_date": testDate, "end_date": testDate}, "cal_date,is_open"},
		{"fut_daily", "期货数据", "期货日线行情", "2000积分",
			map[string]string{"ts_code": "M.DCE", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"fut_weekly_monthly", "期货数据", "期货周月线行情", "2000积分",
			map[string]string{"ts_code": "M.DCE", "start_date": lastMonth, "end_date": testDate}, "ts_code,trade_date,close"},
		{"rt_fut_min", "期货数据", "期货实时分钟", "2000积分",
			map[string]string{"ts_code": "M.DCE", "freq": "5min"}, "ts_code,datetime,close"},
		{"fut_wsr", "期货数据", "仓单日报", "2000积分",
			map[string]string{"ts_code": "M.DCE", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},
		{"fut_settle", "期货数据", "每日结算参数", "2000积分",
			map[string]string{"ts_code": "M.DCE", "trade_date": testDate}, "ts_code,trade_date"},
		{"fut_holding", "期货数据", "每日持仓排名", "2000积分",
			map[string]string{"ts_code": "M.DCE", "trade_date": testDate}, "ts_code,trade_date"},
		{"fut_mapping", "期货数据", "期货主力与连续合约", "2000积分",
			map[string]string{"ts_code": "M.DCE"}, "ts_code,mapping_ts_code"},
		{"index_daily", "期货数据", "南华期货指数行情", "2000积分",
			map[string]string{"ts_code": "NH0100", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"fut_weekly_detail", "期货数据", "期货主要品种交易周报", "2000积分",
			map[string]string{}, "ts_code,trade_date"},
		{"ft_limit", "期货数据", "期货合约涨跌停价格", "2000积分",
			map[string]string{"ts_code": "M.DCE", "trade_date": testDate}, "ts_code,trade_date"},

		// ══════════════════════════════════════════════════════════════
		// 现货数据（2）
		// ══════════════════════════════════════════════════════════════
		{"sge_basic", "现货数据", "上海黄金基础信息", "2000积分",
			map[string]string{}, "ts_code,name"},
		{"sge_daily", "现货数据", "上海黄金现货日行情", "2000积分",
			map[string]string{"ts_code": "Au99.99.SGE", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},

		// ══════════════════════════════════════════════════════════════
		// 期权数据（3）
		// ══════════════════════════════════════════════════════════════
		{"opt_basic", "期权数据", "期权合约信息", "2000积分",
			map[string]string{"exchange": "SSE"}, "ts_code,name"},
		{"opt_daily", "期权数据", "期权日线行情", "2000积分",
			map[string]string{"ts_code": "10001230.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"opt_mins", "期权数据", "期权分钟行情", "2000积分",
			map[string]string{"ts_code": "10001230.SH", "freq": "5min"}, "ts_code,trade_time,close"},

		// ══════════════════════════════════════════════════════════════
		// 债券专题（15）
		// ══════════════════════════════════════════════════════════════
		{"cb_basic", "债券专题", "可转债基础信息", "120积分",
			map[string]string{"exchange": "SSE"}, "ts_code,name"},
		{"cb_issue", "债券专题", "可转债发行", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date"},
		{"cb_call", "债券专题", "可转债赎回信息", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date"},
		{"cb_rate", "债券专题", "可转债票面利率", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date"},
		{"cb_daily", "债券专题", "可转债行情", "120积分",
			map[string]string{"ts_code": "110000.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"top10_cb_holders", "债券专题", "可转债十大持有人", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date,holder_name"},
		{"cb_factor_pro", "债券专题", "可转债技术面因子(专业版)", "5000积分",
			map[string]string{"ts_code": "110000.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date"},
		{"cb_price_chg", "债券专题", "可转债转股价变动", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date"},
		{"cb_share", "债券专题", "可转债转股结果", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date"},
		{"cb_rating", "债券专题", "可转债债券评级", "120积分",
			map[string]string{"ts_code": "110000.SH"}, "ts_code,ann_date"},
		{"repo_daily", "债券专题", "债券回购日行情", "120积分",
			map[string]string{"ts_code": "204001.SH", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"bc_otcqt", "债券专题", "柜台流通式债券报价", "120积分",
			map[string]string{}, "ts_code,price"},
		{"yc_cb", "债券专题", "国债收益率曲线", "120积分",
			map[string]string{"ts_code": "1001.CB", "curve_type": "0"}, "ts_code,trade_date,yield"},
		{"eco_cal", "债券专题", "全球财经事件", "120积分",
			map[string]string{"date": testDate}, "date,title"},

		// ══════════════════════════════════════════════════════════════
		// 外汇数据（2）
		// ══════════════════════════════════════════════════════════════
		{"fx_obasic", "外汇数据", "外汇基础信息", "120积分",
			map[string]string{}, "ts_code,name"},
		{"fx_daily", "外汇数据", "外汇日线行情", "120积分",
			map[string]string{"ts_code": "USDCNH.FXCM", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},

		// ══════════════════════════════════════════════════════════════
		// 港股数据（11）
		// ══════════════════════════════════════════════════════════════
		{"hk_basic", "港股数据", "港股基础信息", "120积分",
			map[string]string{}, "ts_code,name"},
		{"hk_daily", "港股数据", "港股日线行情", "120积分",
			map[string]string{"ts_code": "00700.HK", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"hk_mins", "港股数据", "港股分钟行情", "120积分",
			map[string]string{"ts_code": "00700.HK", "freq": "5min"}, "ts_code,trade_time,close"},
		{"rt_hk_k", "港股数据", "港股实时日线", "120积分",
			map[string]string{"ts_code": "00700.HK"}, "ts_code,close"},
		{"hk_daily_adj", "港股数据", "港股复权行情", "120积分",
			map[string]string{"ts_code": "00700.HK", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"hk_adjfactor", "港股数据", "港股复权因子", "120积分",
			map[string]string{"ts_code": "00700.HK", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,adj_factor"},
		{"hk_income", "港股数据", "港股利润表", "2000积分",
			map[string]string{"ts_code": "00700.HK"}, "ts_code,end_date,revenue"},
		{"hk_balancesheet", "港股数据", "港股资产负债表", "2000积分",
			map[string]string{"ts_code": "00700.HK"}, "ts_code,end_date,total_assets"},
		{"hk_cashflow", "港股数据", "港股现金流量表", "2000积分",
			map[string]string{"ts_code": "00700.HK"}, "ts_code,end_date,n_cashflow_act"},
		{"hk_fina_indicator", "港股数据", "港股财务指标", "2000积分",
			map[string]string{"ts_code": "00700.HK"}, "ts_code,end_date,roe"},
		{"hk_tradecal", "港股数据", "港股交易日历", "120积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "cal_date,is_open"},

		// ══════════════════════════════════════════════════════════════
		// 美股数据（9）
		// ══════════════════════════════════════════════════════════════
		{"us_basic", "美股数据", "美股基础信息", "120积分",
			map[string]string{}, "ts_code,name"},
		{"us_daily", "美股数据", "美股日线行情", "120积分",
			map[string]string{"ts_code": "AAPL", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"us_daily_adj", "美股数据", "美股复权行情", "120积分",
			map[string]string{"ts_code": "AAPL", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,close"},
		{"us_adjfactor", "美股数据", "美股复权因子", "120积分",
			map[string]string{"ts_code": "AAPL", "start_date": testDate, "end_date": testDate}, "ts_code,trade_date,adj_factor"},
		{"us_income", "美股数据", "美股利润表", "2000积分",
			map[string]string{"ts_code": "AAPL"}, "ts_code,end_date,revenue"},
		{"us_balancesheet", "美股数据", "美股资产负债表", "2000积分",
			map[string]string{"ts_code": "AAPL"}, "ts_code,end_date,total_assets"},
		{"us_cashflow", "美股数据", "美股现金流量表", "2000积分",
			map[string]string{"ts_code": "AAPL"}, "ts_code,end_date,n_cashflow_act"},
		{"us_fina_indicator", "美股数据", "美股财务指标", "2000积分",
			map[string]string{"ts_code": "AAPL"}, "ts_code,end_date,roe"},
		{"us_tradecal", "美股数据", "美股交易日历", "120积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "cal_date,is_open"},

		// ══════════════════════════════════════════════════════════════
		// 宏观经济（15）
		// ══════════════════════════════════════════════════════════════
		{"shibor", "宏观经济", "Shibor利率", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,on"},
		{"shibor_quote", "宏观经济", "Shibor报价数据", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,bank"},
		{"shibor_lpr", "宏观经济", "LPR贷款基础利率", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,lpr1y"},
		{"libor", "宏观经济", "Libor利率", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,curr,on"},
		{"hibor", "宏观经济", "Hibor利率", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,on"},
		{"wz_index", "宏观经济", "温州民间借贷利率", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,rate_1m"},
		{"gz_index", "宏观经济", "广州民间借贷利率", "2000积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "date,rate_1m"},
		{"cn_gdp", "宏观经济", "国内生产总值", "2000积分",
			map[string]string{}, "quarter,gdp"},
		{"cn_cpi", "宏观经济", "居民消费价格指数", "2000积分",
			map[string]string{}, "month,nt_val"},
		{"cn_ppi", "宏观经济", "工业生产者出厂价格指数", "2000积分",
			map[string]string{}, "month,ppi_yoy"},
		{"cn_m", "宏观经济", "货币供应量", "2000积分",
			map[string]string{}, "month,m2_yoy"},
		{"sf_month", "宏观经济", "社融增量", "2000积分",
			map[string]string{}, "month,inc"},
		{"cn_pmi", "宏观经济", "采购经理指数", "2000积分",
			map[string]string{}, "month,pmi"},
		{"us_tycr", "宏观经济", "国债收益率曲线利率", "2000积分",
			map[string]string{}, "date,ts_10y"},
		{"cn_schedule", "宏观经济", "中国经济数据发布日程", "2000积分",
			map[string]string{}, "date,title"},

		// ══════════════════════════════════════════════════════════════
		// 大模型语料（8）
		// ══════════════════════════════════════════════════════════════
		{"npr", "大模型语料", "国家政策库", "2000积分",
			map[string]string{}, "title,pub_date"},
		{"research_report", "大模型语料", "券商研究报告", "2000积分",
			map[string]string{}, "title,pub_date"},
		{"news", "大模型语料", "新闻快讯", "120积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "title,pub_time"},
		{"major_news", "大模型语料", "新闻通讯", "120积分",
			map[string]string{"start_date": testDate, "end_date": testDate}, "title,pub_time"},
		{"cctv_news", "大模型语料", "新闻联播文字稿", "120积分",
			map[string]string{"date": testDate}, "title,date"},
		{"anns_d", "大模型语料", "上市公司公告", "120积分",
			map[string]string{"ts_code": "000001.SZ", "start_date": testDate, "end_date": testDate}, "ts_code,title,ann_date"},
		{"irm_qa_sh", "大模型语料", "上证e互动问答", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,question"},
		{"irm_qa_sz", "大模型语料", "深证易互动问答", "120积分",
			map[string]string{"ts_code": "000001.SZ"}, "ts_code,question"},

		// ══════════════════════════════════════════════════════════════
		// 财富管理（2）
		// ══════════════════════════════════════════════════════════════
		{"fund_sales_ratio", "财富管理", "各渠道基金销售占比", "2000积分",
			map[string]string{}, "quarter"},
		{"fund_sales_vol", "财富管理", "销售机构基金销售保有", "2000积分",
			map[string]string{}, "quarter"},
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
