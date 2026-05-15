package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"stock-backend/autosync"
	"stock-backend/db"
	"stock-backend/feeder"
	"stock-backend/logger"
	"syscall"
	"time"
)

var autoSyncManager = autosync.NewManager()

// 全局可取消 context：用于通知所有引擎在退出时立即停手
var appCtx, appCancel = context.WithCancel(context.Background())

// startDynamicLighthouse 自动嗅探网卡并计算子网广播地址。
func startDynamicLighthouse() {
	go func() {
		for {
			interfaces, err := net.Interfaces()
			if err != nil {
				time.Sleep(3 * time.Second)
				continue
			}

			for _, iface := range interfaces {
				if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
					continue
				}

				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}

				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ip := ipnet.IP.To4()
							mask := ipnet.Mask

							bcast := make(net.IP, len(ip))
							for i := 0; i < len(ip); i++ {
								bcast[i] = ip[i] | ^mask[i]
							}

							targetAddr := fmt.Sprintf("%s:8888", bcast.String())
							udpAddr, err := net.ResolveUDPAddr("udp", targetAddr)
							if err == nil {
								conn, err := net.DialUDP("udp", nil, udpAddr)
								if err == nil {
									_, _ = conn.Write([]byte("QUANT_TERMINAL_ONLINE"))
									_ = conn.Close()
								}
							}
						}
					}
				}
			}
			time.Sleep(3 * time.Second)
		}
	}()
	fmt.Println("📡 [动态灯塔] 局域网自适应广播已开启，无视动态 IP 漂移...")
}

func main() {
	// 飞行记录仪：最先启动，确保后续所有子系统的日志都能被记录
	if err := logger.Init("logs/blackbox.log"); err != nil {
		log.Fatalf("日志系统初始化失败: %v", err)
	}
	defer logger.Close()
	logger.Info("=== 系统启动 ===")

	// 将标准 log 包输出重定向到日志文件（捕获 log.Fatal / log.Printf）
	log.SetOutput(io.MultiWriter(os.Stderr, logger.Writer()))

	// 启动顺序：先初始化存储与配置，再启动后台任务与 HTTP 服务。
	db.InitDB()

	// 清理上次崩溃遗留的僵尸任务（服务启动时执行一次）
	_ = db.MarkStaleRunningLabJobsFailed("服务重启，任务中断")

	syncActiveProviderToken("tushare")
	autoSyncManager.Start()
	startDynamicLighthouse()
	feeder.InitGlobalEngine(800 * time.Millisecond)
	registerRoutes()
	registerStaticRoutes()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Printf("🟢 工业级全字段量化引擎启动完毕！监听端口: %s\n", port)

	server := &http.Server{Addr: ":" + port, Handler: nil}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		feeder.LogMsg("🛑 [系统] 收到退出信号: %s，正在执行安全退出...", sig.String())

		// 1. 立即通知所有引擎停手
		appCancel()

		// 2. 标记僵尸任务
		_ = db.MarkStaleRunningRunStepsFailed("人工中断，步骤未完成")
		_ = db.MarkStaleRunningRunsFailed("人工中断，任务未完成")
		_ = db.MarkStaleRunningLabJobsFailed("人工中断，任务未完成")

		// 3. 强杀闹钟：3 秒后物理退出，防止僵死
		time.AfterFunc(3*time.Second, func() {
			logger.Error("[系统] 优雅退出超时，强制退出")
			logger.Close()
			os.Exit(1)
		})

		// 4. 优雅关闭 HTTP 服务
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			feeder.LogMsg("⚠️ [系统] 优雅退出失败: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}
	logger.Info("[系统] 挥手告别，日志已安全封存")
	logger.Close()
	feeder.LogMsg("👋 [系统] 服务已退出")
}
