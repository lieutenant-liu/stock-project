package tushare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// 💥 废弃 hardcode const，启用并发安全的动态 Token 弹夹
var (
	currentToken = "a2fb17ef3159218fabee30c54f270a64b4e6a448252436e6b548da5c" // 默认底火
	tokenMutex   sync.RWMutex
)

const TUSHARE_URL = "https://api.tushare.pro"

// 💥 新增：时间切片分发器 (按年拆分，突破 Tushare 单次 5000 条限制)
func splitDateRange(start, end string, yearsPerChunk int) [][2]string {
	layout := "20060102"
	startTime, err := time.Parse(layout, start)
	if err != nil {
		return [][2]string{{start, end}} // 降级兜底
	}
	endTime, err := time.Parse(layout, end)
	if err != nil {
		return [][2]string{{start, end}}
	}

	var chunks [][2]string
	curr := startTime
	for curr.Before(endTime) || curr.Equal(endTime) {
		next := curr.AddDate(yearsPerChunk, 0, 0)
		if next.After(endTime) {
			next = endTime
		}
		chunks = append(chunks, [2]string{curr.Format(layout), next.Format(layout)})
		curr = next.AddDate(0, 0, 1) // 下一个切片的起点为当前切片终点+1天
	}
	return chunks
}

// SetToken 动态装填高权 Token
func SetToken(newToken string) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	currentToken = newToken
	fmt.Println("🔋 [情报部] Tushare 高阶 Token 已动态装填完毕！")
}

// GetToken 安全读取当前 Token
func GetToken() string {
	tokenMutex.RLock()
	defer tokenMutex.RUnlock()
	return currentToken
}

type TushareRequest struct {
	ApiName string            `json:"api_name"`
	Token   string            `json:"token"`
	Params  map[string]string `json:"params"`
	Fields  string            `json:"fields"`
}

type TushareResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items [][]interface{} `json:"items"`
	} `json:"data"`
}

