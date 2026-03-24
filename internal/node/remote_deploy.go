// Package node provides node management functionality.
package node

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"v/internal/database/repository"
	"v/internal/logger"
)

// RemoteDeployService handles remote agent deployment.
type RemoteDeployService struct {
	logger   logger.Logger
	nodeRepo repository.NodeRepository
	mu       sync.RWMutex
	tasks    map[int64]*DeployTaskStatus
}

// DeployTaskStatus represents async deployment progress for a node.
type DeployTaskStatus struct {
	NodeID     int64        `json:"node_id"`
	Status     string       `json:"status"`
	Message    string       `json:"message"`
	Steps      []DeployStep `json:"steps"`
	Logs       string       `json:"logs"`
	StartedAt  *time.Time   `json:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// NewRemoteDeployService creates a new remote deploy service.
func NewRemoteDeployService(log logger.Logger, nodeRepo repository.NodeRepository) *RemoteDeployService {
	return &RemoteDeployService{
		logger:   log,
		nodeRepo: nodeRepo,
		tasks:    make(map[int64]*DeployTaskStatus),
	}
}

// MarkInstallQueued initializes async deployment status for a node.
func (s *RemoteDeployService) MarkInstallQueued(nodeID int64, message string) {
	if nodeID == 0 {
		return
	}
	now := time.Now()
	s.storeTaskStatus(&DeployTaskStatus{
		NodeID:    nodeID,
		Status:    "queued",
		Message:   message,
		Steps:     []DeployStep{},
		Logs:      "",
		StartedAt: &now,
		UpdatedAt: now,
	})
}

// GetInstallStatus returns the tracked async deployment status for a node.
func (s *RemoteDeployService) GetInstallStatus(nodeID int64) (*DeployTaskStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, ok := s.tasks[nodeID]
	if !ok {
		return nil, false
	}
	return cloneDeployTaskStatus(status), true
}

func (s *RemoteDeployService) storeTaskStatus(status *DeployTaskStatus) {
	if status == nil || status.NodeID == 0 {
		return
	}
	cloned := cloneDeployTaskStatus(status)
	s.mu.Lock()
	s.tasks[status.NodeID] = cloned
	s.mu.Unlock()
	s.persistTaskStatus(cloned)
}

func (s *RemoteDeployService) persistTaskStatus(status *DeployTaskStatus) {
	if s.nodeRepo == nil || status == nil || status.NodeID == 0 {
		return
	}
	stepsJSON, err := json.Marshal(status.Steps)
	if err != nil {
		s.logger.Warn("Failed to marshal install steps", logger.Err(err), logger.F("node_id", status.NodeID))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.nodeRepo.UpdateInstallStatus(ctx, status.NodeID, status.Status, status.Message, string(stepsJSON), status.Logs, status.StartedAt, status.FinishedAt, &status.UpdatedAt); err != nil {
		s.logger.Warn("Failed to persist install status", logger.Err(err), logger.F("node_id", status.NodeID))
	}
}

func cloneDeployTaskStatus(status *DeployTaskStatus) *DeployTaskStatus {
	if status == nil {
		return nil
	}
	cloned := *status
	if status.Steps != nil {
		cloned.Steps = append([]DeployStep(nil), status.Steps...)
	}
	return &cloned
}

// DeployConfig contains configuration for remote deployment.
type DeployConfig struct {
	NodeID          int64  `json:"node_id,omitempty"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	PrivateKey      string `json:"private_key,omitempty"`
	PanelURL        string `json:"panel_url"`
	NodeToken       string `json:"node_token"`
	AgentBinaryPath string `json:"agent_binary_path,omitempty"` // 本地 Agent 二进制文件路径
}

// DeployStep represents a single deployment step with status.
type DeployStep struct {
	Name   string `json:"name"`   // 步骤名称
	Status string `json:"status"` // pending, running, success, failed
}

// DeployResult contains the result of a deployment.
type DeployResult struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Steps   []DeployStep `json:"steps"`
	Logs    string       `json:"logs"`
}

const (
	defaultRemoteCommandTimeout = 2 * time.Minute
	systemInfoCommandTimeout    = 30 * time.Second
	dependencyInstallTimeout    = 5 * time.Minute
	agentInstallTimeout         = 6 * time.Minute
	xrayInstallTimeout          = 8 * time.Minute
	serviceCommandTimeout       = 3 * time.Minute
	fileTransferCommandTimeout  = 45 * time.Second
)

