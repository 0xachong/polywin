package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	serverVersion = "1.0.0"
	serverTag     = "dev"
	serverCommit  = "unknown"
	serverBuildTime = "unknown"
	serverPort    = "8099"
	serverHost    = "0.0.0.0" // 默认监听所有网络接口，支持公网访问
)

func main() {
	// 从环境变量获取配置
	if port := os.Getenv("PORT"); port != "" {
		serverPort = port
	}
	if host := os.Getenv("HOST"); host != "" {
		serverHost = host
	}

	log.Printf("Gin HTTP 服务启动，版本: %s", serverVersion)
	log.Printf("监听地址: %s:%s (支持公网访问)", serverHost, serverPort)

	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	// 创建 Gin 路由
	router := gin.Default()

	// 添加 CORS 中间件，支持跨域访问
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 添加中间件：记录请求日志
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// 健康检查接口
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"version": serverVersion,
			"time":    time.Now().Format("2006-01-02 15:04:05"),
		})
	})


	// 根路径
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "polywin-http-server",
			"version": serverVersion,
			"status":  "running",
		})
	})

	// 获取请求来源 IP 和设备信息，以及服务器版本信息
	router.GET("/info", func(c *gin.Context) {
		// 获取客户端 IP
		clientIP := c.ClientIP()
		
		// 获取真实 IP（考虑代理）
		realIP := c.GetHeader("X-Real-IP")
		if realIP == "" {
			realIP = c.GetHeader("X-Forwarded-For")
			if realIP != "" {
				// X-Forwarded-For 可能包含多个 IP，取第一个
				ips := strings.Split(realIP, ",")
				if len(ips) > 0 {
					realIP = strings.TrimSpace(ips[0])
				}
			}
		}
		if realIP == "" {
			realIP = clientIP
		}

		// 获取设备信息
		userAgent := c.GetHeader("User-Agent")
		
		// 解析 User-Agent 获取设备信息
		deviceInfo := parseUserAgent(userAgent)

		c.JSON(http.StatusOK, gin.H{
			"ip":           realIP,
			"client_ip":    clientIP,
			"user_agent":   userAgent,
			"device_info":  deviceInfo,
			"request_time": time.Now().Format("2006-01-02 15:04:05"),
			"version":      serverVersion,
			"tag":          serverTag,
			"commit":       serverCommit,
			"build_time":  serverBuildTime,
		})
	})

	// 启动 HTTP 服务器，明确绑定到 0.0.0.0 以支持公网访问
	addr := serverHost + ":" + serverPort
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// 在 goroutine 中启动服务器
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	log.Printf("HTTP 服务器已启动，监听地址: %s", addr)
	log.Printf("本地访问: http://localhost:%s", serverPort)
	log.Printf("公网访问: http://<your-ip>:%s", serverPort)
	log.Printf("版本信息: %s (tag: %s, commit: %s, build: %s)", serverVersion, serverTag, serverCommit, serverBuildTime)
	log.Println("API 接口:")
	log.Println("  GET /ping    - Ping/Pong 健康检查")
	log.Println("  GET /info    - 请求来源信息和服务器版本信息")
	log.Println("  GET /        - 服务状态")

	// 等待中断信号以优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
	os.Exit(0)
}

// parseUserAgent 解析 User-Agent 获取设备信息
func parseUserAgent(userAgent string) map[string]string {
	info := make(map[string]string)
	ua := strings.ToLower(userAgent)

	// 操作系统检测
	if strings.Contains(ua, "windows") {
		info["os"] = "Windows"
		if strings.Contains(ua, "windows nt 10.0") || strings.Contains(ua, "windows 10") {
			info["os_version"] = "Windows 10/11"
		} else if strings.Contains(ua, "windows nt 6.3") {
			info["os_version"] = "Windows 8.1"
		} else if strings.Contains(ua, "windows nt 6.2") {
			info["os_version"] = "Windows 8"
		} else if strings.Contains(ua, "windows nt 6.1") {
			info["os_version"] = "Windows 7"
		} else {
			info["os_version"] = "Windows"
		}
	} else if strings.Contains(ua, "mac os x") || strings.Contains(ua, "macintosh") {
		info["os"] = "macOS"
		if strings.Contains(ua, "mac os x 10_") {
			// 提取版本号
			parts := strings.Split(ua, "mac os x ")
			if len(parts) > 1 {
				version := strings.Split(parts[1], "_")[0]
				info["os_version"] = "macOS " + version
			}
		}
	} else if strings.Contains(ua, "linux") {
		info["os"] = "Linux"
	} else if strings.Contains(ua, "android") {
		info["os"] = "Android"
		// 提取 Android 版本
		if strings.Contains(ua, "android ") {
			parts := strings.Split(ua, "android ")
			if len(parts) > 1 {
				version := strings.Fields(parts[1])[0]
				info["os_version"] = "Android " + version
			}
		}
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		info["os"] = "iOS"
		if strings.Contains(ua, "os ") {
			parts := strings.Split(ua, "os ")
			if len(parts) > 1 {
				version := strings.Split(strings.Fields(parts[1])[0], "_")
				if len(version) >= 2 {
					info["os_version"] = "iOS " + version[0] + "." + version[1]
				}
			}
		}
	} else {
		info["os"] = "Unknown"
	}

	// 浏览器/客户端检测
	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		info["browser"] = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		info["browser"] = "Firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		info["browser"] = "Safari"
	} else if strings.Contains(ua, "edg") {
		info["browser"] = "Edge"
	} else if strings.Contains(ua, "opera") {
		info["browser"] = "Opera"
	} else if strings.Contains(ua, "curl") {
		info["browser"] = "curl"
	} else if strings.Contains(ua, "postman") {
		info["browser"] = "Postman"
	} else {
		info["browser"] = "Unknown"
	}

	// 设备类型
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		info["device_type"] = "Mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		info["device_type"] = "Tablet"
	} else {
		info["device_type"] = "Desktop"
	}

	return info
}
