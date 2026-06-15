// =============================================================================
// main.go - 程序入口文件
// =============================================================================
// 这是整个 Go 后端服务的入口文件。在 Go 语言中，程序从 main() 函数开始执行。
//
// 本文件负责：
// 1. 初始化各个子系统（日志、数据库、数据同步引擎等）
// 2. 启动 HTTP 服务器监听前端请求
// 3. 处理程序的优雅退出（收到终止信号时安全关闭）
// =============================================================================

// package main 是 Go 程序的入口包。
// Go 要求可执行程序必须有一个 package main，且包含 main() 函数。
// 每个 Go 源文件都必须声明它属于哪个 package。
package main

// import 块用于导入程序需要用到的外部包（标准库或自定义包）。
// Go 的导入路径是包的路径，不是文件路径。
// 同一个 import 块中的包会被自动分组：标准库在上，自定义包在下。
import (
	"context"  // 提供上下文管理，用于控制 goroutine 的生命周期和取消操作
	"fmt"      // 格式化输出包，类似 C 语言的 printf
	"io"       // 提供 I/O 原语接口，如 Reader、Writer
	"log"      // 标准日志包，提供基本的日志输出功能
	"net"      // 网络编程包，提供 TCP/UDP、IP 地址等网络功能
	"net/http" // HTTP 客户端和服务器实现
	"os"       // 操作系统功能包，如文件操作、环境变量、进程控制
	"os/signal" // 信号处理包，用于捕获系统信号（如 Ctrl+C）
	"stock-backend/autosync" // 自定义包：自动数据同步管理器
	"stock-backend/db"       // 自定义包：数据库操作层
	"stock-backend/feeder"   // 自定义包：数据供给引擎
	"stock-backend/logger"   // 自定义包：日志系统（飞行记录仪）
	"syscall"  // 系统调用包，定义了系统信号常量（如 SIGINT、SIGTERM）
	"time"     // 时间处理包，提供定时器、休眠、时间计算等功能
)

// autoSyncManager 是全局的自动同步管理器实例。
// 使用 autosync.NewManager() 创建，管理定时数据同步任务的生命周期。
// 这是一个包级变量（在 main 函数外部定义），整个包内都可以访问。
var autoSyncManager = autosync.NewManager()

// appCtx 和 appCancel 是全局的上下文控制变量，用于程序优雅退出。
//
// context.WithCancel 的工作原理：
// - context.Background() 创建一个根上下文（永远不会被取消）
// - WithCancel() 返回一个新的子上下文 appCtx 和一个取消函数 appCancel
// - 调用 appCancel() 会取消 appCtx，所有监听 appCtx.Done() 的 goroutine 都会收到通知
// - 这样就可以在程序退出时通知所有后台任务停止工作
//
// 返回值说明：
// - appCtx: context.Context 类型，可以传递给子函数，子函数通过 select 监听 ctx.Done()
// - appCancel: context.CancelFunc 类型，调用它来取消所有使用 appCtx 的操作
var appCtx, appCancel = context.WithCancel(context.Background())

