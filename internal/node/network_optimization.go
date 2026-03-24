package node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	NetworkOptimizationBackupPath = "/etc/vpanel/network-optimization-backup.env"
	networkOptimizationSysctlPath = "/etc/sysctl.d/99-vpanel-network-optimization.conf"
)

// NetworkOptimizationSettings describes node-level network optimization preferences.
type NetworkOptimizationSettings struct {
	EnableBBR         bool   `json:"enable_bbr"`
	EnableFQ          bool   `json:"enable_fq"`
	EnableTCPFastOpen bool   `json:"enable_tcp_fastopen"`
	EnableXraySockopt bool   `json:"enable_xray_sockopt"`
	XrayTCPFastOpen   bool   `json:"xray_tcp_fastopen"`
	XrayTCPCongestion string `json:"xray_tcp_congestion"`
}

// RecommendedNetworkOptimizationSettings returns the default recommended profile.
func RecommendedNetworkOptimizationSettings() NetworkOptimizationSettings {
	return NetworkOptimizationSettings{
		EnableBBR:         true,
		EnableFQ:          true,
		EnableTCPFastOpen: true,
		EnableXraySockopt: true,
		XrayTCPFastOpen:   true,
		XrayTCPCongestion: "bbr",
	}
}

// Normalize ensures optional values are in a safe shape.
func (s NetworkOptimizationSettings) Normalize() NetworkOptimizationSettings {
	normalized := s
	normalized.XrayTCPCongestion = strings.ToLower(strings.TrimSpace(normalized.XrayTCPCongestion))

	if !normalized.EnableXraySockopt {
		normalized.XrayTCPFastOpen = false
		normalized.XrayTCPCongestion = ""
		return normalized
	}

	if normalized.XrayTCPCongestion == "" && normalized.EnableBBR {
		normalized.XrayTCPCongestion = "bbr"
	}
	if normalized.EnableTCPFastOpen && !normalized.XrayTCPFastOpen {
		normalized.XrayTCPFastOpen = true
	}

	return normalized
}

// IsEmpty reports whether no optimization toggle is enabled.
func (s NetworkOptimizationSettings) IsEmpty() bool {
	return !s.EnableBBR &&
		!s.EnableFQ &&
		!s.EnableTCPFastOpen &&
		!s.EnableXraySockopt &&
		!s.XrayTCPFastOpen &&
		strings.TrimSpace(s.XrayTCPCongestion) == ""
}

// ParseNetworkOptimizationSettings decodes persisted settings.
func ParseNetworkOptimizationSettings(raw string) NetworkOptimizationSettings {
	if strings.TrimSpace(raw) == "" {
		return NetworkOptimizationSettings{}
	}

	var settings NetworkOptimizationSettings
	if err := decodeJSON(raw, &settings); err != nil {
		return NetworkOptimizationSettings{}
	}
	return settings.Normalize()
}

// NetworkOptimizationInspectResult describes the current runtime state on a node.
type NetworkOptimizationInspectResult struct {
	KernelVersion               string   `json:"kernel_version"`
	CurrentCongestionControl    string   `json:"current_congestion_control"`
	AvailableCongestionControls []string `json:"available_congestion_controls"`
	DefaultQdisc                string   `json:"default_qdisc"`
	TCPFastOpen                 string   `json:"tcp_fastopen"`
	BBRAvailable                bool     `json:"bbr_available"`
	FQEnabled                   bool     `json:"fq_enabled"`
	XrayConfigPath              string   `json:"xray_config_path"`
	XrayConfigExists            bool     `json:"xray_config_exists"`
	BackupExists                bool     `json:"backup_exists"`
}

// NetworkOptimizationExecutionResult contains execution logs and the resulting state.
type NetworkOptimizationExecutionResult struct {
	AppliedSettings NetworkOptimizationSettings       `json:"applied_settings"`
	State           *NetworkOptimizationInspectResult `json:"state"`
	Log             string                            `json:"log"`
	BackupPath      string                            `json:"backup_path"`
}

