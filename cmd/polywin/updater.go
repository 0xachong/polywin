package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// UpdaterConfig 更新器配置
type UpdaterConfig struct {
	RepoURL          string
	UpdateURL        string
	CheckInterval    time.Duration
	EnableAutoUpdate bool
	CurrentVersion   string
	TargetExecutable string // 目标可执行文件名
	TargetPath       string // 目标可执行文件完整路径
}

// UpdateInfo 更新信息
type UpdateInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	ReleaseDate string `json:"release_date"`
}

// Updater 更新器
type Updater struct {
	config         *UpdaterConfig
	ctx            context.Context
	cancel         context.CancelFunc
	lastReleaseTag string // 记录最后检查的 release tag
	pendingUpdate  bool
	updateMutex    sync.Mutex
}

// NewUpdater 创建新的更新器
func NewUpdater(config *UpdaterConfig) *Updater {
	ctx, cancel := context.WithCancel(context.Background())
	return &Updater{
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		pendingUpdate: false,
	}
}

// HasPendingUpdate 检查是否有待处理的更新
func (u *Updater) HasPendingUpdate() bool {
	u.updateMutex.Lock()
	defer u.updateMutex.Unlock()
	return u.pendingUpdate
}

// setPendingUpdate 设置待处理更新状态
func (u *Updater) setPendingUpdate(value bool) {
	u.updateMutex.Lock()
	defer u.updateMutex.Unlock()
	u.pendingUpdate = value
}

// StartUpdateChecker 启动更新检查器
func (u *Updater) StartUpdateChecker() {
	ticker := time.NewTicker(u.config.CheckInterval)
	defer ticker.Stop()

	// 立即检查一次
	u.checkForUpdates()

	for {
		select {
		case <-u.ctx.Done():
			return
		case <-ticker.C:
			u.checkForUpdates()
		}
	}
}

// Stop 停止更新检查器
func (u *Updater) Stop() {
	u.cancel()
}

// checkForUpdates 检查更新
func (u *Updater) checkForUpdates() {
	// 检查 context 是否已取消
	select {
	case <-u.ctx.Done():
		log.Println("更新检查已取消")
		return
	default:
	}

	log.Println("正在检查更新...")

	var hasUpdate bool
	var newVersion string

	if u.config.RepoURL != "" {
		// 优先检查 GitHub Releases 版本（更准确，不需要克隆仓库）
		hasUpdate, newVersion = u.checkGitHubReleases()
		// 不再检查 Git commit，因为：
		// 1. GitHub Releases 已经能准确反映版本
		// 2. Git 克隆会产生大量输出和网络流量
		// 3. 检查速度更快
	} else if u.config.UpdateURL != "" {
		// 从更新 URL 检查更新
		hasUpdate, newVersion = u.checkURLUpdates()
	} else {
		log.Println("未配置更新源，跳过检查")
		return
	}

	if hasUpdate {
		log.Printf("发现新版本: %s，当前版本: %s", newVersion, u.config.CurrentVersion)
		log.Printf("开始执行更新流程...")
		u.setPendingUpdate(true)
		if err := u.performUpdate(newVersion); err != nil {
			log.Printf("更新失败: %v", err)
			u.setPendingUpdate(false)
		} else {
			log.Println("更新完成，等待服务器程序重启以应用更新")
		}
	} else {
		log.Println("当前已是最新版本")
	}
}

// checkGitHubReleases 从 GitHub Releases 检查更新
func (u *Updater) checkGitHubReleases() (bool, string) {
	// 创建带超时的 HTTP 客户端（15秒超时）
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	// 从 GitHub Releases API 获取最新版本
	apiURL := "https://api.github.com/repos/0xachong/polywin/releases/latest"
	req, err := http.NewRequestWithContext(u.ctx, "GET", apiURL, nil)
	if err != nil {
		log.Printf("创建 GitHub Releases API 请求失败: %v", err)
		return false, ""
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("获取 GitHub Releases 信息失败: %v", err)
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("GitHub Releases API 返回错误状态码: %d", resp.StatusCode)
		return false, ""
	}

	var release struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Printf("解析 GitHub Releases 信息失败: %v", err)
		return false, ""
	}

	// 检查是否有 server.exe
	hasServerExe := false
	for _, asset := range release.Assets {
		if asset.Name == "server.exe" {
			hasServerExe = true
			break
		}
	}

	if !hasServerExe {
		log.Printf("最新 Release (%s) 中没有 server.exe，跳过", release.TagName)
		return false, ""
	}

	// 如果是第一次检查，保存当前 release tag
	if u.lastReleaseTag == "" {
		u.lastReleaseTag = release.TagName
		log.Printf("初始化 Release Tag: %s", release.TagName)
		return false, ""
	}

	// 检查是否有新版本
	if release.TagName != u.lastReleaseTag {
		log.Printf("检测到新的 Release: %s (当前: %s)", release.TagName, u.lastReleaseTag)
		u.lastReleaseTag = release.TagName
		return true, release.TagName
	}

	return false, ""
}

