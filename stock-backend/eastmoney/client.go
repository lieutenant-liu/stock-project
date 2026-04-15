package eastmoney

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"stock-backend/tushare"
	"strconv"
	"strings"
	"time"
)

// FetchStockHistory 💥 责任链总控：东财(主力) -> 腾讯(灾备)
func FetchStockHistory(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	fmt.Printf("🕵️ [责任链] 节点 1: 尝试通过【东方财富 (Trust:50)】拉取 %s...\n", tsCode)
	klines, err := fetchFromEastMoney(tsCode, startDate, endDate)

	if err == nil && len(klines) > 0 {
		return klines, nil // 节点 1 成功，直接返回
	}

	fmt.Printf("⚠️ [责任链] 节点 1 阵亡 (%v)。触发降级，节点 2:【腾讯财经 (Trust:30)】接管...\n", err)
	return fetchFromTencent(tsCode, startDate, endDate)
}

// ==========================================
// 🛡️ 节点 1：东方财富 (全字段主力，TrustLevel: 50)
// ==========================================
func fetchFromEastMoney(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 强制走 HTTP 直连防代理 EOF
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=0&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true // 禁用 Keep-Alive，防服务端掐断
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	if err := json.Unmarshal(body, &emResp); err != nil {
		return nil, err
	}

	if emResp.Data.Klines == nil {
		return []tushare.DailyKLine{}, nil
	}

	var klines []tushare.DailyKLine
	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 11 {
			continue
		}

		open, _ := strconv.ParseFloat(fields[1], 64)
		closePrice, _ := strconv.ParseFloat(fields[2], 64)
		high, _ := strconv.ParseFloat(fields[3], 64)
		low, _ := strconv.ParseFloat(fields[4], 64)
		vol, _ := strconv.ParseFloat(fields[5], 64)
		amount, _ := strconv.ParseFloat(fields[6], 64)
		pctChg, _ := strconv.ParseFloat(fields[8], 64)
		change, _ := strconv.ParseFloat(fields[9], 64)

		klines = append(klines, tushare.DailyKLine{
			TSCode: tsCode, TradeDate: strings.ReplaceAll(fields[0], "-", ""),
			Open: open, Close: closePrice, High: high, Low: low,
			Vol: vol, Amount: amount / 1000.0,
			PctChg: pctChg, Change: change, PreClose: closePrice - change,

			// 💥 注入血缘标记
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}
	return klines, nil
}

// ==========================================
// 🛡️ 节点 2：腾讯财经 (海外节点不死鸟，TrustLevel: 30)
// ==========================================
func fetchFromTencent(tsCode, startDate, endDate string) ([]tushare.DailyKLine, error) {
	tenCode := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		tenCode = strings.ToLower(parts[1]) + parts[0]
	}

	t_start := startDate[:4] + "-" + startDate[4:6] + "-" + startDate[6:8]
	t_end := endDate[:4] + "-" + endDate[4:6] + "-" + endDate[6:8]

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,day,%s,%s,500,", tenCode, t_start, t_end)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("节点 2 也阵亡了: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rawData map[string]interface{}
	json.Unmarshal(body, &rawData)

	dataObj, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return []tushare.DailyKLine{}, nil
	}
	stockData, ok := dataObj[tenCode].(map[string]interface{})
	if !ok {
		return []tushare.DailyKLine{}, nil
	}
	dayData, ok := stockData["day"].([]interface{})
	if !ok {
		return []tushare.DailyKLine{}, nil
	}

	var klines []tushare.DailyKLine
	for _, item := range dayData {
		kArr := item.([]interface{})
		if len(kArr) < 6 {
			continue
		}

		dateStr := strings.ReplaceAll(kArr[0].(string), "-", "")
		open, _ := strconv.ParseFloat(kArr[1].(string), 64)
		closePrice, _ := strconv.ParseFloat(kArr[2].(string), 64)
		high, _ := strconv.ParseFloat(kArr[3].(string), 64)
		low, _ := strconv.ParseFloat(kArr[4].(string), 64)
		vol, _ := strconv.ParseFloat(kArr[5].(string), 64)

		klines = append(klines, tushare.DailyKLine{
			TSCode: tsCode, TradeDate: dateStr,
			Open: open, Close: closePrice, High: high, Low: low,
			Vol: vol, Amount: 0, // 腾讯缺成交额

			// 💥 注入血缘标记 (底层兜底)
			DataSource: "TENCENT",
			TrustLevel: 30,
		})
	}
	return klines, nil
}

// ==========================================
// 💥 节点 1 扩编：高级战术情报 (指数与资金流向)
// ==========================================