// =============================================================================
// startDynamicLighthouse - 局域网设备发现广播功能
// =============================================================================
// 功能说明：
//   这个函数启动一个后台 goroutine，每 3 秒自动扫描本机所有网络接口，
//   计算每个接口所在子网的广播地址，然后向广播地址发送 UDP 消息。
//   这样局域网内的其他设备（如前端客户端）就能自动发现这个服务器。
//
// 业务背景：
//   服务器的 IP 地址可能会动态变化（比如 DHCP 分配新 IP），
//   通过广播方式，客户端不需要硬编码服务器 IP，可以自动找到服务器。
//
// 参数：无
// 返回值：无
// =============================================================================
func startDynamicLighthouse() {
	// go func() 启动一个新的 goroutine（轻量级线程）。
	// goroutine 是 Go 语言的并发原语，比操作系统线程更轻量。
	// 使用 go 关键字 + 函数调用即可启动，函数会在后台并发执行。
	go func() {
		// 无限循环，持续广播。这是后台任务的常见模式。
		for {
			// net.Interfaces() 获取本机所有网络接口（如 eth0、wlan0 等）。
			// 返回 []net.Interface 切片和错误。
			interfaces, err := net.Interfaces()
			if err != nil {
				// 获取失败时，等待 3 秒后重试，避免快速循环消耗 CPU
				time.Sleep(3 * time.Second)
				continue // continue 跳过本次循环，进入下一次迭代
			}

			// 遍历所有网络接口
			// for _, iface 中的 _ 表示忽略索引，只使用值
			for _, iface := range interfaces {
				// 检查接口状态：
				// - iface.Flags&net.FlagUp == 0 表示接口未启用（没插网线或未连接）
				// - iface.Flags&net.FlagLoopback != 0 表示是回环接口（127.0.0.1）
				// & 是按位与运算符，用于检查标志位
				if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
					continue // 跳过未启用或回环接口
				}

				// 获取该接口绑定的所有 IP 地址
				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}

				// 遍历接口上的每个地址
				for _, addr := range addrs {
					// addr.(*net.IPNet) 是类型断言（type assertion）。
					// Go 的接口值可以存储任意类型，这里检查 addr 是否是 *net.IPNet 类型。
					// ok 是布尔值，表示断言是否成功。
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						// To4() 将 IP 地址转为 4 字节 IPv4 格式。
						// 如果不是 IPv4（是 IPv6），返回 nil，这里只处理 IPv4。
						if ipnet.IP.To4() != nil {
							ip := ipnet.IP.To4()   // 获取 IPv4 地址
							mask := ipnet.Mask      // 获取子网掩码

							// 计算广播地址：IP OR (NOT 掩码)
							// 例如 IP=192.168.1.100, 掩码=255.255.255.0
							// NOT 掩码=0.0.0.255, 广播地址=192.168.1.255
							bcast := make(net.IP, len(ip)) // 创建一个与 IP 等长的字节切片
							for i := 0; i < len(ip); i++ {
								// ^mask[i] 是按位取反，然后与 IP 做按位或运算
								bcast[i] = ip[i] | ^mask[i]
							}

							// 构造广播地址字符串，如 "192.168.1.255:8888"
							targetAddr := fmt.Sprintf("%s:8888", bcast.String())
							// 解析 UDP 地址字符串为 *net.UDPAddr 结构体
							udpAddr, err := net.ResolveUDPAddr("udp", targetAddr)
							if err == nil {
								// DialUDP 建立 UDP 连接（UDP 是无连接协议，这里主要是获取发送能力）
								conn, err := net.DialUDP("udp", nil, udpAddr)
								if err == nil {
									// 发送广播消息："QUANT_TERMINAL_ONLINE"
									// []byte(...) 将字符串转为字节切片
									// _, _ 表示故意忽略返回值（写入字节数和错误）
									_, _ = conn.Write([]byte("QUANT_TERMINAL_ONLINE"))
									_ = conn.Close() // 关闭连接，释放资源
								}
							}
						}
					}
				}
			}
			// 每轮扫描后休眠 3 秒，控制广播频率
			time.Sleep(3 * time.Second)
		}
	}()
	fmt.Println("📡 [动态灯塔] 局域网自适应广播已开启，无视动态 IP 漂移...")
}

