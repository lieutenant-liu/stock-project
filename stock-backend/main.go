package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"stock-backend/autosync"
	"stock-backend/db"
	"stock-backend/feeder"
	"syscall"
	"time"
)

var autoSyncManager = autosync.NewManager()

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
	// 启动顺序：先初始化存储与配置，再启动后台任务与 HTTP 服务。
	db.InitDB()
	syncActiveProviderToken("tushare")
	autoSyncManager.Start()
	startDynamicLighthouse()
	feeder.InitGlobalEngine(800 * time.Millisecond)
	registerRoutes()
	fmt.Println("🟢 工业级全字段量化引擎启动完毕！监听端口: 8081")

	server := &http.Server{Addr: ":8081", Handler: nil}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		// 收到退出信号时，先把运行中的任务标记为失败，再执行优雅停机。
		feeder.LogMsg("🛑 [系统] 收到退出信号: %s，正在执行安全退出...", sig.String())
		_ = db.MarkStaleRunningRunStepsFailed("人工中断，步骤未完成")
		_ = db.MarkStaleRunningRunsFailed("人工中断，任务未完成")

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			feeder.LogMsg("⚠️ [系统] 优雅退出失败: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}
	feeder.LogMsg("👋 [系统] 服务已退出")
}
