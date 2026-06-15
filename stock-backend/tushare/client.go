// Package tushare 封装了与 Tushare Pro 金融数据 API 的所有交互逻辑。
// Tushare 是国内主流的股票数据供应商，提供日线行情、基本面、复权因子等多种数据接口。
// 本包负责：构造请求 -> 发送 HTTP POST -> 解析 JSON 响应 -> 返回结构化数据。
package tushare

// ==================== 导入说明 ====================
// bytes:        用于将 JSON 数据包装成 io.Reader，作为 HTTP 请求体
// encoding/json: Go 标准库的 JSON 编解码包，用于序列化请求和反序列化响应
// fmt:          格式化输出，用于构造错误信息和打印日志
// io:           提供 ReadAll 等 IO 工具函数
// net/http:     Go 标准库 HTTP 客户端，用于发送 POST 请求
// sync:         提供互斥锁（Mutex）和读写锁（RWMutex），保证并发安全
// time:         时间处理，用于设置 HTTP 超时和日期解析
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ==================== Token 令牌管理 ====================
// Tushare API 需要 Token 进行身份认证。不同积分等级的 Token 有不同的请求频率限制。
// 这里使用 sync.RWMutex（读写锁）保证多协程并发读写 Token 的安全性：
//   - 读锁（RLock）：多个协程可以同时读取 Token，互不阻塞
//   - 写锁（Lock）：写入时独占，阻塞所有读写操作
var (
	currentToken = "a2fb17ef3159218fabee30c54f270a64b4e6a448252436e6b548da5c" // 默认令牌（低权限）
	tokenMutex   sync.RWMutex                                                 // 读写锁，保护 currentToken 的并发安全
)

// TUSHARE_URL 是 Tushare Pro API 的统一入口地址。
// 所有数据接口（日线、基本面、复权因子等）都通过这一个 URL 发送 POST 请求，
// 具体请求哪个接口由请求体中的 api_name 字段决定。
const TUSHARE_URL = "https://api.tushare.pro"

// splitDateRange 将一个大日期范围拆分成多个小的时间切片。
// 【业务背景】Tushare 单次请求最多返回 5000 条数据。一只股票 10 年约有 2500 个交易日，
// 如果一次请求 20 年数据就可能被截断。所以按 yearsPerChunk 年为单位拆分，分批请求。
//
// 参数:
//   - start: 起始日期，格式 "20060102"（Go 的日期格式化必须用这个特定日期作为模板）
//   - end:   结束日期，格式同上
//   - yearsPerChunk: 每个切片包含的年数（如 10 表示每 10 年一个切片）
//
// 返回值: [][2]string —— 一个二维字符串切片，每个元素是 [startDate, endDate] 的配对。
//
// Go 语法要点：
//   - time.Parse(layout, value): 将字符串解析为 time.Time 对象
//   - time.AddDate(years, months, days): 在当前时间上加减年/月/日
//   - [][2]string: 值类型的二维数组，每个元素固定包含 2 个字符串
func splitDateRange(start, end string, yearsPerChunk int) [][2]string {
	// "20060102" 是 Go 语言的日期格式模板（Go 诞生日期 2006-01-02 15:04:05）
	// 这不是随意写的，而是 Go 的设计约定：必须用这个特定日期来表示格式
	layout := "20060102"

	// 将起始日期字符串解析为 time.Time 对象
	startTime, err := time.Parse(layout, start)
	if err != nil {
		// 解析失败时降级：直接返回原始区间，不做拆分
		return [][2]string{{start, end}}
	}
	// 将结束日期字符串解析为 time.Time 对象
	endTime, err := time.Parse(layout, end)
	if err != nil {
		return [][2]string{{start, end}}
	}

	var chunks [][2]string // 存放所有切片的结果集
	curr := startTime      // 当前切片的起点，从用户指定的起始日期开始

	// 循环切片：只要当前起点没超过结束日期，就继续切
	for curr.Before(endTime) || curr.Equal(endTime) {
		// 计算当前切片的终点 = 起点 + yearsPerChunk 年
		next := curr.AddDate(yearsPerChunk, 0, 0)
		// 如果计算出的终点超过了用户指定的结束日期，就截断到结束日期
		if next.After(endTime) {
			next = endTime
		}
		// 将 [起点, 终点] 配对存入结果集
		chunks = append(chunks, [2]string{curr.Format(layout), next.Format(layout)})
		// 下一个切片的起点 = 当前终点 + 1 天（避免日期重叠）
		curr = next.AddDate(0, 0, 1)
	}
	return chunks
}

// SetToken 动态更新 Tushare API 的 Token（写操作，需要写锁）。
// 【使用场景】程序运行时可以热切换 Token，比如从低权限 Token 升级到高权限 Token，
// 而不需要重启程序。
//
// 参数:
//   - newToken: 新的 Tushare Token 字符串
//
// Go 语法要点：
//   - tokenMutex.Lock(): 获取写锁，此时所有其他协程的读/写操作都会被阻塞
//   - defer tokenMutex.Unlock(): defer 表示在函数 return 时自动执行 Unlock，
//     这是 Go 中保证锁一定会被释放的惯用写法，即使函数中途 panic 也会执行
func SetToken(newToken string) {
	tokenMutex.Lock()         // 获取写锁（独占访问）
	defer tokenMutex.Unlock() // 函数结束时自动释放写锁
	currentToken = newToken   // 更新全局 Token
	fmt.Println("🔋 [情报部] Tushare 高阶 Token 已动态装填完毕！")
}

// GetToken 安全读取当前 Token（读操作，使用读锁）。
// 【为什么用读锁而不是写锁？】因为读操作不修改数据，多个协程可以同时读取，
// 用读锁（RLock）比写锁（Lock）性能更高，不会互相阻塞。
//
// 返回值: 当前生效的 Tushare Token 字符串
func GetToken() string {
	tokenMutex.RLock()         // 获取读锁（共享访问，多个读协程不互斥）
	defer tokenMutex.RUnlock() // 函数结束时自动释放读锁
	return currentToken        // 返回当前 Token
}