// FetchIndexDaily 拉取大盘指数 (逻辑与 K 线完全一致，只是映射不同)
// 💥 FetchIndexDaily 责任链：东财(主力) -> 腾讯(灾备)
func FetchIndexDaily(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	indices, err := fetchIndexFromEastMoney(tsCode, startDate, endDate)
	if err == nil && len(indices) > 0 {
		return indices, nil
	}
	// 东财阵亡，腾讯接管大盘指数！
	return fetchIndexFromTencent(tsCode, startDate, endDate)
}

// 内部函数：东财指数拉取
func fetchIndexFromEastMoney(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=0&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	json.Unmarshal(body, &emResp)

	var indices []tushare.IndexDaily
	if emResp.Data.Klines == nil {
		return indices, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 11 {
			continue
		}
		closePrice, _ := strconv.ParseFloat(fields[2], 64)
		vol, _ := strconv.ParseFloat(fields[5], 64)
		pctChg, _ := strconv.ParseFloat(fields[8], 64)

		indices = append(indices, tushare.IndexDaily{
			TSCode: tsCode, TradeDate: strings.ReplaceAll(fields[0], "-", ""),
			Close: closePrice, Vol: vol, PctChg: pctChg,
			// 💥 补全血缘：
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}
	return indices, nil
}

// 内部函数：腾讯指数拉取 (降级灾备)
func fetchIndexFromTencent(tsCode, startDate, endDate string) ([]tushare.IndexDaily, error) {
	tenCode := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		tenCode = strings.ToLower(parts[1]) + parts[0]
	}

	t_start := startDate[:4] + "-" + startDate[4:6] + "-" + startDate[6:8]
	t_end := endDate[:4] + "-" + endDate[4:6] + "-" + endDate[6:8]

	url := fmt.Sprintf("http://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,day,%s,%s,500,", tenCode, t_start, t_end)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rawData map[string]interface{}
	json.Unmarshal(body, &rawData)

	dataObj, ok := rawData["data"].(map[string]interface{})
	if !ok {
		return []tushare.IndexDaily{}, nil
	}
	stockData, ok := dataObj[tenCode].(map[string]interface{})
	if !ok {
		return []tushare.IndexDaily{}, nil
	}
	dayData, ok := stockData["day"].([]interface{})
	if !ok {
		return []tushare.IndexDaily{}, nil
	}

	var indices []tushare.IndexDaily
	for _, item := range dayData {
		kArr := item.([]interface{})
		if len(kArr) < 6 {
			continue
		}

		dateStr := strings.ReplaceAll(kArr[0].(string), "-", "")
		closePrice, _ := strconv.ParseFloat(kArr[2].(string), 64)
		vol, _ := strconv.ParseFloat(kArr[5].(string), 64)

		indices = append(indices, tushare.IndexDaily{
			TSCode: tsCode, TradeDate: dateStr,
			Close: closePrice, Vol: vol, PctChg: 0, // 腾讯基础包不含涨跌幅，设为0兜底
			// 💥 补全血缘：
			DataSource: "TENCENT",
			TrustLevel: 30,
		})
	}
	return indices, nil
}

// FetchMoneyFlow 拉取主力资金流向 (东财 L2 核心接口)
func FetchMoneyFlow(tsCode, startDate, endDate string) ([]tushare.DailyMoneyFlow, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 💥 东财专属 fflow 资金流历史接口
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get?secid=%s&klt=101&beg=%s&end=%s&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyFlowResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyFlowResp
	json.Unmarshal(body, &emResp)

	var flows []tushare.DailyMoneyFlow
	if emResp.Data.Klines == nil {
		return flows, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 10 {
			continue
		}

		dateStr := strings.ReplaceAll(fields[0], "-", "")

		// 💥 修复：人工时间过滤器，抛弃多余的历史数据
		if dateStr < startDate || dateStr > endDate {
			continue
		}

		mainNet, _ := strconv.ParseFloat(fields[2], 64)

		flows = append(flows, tushare.DailyMoneyFlow{
			TSCode: tsCode, TradeDate: dateStr,
			BuyLgVol: 0, SellLgVol: 0, BuyElgVol: 0, SellElgVol: 0,
			NetMfVol: mainNet / 10000.0,
			// 💥 补全血缘：
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}

	return flows, nil
}

// 💥 内部底层函数：带 fqt 参数的通用 K 线拉取器
func fetchEastMoneyRaw(tsCode, startDate, endDate, fqt string) ([]tushare.DailyKLine, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 注入 fqt 参数控制复权类型 (0:不复权, 1:前复权, 2:后复权)
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=%s&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, fqt, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	// req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	// 💥 注入全套高仿浏览器 Header，突破防爬墙
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "http://quote.eastmoney.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	json.Unmarshal(body, &emResp)

	var klines []tushare.DailyKLine
	if emResp.Data.Klines == nil {
		return klines, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		if len(fields) < 11 {
			continue
		}
		open, _ := strconv.ParseFloat(fields[1], 64)
		closePrice, _ := strconv.ParseFloat(fields[2], 64)
		high, _ := strconv.ParseFloat(fields[3], 64)
		low, _ := strconv.ParseFloat(fields[4], 64)
		vol, _ := strconv.ParseFloat(fields[5], 64)

		klines = append(klines, tushare.DailyKLine{
			TSCode: tsCode, TradeDate: strings.ReplaceAll(fields[0], "-", ""),
			Open: open, Close: closePrice, High: high, Low: low, Vol: vol,
		})
	}
	return klines, nil
}

func FetchDerivedAdjFactors(tsCode, startDate, endDate string) ([]tushare.AdjFactor, error) {
	// 💥 拆除 WaitGroup 并发，改为串行请求，向东财防火墙妥协
	unadjusted, errUnadj := fetchEastMoneyRaw(tsCode, startDate, endDate, "0")
	if errUnadj != nil {
		return nil, fmt.Errorf("不复权拉取失败: %v", errUnadj)
	}

	// 💥 关键修复：第二枪也必须等待系统分配令牌，且增加随机性！
	// 注意：这里需要你手动引入 "math/rand" 和 "time" 包
	time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)

	adjusted, errAdj := fetchEastMoneyRaw(tsCode, startDate, endDate, "2")
	if errAdj != nil {
		return nil, fmt.Errorf("后复权拉取失败: %v", errAdj)
	}

	if len(unadjusted) == 0 || len(adjusted) == 0 {
		return []tushare.AdjFactor{}, nil
	}

	// 2. 构建内存哈希表，准备 O(1) 复杂度拉链对齐
	unadjMap := make(map[string]float64, len(unadjusted))
	for _, k := range unadjusted {
		unadjMap[k.TradeDate] = k.Close
	}

	// 3. 执行逆向除法推导
	var factors []tushare.AdjFactor
	for _, adjK := range adjusted {
		unadjClose, exists := unadjMap[adjK.TradeDate]
		if !exists || unadjClose <= 0 {
			continue // 防御除零异常或空洞数据
		}

		// 核心数学模型：后复权收盘价 / 不复权收盘价 = 复权因子
		factor := adjK.Close / unadjClose

		factors = append(factors, tushare.AdjFactor{
			TSCode:     tsCode,
			TradeDate:  adjK.TradeDate,
			AdjFactor:  factor,
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}

	return factors, nil
}

// ==========================================
// 💎 每日基本面提取器 (开源平替版)
// ==========================================

// FetchDailyBasic 榨取东财 K 线接口中的基本面衍生数据 (换手率)
func FetchDailyBasic(tsCode, startDate, endDate string) ([]tushare.DailyFundamental, error) {
	secid := ""
	parts := strings.Split(tsCode, ".")
	if len(parts) == 2 {
		if parts[1] == "SH" {
			secid = "1." + parts[0]
		} else {
			secid = "0." + parts[0]
		}
	} else {
		return nil, fmt.Errorf("代码异常")
	}

	// 复用 K 线接口，因为 fields2=f51~f61 中蕴含了换手率等衍生数据
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=101&fqt=0&beg=%s&end=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61", secid, startDate, endDate)

	req, _ := http.NewRequest("GET", url, nil)
	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 8 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	type EastMoneyResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var emResp EastMoneyResp
	if err := json.Unmarshal(body, &emResp); err != nil {
		return nil, err
	}

	var fundamentals []tushare.DailyFundamental
	if emResp.Data.Klines == nil {
		return fundamentals, nil
	}

	for _, kStr := range emResp.Data.Klines {
		fields := strings.Split(kStr, ",")
		// 东财的 K 线字符串至少有 11 个字段，索引 10 即为换手率
		if len(fields) < 11 {
			continue
		}

		dateStr := strings.ReplaceAll(fields[0], "-", "")
		turnover, _ := strconv.ParseFloat(fields[10], 64)

		fundamentals = append(fundamentals, tushare.DailyFundamental{
			TSCode:       tsCode,
			TradeDate:    dateStr,
			TurnoverRate: turnover,
			// 开源平替拿不到历史 PE/PB/市值，设为 0 作为占位符，等待 Tushare 高权限覆盖
			PE:      0,
			PB:      0,
			TotalMV: 0,
			DVRatio: 0,

			// 💥 注入血缘防线：标明这是东财的低权数据
			DataSource: "EASTMONEY",
			TrustLevel: 50,
		})
	}

	return fundamentals, nil
}