// Deploy deploys the agent to a remote server.
func (s *RemoteDeployService) Deploy(ctx context.Context, config *DeployConfig) (*DeployResult, error) {
	startedAt := time.Now()
	result := &DeployResult{
		Steps:   []DeployStep{},
		Success: false,
	}

	var logBuffer bytes.Buffer

	publish := func(status, message string, finishedAt *time.Time) {
		if config == nil || config.NodeID == 0 {
			return
		}
		if message == "" {
			message = result.Message
		}
		s.storeTaskStatus(&DeployTaskStatus{
			NodeID:     config.NodeID,
			Status:     status,
			Message:    message,
			Steps:      append([]DeployStep(nil), result.Steps...),
			Logs:       logBuffer.String(),
			StartedAt:  &startedAt,
			FinishedAt: finishedAt,
			UpdatedAt:  time.Now(),
		})
	}

	// 验证配置
	if err := s.validateDeployConfig(config); err != nil {
		result.Message = fmt.Sprintf("配置验证失败: %v", err)
		result.Steps = []DeployStep{{Name: "配置验证", Status: "failed"}}
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}

	// 添加总体超时控制
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	// 记录开始部署
	logBuffer.WriteString("========================================\n")
	logBuffer.WriteString("开始部署 V Panel Agent\n")
	logBuffer.WriteString("========================================\n\n")
	logBuffer.WriteString(fmt.Sprintf("目标服务器: %s:%d\n", config.Host, config.Port))
	logBuffer.WriteString(fmt.Sprintf("用户名: %s\n", config.Username))
	logBuffer.WriteString(fmt.Sprintf("Panel URL: %s\n\n", config.PanelURL))
	publish("running", "开始部署 Agent", nil)

	// 辅助函数：添加步骤并标记为运行中
	addStep := func(name string) int {
		result.Steps = append(result.Steps, DeployStep{Name: name, Status: "running"})
		publish("running", "正在"+name, nil)
		return len(result.Steps) - 1
	}

	// 辅助函数：标记步骤完成
	markSuccess := func(index int) {
		if index >= 0 && index < len(result.Steps) {
			result.Steps[index].Status = "success"
		}
	}

	// 辅助函数：标记步骤失败
	markFailed := func(index int) {
		if index >= 0 && index < len(result.Steps) {
			result.Steps[index].Status = "failed"
		}
	}

	// Step 1: Connect to remote server
	stepIdx := addStep("连接到远程服务器")
	logBuffer.WriteString("步骤 1/8: 连接到远程服务器...\n")

	client, err := s.connectSSH(config)
	if err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("SSH 连接失败: %v", err)
		logBuffer.WriteString(fmt.Sprintf("✗ 连接失败: %v\n", err))
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	defer client.Close()

	markSuccess(stepIdx)
	logBuffer.WriteString("✓ SSH 连接成功\n\n")
	s.logger.Info("SSH connected", logger.F("host", config.Host))

	// Step 2: Check system requirements
	stepIdx = addStep("检查系统要求")
	logBuffer.WriteString("步骤 2/8: 检查系统要求...\n")

	if err := s.checkSystemRequirements(ctx, client, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("系统检查失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)
	logBuffer.WriteString("\n")

	// Step 3: Install dependencies
	stepIdx = addStep("安装依赖")
	logBuffer.WriteString("步骤 3/8: 安装依赖...\n")

	if err := s.installDependencies(ctx, client, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("依赖安装失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)
	logBuffer.WriteString("\n")

	// Step 3.5: Stop old agent processes (before uploading new binary)
	logBuffer.WriteString("清理旧的 Agent 进程...\n")
	if err := s.stopOldAgentProcesses(ctx, client, &logBuffer); err != nil {
		// 记录错误但继续执行（可能是首次安装）
		s.logger.Warn("停止旧进程时出现错误（可能是首次安装）", logger.Err(err))
		logBuffer.WriteString("⚠ 停止旧进程时出现错误（可能是首次安装）\n")
	}
	logBuffer.WriteString("\n")

	// Step 4: Download and install agent
	stepIdx = addStep("下载并安装 Agent")
	logBuffer.WriteString("步骤 4/8: 下载并安装 Agent...\n")

	if err := s.installAgent(ctx, client, config, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("Agent 安装失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)
	logBuffer.WriteString("\n")

	// Step 5: Install Xray
	stepIdx = addStep("安装 Xray")
	logBuffer.WriteString("步骤 5/8: 安装 Xray...\n")

	if err := s.installXray(ctx, client, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("Xray 安装失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)
	logBuffer.WriteString("\n")

	// Step 6: Configure agent
	stepIdx = addStep("配置 Agent")
	logBuffer.WriteString("步骤 6/8: 配置 Agent...\n")

	if err := s.configureAgent(ctx, client, config, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("Agent 配置失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)
	logBuffer.WriteString("\n")

	// Step 7: Start agent service
	stepIdx = addStep("启动 Agent 服务")
	logBuffer.WriteString("步骤 7/8: 启动 Agent 服务...\n")

	if err := s.startAgentService(ctx, client, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("Agent 启动失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)
	logBuffer.WriteString("\n")

	// Step 8: Verify installation
	stepIdx = addStep("验证安装")
	logBuffer.WriteString("步骤 8/8: 验证安装...\n")

	if err := s.verifyInstallation(ctx, client, &logBuffer); err != nil {
		markFailed(stepIdx)
		result.Message = fmt.Sprintf("安装验证失败: %v", err)
		result.Logs = logBuffer.String()
		finishedAt := time.Now()
		publish("failed", result.Message, &finishedAt)
		return result, err
	}
	markSuccess(stepIdx)

	logBuffer.WriteString("\n========================================\n")
	logBuffer.WriteString("✓ Agent 部署成功！\n")
	logBuffer.WriteString("========================================\n")

	result.Success = true
	result.Message = "Agent 部署成功"
	result.Logs = logBuffer.String()

	finishedAt := time.Now()
	publish("success", result.Message, &finishedAt)

	s.logger.Info("Agent deployed successfully", logger.F("host", config.Host))

	return result, nil
}

// connectSSH establishes an SSH connection.
func (s *RemoteDeployService) connectSSH(config *DeployConfig) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	// Password authentication
	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
		// Also try keyboard-interactive for servers that require it
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = config.Password
			}
			return answers, nil
		}))
	}

	// Private key authentication
	if config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("私钥解析失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未提供认证方式，请提供密码或私钥")
	}

	port := config.Port
	if port == 0 {
		port = 22
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
		// 添加更多加密算法和密钥交换算法以提高兼容性
		Config: ssh.Config{
			// 支持更多加密算法（按优先级排序）
			Ciphers: []string{
				"aes128-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes256-gcm@openssh.com",
				"aes128-ctr",
				"aes192-ctr",
				"aes256-ctr",
				"aes128-cbc",
				"aes192-cbc",
				"aes256-cbc",
				"3des-cbc",
			},
			// 添加密钥交换算法
			KeyExchanges: []string{
				"curve25519-sha256",
				"curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha1",
			},
		},
	}

	addr := fmt.Sprintf("%s:%d", config.Host, port)

	s.logger.Info("尝试连接 SSH",
		logger.F("address", addr),
		logger.F("username", config.Username))

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		// Provide more helpful error messages
		if strings.Contains(err.Error(), "connection refused") {
			return nil, fmt.Errorf("连接被拒绝，请检查: 1) SSH 服务是否运行 2) 端口 %d 是否正确 3) 防火墙是否允许", port)
		}
		if strings.Contains(err.Error(), "no supported methods remain") {
			return nil, fmt.Errorf("认证失败，请检查: 1) 用户名是否正确 2) 密码/私钥是否正确 3) 服务器是否允许密码认证")
		}
		if strings.Contains(err.Error(), "handshake failed") {
			return nil, fmt.Errorf("SSH 握手失败，请检查: 1) 服务器 SSH 版本是否兼容 2) 网络连接是否稳定 3) 是否有中间代理")
		}
		if strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("连接超时，请检查: 1) 服务器地址是否正确 2) 网络是否可达 3) 防火墙规则")
		}
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}

	s.logger.Info("SSH 连接成功", logger.F("address", addr))
	return client, nil
}

func appendRemoteCommandOutput(logBuffer *bytes.Buffer, stdout, stderr string) {
	if logBuffer == nil {
		return
	}
	if stdout != "" {
		logBuffer.WriteString(stdout)
		if !strings.HasSuffix(stdout, "\n") {
			logBuffer.WriteString("\n")
		}
	}
	if stderr != "" {
		logBuffer.WriteString(stderr)
		if !strings.HasSuffix(stderr, "\n") {
			logBuffer.WriteString("\n")
		}
	}
}

func recordRemoteCommandFailure(logBuffer *bytes.Buffer, message, stdout, stderr string) {
	if logBuffer == nil {
		return
	}
	logBuffer.WriteString(message)
	if !strings.HasSuffix(message, "\n") {
		logBuffer.WriteString("\n")
	}
	if stdout != "" {
		logBuffer.WriteString(fmt.Sprintf("输出: %s", stdout))
		if !strings.HasSuffix(stdout, "\n") {
			logBuffer.WriteString("\n")
		}
	}
	if stderr != "" {
		logBuffer.WriteString(fmt.Sprintf("错误: %s", stderr))
		if !strings.HasSuffix(stderr, "\n") {
			logBuffer.WriteString("\n")
		}
	}
}

func (s *RemoteDeployService) runRemoteCommand(ctx context.Context, client *ssh.Client, command string, timeout time.Duration) (string, string, error) {
	if client == nil {
		return "", "", fmt.Errorf("SSH client 不能为空")
	}
	if timeout <= 0 {
		timeout = defaultRemoteCommandTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	s.logger.Debug("执行命令", logger.F("command", command))

	if err := session.Start(command); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("启动命令失败: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return stdout.String(), stderr.String(), fmt.Errorf("命令执行失败: %w", err)
		}
		return stdout.String(), stderr.String(), nil
	case <-cmdCtx.Done():
		if signalErr := session.Signal(ssh.SIGKILL); signalErr != nil {
			s.logger.Debug("发送远程终止信号失败", logger.Err(signalErr))
		}
		if closeErr := session.Close(); closeErr != nil {
			s.logger.Debug("关闭 SSH 会话失败", logger.Err(closeErr))
		}

		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}

		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return stdout.String(), stderr.String(), fmt.Errorf("命令执行超时（>%s）", timeout)
		}
		return stdout.String(), stderr.String(), fmt.Errorf("命令执行被取消: %w", cmdCtx.Err())
	}
}

// executeCommand executes a command on the remote server.
func (s *RemoteDeployService) executeCommand(client *ssh.Client, command string, logBuffer *bytes.Buffer) error {
	return s.executeCommandWithTimeout(context.Background(), client, command, defaultRemoteCommandTimeout, logBuffer)
}

func (s *RemoteDeployService) executeCommandWithTimeout(ctx context.Context, client *ssh.Client, command string, timeout time.Duration, logBuffer *bytes.Buffer) error {
	stdout, stderr, err := s.runRemoteCommand(ctx, client, command, timeout)
	if err != nil {
		recordRemoteCommandFailure(logBuffer, "✗ "+err.Error(), stdout, stderr)
		return err
	}

	appendRemoteCommandOutput(logBuffer, stdout, stderr)
	return nil
}

// checkSystemRequirements checks if the system meets requirements.
func (s *RemoteDeployService) checkSystemRequirements(ctx context.Context, client *ssh.Client, logBuffer *bytes.Buffer) error {
	commands := []string{
		"uname -a",                    // OS info
		"cat /etc/os-release || true", // Distribution info
		"free -h",                     // Memory info
		"df -h /",                     // Disk space
	}

	for _, cmd := range commands {
		if err := s.executeCommandWithTimeout(ctx, client, cmd, systemInfoCommandTimeout, logBuffer); err != nil {
			// Don't fail on info commands
			s.logger.Warn("System check command failed", logger.F("command", cmd))
		}
	}

	logBuffer.WriteString("✓ 系统检查完成\n")
	return nil
}

// installDependencies installs required dependencies.
func (s *RemoteDeployService) installDependencies(ctx context.Context, client *ssh.Client, logBuffer *bytes.Buffer) error {
	// Detect package manager and install dependencies
	script := `
set -e
echo "检测操作系统和包管理器..."

# 检测操作系统
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    OS_VERSION=$VERSION_ID
    echo "操作系统: $NAME $VERSION_ID"
else
    echo "警告: 无法检测操作系统版本"
fi

# 根据不同系统安装依赖
if command -v apt-get &> /dev/null; then
    echo "使用 apt-get 安装依赖 (Ubuntu/Debian)..."
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq || true
    apt-get install -y curl wget unzip tar ca-certificates 2>&1 || {
        echo "警告: 部分依赖安装失败，继续..."
    }
elif command -v dnf &> /dev/null; then
    echo "使用 dnf 安装依赖 (Fedora/CentOS 8+/AlmaLinux/Rocky)..."
    dnf install -y curl wget unzip tar ca-certificates 2>&1 || {
        echo "警告: 部分依赖安装失败，继续..."
    }
elif command -v yum &> /dev/null; then
    echo "使用 yum 安装依赖 (CentOS/RHEL 7)..."
    yum install -y curl wget unzip tar ca-certificates 2>&1 || {
        echo "警告: 部分依赖安装失败，继续..."
    }
elif command -v apk &> /dev/null; then
    echo "使用 apk 安装依赖 (Alpine)..."
    apk add --no-cache curl wget unzip tar ca-certificates || {
        echo "警告: 部分依赖安装失败，继续..."
    }
elif command -v zypper &> /dev/null; then
    echo "使用 zypper 安装依赖 (openSUSE)..."
    zypper install -y curl wget unzip tar ca-certificates || {
        echo "警告: 部分依赖安装失败，继续..."
    }
else
    echo "警告: 未检测到支持的包管理器，跳过依赖安装"
fi

# 验证关键命令是否可用
echo "验证依赖..."
command -v curl >/dev/null 2>&1 || echo "警告: curl 未安装"
command -v wget >/dev/null 2>&1 || echo "警告: wget 未安装"
command -v unzip >/dev/null 2>&1 || echo "警告: unzip 未安装"
command -v tar >/dev/null 2>&1 || echo "警告: tar 未安装"

echo "依赖检查完成"
`

	if err := s.executeCommandWithTimeout(ctx, client, script, dependencyInstallTimeout, logBuffer); err != nil {
		// 不因为依赖安装失败而中断，继续尝试
		s.logger.Warn("依赖安装有警告，但继续执行", logger.F("error", err))
		logBuffer.WriteString("⚠ 依赖安装有警告，继续执行\n")
		return nil
	}

	logBuffer.WriteString("✓ 依赖安装完成\n")
	return nil
}

// installAgent uploads and installs the agent binary.
func (s *RemoteDeployService) installAgent(ctx context.Context, client *ssh.Client, config *DeployConfig, logBuffer *bytes.Buffer) error {
	// 先创建目录
	script := `
# 创建目录
mkdir -p /usr/local/bin
mkdir -p /etc/vpanel
mkdir -p /var/log/vpanel

# 检测系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l)
        ARCH="arm"
        ;;
    *)
        echo "错误: 不支持的架构: $ARCH"
        exit 1
        ;;
esac

echo "检测到系统架构: $ARCH"
`

	if err := s.executeCommandWithTimeout(ctx, client, script, systemInfoCommandTimeout, logBuffer); err != nil {
		return err
	}

	// 尝试通过 Panel URL 下载（如果配置了公网地址）
	if config.PanelURL != "" && !strings.Contains(config.PanelURL, "localhost") && !strings.Contains(config.PanelURL, "127.0.0.1") {
		logBuffer.WriteString("尝试从 Panel 服务器下载 Agent...\n")
		downloadScript := fmt.Sprintf(`
PANEL_URL="%s"
DOWNLOAD_URL="${PANEL_URL}/api/admin/nodes/agent/download?arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/;s/armv7l/arm/')"

echo "下载地址: ${DOWNLOAD_URL}"

if command -v wget &> /dev/null; then
    wget --no-check-certificate -O /usr/local/bin/vpanel-agent "$DOWNLOAD_URL" 2>&1 && chmod +x /usr/local/bin/vpanel-agent && echo "✓ Agent 下载成功" && exit 0
elif command -v curl &> /dev/null; then
    curl -k -L -o /usr/local/bin/vpanel-agent "$DOWNLOAD_URL" 2>&1 && chmod +x /usr/local/bin/vpanel-agent && echo "✓ Agent 下载成功" && exit 0
fi

echo "下载失败，将使用上传方式"
exit 1
`, config.PanelURL)

		// 尝试下载，失败也不报错
		if err := s.executeCommandWithTimeout(ctx, client, downloadScript, agentInstallTimeout, logBuffer); err == nil {
			logBuffer.WriteString("✓ Agent 安装完成（通过下载）\n")
			return nil
		}
	}

	// 下载失败或未配置公网地址，使用 SCP 上传方式
	logBuffer.WriteString("使用 SCP 上传方式安装 Agent...\n")

	// 读取本地 Agent 文件
	agentPath := config.AgentBinaryPath
	if agentPath == "" {
		// 默认路径，readAgentBinary 会尝试多个位置
		agentPath = ""
	}

	agentData, err := s.readAgentBinary(agentPath)
	if err != nil {
		logBuffer.WriteString(fmt.Sprintf("✗ 无法读取 Agent 文件: %v\n", err))
		logBuffer.WriteString("请确保已编译 Agent: cd cmd/agent && go build -o ../../bin/vpanel-agent\n")
		return fmt.Errorf("无法读取 Agent 文件: %w", err)
	}

	logBuffer.WriteString(fmt.Sprintf("正在上传 Agent (大小: %d bytes)...\n", len(agentData)))

	// 计算本地文件哈希
	localHash := sha256.Sum256(agentData)
	localHashStr := hex.EncodeToString(localHash[:])
	s.logger.Info("Agent 文件哈希", logger.F("sha256", localHashStr))

	// 使用 SCP 协议上传文件（包含验证）
	if err := s.uploadFileSCP(ctx, client, agentData, "/usr/local/bin/vpanel-agent", localHashStr, logBuffer); err != nil {
		return fmt.Errorf("Agent 上传失败: %w", err)
	}

	logBuffer.WriteString("✓ Agent 安装完成（通过 SCP 上传）\n")
	return nil
}

// installXray installs Xray using the official script.
func (s *RemoteDeployService) installXray(ctx context.Context, client *ssh.Client, logBuffer *bytes.Buffer) error {
	script := `
set -e

download() {
    local url="$1"
    local output="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 15 --max-time 180 --retry 2 --retry-delay 2 -o "$output" "$url"
        return
    fi
    if command -v wget >/dev/null 2>&1; then
        wget --timeout=30 --tries=2 -O "$output" "$url"
        return
    fi
    echo "错误: 未找到 curl 或 wget"
    exit 1
}

# 检查 Xray 是否已安装
if command -v xray &> /dev/null; then
    echo "Xray 已安装，跳过"
    xray version | head -n 1
else
    echo "正在安装 Xray..."

    # 尝试使用官方安装脚本
    if download "https://github.com/XTLS/Xray-install/raw/main/install-release.sh" /tmp/install-xray.sh; then
        chmod +x /tmp/install-xray.sh
        bash /tmp/install-xray.sh install
        rm -f /tmp/install-xray.sh
    else
        echo "官方脚本下载失败，尝试手动安装..."
        
        # 检测系统架构
        ARCH=$(uname -m)
        case "$ARCH" in
            x86_64)
                XRAY_ARCH="linux-64"
                ;;
            aarch64|arm64)
                XRAY_ARCH="linux-arm64-v8a"
                ;;
            armv7l)
                XRAY_ARCH="linux-arm32-v7a"
                ;;
            *)
                echo "错误: 不支持的架构: $ARCH"
                exit 1
                ;;
        esac

        XRAY_URL="https://github.com/XTLS/Xray-core/releases/latest/download/Xray-${XRAY_ARCH}.zip"

        echo "下载 Xray for ${XRAY_ARCH}..."
        download "$XRAY_URL" /tmp/xray.zip

        echo "解压并安装..."
        mkdir -p /tmp/xray
        unzip -o /tmp/xray.zip -d /tmp/xray >/dev/null
        mv /tmp/xray/xray /usr/local/bin/
        chmod +x /usr/local/bin/xray
        rm -rf /tmp/xray /tmp/xray.zip
    fi
    
    # 验证安装
    if command -v xray &> /dev/null; then
        echo "✓ Xray 安装成功"
        xray version | head -n 1
    else
        echo "✗ Xray 安装失败"
        exit 1
    fi
fi

# 创建 Xray 配置目录
mkdir -p /etc/xray
mkdir -p /var/log/xray
echo "✓ Xray 目录创建完成"
`

	if err := s.executeCommandWithTimeout(ctx, client, script, xrayInstallTimeout, logBuffer); err != nil {
		return err
	}

	logBuffer.WriteString("✓ Xray 安装完成\n")
	return nil
}

// stopOldAgentProcesses 停止所有旧的 Agent 进程
func (s *RemoteDeployService) stopOldAgentProcesses(ctx context.Context, client *ssh.Client, logBuffer *bytes.Buffer) error {
	stopScript := `
echo "=== 清理旧的 Agent 进程 ==="

# 停止 systemd 服务
if systemctl is-active --quiet vpanel-agent 2>/dev/null; then
    echo "停止 systemd 服务..."
    systemctl stop vpanel-agent
    systemctl disable vpanel-agent 2>/dev/null || true
    sleep 2
fi

# 强制杀死所有 vpanel-agent 进程（多次尝试确保清理干净）
for i in {1..3}; do
    if pgrep -x vpanel-agent >/dev/null 2>&1; then
        echo "第 $i 次尝试：发现残留的 Agent 进程，强制终止..."
        pkill -9 vpanel-agent 2>/dev/null || true
        sleep 1
    else
        echo "✓ 所有旧进程已清理"
        break
    fi
done

# 最终验证
if pgrep -x vpanel-agent >/dev/null 2>&1; then
    echo "⚠ 警告：仍有 Agent 进程在运行，尝试使用 killall"
    killall -9 vpanel-agent 2>/dev/null || true
    sleep 1
fi

# 删除旧的配置文件（避免读取旧配置）
if [ -f /etc/vpanel/agent.yaml ]; then
    echo "备份并删除旧配置文件..."
    cp /etc/vpanel/agent.yaml /etc/vpanel/agent.yaml.backup.$(date +%s) 2>/dev/null || true
    rm -f /etc/vpanel/agent.yaml
fi

echo "✓ 清理完成"
`
	return s.executeCommandWithTimeout(ctx, client, stopScript, serviceCommandTimeout, logBuffer)
}

// configureAgent creates the agent configuration file.
func (s *RemoteDeployService) configureAgent(ctx context.Context, client *ssh.Client, config *DeployConfig, logBuffer *bytes.Buffer) error {
	// 验证 token 不为空
	if config.NodeToken == "" {
		return fmt.Errorf("节点 token 为空，无法配置 Agent")
	}

	// 记录配置信息用于调试
	s.logger.Info("Configuring agent",
		logger.F("panel_url", config.PanelURL),
		logger.F("node_token_length", len(config.NodeToken)),
		logger.F("node_token_prefix", config.NodeToken[:min(16, len(config.NodeToken))]),
		logger.F("node_token_suffix", config.NodeToken[max(0, len(config.NodeToken)-8):]))

	logBuffer.WriteString(fmt.Sprintf("配置 Token 长度: %d\n", len(config.NodeToken)))
	logBuffer.WriteString(fmt.Sprintf("配置 Token 前缀: %s\n", config.NodeToken[:min(16, len(config.NodeToken))]))
	logBuffer.WriteString(fmt.Sprintf("配置 Token 后缀: %s\n\n", config.NodeToken[max(0, len(config.NodeToken)-8):]))

	// 创建新的 Agent 配置
	agentConfig := fmt.Sprintf(`node:
  token: "%s"

panel:
  url: "%s"

xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"

health:
  port: 18443
`, config.NodeToken, config.PanelURL)

	// 使用 base64 编码配置内容，避免特殊字符问题
	encoded := base64Encode([]byte(agentConfig))

	// 第三步：写入新配置文件
	script := fmt.Sprintf(`
echo "=== 写入新的 Agent 配置 ==="

# 确保目录存在
mkdir -p /etc/vpanel

# 写入新配置（强制覆盖）
echo '%s' | base64 -d > /etc/vpanel/agent.yaml
chmod 644 /etc/vpanel/agent.yaml

echo "✓ 配置文件已创建"
echo ""
echo "=== 配置内容验证 ==="
cat /etc/vpanel/agent.yaml
echo ""
echo "=== Token 验证 ==="
TOKEN_LINE=$(grep "token:" /etc/vpanel/agent.yaml | head -1)
echo "$TOKEN_LINE"
TOKEN_VALUE=$(echo "$TOKEN_LINE" | sed 's/.*token: *"\(.*\)".*/\1/')
echo "Token 长度: ${#TOKEN_VALUE}"
echo "Token 前缀: ${TOKEN_VALUE:0:16}"
echo "Token 后缀: ${TOKEN_VALUE: -8}"
`, encoded)

	if err := s.executeCommandWithTimeout(ctx, client, script, serviceCommandTimeout, logBuffer); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 第四步：创建 systemd 服务文件
	serviceFile := `[Unit]
Description=V Panel Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/vpanel-agent -config /etc/vpanel/agent.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`

	script = fmt.Sprintf(`
echo "=== 创建 systemd 服务 ==="
cat > /etc/systemd/system/vpanel-agent.service <<'EOF'
%s
EOF
systemctl daemon-reload
echo "✓ systemd 服务文件已创建"
`, serviceFile)

	if err := s.executeCommandWithTimeout(ctx, client, script, serviceCommandTimeout, logBuffer); err != nil {
		return fmt.Errorf("创建服务文件失败: %w", err)
	}

	logBuffer.WriteString("✓ Agent 配置完成\n")
	return nil
}

// startAgentService starts the agent service.
func (s *RemoteDeployService) startAgentService(ctx context.Context, client *ssh.Client, logBuffer *bytes.Buffer) error {
	script := `
# 重新加载 systemd
systemctl daemon-reload

# 启用并启动服务
systemctl enable vpanel-agent
systemctl restart vpanel-agent
sleep 3

# 检查服务状态
if systemctl is-active --quiet vpanel-agent; then
    echo "✓ Agent 服务启动成功"
    systemctl status vpanel-agent --no-pager | head -20
else
    echo "✗ Agent 服务启动失败"
    echo ""
    echo "=== 服务状态 ==="
    systemctl status vpanel-agent --no-pager || true
    echo ""
    echo "=== 最近日志 ==="
    journalctl -u vpanel-agent -n 50 --no-pager || true
    echo ""
    echo "=== 配置文件检查 ==="
    cat /etc/vpanel/agent.yaml || true
    exit 1
fi
`

	if err := s.executeCommandWithTimeout(ctx, client, script, serviceCommandTimeout, logBuffer); err != nil {
		return err
	}

	logBuffer.WriteString("✓ Agent 服务启动完成\n")
	return nil
}

// verifyInstallation verifies the installation.
func (s *RemoteDeployService) verifyInstallation(ctx context.Context, client *ssh.Client, logBuffer *bytes.Buffer) error {
	script := `
echo "=== 验证安装 ==="

# 1. 检查 Agent 服务状态
if systemctl is-active --quiet vpanel-agent; then
    echo "✓ Agent 服务运行中"
else
    echo "✗ Agent 服务未运行"
    systemctl status vpanel-agent --no-pager || true
    exit 1
fi

# 2. 检查 Agent 进程
if pgrep -x vpanel-agent >/dev/null 2>&1; then
    AGENT_PID=$(pgrep -x vpanel-agent)
    echo "✓ Agent 进程运行中 (PID: $AGENT_PID)"
else
    echo "✗ Agent 进程未运行"
    exit 1
fi

# 3. 检查 Xray 安装
if command -v xray &> /dev/null; then
    XRAY_VERSION=$(xray version 2>/dev/null | head -1 || echo "unknown")
    echo "✓ Xray 已安装: $XRAY_VERSION"
else
    echo "✗ Xray 未安装"
    exit 1
fi

# 4. 检查配置文件
if [ -f /etc/vpanel/agent.yaml ]; then
    echo "✓ 配置文件存在"
    
    # 验证配置文件可读
    if [ -r /etc/vpanel/agent.yaml ]; then
        echo "✓ 配置文件可读"
        
        # 检查 Token 是否存在
        if grep -q "token:" /etc/vpanel/agent.yaml; then
            TOKEN_LINE=$(grep "token:" /etc/vpanel/agent.yaml | head -1)
            TOKEN_VALUE=$(echo "$TOKEN_LINE" | sed 's/.*token: *"\(.*\)".*/\1/')
            TOKEN_LEN=${#TOKEN_VALUE}
            if [ "$TOKEN_LEN" -ge 32 ]; then
                echo "✓ Token 配置正确 (长度: $TOKEN_LEN)"
            else
                echo "✗ Token 长度不足 (长度: $TOKEN_LEN)"
                exit 1
            fi
        else
            echo "✗ 配置文件中未找到 Token"
            exit 1
        fi
    else
        echo "✗ 配置文件不可读"
        exit 1
    fi
else
    echo "✗ 配置文件不存在"
    exit 1
fi

# 5. 检查健康检查端口
HEALTH_PORT=18443
if netstat -tuln 2>/dev/null | grep -q ":$HEALTH_PORT " || ss -tuln 2>/dev/null | grep -q ":$HEALTH_PORT "; then
    echo "✓ 健康检查端口 $HEALTH_PORT 已监听"
else
    echo "⚠ 健康检查端口 $HEALTH_PORT 未监听（可能正在启动）"
fi

# 6. 检查 Xray 是否已被 Agent 自动拉起
XRAY_READY=0
for i in 1 2 3 4 5 6 7 8; do
    if curl -fsS http://127.0.0.1:${HEALTH_PORT}/xray/status 2>/dev/null | grep -q '"running":true'; then
        echo "✓ Xray 已由 Agent 自动拉起"
        XRAY_READY=1
        break
    fi
    sleep 2
done

if [ "$XRAY_READY" -ne 1 ]; then
    echo "⚠ Xray 暂未就绪，Agent 将继续后台自动拉起/自愈"
    systemctl status xray --no-pager 2>/dev/null | head -20 || true
fi

# 7. 检查最近日志（查看是否有错误）
echo ""
echo "=== 最近日志 ==="
journalctl -u vpanel-agent -n 10 --no-pager 2>/dev/null || echo "无法读取日志"

echo ""
echo "✓ 安装验证完成"
`

	if err := s.executeCommandWithTimeout(ctx, client, script, serviceCommandTimeout, logBuffer); err != nil {
		return err
	}

	logBuffer.WriteString("✓ 安装验证完成\n")
	return nil
}

// UploadAgentBinary uploads the agent binary to the remote server.
func (s *RemoteDeployService) UploadAgentBinary(client *ssh.Client, localPath string) error {
	// Open SFTP session
	sftp, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SFTP session: %w", err)
	}
	defer sftp.Close()

	// TODO: Implement file upload using SFTP
	// This requires the golang.org/x/crypto/ssh package with SFTP support

	return nil
}