// 💥 升级：11大金刚全字段
type DailyKLine struct {
	TSCode    string  `json:"ts_code"`
	TradeDate string  `json:"trade_date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	PreClose  float64 `json:"pre_close"`
	Change    float64 `json:"change"`
	PctChg    float64 `json:"pct_chg"`
	Vol       float64 `json:"vol"`
	Amount    float64 `json:"amount"`
	// ==========================================
	// 💥 V2.2 数据血缘与治理字段
	DataSource string `json:"data_source"` // 数据源标记 (如 TUSHARE, EASTMONEY)
	TrustLevel int    `json:"trust_level"` // 可信度权重 (如 100, 40)
	// ==========================================
}

// 极其硬核的类型防错转换器（应对 Tushare 偶尔返回 int 或 nil 的情况）
func parseFloat(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case float32:
		return float64(v)
	}
	return 0
}

// FetchStockHistory 升级版：带防截断时间切片的全量日线拉取
func FetchStockHistory(tsCode string, startDate string, endDate string) ([]DailyKLine, error) {
	// 💥 10年一个切片 (约2500个交易日，绝对不会触发 5000 条截断)
	chunks := splitDateRange(startDate, endDate, 10)
	var allKLines []DailyKLine

	for _, chunk := range chunks {
		reqBody := TushareRequest{
			ApiName: "daily",
			Token:   GetToken(),
			Params: map[string]string{
				"ts_code":    tsCode,
				"start_date": chunk[0], // 使用切片起点
				"end_date":   chunk[1], // 使用切片终点
			},
			Fields: "ts_code,trade_date,open,high,low,close,pre_close,change,pct_chg,vol,amount",
		}

		jsonData, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			return nil, fmt.Errorf("网络请求失败: %v", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var tsResp TushareResponse
		json.Unmarshal(body, &tsResp)

		if tsResp.Code != 0 {
			return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
		}

		for _, item := range tsResp.Data.Items {
			code, _ := item[0].(string)
			date, _ := item[1].(string)
			allKLines = append(allKLines, DailyKLine{
				TSCode:     code,
				TradeDate:  date,
				Open:       parseFloat(item[2]),
				High:       parseFloat(item[3]),
				Low:        parseFloat(item[4]),
				Close:      parseFloat(item[5]),
				PreClose:   parseFloat(item[6]),
				Change:     parseFloat(item[7]),
				PctChg:     parseFloat(item[8]),
				Vol:        parseFloat(item[9]),
				Amount:     parseFloat(item[10]),
				DataSource: "TUSHARE",
				TrustLevel: 100,
			})
		}
	}

	return allKLines, nil
}

// ... 保持上面的代码不动 ...

// StockBasicInfo 定义了公司档案的核心要素
type StockBasicInfo struct {
	TSCode   string
	Name     string
	Industry string
	Market   string
	ListDate string
}

// FetchStockBasic 拉取全市场股票花名册
func FetchStockBasic() ([]StockBasicInfo, error) {
	fmt.Println("🕵️ [情报部] 正在向 Tushare 索要 A 股全市场花名册...")

	reqBody := TushareRequest{
		ApiName: "stock_basic",
		Token:   GetToken(),
		Params: map[string]string{
			"list_status": "L", // 只获取正常上市的股票 (L=上市, D=退市, P=暂停上市)
		},
		// 我们需要：代码，名称，行业，市场类型，上市日期
		Fields: "ts_code,name,industry,market,list_date",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var basics []StockBasicInfo
	for _, item := range tsResp.Data.Items {
		// Tushare 返回的数据可能有 nil，需要做类型断言防错
		tsCode, _ := item[0].(string)
		name, _ := item[1].(string)
		industry, _ := item[2].(string)
		market, _ := item[3].(string)
		listDate, _ := item[4].(string)

		basics = append(basics, StockBasicInfo{
			TSCode:   tsCode,
			Name:     name,
			Industry: industry,
			Market:   market,
			ListDate: listDate,
		})
	}

	fmt.Printf("📦 [情报部] 成功获取 %d 只股票的档案信息！\n", len(basics))
	return basics, nil
}

// ==========================================
// 💎 基本面情报中心 (Daily Basic)
// ==========================================

// DailyFundamental 每日基本面核心指标
type DailyFundamental struct {
	TSCode       string  `json:"ts_code"`
	TradeDate    string  `json:"trade_date"`
	PE           float64 `json:"pe"`
	PB           float64 `json:"pb"`
	TotalMV      float64 `json:"total_mv"`      // 总市值 (万元)
	TurnoverRate float64 `json:"turnover_rate"` // 换手率 (%)
	DVRatio      float64 `json:"dv_ratio"`      // 股息率 (%)
	// 💥 V2.2 数据血缘与治理字段
	DataSource string `json:"data_source"`
	TrustLevel int    `json:"trust_level"`
}

// FetchDailyBasic 拉取指定区间的每日基本面指标
func FetchDailyBasic(tsCode string, startDate string, endDate string) ([]DailyFundamental, error) {
	reqBody := TushareRequest{
		ApiName: "daily_basic",
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		// 索要基本面核心大杀器
		Fields: "ts_code,trade_date,pe,pb,total_mv,turnover_rate,dv_ratio",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 基本面报错: %s", tsResp.Msg)
	}

	var fundamentals []DailyFundamental
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)

		fundamentals = append(fundamentals, DailyFundamental{
			TSCode:       code,
			TradeDate:    date,
			PE:           parseFloat(item[2]),
			PB:           parseFloat(item[3]),
			TotalMV:      parseFloat(item[4]),
			TurnoverRate: parseFloat(item[5]),
			DVRatio:      parseFloat(item[6]),
			DataSource:   "TUSHARE",
			TrustLevel:   100,
		})
	}

	return fundamentals, nil
}

// ==========================================
// 💥 V2.0 新增核心情报兵种
// ==========================================

// TradeCalendar 交易日历
type TradeCalendar struct {
	CalDate string  `json:"cal_date"`
	IsOpen  float64 `json:"is_open"` // Tushare 返回 1 或 0
}

func FetchTradeCalendar(startDate string, endDate string) ([]TradeCalendar, error) {
	reqBody := TushareRequest{
		ApiName: "trade_cal",
		Token:   GetToken(),
		Params: map[string]string{
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "cal_date,is_open",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var calendars []TradeCalendar
	for _, item := range tsResp.Data.Items {
		date, _ := item[0].(string)
		calendars = append(calendars, TradeCalendar{
			CalDate: date,
			IsOpen:  parseFloat(item[1]),
		})
	}
	return calendars, nil
}

// 升级复权因子结构体
type AdjFactor struct {
	TSCode     string  `json:"ts_code"`
	TradeDate  string  `json:"trade_date"`
	AdjFactor  float64 `json:"adj_factor"`
	DataSource string  `json:"data_source"` // 💥 新增血缘
	TrustLevel int     `json:"trust_level"` // 💥 新增权重
}

func FetchAdjFactors(tsCode string, startDate string, endDate string) ([]AdjFactor, error) {
	reqBody := TushareRequest{
		ApiName: "adj_factor",
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "ts_code,trade_date,adj_factor",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var factors []AdjFactor
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)
		factors = append(factors, AdjFactor{
			TSCode:     code,
			TradeDate:  date,
			AdjFactor:  parseFloat(item[2]),
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return factors, nil
}

// FinaIndicator 季报财务指标
type FinaIndicator struct {
	TSCode       string  `json:"ts_code"`
	AnnDate      string  `json:"ann_date"`
	EndDate      string  `json:"end_date"`
	UpdateFlag   string  `json:"update_flag"`
	ROE          float64 `json:"roe"`
	NetProfitYOY float64 `json:"netprofit_yoy"`
	CFPS         float64 `json:"cfps"`
	DataSource   string  `json:"data_source"` // 💥 新增血缘
	TrustLevel   int     `json:"trust_level"` // 💥 新增权重
}

func FetchFinaIndicators(tsCode string, startDate string, endDate string) ([]FinaIndicator, error) {
	reqBody := TushareRequest{
		ApiName: "fina_indicator",
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "ts_code,ann_date,end_date,update_flag,roe,netprofit_yoy,cfps",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", TUSHARE_URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	var indicators []FinaIndicator
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		annDate, _ := item[1].(string)
		endDate, _ := item[2].(string)
		flag, _ := item[3].(string)

		indicators = append(indicators, FinaIndicator{
			TSCode:       code,
			AnnDate:      annDate,
			EndDate:      endDate,
			UpdateFlag:   flag,
			ROE:          parseFloat(item[4]),
			NetProfitYOY: parseFloat(item[5]),
			CFPS:         parseFloat(item[6]),
			DataSource:   "TUSHARE",
			TrustLevel:   100,
		})
	}
	return indicators, nil
}

// ==========================================
// 💥 V2.0 高阶围猎数据兵种 (需 Tushare 2000 积分)
// ==========================================

// 1. 大单资金流向 (MoneyFlow)
type DailyMoneyFlow struct {
	TSCode     string  `json:"ts_code"`
	TradeDate  string  `json:"trade_date"`
	BuyLgVol   float64 `json:"buy_lg_vol"`
	SellLgVol  float64 `json:"sell_lg_vol"`
	BuyElgVol  float64 `json:"buy_elg_vol"`
	SellElgVol float64 `json:"sell_elg_vol"`
	NetMfVol   float64 `json:"net_mf_vol"`
	DataSource string  `json:"data_source"` // 💥 新增血缘
	TrustLevel int     `json:"trust_level"` // 💥 新增权重
}

func FetchMoneyFlow(tsCode, startDate, endDate string) ([]DailyMoneyFlow, error) {
	reqBody := TushareRequest{
		ApiName: "moneyflow",
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "start_date": startDate, "end_date": endDate},
		Fields:  "ts_code,trade_date,buy_lg_vol,sell_lg_vol,buy_elg_vol,sell_elg_vol,net_mf_vol",
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(TUSHARE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)
	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare报错: %s", tsResp.Msg)
	}

	var flows []DailyMoneyFlow
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)
		flows = append(flows, DailyMoneyFlow{
			TSCode: code, TradeDate: date,
			BuyLgVol: parseFloat(item[2]), SellLgVol: parseFloat(item[3]),
			BuyElgVol: parseFloat(item[4]), SellElgVol: parseFloat(item[5]), NetMfVol: parseFloat(item[6]),
			// 💥 补全血缘：
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return flows, nil
}

// ==========================================
// 💥 降维打击：2000积分专属每日涨跌停价格 (StkLimit)
// ==========================================
type StkLimit struct {
	TradeDate  string  `json:"trade_date"`
	TSCode     string  `json:"ts_code"`
	UpLimit    float64 `json:"up_limit"`
	DownLimit  float64 `json:"down_limit"`
	DataSource string  `json:"data_source"`
	TrustLevel int     `json:"trust_level"`
}

// FetchStkLimit 拉取每日涨跌停绝对价格 (2000积分高射速版)
func FetchStkLimit(tradeDate string) ([]StkLimit, error) {
	reqBody := TushareRequest{
		ApiName: "stk_limit", // 💥 官方正宗 2000 积分接口
		Token:   GetToken(),
		Params:  map[string]string{"trade_date": tradeDate},
		Fields:  "trade_date,ts_code,up_limit,down_limit",
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(TUSHARE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)
	if tsResp.Code != 0 {
		return nil, fmt.Errorf("Tushare报错: %s", tsResp.Msg)
	}

	var limits []StkLimit
	for _, item := range tsResp.Data.Items {
		if len(item) < 4 {
			continue // 防脏数据装甲
		}

		date, _ := item[0].(string)
		code, _ := item[1].(string)

		limits = append(limits, StkLimit{
			TradeDate: date, TSCode: code,
			UpLimit: parseFloat(item[2]), DownLimit: parseFloat(item[3]),
			DataSource: "TUSHARE", TrustLevel: 100,
		})
	}
	return limits, nil
}

// 3. 大盘指数行情 (IndexDaily) - 比如上证指数 000001.SH
type IndexDaily struct {
	TSCode     string  `json:"ts_code"`
	TradeDate  string  `json:"trade_date"`
	Close      float64 `json:"close"`
	Vol        float64 `json:"vol"`
	PctChg     float64 `json:"pct_chg"`
	DataSource string  `json:"data_source"` // 💥 新增血缘
	TrustLevel int     `json:"trust_level"` // 💥 新增权重
}

func FetchIndexDaily(tsCode, startDate, endDate string) ([]IndexDaily, error) {
	reqBody := TushareRequest{
		ApiName: "index_daily",
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "start_date": startDate, "end_date": endDate},
		Fields:  "ts_code,trade_date,close,vol,pct_chg",
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(TUSHARE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tsResp TushareResponse
	json.Unmarshal(body, &tsResp)

	var indices []IndexDaily
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)
		date, _ := item[1].(string)
		indices = append(indices, IndexDaily{
			TSCode: code, TradeDate: date,
			Close: parseFloat(item[2]), Vol: parseFloat(item[3]), PctChg: parseFloat(item[4]),
			// 💥 补全血缘：
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return indices, nil
}