func (s *RemoteDeployService) executeCommandOutput(ctx context.Context, client *ssh.Client, command string, timeout time.Duration) (string, string, error) {
	return s.runRemoteCommand(ctx, client, command, timeout)
}

// InspectNetworkOptimization connects to a node over SSH and returns its current optimization state.
func (s *RemoteDeployService) InspectNetworkOptimization(ctx context.Context, config *DeployConfig) (*NetworkOptimizationInspectResult, string, error) {
	if err := validateNetworkOptimizationConfig(config); err != nil {
		return nil, "", err
	}

	client, err := s.connectSSH(config)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	state, logOutput, err := s.inspectNetworkOptimizationState(ctx, client)
	if err != nil {
		return nil, logOutput, err
	}
	return state, logOutput, nil
}

// ApplyNetworkOptimization applies Linux network tuning to the target node.
func (s *RemoteDeployService) ApplyNetworkOptimization(ctx context.Context, config *DeployConfig, settings NetworkOptimizationSettings) (*NetworkOptimizationExecutionResult, error) {
	if err := validateNetworkOptimizationConfig(config); err != nil {
		return nil, err
	}

	client, err := s.connectSSH(config)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	normalized := settings.Normalize()
	logOutput, err := s.applyNetworkOptimizationState(ctx, client, normalized)
	if err != nil {
		return nil, err
	}

	state, inspectLog, inspectErr := s.inspectNetworkOptimizationState(ctx, client)
	if inspectErr == nil && inspectLog != "" {
		if logOutput != "" && !strings.HasSuffix(logOutput, "\n") {
			logOutput += "\n"
		}
		logOutput += inspectLog
	}

	return &NetworkOptimizationExecutionResult{
		AppliedSettings: normalized,
		State:           state,
		Log:             strings.TrimSpace(logOutput),
		BackupPath:      NetworkOptimizationBackupPath,
	}, inspectErr
}

// RollbackNetworkOptimization restores the node to the pre-optimization sysctl values.
func (s *RemoteDeployService) RollbackNetworkOptimization(ctx context.Context, config *DeployConfig) (*NetworkOptimizationExecutionResult, error) {
	if err := validateNetworkOptimizationConfig(config); err != nil {
		return nil, err
	}

	client, err := s.connectSSH(config)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	logOutput, err := s.rollbackNetworkOptimizationState(ctx, client)
	if err != nil {
		return nil, err
	}

	state, inspectLog, inspectErr := s.inspectNetworkOptimizationState(ctx, client)
	if inspectErr == nil && inspectLog != "" {
		if logOutput != "" && !strings.HasSuffix(logOutput, "\n") {
			logOutput += "\n"
		}
		logOutput += inspectLog
	}

	return &NetworkOptimizationExecutionResult{
		AppliedSettings: NetworkOptimizationSettings{},
		State:           state,
		Log:             strings.TrimSpace(logOutput),
		BackupPath:      NetworkOptimizationBackupPath,
	}, inspectErr
}

func (s *RemoteDeployService) inspectNetworkOptimizationState(ctx context.Context, client *ssh.Client) (*NetworkOptimizationInspectResult, string, error) {
	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	default:
	}

	stdout, stderr, err := s.executeCommandOutput(ctx, client, inspectNetworkOptimizationScript(), systemInfoCommandTimeout)
	combined := strings.TrimSpace(strings.Join(filterNonEmptyStrings(stdout, stderr), "\n"))
	if err != nil {
		return nil, combined, err
	}

	state := &NetworkOptimizationInspectResult{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "VPANEL_") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "VPANEL_KERNEL_VERSION":
			state.KernelVersion = value
		case "VPANEL_CURRENT_CONGESTION_CONTROL":
			state.CurrentCongestionControl = value
		case "VPANEL_AVAILABLE_CONGESTION_CONTROLS":
			if value != "" {
				state.AvailableCongestionControls = strings.Fields(value)
			}
		case "VPANEL_DEFAULT_QDISC":
			state.DefaultQdisc = value
		case "VPANEL_TCP_FASTOPEN":
			state.TCPFastOpen = value
		case "VPANEL_BBR_AVAILABLE":
			state.BBRAvailable = value == "true"
		case "VPANEL_FQ_ENABLED":
			state.FQEnabled = value == "true"
		case "VPANEL_XRAY_CONFIG_PATH":
			state.XrayConfigPath = value
		case "VPANEL_XRAY_CONFIG_EXISTS":
			state.XrayConfigExists = value == "true"
		case "VPANEL_BACKUP_EXISTS":
			state.BackupExists = value == "true"
		}
	}

	return state, combined, nil
}