// GetDeployScript returns the deployment script for manual installation.
func (s *RemoteDeployService) GetDeployScript(panelURL, nodeToken string) string {
	return fmt.Sprintf(`#!/bin/bash
# V Panel Agent 自动部署脚本
# 支持: Ubuntu, Debian, CentOS, AlmaLinux, Rocky Linux, Fedora
# 使用方法: bash install-agent.sh

set -e

PANEL_URL="%s"
NODE_TOKEN="%s"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "  V Panel Agent 自动部署脚本"
echo "=========================================="
echo ""

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}错误: 请使用 root 用户运行此脚本${NC}"
    exit 1
fi

echo "开始部署 V Panel Agent..."

# 检测操作系统
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    echo -e "${GREEN}检测到操作系统: $NAME $VERSION_ID${NC}"
else
    echo -e "${YELLOW}警告: 无法检测操作系统版本${NC}"
    OS="unknown"
fi

# 检测系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    armv7l)
        ARCH="arm"
        ;;
    *)
        echo -e "${RED}错误: 不支持的架构: $ARCH${NC}"
        exit 1
        ;;
esac

echo "检测到系统架构: $ARCH"

# 安装依赖
echo "正在安装依赖..."
if command -v apt-get &> /dev/null; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq curl wget unzip tar ca-certificates
elif command -v dnf &> /dev/null; then
    dnf install -y -q curl wget unzip tar ca-certificates
elif command -v yum &> /dev/null; then
    yum install -y -q curl wget unzip tar ca-certificates
else
    echo -e "${RED}错误: 不支持的包管理器${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 依赖安装完成${NC}"

# 安装 Xray
if ! command -v xray &> /dev/null; then
    echo "正在安装 Xray..."
    if curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh -o /tmp/install-xray.sh 2>&1; then
        bash /tmp/install-xray.sh install
        rm -f /tmp/install-xray.sh
        echo -e "${GREEN}✓ Xray 安装完成${NC}"
    else
        echo -e "${YELLOW}警告: Xray 安装失败，请手动安装${NC}"
    fi
else
    echo -e "${GREEN}✓ Xray 已安装${NC}"
    xray version | head -1
fi

# 创建目录
echo "正在创建目录..."
mkdir -p /etc/vpanel
mkdir -p /etc/xray
mkdir -p /var/log/vpanel
mkdir -p /var/log/xray
mkdir -p /usr/local/bin

# 下载 Agent 二进制文件
echo "正在下载 Agent..."
# 注意: 请替换为实际的下载地址
# AGENT_VERSION="latest"
# DOWNLOAD_URL="https://github.com/your-org/vpanel/releases/download/${AGENT_VERSION}/vpanel-agent-linux-${ARCH}"
# wget -O /usr/local/bin/vpanel-agent "$DOWNLOAD_URL" || curl -L -o /usr/local/bin/vpanel-agent "$DOWNLOAD_URL"

# 临时方案: 提示用户手动上传
echo -e "${YELLOW}⚠ 注意: 当前需要手动上传 agent 二进制文件${NC}"
echo "请将编译好的 vpanel-agent 上传到 /usr/local/bin/vpanel-agent"
echo ""
echo "编译命令:"
echo "  cd cmd/agent && go build -o vpanel-agent"
echo ""
echo "上传命令:"
echo "  scp vpanel-agent root@YOUR_SERVER:/usr/local/bin/"
echo ""
read -p "按回车键继续 (确认已上传 agent 二进制文件)..."

# 设置权限
chmod +x /usr/local/bin/vpanel-agent

# 创建配置文件
echo "正在创建配置文件..."
cat > /etc/vpanel/agent.yaml <<EOF
node:
  token: "$NODE_TOKEN"

panel:
  url: "$PANEL_URL"

xray:
  binary_path: "/usr/local/bin/xray"
  config_path: "/usr/local/etc/xray/config.json"

health:
  port: 18443

log:
  level: "info"
  file: "/var/log/vpanel/agent.log"
EOF

chmod 600 /etc/vpanel/agent.yaml
echo -e "${GREEN}✓ 配置文件创建完成${NC}"

# 创建 systemd 服务
echo "正在创建 systemd 服务..."
cat > /etc/systemd/system/vpanel-agent.service <<EOF
[Unit]
Description=V Panel Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/vpanel-agent -config /etc/vpanel/agent.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

StandardOutput=journal
StandardError=journal
SyslogIdentifier=vpanel-agent

[Install]
WantedBy=multi-user.target
EOF

# 重载 systemd
systemctl daemon-reload
echo -e "${GREEN}✓ systemd 服务创建完成${NC}"

# 启动服务
echo "正在启动 Agent 服务..."
systemctl enable vpanel-agent
systemctl start vpanel-agent

# 等待服务启动
sleep 2

# 检查服务状态
if systemctl is-active --quiet vpanel-agent; then
    echo -e "${GREEN}✓ Agent 服务启动成功${NC}"
else
    echo -e "${RED}✗ Agent 服务启动失败${NC}"
    echo "查看日志: journalctl -u vpanel-agent -n 50"
    exit 1
fi

echo ""
echo "=========================================="
echo "  部署完成！"
echo "=========================================="
echo ""
echo "常用命令:"
echo "  查看状态: systemctl status vpanel-agent"
echo "  查看日志: journalctl -u vpanel-agent -f"
echo "  重启服务: systemctl restart vpanel-agent"
echo "  停止服务: systemctl stop vpanel-agent"
echo ""
`, panelURL, nodeToken)
}