// TushareRequest 定义了发送给 Tushare API 的请求体结构。
// 所有 Tushare 接口（daily、stock_basic、daily_basic 等）都使用相同的请求格式，
// 只是 api_name 和 params/fields 不同。
//
// Go 语法要点 —— 结构体标签（struct tag）：
//   - `json:"api_name"` 告诉 json.Marshal 将 Go 字段 ApiName 序列化为 JSON 的 "api_name"
//   - 如果不加标签，Go 默认使用字段名本身作为 JSON key（如 "ApiName"）
//
// 字段说明：
//   - ApiName: 接口名称，如 "daily"（日线）、"stock_basic"（股票列表）等
//   - Token:   身份认证令牌
//   - Params:  查询参数，如 ts_code（股票代码）、start_date（起始日期）等
//   - Fields:  逗号分隔的字段列表，告诉 API 只返回我们需要的字段（减少传输量）
type TushareRequest struct {
	ApiName string            `json:"api_name"` // 接口名称
	Token   string            `json:"token"`    // 认证令牌
	Params  map[string]string `json:"params"`   // 查询参数（键值对）
	Fields  string            `json:"fields"`   // 需要返回的字段列表（逗号分隔）
}

// TushareResponse 定义了 Tushare API 返回的响应体结构。
// Tushare 的所有接口都返回统一的 JSON 格式：
//
//	{
//	  "code": 0,           // 0 表示成功，非 0 表示失败
//	  "msg": "",           // 错误信息（成功时为空）
//	  "data": {
//	    "items": [         // 二维数组，每行是一条记录，每列对应一个字段
//	      ["000001.SZ", "20230101", 10.5, ...],
//	      ...
//	    ]
//	  }
//	}
//
// Go 语法要点 —— 嵌套匿名结构体：
//   - Data 字段的类型是 struct{ Items [][]interface{} }，这是一个匿名结构体
//   - [][]interface{} 是二维的空接口切片，interface{} 可以存放任意类型
//     （因为 Tushare 返回的数值可能是 float64 也可能是 int，甚至是 nil）
type TushareResponse struct {
	Code int    `json:"code"` // 状态码：0=成功
	Msg  string `json:"msg"`  // 错误信息
	Data struct {
		Items [][]interface{} `json:"items"` // 数据行（二维数组）
	} `json:"data"`
}

// executeTushareRequest 是与 Tushare API 交互的核心函数，封装了完整的 HTTP 请求流程。
// 所有数据拉取函数（FetchStockHistory、FetchStockBasic 等）最终都调用这个函数。
//
// 请求流程：
//  1. 将请求体结构体序列化为 JSON 字节
//  2. 构造 HTTP POST 请求
//  3. 发送请求并等待响应
//  4. 读取响应体并反序列化为 Go 结构体
//  5. 检查业务状态码
//
// 参数:
//   - reqBody: TushareRequest 结构体，包含 api_name、token、params、fields
//
// 返回值:
//   - TushareResponse: 解析后的响应数据
//   - error: 如果任何步骤出错，返回包装了上下文信息的错误
//
// Go 语法要点：
//   - json.Marshal(): 将 Go 结构体序列化为 JSON 字节切片 []byte
//   - bytes.NewBuffer(): 将 []byte 包装成 io.Reader 接口（http.Post 需要这个类型）
//   - defer resp.Body.Close(): 确保 HTTP 响应体被关闭，防止连接泄漏
//   - fmt.Errorf("...: %w", err): %w 是 Go 1.13+ 的错误包装动词，
//     保留原始错误链，方便上层用 errors.Is/As 判断错误类型
//   - json.Unmarshal(): 将 JSON 字节反序列化到 Go 结构体中
func executeTushareRequest(reqBody TushareRequest) (TushareResponse, error) {
	var tsResp TushareResponse // 声明响应变量（零值初始化）

	// 第 1 步：将请求体序列化为 JSON
	// json.Marshal 会根据结构体标签（如 `json:"api_name"`）生成对应的 JSON key
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return tsResp, fmt.Errorf("请求序列化失败: %w", err)
	}

	// 第 2 步：构造 HTTP POST 请求
	// http.NewRequest(method, url, body) —— body 需要 io.Reader 类型
	// bytes.NewBuffer(jsonData) 将 []byte 转换为 io.Reader
	req, err := http.NewRequest(http.MethodPost, TUSHARE_URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return tsResp, fmt.Errorf("请求创建失败: %w", err)
	}
	// 设置请求头：告诉服务器我们发送的是 JSON 格式的数据
	req.Header.Set("Content-Type", "application/json")

	// 第 3 步：发送 HTTP 请求
	// 创建 HTTP 客户端，设置 30 秒超时（防止网络卡死导致协程永远阻塞）
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req) // 发送请求，阻塞等待响应
	if err != nil {
		return tsResp, fmt.Errorf("网络请求失败: %w", err)
	}
	// defer 表示在函数返回前关闭响应体，释放底层 TCP 连接
	// 如果不关闭，连接会一直占用，最终耗尽连接池
	defer resp.Body.Close()

	// 第 4 步：读取响应体全部内容
	// io.ReadAll 会一直读取直到 EOF，返回 []byte
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tsResp, fmt.Errorf("响应读取失败: %w", err)
	}

	// 第 5 步：将 JSON 响应反序列化到 TushareResponse 结构体
	// &tsResp 传入指针，让 Unmarshal 能修改这个变量的值
	if err := json.Unmarshal(body, &tsResp); err != nil {
		return tsResp, fmt.Errorf("响应解析失败: %w", err)
	}

	// 第 6 步：检查业务状态码
	// Tushare 约定 code=0 表示成功，非 0 表示请求有误（如 Token 无效、参数错误等）
	if tsResp.Code != 0 {
		return tsResp, fmt.Errorf("Tushare 报错: %s", tsResp.Msg)
	}

	return tsResp, nil // 返回解析好的响应和 nil 错误（表示成功）
}

