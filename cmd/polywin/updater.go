package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
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
	lastCommit     string
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
	log.Println("正在检查更新...")

	var hasUpdate bool
	var newVersion string

	if u.config.RepoURL != "" {
		// 从 Git 仓库检查更新
		hasUpdate, newVersion = u.checkGitUpdates()
	} else if u.config.UpdateURL != "" {
		// 从更新 URL 检查更新
		hasUpdate, newVersion = u.checkURLUpdates()
	} else {
		log.Println("未配置更新源，跳过检查")
		return
	}

	if hasUpdate {
		log.Printf("发现新版本: %s，当前版本: %s", newVersion, u.config.CurrentVersion)
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

// checkGitUpdates 从 Git 仓库检查更新
func (u *Updater) checkGitUpdates() (bool, string) {
	// 创建临时目录克隆仓库
	tempDir := filepath.Join(os.TempDir(), "polywin_update_check")
	defer os.RemoveAll(tempDir)

	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:      u.config.RepoURL,
		Progress: os.Stdout,
		Depth:    1,
	})
	if err != nil {
		log.Printf("克隆仓库失败: %v", err)
		return false, ""
	}

	ref, err := repo.Head()
	if err != nil {
		log.Printf("获取 HEAD 失败: %v", err)
		return false, ""
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		log.Printf("获取提交信息失败: %v", err)
		return false, ""
	}

	currentCommit := commit.Hash.String()
	
	// 如果是第一次检查，保存当前提交
	if u.lastCommit == "" {
		u.lastCommit = currentCommit
		log.Printf("初始化提交哈希: %s", currentCommit)
		return false, ""
	}

	// 检查是否有新提交
	if currentCommit != u.lastCommit {
		u.lastCommit = currentCommit
		return true, currentCommit[:8] // 返回短哈希作为版本号
	}

	return false, ""
}

// checkURLUpdates 从更新 URL 检查更新
func (u *Updater) checkURLUpdates() (bool, string) {
	resp, err := http.Get(u.config.UpdateURL)
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

	// 直接从 GitHub Releases 下载新版本（不再构建）
	if err := u.downloadServerFromGitHubReleases(execDir, execName); err != nil {
		return fmt.Errorf("下载新版本失败: %v", err)
	}

	// 执行更新（不重启，由守护程序监控重启）
	return u.updateTarget(targetPath)
}

// buildNewVersion 构建新版本
func (u *Updater) buildNewVersion(targetDir, execName string) error {
	log.Println("开始构建新版本...")

	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "polywin_build")
	defer os.RemoveAll(tempDir)

	// 克隆仓库（支持 HTTPS 和 SSH 格式）
	repoURL := u.config.RepoURL
	// 如果 URL 格式是 github.com:user/repo.git，转换为 SSH 格式
	if strings.HasPrefix(repoURL, "github.com:") {
		repoURL = "git@" + repoURL
	}
	
	_, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("克隆仓库失败: %v", err)
	}

	// 构建 server 可执行文件（从 cmd/server 目录）
	outputPath := filepath.Join(targetDir, execName+".new")
	
	// 构建 server 程序
	var buildCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		buildCmd = exec.Command("go", "build", "-o", outputPath, "./cmd/server")
	} else {
		buildCmd = exec.Command("go", "build", "-o", outputPath, "./cmd/server")
	}

	buildCmd.Dir = tempDir
	buildCmd.Env = os.Environ()
	
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("构建失败: %v, 输出: %s", err, string(output))
	}

	log.Println("构建完成")
	return nil
}

// downloadNewVersion 下载新版本
func (u *Updater) downloadNewVersion(targetDir, execName string) error {
	log.Println("开始下载新版本...")

	// 获取更新信息
	resp, err := http.Get(u.config.UpdateURL)
	if err != nil {
		return fmt.Errorf("获取更新信息失败: %v", err)
	}
	defer resp.Body.Close()

	var updateInfo UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&updateInfo); err != nil {
		return fmt.Errorf("解析更新信息失败: %v", err)
	}

	// 下载新版本
	downloadResp, err := http.Get(updateInfo.DownloadURL)
	if err != nil {
		return fmt.Errorf("下载新版本失败: %v", err)
	}
	defer downloadResp.Body.Close()

	outputPath := filepath.Join(targetDir, execName+".new")
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %v", err)
	}
	defer outFile.Close()

	// 计算校验和
	hash := sha256.New()
	tee := io.TeeReader(downloadResp.Body, hash)

	if _, err := io.Copy(outFile, tee); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	// 验证校验和
	calculatedChecksum := hex.EncodeToString(hash.Sum(nil))
	if updateInfo.Checksum != "" && calculatedChecksum != updateInfo.Checksum {
		os.Remove(outputPath)
		return fmt.Errorf("校验和不匹配: 期望 %s, 实际 %s", updateInfo.Checksum, calculatedChecksum)
	}

	// 在 Windows 上设置可执行权限
	if runtime.GOOS == "windows" {
		// Windows 不需要特殊权限设置
	} else {
		if err := os.Chmod(outputPath, 0755); err != nil {
			return fmt.Errorf("设置可执行权限失败: %v", err)
		}
	}

	log.Println("下载完成")
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

	// 首先尝试从 Releases API 获取
	apiURL := "https://api.github.com/repos/0xachong/polywin/releases/latest"
	resp, err := http.Get(apiURL)
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
	}

	// 尝试每个下载源
	outputPath := filepath.Join(targetDir, execName+".new")
	var lastErr error
	for _, source := range downloadSources {
		if source.url == "" {
			continue
		}

		log.Printf("尝试从 %s 下载...", source.name)
		if err := u.downloadFileToPath(source.url, outputPath); err != nil {
			log.Printf("从 %s 下载失败: %v", source.name, err)
			lastErr = err
			continue
		}

		log.Printf("从 %s 下载成功！", source.name)
		return nil
	}

	return fmt.Errorf("所有下载源都失败，最后一个错误: %v", lastErr)
}

// downloadFileToPath 下载文件到指定路径
func (u *Updater) downloadFileToPath(url, outputPath string) error {
	resp, err := http.Get(url)
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
	u.setPendingUpdate(false) // 标记更新完成，等待重启
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