// checkURLUpdates 从更新 URL 检查更新
func (u *Updater) checkURLUpdates() (bool, string) {
	// 创建带超时的 HTTP 客户端（10秒超时）
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(u.ctx, "GET", u.config.UpdateURL, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return false, ""
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("获取更新信息失败: %v", err)
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("更新服务器返回错误状态码: %d", resp.StatusCode)
		return false, ""
	}

	var updateInfo UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&updateInfo); err != nil {
		log.Printf("解析更新信息失败: %v", err)
		return false, ""
	}

	// 比较版本
	if updateInfo.Version != u.config.CurrentVersion {
		return true, updateInfo.Version
	}

	return false, ""
}

// performUpdate 执行更新
func (u *Updater) performUpdate(newVersion string) error {
	log.Printf("开始执行更新到版本: %s", newVersion)

	// 使用配置的目标程序路径
	targetPath := u.config.TargetPath
	if targetPath == "" {
		return fmt.Errorf("目标程序路径未配置")
	}

	execDir := filepath.Dir(targetPath)
	execName := filepath.Base(targetPath)

	log.Printf("准备从 GitHub Releases 下载新版本到: %s", execDir)

	// 直接从 GitHub Releases 下载新版本（不再构建）
	if err := u.downloadServerFromGitHubReleases(execDir, execName); err != nil {
		log.Printf("下载失败，错误详情: %v", err)
		return fmt.Errorf("下载新版本失败: %v", err)
	}

	log.Printf("下载成功，准备替换文件...")

	// 执行更新（不重启，由守护程序监控重启）
	if err := u.updateTarget(targetPath); err != nil {
		log.Printf("文件替换失败，错误详情: %v", err)
		return fmt.Errorf("文件替换失败: %v", err)
	}

	log.Printf("更新流程完成")
	return nil
}

