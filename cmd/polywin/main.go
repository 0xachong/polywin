package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var (
	version = "1.0.0"
	// 硬编码的配置
	repoURL          = "https://github.com/0xachong/polywin.git"
	targetExecutable = "server.exe"
	checkInterval    = 30 * time.Second
	enableAutoUpdate = true
)

var serverCmd *exec.Cmd

func main() {
	log.Printf("PolyWin 守护程序启动，版本: %s", version)
	log.Printf("目标程序: %s", targetExecutable)
	log.Printf("Git 仓库: %s", repoURL)
	log.Printf("更新检查间隔: %v", checkInterval)

	// 获取当前目录
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("获取可执行文件路径失败: %v", err)
	}
	execDir := filepath.Dir(execPath)
	targetPath := filepath.Join(execDir, targetExecutable)

	// 检查目标程序是否存在，不存在则从 GitHub Releases 下载
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		log.Printf("目标程序 %s 不存在，尝试从 GitHub Releases 下载...", targetPath)
		if err := downloadServerFromGitHub(execDir); err != nil {
			log.Fatalf("无法下载目标程序: %v", err)
		}
		log.Printf("目标程序下载成功: %s", targetPath)
	}

	// 创建更新器
	updater := NewUpdater(&UpdaterConfig{
		RepoURL:          repoURL,
		CheckInterval:    checkInterval,
		EnableAutoUpdate: enableAutoUpdate,
		CurrentVersion:   version,
		TargetExecutable: targetExecutable,
		TargetPath:       targetPath,
	})

	// 启动更新检查协程
	if enableAutoUpdate {
		go updater.StartUpdateChecker()
		log.Println("自动更新检查已启动")
	}

	// 启动目标程序
	startServer(targetPath)

	// 监控目标程序，如果退出则重启
	go monitorServer(targetPath, updater)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("程序正在关闭...")
	stopServer()
	updater.Stop()
	os.Exit(0)
}

// downloadServerFromGitHub 从 GitHub Releases 下载 server.exe
func downloadServerFromGitHub(targetDir string) error {
	log.Println("正在从 GitHub 下载 server.exe...")

	// 尝试多个下载源（不再使用 GitHub API，避免 403 问题）
	downloadSources := []struct {
		name string
		url  string
	}{
		{
			name: "GitHub Releases (latest tag)",
			url:  "https://github.com/0xachong/polywin/releases/latest/download/server.exe",
		},
		{
			name: "GitHub raw (releases 目录)",
			url:  "https://raw.githubusercontent.com/0xachong/polywin/main/releases/server.exe",
		},
	}

	// 尝试每个下载源
	var lastErr error
	for _, source := range downloadSources {
		if source.url == "" {
			continue
		}

		log.Printf("尝试从 %s 下载...", source.name)
		if err := downloadFile(source.url, filepath.Join(targetDir, "server.exe")); err != nil {
			log.Printf("从 %s 下载失败: %v", source.name, err)
			lastErr = err
			continue
		}

		log.Printf("从 %s 下载成功！", source.name)
		return nil
	}

	return fmt.Errorf("所有下载源都失败，最后一个错误: %v", lastErr)
}

// downloadFile 下载文件
func downloadFile(url, outputPath string) error {
	// 创建带超时的 HTTP 客户端（60秒超时）
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("写入文件失败: %v", err)
	}

	if written == 0 {
		os.Remove(outputPath)
		return fmt.Errorf("下载的文件为空")
	}

	log.Printf("下载完成，文件大小: %d 字节", written)
	return nil
}

// startServer 启动服务器程序
func startServer(serverPath string) {
	log.Printf("启动服务器程序: %s", serverPath)

	serverCmd = exec.Command(serverPath)
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	serverCmd.Dir = filepath.Dir(serverPath)
	serverCmd.Env = os.Environ()

	if err := serverCmd.Start(); err != nil {
		log.Fatalf("启动服务器程序失败: %v", err)
	}

	log.Printf("服务器程序已启动，PID: %d", serverCmd.Process.Pid)
}

// stopServer 停止服务器程序
func stopServer() {
	if serverCmd != nil && serverCmd.Process != nil {
		log.Println("正在停止服务器程序...")
		if err := serverCmd.Process.Kill(); err != nil {
			log.Printf("停止服务器程序失败: %v", err)
		} else {
			log.Println("服务器程序已停止")
		}
	}
}

// monitorServer 监控服务器程序，如果退出则重启
func monitorServer(serverPath string, updater *Updater) {
	for {
		if serverCmd != nil {
			// 等待进程退出
			err := serverCmd.Wait()
			if err != nil {
				log.Printf("服务器程序异常退出: %v", err)
			} else {
				log.Println("服务器程序正常退出")
			}

			// 检查是否有待处理的更新
			if updater.HasPendingUpdate() {
				log.Println("检测到待更新版本，等待文件替换完成...")

				// 检查新版本文件是否存在
				newExecPath := serverPath + ".new"
				maxWait := 30 // 最多等待30秒
				waited := 0

				for waited < maxWait {
					// 检查新版本文件是否存在
					if _, err := os.Stat(newExecPath); err == nil {
						// 检查原文件是否已被替换（通过检查 .old 文件是否存在）
						oldExecPath := serverPath + ".old"
						if _, err := os.Stat(oldExecPath); err == nil {
							log.Println("检测到文件已替换，准备重启...")
							updater.setPendingUpdate(false)
							break
						}
					}

					time.Sleep(1 * time.Second)
					waited++
					if waited%5 == 0 {
						log.Printf("等待文件替换中... (%d/%d 秒)", waited, maxWait)
					}
				}

				if waited >= maxWait {
					log.Println("等待文件替换超时，尝试直接重启...")
					updater.setPendingUpdate(false)
				}
			}

			// 等待一段时间后重启
			log.Println("等待 3 秒后重启服务器程序...")
			time.Sleep(3 * time.Second)

			// 重启服务器
			startServer(serverPath)
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}
