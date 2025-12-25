//go:build server
// +build server

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	serverVersion = "1.0.0"
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

	// 版本信息接口
	router.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version": serverVersion,
			"port":    serverPort,
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
	log.Println("API 接口:")
	log.Println("  GET /ping    - Ping/Pong 健康检查")
	log.Println("  GET /version - 版本信息")
	log.Println("  GET /        - 服务状态")

	// 等待中断信号以优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
	os.Exit(0)
}