// DailyKLine 定义了日线行情（K线）的数据结构。
// K线是股票交易中最基础的数据，记录了每个交易日的开盘价、最高价、最低价、收盘价等信息。
//
// 字段说明：
//   - TSCode:    股票代码（Tushare 格式），如 "000001.SZ"（平安银行）
//   - TradeDate: 交易日期，格式 "20060102"
//   - Open:      开盘价（当天第一笔成交价）
//   - High:      最高价（当天成交的最高价）
//   - Low:       最低价（当天成交的最低价）
//   - Close:     收盘价（当天最后一笔成交价）
//   - PreClose:  昨收价（上一个交易日的收盘价）
//   - Change:    涨跌额 = 收盘价 - 昨收价
//   - PctChg:    涨跌幅（%）= 涨跌额 / 昨收价 * 100
//   - Vol:       成交量（手，1手=100股）
//   - Amount:    成交额（千元）
//   - DataSource: 数据来源标记（如 "TUSHARE"、"EASTMONEY"、"SYSTEM_GHOST"）
//   - TrustLevel: 数据可信度权重（100=权威数据，50=开源数据，-1=幽灵占位数据）
type DailyKLine struct {
	TSCode    string  `json:"ts_code"`     // 股票代码
	TradeDate string  `json:"trade_date"`  // 交易日期
	Open      float64 `json:"open"`        // 开盘价
	High      float64 `json:"high"`        // 最高价
	Low       float64 `json:"low"`         // 最低价
	Close     float64 `json:"close"`       // 收盘价
	PreClose  float64 `json:"pre_close"`   // 昨收价
	Change    float64 `json:"change"`      // 涨跌额
	PctChg    float64 `json:"pct_chg"`     // 涨跌幅(%)
	Vol       float64 `json:"vol"`         // 成交量(手)
	Amount    float64 `json:"amount"`      // 成交额(千元)
	// 数据溯源字段：用于追踪数据来源和可信度
	DataSource string `json:"data_source"` // 数据源标记 (如 TUSHARE, EASTMONEY)
	TrustLevel int    `json:"trust_level"` // 可信度权重 (如 100, 40, -1)
}

// parseFloat 是一个安全的类型转换函数，将 interface{} 转换为 float64。
// 【为什么需要这个函数？】
// Tushare API 返回的 JSON 数据经过 json.Unmarshal 解析后，数值类型可能是：
//   - float64（JSON 数字默认解析为 float64）
//   - int（某些情况下 Go 会优化为 int）
//   - nil（字段值为空时）
//
// 直接用类型断言 item[2].(float64) 如果类型不匹配会 panic（程序崩溃），
// 所以需要这个函数做安全的类型分支处理。
//
// Go 语法要点 —— type switch（类型开关）：
//   - switch v := val.(type) { case float64: ... case int: ... }
//   - 这是 Go 特有的语法，用于判断 interface{} 中存储的实际类型
//   - 每个 case 分支中 v 会被自动转换为对应类型
//
// 参数:
//   - val: 任意类型的值（来自 Tushare 响应的二维数组）
//
// 返回值: 转换后的 float64 值，如果是 nil 或未知类型则返回 0
func parseFloat(val interface{}) float64 {
	if val == nil {
		return 0 // nil 值当作 0 处理，避免空指针
	}
	// type switch：根据 val 的实际运行时类型执行不同的分支
	switch v := val.(type) {
	case float64: // JSON 数字的默认类型
		return v
	case int: // Go 内部可能优化为整型
		return float64(v) // 整型转浮点型
	case float32:
		return float64(v)
	}
	return 0 // 未知类型，安全降级返回 0
}

// FetchStockHistory 拉取指定股票在指定日期范围内的全部日线行情数据。
// 这是获取历史 K 线数据的核心函数。
//
// 【防截断机制】Tushare 单次请求最多返回 5000 条数据。一只股票 10 年约有 2500 个交易日，
// 所以按 10 年一个切片拆分请求，确保每个切片不超过 5000 条限制。
//
// 参数:
//   - tsCode:    股票代码（Tushare 格式），如 "000001.SZ"
//   - startDate: 起始日期，格式 "20060102"，如 "20150101"
//   - endDate:   结束日期，格式 "20060102"，如 "20251231"
//
// 返回值:
//   - []DailyKLine: 日线数据切片（按时间顺序）
//   - error: 请求失败时返回错误信息
func FetchStockHistory(tsCode string, startDate string, endDate string) ([]DailyKLine, error) {
	// 将大日期范围拆分为 10 年一个的小切片，防止触发 Tushare 的 5000 条截断限制
	chunks := splitDateRange(startDate, endDate, 10)
	var allKLines []DailyKLine // 存放所有切片的结果

	// 遍历每个时间切片，逐批请求数据
	for _, chunk := range chunks {
		// 构造 Tushare 请求体
		reqBody := TushareRequest{
			ApiName: "daily",      // 接口名称：日线行情
			Token:   GetToken(),   // 安全读取当前 Token
			Params: map[string]string{
				"ts_code":    tsCode,    // 股票代码
				"start_date": chunk[0],  // 当前切片的起始日期
				"end_date":   chunk[1],  // 当前切片的结束日期
			},
			// 指定需要返回的字段（逗号分隔），减少不必要的数据传输
			Fields: "ts_code,trade_date,open,high,low,close,pre_close,change,pct_chg,vol,amount",
		}

		// 发送 HTTP 请求并解析响应
		tsResp, err := executeTushareRequest(reqBody)
		if err != nil {
			return nil, err // 任何切片失败，整体返回错误
		}

		// 将 Tushare 返回的二维数组转换为结构化的 DailyKLine 切片
		// tsResp.Data.Items 是 [][]interface{}，每行是一条记录，每列对应一个字段
		// 字段顺序与 Fields 参数一致：ts_code[0], trade_date[1], open[2], ...
		for _, item := range tsResp.Data.Items {
			code, _ := item[0].(string) // 类型断言：interface{} -> string
			date, _ := item[1].(string)
			allKLines = append(allKLines, DailyKLine{
				TSCode:     code,
				TradeDate:  date,
				Open:       parseFloat(item[2]),  // 开盘价
				High:       parseFloat(item[3]),  // 最高价
				Low:        parseFloat(item[4]),  // 最低价
				Close:      parseFloat(item[5]),  // 收盘价
				PreClose:   parseFloat(item[6]),  // 昨收价
				Change:     parseFloat(item[7]),  // 涨跌额
				PctChg:     parseFloat(item[8]),  // 涨跌幅
				Vol:        parseFloat(item[9]),  // 成交量
				Amount:     parseFloat(item[10]), // 成交额
				DataSource: "TUSHARE",           // 标记数据来源
				TrustLevel: 100,                 // Tushare 数据权重最高
			})
		}
	}

	return allKLines, nil // 返回合并后的全部日线数据
}