// downloadServerFromGitHubReleases 从 GitHub Releases 下载 server.exe
func (u *Updater) downloadServerFromGitHubReleases(targetDir, execName string) error {
	log.Println("开始从 GitHub Releases 下载新版本...")

	// 尝试多个下载源
	downloadSources := []struct {
		name string
		url  string
	}{
		{
			name: "GitHub Releases (latest)",
			url:  "", // 需要从 API 获取
		},
		{
			name: "GitHub Releases (latest tag)",
			url:  "https://github.com/0xachong/polywin/releases/latest/download/server.exe",
		},
		{
			name: "GitHub raw (releases 目录)",
			url:  "https://raw.githubusercontent.com/0xachong/polywin/main/releases/server.exe",
		},
	}

	// 创建带超时的 HTTP 客户端（15秒超时）
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	// 首先尝试从 Releases API 获取
	apiURL := "https://api.github.com/repos/0xachong/polywin/releases/latest"
	req, err := http.NewRequestWithContext(u.ctx, "GET", apiURL, nil)
	if err == nil {
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var release struct {
				TagName string `json:"tag_name"`
				Assets  []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				} `json:"assets"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&release); err == nil {
				// 查找 server.exe
				for _, asset := range release.Assets {
					if asset.Name == "server.exe" {
						downloadSources[0].url = asset.BrowserDownloadURL
						log.Printf("从 GitHub Releases 找到 server.exe，版本: %s", release.TagName)
						break
					}
				}
			}
			resp.Body.Close()
		} else if err != nil {
			log.Printf("获取 GitHub Releases API 失败: %v", err)
		}
	}

	// 尝试每个下载源
	outputPath := filepath.Join(targetDir, execName+".new")
	log.Printf("目标下载路径: %s", outputPath)

	var lastErr error
	for i, source := range downloadSources {
		if source.url == "" {
			log.Printf("跳过下载源 %d (%s): URL 为空", i+1, source.name)
			continue
		}

		log.Printf("尝试从 %s 下载 (URL: %s)...", source.name, source.url)
		if err := u.downloadFileToPath(source.url, outputPath); err != nil {
			log.Printf("从 %s 下载失败: %v", source.name, err)
			lastErr = err
			continue
		}

		// 验证文件是否存在且大小大于0
		if info, err := os.Stat(outputPath); err != nil {
			log.Printf("下载的文件不存在: %v", err)
			lastErr = fmt.Errorf("下载的文件不存在: %v", err)
			continue
		} else if info.Size() == 0 {
			log.Printf("下载的文件大小为0，删除并尝试下一个源")
			os.Remove(outputPath)
			lastErr = fmt.Errorf("下载的文件大小为0")
			continue
		}

		// 再次获取文件信息以确认
		fileInfo, _ := os.Stat(outputPath)
		log.Printf("从 %s 下载成功！文件大小: %d 字节", source.name, fileInfo.Size())
		return nil
	}

	return fmt.Errorf("所有下载源都失败，最后一个错误: %v", lastErr)
}

// downloadFileToPath 下载文件到指定路径
func (u *Updater) downloadFileToPath(url, outputPath string) error {
	// 创建带超时的 HTTP 客户端（60秒超时，下载文件需要更长时间）
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequestWithContext(u.ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	resp, err := client.Do(req)
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

// updateTarget 更新目标程序（不重启，由守护程序负责重启）
func (u *Updater) updateTarget(targetPath string) error {
	log.Println("准备更新目标程序...")

	execDir := filepath.Dir(targetPath)
	execName := filepath.Base(targetPath)
	newExecPath := filepath.Join(execDir, execName+".new")
	oldExecPath := filepath.Join(execDir, execName+".old")

	// 检查新版本文件是否存在
	if _, err := os.Stat(newExecPath); os.IsNotExist(err) {
		return fmt.Errorf("新版本文件不存在: %s", newExecPath)
	}

	// 在 Windows 上，我们需要使用批处理脚本来替换文件
	if runtime.GOOS == "windows" {
		return u.updateWindows(targetPath, newExecPath, oldExecPath)
	}

	// 在 Unix 系统上，可以直接替换
	return u.updateUnix(targetPath, newExecPath, oldExecPath)
}

// updateWindows Windows 系统更新（不重启程序）
func (u *Updater) updateWindows(targetPath, newExecPath, oldExecPath string) error {
	// 在 Windows 上，如果目标程序正在运行，无法直接替换
	// 我们创建一个批处理脚本，在目标程序退出后执行替换
	scriptPath := filepath.Join(filepath.Dir(targetPath), "update_server.bat")
	scriptContent := fmt.Sprintf(`@echo off
:loop
timeout /t 1 /nobreak >nul
tasklist /FI "IMAGENAME eq %s" 2>NUL | find /I /N "%s">NUL
if "%%ERRORLEVEL%%"=="0" goto loop
move /Y "%s" "%s" 2>NUL
move /Y "%s" "%s" 2>NUL
del "%%~f0"
`, filepath.Base(targetPath), filepath.Base(targetPath), targetPath, oldExecPath, newExecPath, targetPath)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return fmt.Errorf("创建更新脚本失败: %v", err)
	}

	// 启动更新脚本（后台运行）
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "start", "/min", "", scriptPath)
	} else {
		cmd = exec.Command("cmd", "/C", scriptPath)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动更新脚本失败: %v", err)
	}

	log.Println("更新脚本已启动，将在目标程序退出后自动替换文件")
	// 注意：不在这里设置 pendingUpdate = false
	// 让 monitorServer 在重启时检查文件是否已替换
	return nil
}

// updateUnix Unix 系统更新（不重启程序）
func (u *Updater) updateUnix(targetPath, newExecPath, oldExecPath string) error {
	// 重命名当前可执行文件
	if err := os.Rename(targetPath, oldExecPath); err != nil {
		return fmt.Errorf("重命名当前可执行文件失败: %v", err)
	}

	// 移动新版本到目标位置
	if err := os.Rename(newExecPath, targetPath); err != nil {
		// 如果失败，尝试恢复
		os.Rename(oldExecPath, targetPath)
		return fmt.Errorf("移动新版本失败: %v", err)
	}

	// 设置可执行权限
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("设置可执行权限失败: %v", err)
	}

	// 删除旧版本文件
	go func() {
		time.Sleep(5 * time.Second)
		os.Remove(oldExecPath)
	}()

	log.Println("目标程序已更新，等待守护程序重启")
	u.setPendingUpdate(false) // 标记更新完成，等待重启
	return nil
}