// TestConnection tests SSH connection without deploying.
func (s *RemoteDeployService) TestConnection(ctx context.Context, config *DeployConfig) error {
	s.logger.Info("开始测试 SSH 连接",
		logger.F("host", config.Host),
		logger.F("port", config.Port),
		logger.F("username", config.Username))

	// 连接 SSH
	client, err := s.connectSSH(config)
	if err != nil {
		s.logger.Error("SSH 连接失败", logger.Err(err))
		return err
	}
	defer client.Close()

	s.logger.Info("SSH 连接成功，测试执行命令")

	// 执行测试命令
	stdout, stderr, err := s.runRemoteCommand(ctx, client, "echo 'Connection test successful' && whoami && pwd", systemInfoCommandTimeout)
	if err != nil {
		s.logger.Error("执行测试命令失败", logger.Err(err))
		return fmt.Errorf("执行测试命令失败: %w", err)
	}

	s.logger.Info("SSH 连接测试成功",
		logger.F("output", strings.TrimSpace(strings.Join(filterNonEmptyStrings(stdout, stderr), "\n"))))

	return nil
}

// validateDeployConfig 验证部署配置
func (s *RemoteDeployService) validateDeployConfig(config *DeployConfig) error {
	if config.Host == "" {
		return fmt.Errorf("服务器地址不能为空")
	}

	if config.Username == "" {
		return fmt.Errorf("用户名不能为空")
	}

	if config.Password == "" && config.PrivateKey == "" {
		return fmt.Errorf("必须提供密码或私钥")
	}

	if config.NodeToken == "" {
		return fmt.Errorf("节点 Token 不能为空")
	}

	if len(config.NodeToken) < 32 {
		return fmt.Errorf("节点 Token 长度不足（至少 32 字符，当前 %d 字符）", len(config.NodeToken))
	}

	if config.PanelURL == "" {
		return fmt.Errorf("Panel URL 不能为空")
	}

	if strings.Contains(config.PanelURL, "localhost") || strings.Contains(config.PanelURL, "127.0.0.1") {
		return fmt.Errorf("Panel URL 不能使用 localhost 或 127.0.0.1，远程节点无法访问")
	}

	return nil
}