// StockBasicInfo 定义了股票基本信息（公司档案）的数据结构。
// 这是每只股票的"身份证"，包含代码、名称、所属行业、市场类型和上市日期。
//
// 字段说明：
//   - TSCode:   股票代码（Tushare 格式），如 "000001.SZ"
//   - Name:     股票名称，如 "平安银行"
//   - Industry: 所属行业，如 "银行"、"房地产"
//   - Market:   市场类型，如 "主板"、"创业板"、"科创板"
//   - ListDate: 上市日期，格式 "20060102"
type StockBasicInfo struct {
	TSCode   string // 股票代码
	Name     string // 股票名称
	Industry string // 所属行业
	Market   string // 市场类型
	ListDate string // 上市日期
}

// FetchStockBasic 拉取全市场股票的基本信息列表（花名册）。
// 这个函数会获取所有正常上市（list_status="L"）的 A 股股票信息。
//
// 【使用场景】系统初始化时需要知道有哪些股票可以采集数据，
// 这个函数返回的列表就是后续所有数据采集的"任务清单"。
//
// 返回值:
//   - []StockBasicInfo: 股票基本信息切片（包含全市场所有上市股票）
//   - error: 请求失败时返回错误
func FetchStockBasic() ([]StockBasicInfo, error) {
	fmt.Println("🕵️ [情报部] 正在向 Tushare 索要 A 股全市场花名册...")

	// 构造请求体
	reqBody := TushareRequest{
		ApiName: "stock_basic", // 接口名称：股票基本信息
		Token:   GetToken(),
		Params: map[string]string{
			// list_status 参数控制获取哪些状态的股票：
			// "L" = Listed（上市中）, "D" = Delisted（已退市）, "P" = Paused（暂停上市）
			"list_status": "L",
		},
		// 指定需要的字段：代码、名称、行业、市场类型、上市日期
		Fields: "ts_code,name,industry,market,list_date",
	}

	// 发送请求
	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	// 将二维数组转换为结构体切片
	var basics []StockBasicInfo
	for _, item := range tsResp.Data.Items {
		// 使用逗号-ok 模式进行安全的类型断言
		// 如果 item[x] 是 nil，断言会返回零值（空字符串）和 false，不会 panic
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
// 基本面情报中心 (Daily Basic)
// ==========================================
// 基本面数据反映的是公司的内在价值指标，与价格（K线）不同，
// 基本面数据帮助投资者判断一只股票是"贵了"还是"便宜了"。

// DailyFundamental 定义了每日基本面核心指标的数据结构。
// 这些指标是价值投资和量化分析的重要参考。
//
// 字段说明：
//   - TSCode:       股票代码
//   - TradeDate:    交易日期
//   - PE:           市盈率（Price-to-Earnings Ratio）= 股价 / 每股收益
//                   PE 越低说明股票越"便宜"，但也要结合行业对比
//   - PB:           市净率（Price-to-Book Ratio）= 股价 / 每股净资产
//                   PB < 1 说明股价低于公司净资产，可能是价值洼地
//   - TotalMV:      总市值（单位：万元），反映公司规模大小
//   - TurnoverRate: 换手率（%）= 当日成交量 / 流通股本 * 100
//                   换手率高说明交易活跃，低说明流动性差
//   - DVRatio:      股息率（%）= 每股分红 / 股价 * 100
//                   股息率高说明公司分红慷慨，适合长期持有
type DailyFundamental struct {
	TSCode       string  `json:"ts_code"`        // 股票代码
	TradeDate    string  `json:"trade_date"`     // 交易日期
	PE           float64 `json:"pe"`             // 市盈率
	PB           float64 `json:"pb"`             // 市净率
	TotalMV      float64 `json:"total_mv"`       // 总市值(万元)
	TurnoverRate float64 `json:"turnover_rate"`  // 换手率(%)
	DVRatio      float64 `json:"dv_ratio"`       // 股息率(%)
	// 数据溯源字段
	DataSource string `json:"data_source"` // 数据来源
	TrustLevel int    `json:"trust_level"` // 可信度权重
}

// FetchDailyBasic 拉取指定股票在指定日期范围内的每日基本面指标。
//
// 【与 FetchStockHistory 的区别】
//   - FetchStockHistory 获取的是价格/成交量数据（K线）
//   - FetchDailyBasic 获取的是估值指标（PE、PB、市值等）
//
// 参数:
//   - tsCode:    股票代码，如 "000001.SZ"
//   - startDate: 起始日期，格式 "20060102"
//   - endDate:   结束日期，格式 "20060102"
//
// 返回值:
//   - []DailyFundamental: 每日基本面数据切片
//   - error: 请求失败时返回错误
func FetchDailyBasic(tsCode string, startDate string, endDate string) ([]DailyFundamental, error) {
	reqBody := TushareRequest{
		ApiName: "daily_basic", // 接口名称：每日基本面指标
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		// 指定需要的基本面字段
		Fields: "ts_code,trade_date,pe,pb,total_mv,turnover_rate,dv_ratio",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		// 使用 %w 包装错误，保留原始错误链
		return nil, fmt.Errorf("拉取日线基本面失败: %w", err)
	}

	// 将二维数组转换为结构体切片
	var fundamentals []DailyFundamental
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)  // ts_code
		date, _ := item[1].(string)  // trade_date

		fundamentals = append(fundamentals, DailyFundamental{
			TSCode:       code,
			TradeDate:    date,
			PE:           parseFloat(item[2]), // 市盈率
			PB:           parseFloat(item[3]), // 市净率
			TotalMV:      parseFloat(item[4]), // 总市值
			TurnoverRate: parseFloat(item[5]), // 换手率
			DVRatio:      parseFloat(item[6]), // 股息率
			DataSource:   "TUSHARE",           // 标记数据来源
			TrustLevel:   100,                 // Tushare 数据权重最高
		})
	}

	return fundamentals, nil
}

// ==========================================
// 交易日历与复权因子模块
// ==========================================