func (s *RemoteDeployService) applyNetworkOptimizationState(ctx context.Context, client *ssh.Client, settings NetworkOptimizationSettings) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	enableBBR := 0
	if settings.EnableBBR {
		enableBBR = 1
	}
	enableFQ := 0
	if settings.EnableFQ {
		enableFQ = 1
	}
	enableTFO := 0
	if settings.EnableTCPFastOpen {
		enableTFO = 1
	}

	command := fmt.Sprintf(
		applyNetworkOptimizationScriptTemplate,
		NetworkOptimizationBackupPath,
		networkOptimizationSysctlPath,
		enableBBR,
		enableFQ,
		enableTFO,
	)
	stdout, stderr, err := s.executeCommandOutput(ctx, client, command, serviceCommandTimeout)
	combined := strings.TrimSpace(strings.Join(filterNonEmptyStrings(stdout, stderr), "\n"))
	if err != nil {
		return combined, err
	}
	return combined, nil
}

func (s *RemoteDeployService) rollbackNetworkOptimizationState(ctx context.Context, client *ssh.Client) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	stdout, stderr, err := s.executeCommandOutput(ctx, client, rollbackNetworkOptimizationScript(), serviceCommandTimeout)
	combined := strings.TrimSpace(strings.Join(filterNonEmptyStrings(stdout, stderr), "\n"))
	if err != nil {
		return combined, err
	}
	return combined, nil
}

func filterNonEmptyStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func validateNetworkOptimizationConfig(config *DeployConfig) error {
	if config == nil {
		return fmt.Errorf("SSH 配置不能为空")
	}
	if strings.TrimSpace(config.Host) == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	if strings.TrimSpace(config.Username) == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if strings.TrimSpace(config.Password) == "" && strings.TrimSpace(config.PrivateKey) == "" {
		return fmt.Errorf("必须提供密码或私钥")
	}
	return nil
}