// readAgentBinary 读取本地 Agent 二进制文件
func (s *RemoteDeployService) readAgentBinary(path string) ([]byte, error) {
	// 尝试多个可能的路径
	paths := []string{
		path,
		"./bin/vpanel-agent",
		"./bin/vpanel-agent-amd64",
		"./bin/vpanel-agent-arm64",
		"./vpanel-agent",
		"../bin/vpanel-agent",
		"../bin/vpanel-agent-amd64",
		"/usr/local/bin/vpanel-agent",
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err == nil {
			s.logger.Info("找到 Agent 文件", logger.F("path", p), logger.F("size", len(data)))
			return data, nil
		}
	}

	return nil, fmt.Errorf("未找到 Agent 二进制文件，请先编译: cd cmd/agent && go build -o ../../bin/vpanel-agent")
}

// base64Encode 对数据进行 base64 编码
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// uploadFileSCP 使用 SCP 协议上传文件
func (s *RemoteDeployService) uploadFileSCP(ctx context.Context, client *ssh.Client, data []byte, remotePath string, expectedHash string, logBuffer *bytes.Buffer) error {
	// 先停止可能正在运行的 Agent 进程，避免 "Text file busy" 错误
	stopScript := `
if systemctl is-active --quiet vpanel-agent 2>/dev/null; then
    systemctl stop vpanel-agent 2>/dev/null || true
fi
pkill -9 vpanel-agent 2>/dev/null || true
sleep 1
`
	s.executeCommandWithTimeout(ctx, client, stopScript, serviceCommandTimeout, logBuffer)

	// 创建临时文件并上传
	tmpPath := "/tmp/vpanel-agent.tmp"

	// 使用 base64 编码上传（更可靠）
	encoded := base64Encode(data)

	// 分块上传（每块 100KB）
	chunkSize := 100 * 1024
	totalChunks := (len(encoded) + chunkSize - 1) / chunkSize

	logBuffer.WriteString(fmt.Sprintf("开始上传文件 (共 %d 块)...\n", totalChunks))

	// 清空临时文件
	if err := s.executeCommandWithTimeout(ctx, client, fmt.Sprintf("rm -f %s", tmpPath), fileTransferCommandTimeout, logBuffer); err != nil {
		return fmt.Errorf("清空临时文件失败: %w", err)
	}

	// 分块上传
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}

		chunk := encoded[i:end]
		chunkNum := i/chunkSize + 1

		// 每 10 块显示一次进度
		if chunkNum%10 == 0 || chunkNum == totalChunks {
			logBuffer.WriteString(fmt.Sprintf("上传进度: %d/%d\n", chunkNum, totalChunks))
		}

		uploadCmd := fmt.Sprintf("echo '%s' | base64 -d >> %s", chunk, tmpPath)
		if err := s.executeCommandWithTimeout(ctx, client, uploadCmd, fileTransferCommandTimeout, logBuffer); err != nil {
			return fmt.Errorf("上传第 %d 块失败: %w", chunkNum, err)
		}
	}

	// 移动到目标位置并设置权限
	moveScript := fmt.Sprintf(`
mv -f %s %s
chmod +x %s

# 验证文件大小
FILE_SIZE=$(stat -c%%s %s 2>/dev/null || stat -f%%z %s 2>/dev/null || echo "0")
echo "文件大小: ${FILE_SIZE} bytes"

if [ "$FILE_SIZE" -lt 1000 ]; then
    echo "错误: 文件大小异常"
    exit 1
fi

# 计算文件哈希
REMOTE_HASH=$(sha256sum %s 2>/dev/null | awk '{print $1}' || shasum -a 256 %s 2>/dev/null | awk '{print $1}')
echo "远程文件哈希: ${REMOTE_HASH}"
`, tmpPath, remotePath, remotePath, remotePath, remotePath, remotePath, remotePath)

	if err := s.executeCommandWithTimeout(ctx, client, moveScript, serviceCommandTimeout, logBuffer); err != nil {
		return fmt.Errorf("移动文件失败: %w", err)
	}

	// 验证哈希值
	verifyScript := fmt.Sprintf(`
REMOTE_HASH=$(sha256sum %s 2>/dev/null | awk '{print $1}' || shasum -a 256 %s 2>/dev/null | awk '{print $1}')
if [ "$REMOTE_HASH" != "%s" ]; then
    echo "✗ 文件校验失败"
    echo "  期望: %s"
    echo "  实际: ${REMOTE_HASH}"
    exit 1
fi
echo "✓ 文件校验通过"
`, remotePath, remotePath, expectedHash, expectedHash)

	if err := s.executeCommandWithTimeout(ctx, client, verifyScript, fileTransferCommandTimeout, logBuffer); err != nil {
		return fmt.Errorf("文件完整性校验失败: %w", err)
	}

	logBuffer.WriteString("✓ 文件上传完成并通过校验\n")
	return nil
}