// TradeCalendar 定义了交易日历的数据结构。
// 交易日历记录了哪些日期是交易日（开盘），哪些是休市日（周末、节假日等）。
//
// 【使用场景】系统需要知道哪些日期有交易数据可采集，
// 避免在休市日去请求数据（会返回空结果，浪费 API 调用次数）。
//
// 字段说明：
//   - CalDate: 日历日期，格式 "20060102"
//   - IsOpen:  是否开盘，1=开盘（交易日），0=休市（非交易日）
//              注意：Tushare 返回的是 float64 类型而非 bool
type TradeCalendar struct {
	CalDate string  `json:"cal_date"` // 日期
	IsOpen  float64 `json:"is_open"`  // 1=开盘, 0=休市
}

// FetchTradeCalendar 拉取指定日期范围内的交易日历。
//
// 参数:
//   - startDate: 起始日期，格式 "20060102"
//   - endDate:   结束日期，格式 "20060102"
//
// 返回值:
//   - []TradeCalendar: 交易日历切片
//   - error: 请求失败时返回错误
func FetchTradeCalendar(startDate string, endDate string) ([]TradeCalendar, error) {
	reqBody := TushareRequest{
		ApiName: "trade_cal", // 接口名称：交易日历
		Token:   GetToken(),
		Params: map[string]string{
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "cal_date,is_open",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var calendars []TradeCalendar
	for _, item := range tsResp.Data.Items {
		date, _ := item[0].(string) // cal_date
		calendars = append(calendars, TradeCalendar{
			CalDate: date,
			IsOpen:  parseFloat(item[1]), // is_open: 1 或 0
		})
	}
	return calendars, nil
}

// AdjFactor 定义了复权因子的数据结构。
// 【什么是复权因子？】
// 股票会经历分红、送股、配股等事件，导致历史价格不连续。
// 例如：某股票今天收盘 10 元，明天 10 送 10（每股送一股），
// 那么明天的理论价格变成 5 元，但实际价值没变。
//
// 复权因子就是用来修正这种价格断裂的系数：
//   - 前复权价格 = 实际价格 * (当日复权因子 / 最新复权因子)
//   - 后复权价格 = 实际价格 * 当日复权因子
//
// 使用复权因子可以让历史价格具有可比性，是技术分析的基础。
//
// 字段说明：
//   - TSCode:    股票代码
//   - TradeDate: 交易日期
//   - AdjFactor: 复权因子（一个比例系数，通常在 1 附近浮动）
type AdjFactor struct {
	TSCode     string  `json:"ts_code"`     // 股票代码
	TradeDate  string  `json:"trade_date"`  // 交易日期
	AdjFactor  float64 `json:"adj_factor"`  // 复权因子
	DataSource string  `json:"data_source"` // 数据来源
	TrustLevel int     `json:"trust_level"` // 可信度权重
}

// FetchAdjFactors 拉取指定股票在指定日期范围内的复权因子。
//
// 参数:
//   - tsCode:    股票代码，如 "000001.SZ"
//   - startDate: 起始日期，格式 "20060102"
//   - endDate:   结束日期，格式 "20060102"
//
// 返回值:
//   - []AdjFactor: 复权因子切片
//   - error: 请求失败时返回错误
func FetchAdjFactors(tsCode string, startDate string, endDate string) ([]AdjFactor, error) {
	reqBody := TushareRequest{
		ApiName: "adj_factor", // 接口名称：复权因子
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "ts_code,trade_date,adj_factor",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var factors []AdjFactor
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string) // ts_code
		date, _ := item[1].(string) // trade_date
		factors = append(factors, AdjFactor{
			TSCode:     code,
			TradeDate:  date,
			AdjFactor:  parseFloat(item[2]), // 复权因子
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return factors, nil
}

// FinaIndicator 定义了季报财务指标的数据结构。
// 【什么是季报财务指标？】
// 上市公司每季度发布财务报告，这些指标反映公司的盈利能力和成长性。
// 与每日基本面（PE/PB）不同，财务指标是季度更新的，变化频率更低。
//
// 字段说明：
//   - TSCode:       股票代码
//   - AnnDate:      公告日期（财报发布的日期）
//   - EndDate:      报告期截止日期（如 20231231 表示 2023 年年报）
//   - UpdateFlag:   更新标记（标识是否为修正后的数据）
//   - ROE:          净资产收益率（Return on Equity）= 净利润 / 净资产 * 100
//                   ROE 越高说明公司赚钱能力越强，巴菲特最看重的指标之一
//   - NetProfitYOY: 净利润同比增长率（%）= (本期净利润 - 上期净利润) / 上期净利润 * 100
//                   正数表示利润增长，负数表示利润下降
//   - CFPS:         每股经营现金流（Cash Flow Per Share）
//                   反映公司实际收到的现金，比净利润更真实
type FinaIndicator struct {
	TSCode       string  `json:"ts_code"`        // 股票代码
	AnnDate      string  `json:"ann_date"`       // 公告日期
	EndDate      string  `json:"end_date"`       // 报告期截止日
	UpdateFlag   string  `json:"update_flag"`    // 更新标记
	ROE          float64 `json:"roe"`            // 净资产收益率(%)
	NetProfitYOY float64 `json:"netprofit_yoy"`  // 净利润同比(%)
	CFPS         float64 `json:"cfps"`           // 每股经营现金流
	DataSource   string  `json:"data_source"`    // 数据来源
	TrustLevel   int     `json:"trust_level"`    // 可信度权重
}

// FetchFinaIndicators 拉取指定股票在指定日期范围内的季报财务指标。
//
// 参数:
//   - tsCode:    股票代码，如 "000001.SZ"
//   - startDate: 起始日期，格式 "20060102"
//   - endDate:   结束日期，格式 "20060102"
//
// 返回值:
//   - []FinaIndicator: 财务指标切片
//   - error: 请求失败时返回错误
func FetchFinaIndicators(tsCode string, startDate string, endDate string) ([]FinaIndicator, error) {
	reqBody := TushareRequest{
		ApiName: "fina_indicator", // 接口名称：财务指标
		Token:   GetToken(),
		Params: map[string]string{
			"ts_code":    tsCode,
			"start_date": startDate,
			"end_date":   endDate,
		},
		Fields: "ts_code,ann_date,end_date,update_flag,roe,netprofit_yoy,cfps",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var indicators []FinaIndicator
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string)    // ts_code
		annDate, _ := item[1].(string) // ann_date
		endDate, _ := item[2].(string) // end_date
		flag, _ := item[3].(string)    // update_flag

		indicators = append(indicators, FinaIndicator{
			TSCode:       code,
			AnnDate:      annDate,
			EndDate:      endDate,
			UpdateFlag:   flag,
			ROE:          parseFloat(item[4]), // 净资产收益率
			NetProfitYOY: parseFloat(item[5]), // 净利润同比
			CFPS:         parseFloat(item[6]), // 每股经营现金流
			DataSource:   "TUSHARE",
			TrustLevel:   100,
		})
	}
	return indicators, nil
}

// ==========================================
// 高阶数据模块 (需 Tushare 2000+ 积分)
// ==========================================
// 以下接口需要较高的 Tushare 积分才能使用，提供更深层的市场洞察。

// DailyMoneyFlow 定义了每日资金流向的数据结构。
// 【什么是资金流向？】
// 资金流向反映的是"大资金"（机构、游资）的买卖动向。
// 将成交按单笔金额大小分为大单、超大单等类别，
// 统计各类别的买入量和卖出量，帮助判断主力资金是在进场还是离场。
//
// 字段说明：
//   - TSCode:     股票代码
//   - TradeDate:  交易日期
//   - BuyLgVol:   大单买入量（手）
//   - SellLgVol:  大单卖出量（手）
//   - BuyElgVol:  超大单买入量（手）
//   - SellElgVol: 超大单卖出量（手）
//   - NetMfVol:   主力净流入量（手）= (大单买入+超大单买入) - (大单卖出+超大单卖出)
//                 正数表示主力净流入，负数表示主力净流出
type DailyMoneyFlow struct {
	TSCode     string  `json:"ts_code"`       // 股票代码
	TradeDate  string  `json:"trade_date"`    // 交易日期
	BuyLgVol   float64 `json:"buy_lg_vol"`    // 大单买入量
	SellLgVol  float64 `json:"sell_lg_vol"`   // 大单卖出量
	BuyElgVol  float64 `json:"buy_elg_vol"`   // 超大单买入量
	SellElgVol float64 `json:"sell_elg_vol"`  // 超大单卖出量
	NetMfVol   float64 `json:"net_mf_vol"`    // 主力净流入量
	DataSource string  `json:"data_source"`   // 数据来源
	TrustLevel int     `json:"trust_level"`   // 可信度权重
}

// FetchMoneyFlow 拉取指定股票在指定日期范围内的每日资金流向数据。
//
// 参数:
//   - tsCode:    股票代码，如 "000001.SZ"
//   - startDate: 起始日期，格式 "20060102"
//   - endDate:   结束日期，格式 "20060102"
//
// 返回值:
//   - []DailyMoneyFlow: 资金流向数据切片
//   - error: 请求失败时返回错误
func FetchMoneyFlow(tsCode, startDate, endDate string) ([]DailyMoneyFlow, error) {
	reqBody := TushareRequest{
		ApiName: "moneyflow", // 接口名称：资金流向
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "start_date": startDate, "end_date": endDate},
		Fields:  "ts_code,trade_date,buy_lg_vol,sell_lg_vol,buy_elg_vol,sell_elg_vol,net_mf_vol",
	}
	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var flows []DailyMoneyFlow
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string) // ts_code
		date, _ := item[1].(string) // trade_date
		flows = append(flows, DailyMoneyFlow{
			TSCode: code, TradeDate: date,
			BuyLgVol: parseFloat(item[2]),   // 大单买入
			SellLgVol: parseFloat(item[3]),  // 大单卖出
			BuyElgVol: parseFloat(item[4]),  // 超大单买入
			SellElgVol: parseFloat(item[5]), // 超大单卖出
			NetMfVol: parseFloat(item[6]),   // 主力净流入
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return flows, nil
}

// StkLimit 定义了涨跌停价格的数据结构。
// 【什么是涨跌停？】
// A 股市场有涨跌幅限制：主板股票涨跌幅限制为 10%，ST 股为 5%，创业板/科创板为 20%。
// 涨停价 = 昨收价 * (1 + 涨跌幅限制)，跌停价 = 昨收价 * (1 - 涨跌幅限制)。
// 当股价触及涨停/跌停价时，交易会受到限制。
//
// 【使用场景】知道涨跌停价格可以帮助：
//   - 判断股票是否接近涨停/跌停
//   - 回测时模拟真实的交易限制
//   - 监控涨停板打板策略
//
// 字段说明：
//   - TradeDate: 交易日期
//   - TSCode:    股票代码
//   - UpLimit:   涨停价（当日最高可交易价格）
//   - DownLimit: 跌停价（当日最低可交易价格）
type StkLimit struct {
	TradeDate  string  `json:"trade_date"` // 交易日期
	TSCode     string  `json:"ts_code"`    // 股票代码
	UpLimit    float64 `json:"up_limit"`   // 涨停价
	DownLimit  float64 `json:"down_limit"` // 跌停价
	DataSource string  `json:"data_source"` // 数据来源
	TrustLevel int     `json:"trust_level"` // 可信度权重
}

// FetchStkLimit 拉取指定交易日全市场所有股票的涨跌停价格。
// 【注意】这个接口按日期查询（不是按股票），一次返回全市场所有股票的涨跌停数据。
//
// 参数:
//   - tradeDate: 交易日期，格式 "20060102"
//
// 返回值:
//   - []StkLimit: 涨跌停数据切片（包含全市场所有股票）
//   - error: 请求失败时返回错误
func FetchStkLimit(tradeDate string) ([]StkLimit, error) {
	reqBody := TushareRequest{
		ApiName: "stk_limit", // 接口名称：涨跌停价格（需 2000 积分）
		Token:   GetToken(),
		Params:  map[string]string{"trade_date": tradeDate},
		Fields:  "trade_date,ts_code,up_limit,down_limit",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var limits []StkLimit
	for _, item := range tsResp.Data.Items {
		// 防御性编程：检查数据长度，防止脏数据导致数组越界 panic
		if len(item) < 4 {
			continue // 跳过不完整的数据行
		}

		date, _ := item[0].(string) // trade_date
		code, _ := item[1].(string) // ts_code

		limits = append(limits, StkLimit{
			TradeDate: date, TSCode: code,
			UpLimit: parseFloat(item[2]),   // 涨停价
			DownLimit: parseFloat(item[3]), // 跌停价
			DataSource: "TUSHARE", TrustLevel: 100,
		})
	}
	return limits, nil
}

// IndexDaily 定义了大盘指数行情的数据结构。
// 【什么是指数？】
// 指数是衡量整个市场或某个板块表现的"晴雨表"，例如：
//   - 000001.SH = 上证指数（反映上海市场整体表现）
//   - 399001.SZ = 深证成指（反映深圳市场整体表现）
//   - 399006.SZ = 创业板指（反映创业板整体表现）
//
// 【使用场景】
//   - 判断大盘趋势（牛市/熊市/震荡）
//   - 计算个股相对于大盘的超额收益（Alpha）
//   - 作为量化策略的基准（Benchmark）
//
// 字段说明：
//   - TSCode:    指数代码，如 "000001.SH"
//   - TradeDate: 交易日期
//   - Close:     收盘点位
//   - Vol:       成交量（手）
//   - PctChg:    涨跌幅（%）
type IndexDaily struct {
	TSCode     string  `json:"ts_code"`     // 指数代码
	TradeDate  string  `json:"trade_date"`  // 交易日期
	Close      float64 `json:"close"`       // 收盘点位
	Vol        float64 `json:"vol"`         // 成交量
	PctChg     float64 `json:"pct_chg"`     // 涨跌幅(%)
	DataSource string  `json:"data_source"` // 数据来源
	TrustLevel int     `json:"trust_level"` // 可信度权重
}

// FetchIndexDaily 拉取指定指数在指定日期范围内的行情数据。
//
// 参数:
//   - tsCode:    指数代码，如 "000001.SH"（上证指数）
//   - startDate: 起始日期，格式 "20060102"
//   - endDate:   结束日期，格式 "20060102"
//
// 返回值:
//   - []IndexDaily: 指数行情数据切片
//   - error: 请求失败时返回错误
func FetchIndexDaily(tsCode, startDate, endDate string) ([]IndexDaily, error) {
	reqBody := TushareRequest{
		ApiName: "index_daily", // 接口名称：指数行情
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "start_date": startDate, "end_date": endDate},
		Fields:  "ts_code,trade_date,close,vol,pct_chg",
	}
	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var indices []IndexDaily
	for _, item := range tsResp.Data.Items {
		code, _ := item[0].(string) // ts_code
		date, _ := item[1].(string) // trade_date
		indices = append(indices, IndexDaily{
			TSCode: code, TradeDate: date,
			Close: parseFloat(item[2]),  // 收盘点位
			Vol: parseFloat(item[3]),    // 成交量
			PctChg: parseFloat(item[4]), // 涨跌幅
			DataSource: "TUSHARE",
			TrustLevel: 100,
		})
	}
	return indices, nil
}

// ==========================================
// 筹码分布 / 技术因子 (5000积分高阶接口)
// ==========================================
// 以下接口需要 5000 积分，提供更深层次的技术分析数据。

// CyqPerf 定义了筹码分布及胜率指标的数据结构。
// 【什么是筹码分布？】
// 筹码分布（CYQ = Chip Your Quantity）是一种技术分析方法，
// 它通过统计历史上各个价位的成交量，推算出当前持有者的成本分布。
//
// 核心思想：如果某价位成交量大，说明那里"套牢"或"获利"的人多。
//
// 字段说明：
//   - TSCode:     股票代码
//   - TradeDate:  交易日期
//   - ProfitPct:  获利比例（%）= 当前价位以下的筹码占比
//                 获利比例高说明大部分人赚钱了，抛压可能较大
//   - WinnerRate: 胜率（%）= 获利筹码占全部筹码的比例
//   - Cost5Pct:   5%成本分位 —— 5%的筹码在这个价格以下买入
//   - Cost15Pct:  15%成本分位
//   - Cost50Pct:  50%成本分位（中位数成本）—— 一半人在这个价格以下买入
//                 这是最重要的参考价，代表"市场平均成本"
//   - Cost85Pct:  85%成本分位
//   - WeightAvg:  加权平均成本 —— 所有筹码的加权平均买入价
//   - HisLow:     历史最低价
//   - HisHigh:    历史最高价
type CyqPerf struct {
	TSCode     string  `json:"ts_code"`      // 股票代码
	TradeDate  string  `json:"trade_date"`   // 交易日期
	ProfitPct  float64 `json:"profit_pct"`   // 获利比例(%)
	WinnerRate float64 `json:"winner_rate"`  // 胜率
	Cost5Pct   float64 `json:"cost_5pct"`    // 5%成本分位
	Cost15Pct  float64 `json:"cost_15pct"`   // 15%成本分位
	Cost50Pct  float64 `json:"cost_50pct"`   // 50%成本分位(中位数成本)
	Cost85Pct  float64 `json:"cost_85pct"`   // 85%成本分位
	WeightAvg  float64 `json:"weight_avg"`   // 加权平均成本
	HisLow     float64 `json:"his_low"`      // 历史最低价
	HisHigh    float64 `json:"his_high"`     // 历史最高价
	DataSource string  `json:"data_source"`  // 数据来源
	TrustLevel int     `json:"trust_level"`  // 可信度权重
}

// FetchCyqPerf 拉取单只股票在指定交易日的筹码分布数据。
// 【注意】这是 5000 积分接口，且按"股票+日期"查询，一次只返回一天的数据。
//
// 参数:
//   - tsCode:    股票代码，如 "000001.SZ"
//   - tradeDate: 交易日期，格式 "20060102"
//
// 返回值:
//   - []CyqPerf: 筹码分布数据切片（通常只有一条记录）
//   - error: 请求失败时返回错误
func FetchCyqPerf(tsCode, tradeDate string) ([]CyqPerf, error) {
	reqBody := TushareRequest{
		ApiName: "cyq_perf", // 接口名称：筹码分布（需 5000 积分）
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "trade_date": tradeDate},
		Fields:  "ts_code,trade_date,profit_pct,winner_rate,cost_5pct,cost_15pct,cost_50pct,cost_85pct,weight_avg,his_low,his_high",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var perfs []CyqPerf
	for _, item := range tsResp.Data.Items {
		// 防御性编程：确保数据有 11 个字段，防止数组越界
		if len(item) < 11 {
			continue
		}
		code, _ := item[0].(string) // ts_code
		date, _ := item[1].(string) // trade_date
		perfs = append(perfs, CyqPerf{
			TSCode: code, TradeDate: date,
			ProfitPct: parseFloat(item[2]),   // 获利比例
			WinnerRate: parseFloat(item[3]),  // 胜率
			Cost5Pct: parseFloat(item[4]),    // 5%成本分位
			Cost15Pct: parseFloat(item[5]),   // 15%成本分位
			Cost50Pct: parseFloat(item[6]),   // 50%成本分位
			Cost85Pct: parseFloat(item[7]),   // 85%成本分位
			WeightAvg: parseFloat(item[8]),   // 加权平均成本
			HisLow: parseFloat(item[9]),      // 历史最低价
			HisHigh: parseFloat(item[10]),    // 历史最高价
			DataSource: "TUSHARE", TrustLevel: 100,
		})
	}
	return perfs, nil
}

// StkFactorPro 定义了技术因子专业版的数据结构。
// 【什么是技术因子？】
// 技术因子是通过数学公式对价格和成交量进行计算得到的指标，
// 用于判断股票的超买/超卖状态、趋势方向和买卖时机。
//
// 字段说明：
//   - TSCode:     股票代码
//   - TradeDate:  交易日期
//   - MACD:       MACD 指标（Moving Average Convergence Divergence，异同移动平均线）
//                 是趋势跟踪类指标，由快线、慢线和柱状图组成
//   - MACDSignal: MACD 信号线（MACD 的移动平均线）
//   - MACDHist:   MACD 柱状图 = MACD - Signal
//                 柱状图为正表示多头趋势，为负表示空头趋势
//   - RSI6:       6日 RSI（Relative Strength Index，相对强弱指标）
//                 RSI > 70 表示超买（可能回调），RSI < 30 表示超卖（可能反弹）
//   - RSI12:      12日 RSI
//   - KDJ_K:      KDJ 指标的 K 值（随机指标，用于判断超买超卖）
//   - KDJ_D:      KDJ 指标的 D 值
//   - KDJ_J:      KDJ 指标的 J 值（J = 3K - 2D，最敏感）
//   - BollUpper:  布林带上轨（Bollinger Bands Upper）
//                 股价触及上轨可能回调，触及下轨可能反弹
//   - BollLower:  布林带下轨（Bollinger Bands Lower）
type StkFactorPro struct {
	TSCode     string  `json:"ts_code"`      // 股票代码
	TradeDate  string  `json:"trade_date"`   // 交易日期
	MACD       float64 `json:"macd"`         // MACD 值
	MACDSignal float64 `json:"macd_signal"`  // MACD 信号线
	MACDHist   float64 `json:"macd_hist"`    // MACD 柱状图
	RSI6       float64 `json:"rsi_6"`        // 6日RSI
	RSI12      float64 `json:"rsi_12"`       // 12日RSI
	KDJ_K      float64 `json:"kdj_k"`        // KDJ-K值
	KDJ_D      float64 `json:"kdj_d"`        // KDJ-D值
	KDJ_J      float64 `json:"kdj_j"`        // KDJ-J值
	BollUpper  float64 `json:"boll_upper"`   // 布林带上轨
	BollLower  float64 `json:"boll_lower"`   // 布林带下轨
	DataSource string  `json:"data_source"`  // 数据来源
	TrustLevel int     `json:"trust_level"`  // 可信度权重
}

// FetchStkFactorPro 拉取单只股票在指定交易日的技术因子专业版数据。
// 【注意】这是 5000 积分接口，且按"股票+日期"查询，一次只返回一天的数据。
//
// 参数:
//   - tsCode:    股票代码，如 "000001.SZ"
//   - tradeDate: 交易日期，格式 "20060102"
//
// 返回值:
//   - []StkFactorPro: 技术因子数据切片（通常只有一条记录）
//   - error: 请求失败时返回错误
func FetchStkFactorPro(tsCode, tradeDate string) ([]StkFactorPro, error) {
	reqBody := TushareRequest{
		ApiName: "stk_factor_pro", // 接口名称：技术因子专业版（需 5000 积分）
		Token:   GetToken(),
		Params:  map[string]string{"ts_code": tsCode, "trade_date": tradeDate},
		Fields:  "ts_code,trade_date,macd,macd_signal,macd_hist,rsi_6,rsi_12,kdj_k,kdj_d,kdj_j,boll_upper,boll_lower",
	}

	tsResp, err := executeTushareRequest(reqBody)
	if err != nil {
		return nil, err
	}

	var factors []StkFactorPro
	for _, item := range tsResp.Data.Items {
		// 防御性编程：确保数据有 12 个字段
		if len(item) < 12 {
			continue
		}
		code, _ := item[0].(string) // ts_code
		date, _ := item[1].(string) // trade_date
		factors = append(factors, StkFactorPro{
			TSCode: code, TradeDate: date,
			MACD: parseFloat(item[2]),        // MACD
			MACDSignal: parseFloat(item[3]),  // MACD 信号线
			MACDHist: parseFloat(item[4]),    // MACD 柱状图
			RSI6: parseFloat(item[5]),        // 6日RSI
			RSI12: parseFloat(item[6]),       // 12日RSI
			KDJ_K: parseFloat(item[7]),       // KDJ-K
			KDJ_D: parseFloat(item[8]),       // KDJ-D
			KDJ_J: parseFloat(item[9]),       // KDJ-J
			BollUpper: parseFloat(item[10]),  // 布林带上轨
			BollLower: parseFloat(item[11]),  // 布林带下轨
			DataSource: "TUSHARE", TrustLevel: 100,
		})
	}
	return factors, nil
}

// ExecuteTestRequest 导出内部请求执行器，供权限探测模块使用。
// 【为什么需要这个函数？】
// executeTushareRequest 是包内部函数（小写开头），外部包无法直接调用。
// 这个函数以大写开头（导出函数），将内部函数的能力暴露给外部，
// 用于测试 Token 权限是否正常（如探测某个接口是否可用）。
//
// 参数:
//   - req: TushareRequest 请求体
//
// 返回值:
//   - TushareResponse: 响应数据
//   - error: 请求失败时返回错误
func ExecuteTestRequest(req TushareRequest) (TushareResponse, error) {
	return executeTushareRequest(req)
}