func inspectNetworkOptimizationScript() string {
	return fmt.Sprintf(`
set -eu

if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
  SUDO="sudo"
else
  SUDO=""
fi

if command -v modprobe >/dev/null 2>&1; then
  $SUDO modprobe tcp_bbr >/dev/null 2>&1 || true
fi

CURRENT_CC=$($SUDO sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null || echo "")
AVAILABLE_CC=$($SUDO sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null || echo "")
DEFAULT_QDISC=$($SUDO sysctl -n net.core.default_qdisc 2>/dev/null || echo "")
TCP_FASTOPEN=$($SUDO sysctl -n net.ipv4.tcp_fastopen 2>/dev/null || echo "")
KERNEL_VERSION=$(uname -r 2>/dev/null || echo "")
BACKUP_EXISTS=false
if [ -f %q ]; then
  BACKUP_EXISTS=true
fi
BBR_AVAILABLE=false
if echo " ${AVAILABLE_CC} " | grep -q " bbr "; then
  BBR_AVAILABLE=true
fi
FQ_ENABLED=false
if [ "$DEFAULT_QDISC" = "fq" ]; then
  FQ_ENABLED=true
fi

XRAY_CONFIG_PATH=""
if [ -f /etc/vpanel/agent.yaml ]; then
  XRAY_CONFIG_PATH=$(awk -F': ' '/config_path:/ {gsub(/"/, "", $2); print $2; exit}' /etc/vpanel/agent.yaml 2>/dev/null || true)
fi
if [ -z "$XRAY_CONFIG_PATH" ]; then
  for candidate in /usr/local/etc/xray/config.json /etc/xray/config.json; do
    if [ -f "$candidate" ]; then
      XRAY_CONFIG_PATH="$candidate"
      break
    fi
  done
fi

XRAY_CONFIG_EXISTS=false
if [ -n "$XRAY_CONFIG_PATH" ] && [ -f "$XRAY_CONFIG_PATH" ]; then
  XRAY_CONFIG_EXISTS=true
fi

echo "VPANEL_KERNEL_VERSION=${KERNEL_VERSION}"
echo "VPANEL_CURRENT_CONGESTION_CONTROL=${CURRENT_CC}"
echo "VPANEL_AVAILABLE_CONGESTION_CONTROLS=${AVAILABLE_CC}"
echo "VPANEL_DEFAULT_QDISC=${DEFAULT_QDISC}"
echo "VPANEL_TCP_FASTOPEN=${TCP_FASTOPEN}"
echo "VPANEL_BBR_AVAILABLE=${BBR_AVAILABLE}"
echo "VPANEL_FQ_ENABLED=${FQ_ENABLED}"
echo "VPANEL_XRAY_CONFIG_PATH=${XRAY_CONFIG_PATH}"
echo "VPANEL_XRAY_CONFIG_EXISTS=${XRAY_CONFIG_EXISTS}"
echo "VPANEL_BACKUP_EXISTS=${BACKUP_EXISTS}"
`, NetworkOptimizationBackupPath)
}

const applyNetworkOptimizationScriptTemplate = `
set -eu

if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  else
    echo "当前用户没有 root 权限，且未安装 sudo"
    exit 1
  fi
else
  SUDO=""
fi

BACKUP_FILE=%q
CONF_FILE=%q
ENABLE_BBR=%d
ENABLE_FQ=%d
ENABLE_TFO=%d

mkdir -p /etc/vpanel /etc/sysctl.d

CURRENT_CC=$($SUDO sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null || echo "")
AVAILABLE_CC=$($SUDO sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null || echo "")
DEFAULT_QDISC=$($SUDO sysctl -n net.core.default_qdisc 2>/dev/null || echo "")
TCP_FASTOPEN=$($SUDO sysctl -n net.ipv4.tcp_fastopen 2>/dev/null || echo "")

if [ ! -f "$BACKUP_FILE" ]; then
  TMP_BACKUP=$(mktemp)
  cat > "$TMP_BACKUP" <<EOF
ORIGINAL_CONGESTION_CONTROL=${CURRENT_CC}
ORIGINAL_DEFAULT_QDISC=${DEFAULT_QDISC}
ORIGINAL_TCP_FASTOPEN=${TCP_FASTOPEN}
EOF
  $SUDO mv "$TMP_BACKUP" "$BACKUP_FILE"
  $SUDO chmod 600 "$BACKUP_FILE"
  echo "已创建网络优化备份: $BACKUP_FILE"
fi

if [ -f "$BACKUP_FILE" ]; then
  . "$BACKUP_FILE"
fi

TARGET_QDISC="$DEFAULT_QDISC"
if [ "$ENABLE_FQ" = "1" ]; then
  TARGET_QDISC="fq"
elif [ -n "${ORIGINAL_DEFAULT_QDISC:-}" ]; then
  TARGET_QDISC="${ORIGINAL_DEFAULT_QDISC}"
fi
if [ -z "$TARGET_QDISC" ]; then
  TARGET_QDISC="fq_codel"
fi

TARGET_CC="$CURRENT_CC"
if [ "$ENABLE_BBR" = "1" ]; then
  if command -v modprobe >/dev/null 2>&1; then
    $SUDO modprobe tcp_bbr >/dev/null 2>&1 || true
  fi
  AVAILABLE_CC=$($SUDO sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null || echo "$AVAILABLE_CC")
  if ! echo " ${AVAILABLE_CC} " | grep -q " bbr "; then
    echo "当前内核不支持 BBR，请升级内核或加载 tcp_bbr 模块"
    exit 1
  fi
  TARGET_CC="bbr"
elif [ -n "${ORIGINAL_CONGESTION_CONTROL:-}" ]; then
  TARGET_CC="${ORIGINAL_CONGESTION_CONTROL}"
elif echo " ${AVAILABLE_CC} " | grep -q " cubic "; then
  TARGET_CC="cubic"
fi
if [ -z "$TARGET_CC" ]; then
  TARGET_CC="$CURRENT_CC"
fi

TARGET_TFO="$TCP_FASTOPEN"
if [ "$ENABLE_TFO" = "1" ]; then
  TARGET_TFO="3"
elif [ -n "${ORIGINAL_TCP_FASTOPEN:-}" ]; then
  TARGET_TFO="${ORIGINAL_TCP_FASTOPEN}"
else
  TARGET_TFO="0"
fi

TMP_CONF=$(mktemp)
cat > "$TMP_CONF" <<EOF
# Managed by VPanel node network optimization
net.core.default_qdisc = ${TARGET_QDISC}
net.ipv4.tcp_congestion_control = ${TARGET_CC}
net.ipv4.tcp_fastopen = ${TARGET_TFO}
EOF

$SUDO mv "$TMP_CONF" "$CONF_FILE"
$SUDO chmod 644 "$CONF_FILE"
$SUDO sysctl -w net.core.default_qdisc="${TARGET_QDISC}" >/dev/null
$SUDO sysctl -w net.ipv4.tcp_congestion_control="${TARGET_CC}" >/dev/null
$SUDO sysctl -w net.ipv4.tcp_fastopen="${TARGET_TFO}" >/dev/null

echo "已应用节点网络优化"
echo "default_qdisc=${TARGET_QDISC}"
echo "tcp_congestion_control=${TARGET_CC}"
echo "tcp_fastopen=${TARGET_TFO}"
`