// =============================================================================
// main - 程序入口函数
// =============================================================================
// 功能说明：
//   这是整个程序的入口点，Go 运行时会自动调用此函数。
//   它负责按照正确的顺序初始化所有子系统，启动 HTTP 服务器，
//   并处理程序的优雅退出（graceful shutdown）。
//
// 执行顺序：
//   1. 初始化日志系统（飞行记录仪）
//   2. 初始化数据库
//   3. 清理上次崩溃遗留的任务
//   4. 启动数据同步、局域网广播等后台服务
//   5. 注册 HTTP 路由
//   6. 启动 HTTP 服务器
//   7. 监听系统退出信号，执行优雅退出
//
// 参数：无
// 返回值：无（main 函数没有返回值）
// =============================================================================
func main() {
	// =========================================================================
	// 第一步：初始化日志系统（飞行记录仪）
	// =========================================================================
	// logger.Init 初始化日志文件，所有后续日志都会写入 "logs/blackbox.log"。
	// 这是最先初始化的系统，确保后续所有子系统的日志都能被记录。
	if err := logger.Init("logs/blackbox.log"); err != nil {
		// log.Fatalf 输出日志后会调用 os.Exit(1) 终止程序。
		// %v 是格式化占位符，表示用默认格式打印 err 的值。
		log.Fatalf("日志系统初始化失败: %v", err)
	}
	// defer 表示"延迟执行"，会在 main 函数返回前自动调用。
	// 无论 main 函数如何退出（正常返回或 panic），Close 都会被执行。
	// LIFO（后进先出）顺序：多个 defer 按声明的逆序执行。
	defer logger.Close()
	logger.Info("=== 系统启动 ===")

	// io.MultiWriter 创建一个多重写入器，将输出同时写入多个目标。
	// 这里将标准 log 包的输出同时写入 stderr（终端）和日志文件。
	// 这样 log.Fatal、log.Printf 等标准日志也会被捕获到文件中。
	log.SetOutput(io.MultiWriter(os.Stderr, logger.Writer()))

	// =========================================================================
	// 第二步：初始化数据库
	// =========================================================================
	// db.InitDB() 初始化数据库连接，创建必要的表结构。
	// 数据库是所有业务功能的基础，必须在其他服务之前初始化。
	db.InitDB()

	// =========================================================================
	// 第三步：清理上次崩溃遗留的僵尸任务
	// =========================================================================
	// 如果上次服务异常退出（如被 kill 或断电），数据库中可能有"运行中"状态的任务。
	// 这些任务实际上已经中断了，需要将它们标记为"失败"状态。
	// _ 表示忽略返回值（这里只关心清理操作，不关心返回的错误）。
	_ = db.MarkStaleRunningLabJobsFailed("服务重启，任务中断")

	// =========================================================================
	// 第四步：启动各种后台服务
	// =========================================================================

	// 同步 Tushare 数据源的 API Token
	// "tushare" 是一个股票数据服务商，需要有效的 Token 才能调用其 API
	syncActiveProviderToken("tushare")

	// 启动自动同步管理器，它会根据配置定时执行数据同步任务
	autoSyncManager.Start()

	// 启动局域网广播，让前端客户端能自动发现这个服务器
	startDynamicLighthouse()

	// 初始化全局数据供给引擎，800ms 是心跳间隔
	// 引擎负责定期从数据源拉取最新的股票数据
	feeder.InitGlobalEngine(800 * time.Millisecond)

	// 注册所有 HTTP API 路由（定义了前端可以调用哪些接口）
	registerRoutes()

	// 注册静态文件路由（前端页面、JS、CSS 等）
	registerStaticRoutes()

	// =========================================================================
	// 第五步：配置并启动 HTTP 服务器
	// =========================================================================

	// 从环境变量读取端口号，如果未设置则使用默认端口 8081
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Printf("🟢 工业级全字段量化引擎启动完毕！监听端口: %s\n", port)

	// 创建 HTTP 服务器实例
	// Handler: nil 表示使用 http.DefaultServeMux（即 http.HandleFunc 注册的路由）
	server := &http.Server{Addr: ":" + port, Handler: nil}

	// =========================================================================
	// 第六步：设置信号监听，准备优雅退出
	// =========================================================================

	// 创建一个带缓冲的 channel，容量为 1，用于接收系统信号。
	// channel 是 Go 的并发通信机制，goroutine 之间通过 channel 传递数据。
	// 缓冲容量为 1 意味着可以缓存一个信号，不会阻塞发送方。
	sigCh := make(chan os.Signal, 1)

	// signal.Notify 将系统信号转发到指定的 channel。
	// SIGINT: 用户按 Ctrl+C 时发送
	// SIGTERM: 服务管理器（如 systemd）请求停止时发送
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// =========================================================================
	// 第七步：启动信号处理 goroutine（后台监听退出信号）
	// =========================================================================
	go func() {
		// sigCh 是一个 channel，<-sigCh 会阻塞直到收到信号。
		// 这是 Go channel 的接收操作，会一直等待直到有数据可读。
		sig := <-sigCh
		feeder.LogMsg("🛑 [系统] 收到退出信号: %s，正在执行安全退出...", sig.String())

		// 第 1 步：调用 appCancel() 取消全局上下文。
		// 所有监听 appCtx.Done() 的 goroutine 都会收到通知并停止工作。
		appCancel()

		// 第 2 步：将数据库中还在"运行中"状态的任务标记为"失败"。
		// 这样下次重启时不会误认为这些任务还在运行。
		_ = db.MarkStaleRunningRunStepsFailed("人工中断，步骤未完成")
		_ = db.MarkStaleRunningRunsFailed("人工中断，任务未完成")
		_ = db.MarkStaleRunningLabJobsFailed("人工中断，任务未完成")

		// 第 3 步：设置 3 秒超时强制退出。
		// time.AfterFunc 在指定时间后执行回调函数。
		// 如果 3 秒内无法优雅退出，就强制终止进程（os.Exit(1)）。
		// 这是为了防止程序"僵死"（卡住不动）。
		time.AfterFunc(3*time.Second, func() {
			logger.Error("[系统] 优雅退出超时，强制退出")
			logger.Close()
			os.Exit(1) // 强制退出，退出码 1 表示异常退出
		})

		// 第 4 步：优雅关闭 HTTP 服务器。
		// context.WithTimeout 创建一个 2 秒后自动取消的上下文。
		// server.Shutdown 会：
		//   - 停止接受新的连接
		//   - 等待正在处理的请求完成（但不超过 2 秒）
		//   - 关闭所有空闲连接
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel() // 确保 cancel 函数被调用，释放资源
		if err := server.Shutdown(ctx); err != nil {
			feeder.LogMsg("⚠️ [系统] 优雅退出失败: %v", err)
		}
	}()

	// =========================================================================
	// 第八步：启动 HTTP 服务器（阻塞式）
	// =========================================================================
	// ListenAndServe 启动 HTTP 服务器并开始监听请求。
	// 这是一个阻塞调用，会一直运行直到服务器关闭。
	// http.ErrServerClosed 是正常关闭时返回的错误，不需要当作错误处理。
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}

	// 服务器正常关闭后执行清理工作
	logger.Info("[系统] 挥手告别，日志已安全封存")
	logger.Close()
	feeder.LogMsg("👋 [系统] 服务已退出")
}