func rollbackNetworkOptimizationScript() string {
	return fmt.Sprintf(`
set -eu

if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  else
    echo "当前用户没有 root 权限，且未安装 sudo"
    exit 1
  fi
else
  SUDO=""
fi

BACKUP_FILE=%q
CONF_FILE=%q

if [ ! -f "$BACKUP_FILE" ]; then
  echo "未找到网络优化备份文件: $BACKUP_FILE"
  exit 1
fi

. "$BACKUP_FILE"

AVAILABLE_CC=$($SUDO sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null || echo "")
TARGET_CC="${ORIGINAL_CONGESTION_CONTROL:-}"
if [ -z "$TARGET_CC" ]; then
  if echo " ${AVAILABLE_CC} " | grep -q " cubic "; then
    TARGET_CC="cubic"
  else
    TARGET_CC=$($SUDO sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null || echo "")
  fi
fi

TARGET_QDISC="${ORIGINAL_DEFAULT_QDISC:-fq_codel}"
TARGET_TFO="${ORIGINAL_TCP_FASTOPEN:-0}"

$SUDO rm -f "$CONF_FILE"
$SUDO sysctl -w net.core.default_qdisc="${TARGET_QDISC}" >/dev/null
$SUDO sysctl -w net.ipv4.tcp_congestion_control="${TARGET_CC}" >/dev/null
$SUDO sysctl -w net.ipv4.tcp_fastopen="${TARGET_TFO}" >/dev/null
$SUDO rm -f "$BACKUP_FILE"

echo "已回滚节点网络优化"
echo "default_qdisc=${TARGET_QDISC}"
echo "tcp_congestion_control=${TARGET_CC}"
echo "tcp_fastopen=${TARGET_TFO}"
`, NetworkOptimizationBackupPath, networkOptimizationSysctlPath)
}

func decodeJSON(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
