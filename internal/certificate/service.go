// Package certificate provides TLS certificate management functionality.
package certificate

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"v/internal/database/repository"
	"v/internal/logger"
	nodepkg "v/internal/node"
	apperrors "v/pkg/errors"
)

// Input validation patterns
var (
	// Email validation (RFC 5322 simplified)
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	// Domain validation (supports wildcards)
	domainRegex = regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	// DNS provider validation (alphanumeric and underscore only)
	dnsProviderRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	// Environment variable key validation. POSIX env names are uppercase,
	// but acme.sh's DNS-API plugins use mixed case by convention
	// (CF_Token, CF_Zone_ID, Ali_Key, DP_Id, etc.). Accept letters of either
	// case plus digits and underscores; still rejects shell metacharacters
	// since invalid characters cannot pass through here into the exec env.
	envKeyRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// sanitizeDomainForPath sanitizes domain name for safe use in file paths
// Prevents path traversal attacks
func sanitizeDomainForPath(domain string) (string, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	rawDomain := domain

	// Validate domain format
	if err := validateDomain(domain); err != nil {
		return "", err
	}

	// Use a literal-safe directory for wildcard certificates. The previous
	// implementation stripped "*.", which made *.example.com share the same
	// directory as example.com and could overwrite the wrong certificate.
	if strings.HasPrefix(rawDomain, "*.") {
		domain = "_wildcard_." + strings.TrimPrefix(rawDomain, "*.")
	}

	// Clean the path to remove any .. or . components
	cleaned := filepath.Clean(domain)

	// Ensure the cleaned path doesn't contain path separators
	// (which would indicate an attempt to escape the directory)
	if strings.Contains(cleaned, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid domain: contains path separators")
	}

	// Ensure the cleaned path is the same as the original
	// (prevents attempts to use . or .. in domain names)
	if cleaned != domain {
		return "", fmt.Errorf("invalid domain: path traversal attempt detected")
	}

	return cleaned, nil
}

func certificatePathWithinBase(path string, base string) bool {
	cleanBase := filepath.Clean(base)
	cleanPath := filepath.Clean(path)
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func normalizePEMData(data []byte) []byte {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	if data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data
}

func parseFirstCertificate(certData []byte) (*x509.Certificate, error) {
	rest := certData
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("未找到 CERTIFICATE 块")
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("证书解析失败: %w", err)
		}
		return cert, nil
	}
}

func parsePrivateKeyPublic(keyData []byte) (crypto.PublicKey, error) {
	rest := keyData
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("未找到私钥 PEM 块")
		}
		rest = remaining
		if !strings.Contains(block.Type, "PRIVATE KEY") {
			continue
		}

		switch block.Type {
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("RSA 私钥解析失败: %w", err)
			}
			return key.Public(), nil
		case "EC PRIVATE KEY":
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("EC 私钥解析失败: %w", err)
			}
			return key.Public(), nil
		default:
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("PKCS#8 私钥解析失败: %w", err)
			}
			signer, ok := key.(crypto.Signer)
			if !ok {
				return nil, fmt.Errorf("不支持的私钥类型 %T", key)
			}
			return signer.Public(), nil
		}
	}
}

func publicKeysMatch(certPublicKey crypto.PublicKey, keyPublicKey crypto.PublicKey) bool {
	certDER, certErr := x509.MarshalPKIXPublicKey(certPublicKey)
	keyDER, keyErr := x509.MarshalPKIXPublicKey(keyPublicKey)
	return certErr == nil && keyErr == nil && bytes.Equal(certDER, keyDER)
}

func certificateCoversDomain(cert *x509.Certificate, domain string) error {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	if strings.HasPrefix(domain, "*.") {
		baseDomain := strings.TrimPrefix(domain, "*.")
		probeDomain := "vpanel-check." + baseDomain
		if err := cert.VerifyHostname(probeDomain); err == nil {
			return nil
		}
		for _, dnsName := range cert.DNSNames {
			if strings.EqualFold(dnsName, domain) {
				return nil
			}
		}
		if strings.EqualFold(cert.Subject.CommonName, domain) {
			return nil
		}
		return fmt.Errorf("证书不包含泛域名 %s", domain)
	}

	if err := cert.VerifyHostname(domain); err == nil {
		return nil
	}
	if strings.EqualFold(cert.Subject.CommonName, domain) {
		return nil
	}
	return fmt.Errorf("证书不包含域名 %s", domain)
}

func validateCertificateMaterial(domain string, certData []byte, keyData []byte) (*x509.Certificate, []byte, []byte, error) {
	certData = normalizePEMData(certData)
	keyData = normalizePEMData(keyData)
	if len(certData) == 0 {
		return nil, nil, nil, apperrors.NewBadRequestError("证书内容不能为空")
	}
	if len(keyData) == 0 {
		return nil, nil, nil, apperrors.NewBadRequestError("私钥内容不能为空")
	}

	cert, err := parseFirstCertificate(certData)
	if err != nil {
		return nil, nil, nil, apperrors.NewBadRequestError(err.Error())
	}

	now := time.Now()
	if !cert.NotAfter.After(now) {
		return nil, nil, nil, apperrors.NewBadRequestError(fmt.Sprintf("证书已过期，过期时间: %s", cert.NotAfter.UTC().Format(time.RFC3339)))
	}
	if cert.NotBefore.After(now.Add(5 * time.Minute)) {
		return nil, nil, nil, apperrors.NewBadRequestError(fmt.Sprintf("证书尚未生效，生效时间: %s", cert.NotBefore.UTC().Format(time.RFC3339)))
	}
	if err := certificateCoversDomain(cert, domain); err != nil {
		return nil, nil, nil, apperrors.NewBadRequestError(err.Error())
	}

	keyPublicKey, err := parsePrivateKeyPublic(keyData)
	if err != nil {
		return nil, nil, nil, apperrors.NewBadRequestError(err.Error())
	}
	if !publicKeysMatch(cert.PublicKey, keyPublicKey) {
		return nil, nil, nil, apperrors.NewBadRequestError("证书与私钥不匹配")
	}

	return cert, certData, keyData, nil
}

func (s *Service) certificateStoragePaths(domain string) (string, string, error) {
	safeDomain, err := sanitizeDomainForPath(domain)
	if err != nil {
		return "", "", fmt.Errorf("invalid domain for path: %w", err)
	}

	certDir := filepath.Join(s.certDir, safeDomain)
	if !certificatePathWithinBase(certDir, s.certDir) {
		return "", "", fmt.Errorf("path traversal attempt detected")
	}
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return "", "", fmt.Errorf("创建证书目录失败: %w", err)
	}

	return filepath.Join(certDir, "fullchain.pem"), filepath.Join(certDir, "privkey.pem"), nil
}

func (s *Service) writeCertificateFiles(domain string, certData, keyData []byte) (string, string, error) {
	certFile, keyFile, err := s.certificateStoragePaths(domain)
	if err != nil {
		return "", "", err
	}

	if err := os.WriteFile(certFile, certData, 0644); err != nil {
		return "", "", fmt.Errorf("保存证书文件失败: %w", err)
	}
	if err := os.Chmod(certFile, 0644); err != nil {
		s.logger.Warn("设置证书文件权限失败", logger.Err(err))
	}

	if err := os.WriteFile(keyFile, keyData, 0600); err != nil {
		return "", "", fmt.Errorf("保存私钥文件失败: %w", err)
	}
	if err := os.Chmod(keyFile, 0600); err != nil {
		s.logger.Warn("设置私钥文件权限失败", logger.Err(err))
	}

	return certFile, keyFile, nil
}

func readStoredCertificateMaterial(cert *repository.Certificate) ([]byte, []byte, error) {
	var certData []byte
	if strings.TrimSpace(cert.Certificate) != "" {
		certData = []byte(cert.Certificate)
	} else if strings.TrimSpace(cert.CertPath) != "" {
		data, err := os.ReadFile(cert.CertPath)
		if err != nil {
			return nil, nil, fmt.Errorf("读取证书文件失败: %w", err)
		}
		certData = data
	}

	var keyData []byte
	if strings.TrimSpace(cert.PrivateKey) != "" {
		keyData = []byte(cert.PrivateKey)
	} else if strings.TrimSpace(cert.KeyPath) != "" {
		data, err := os.ReadFile(cert.KeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("读取私钥文件失败: %w", err)
		}
		keyData = data
	}

	if len(bytes.TrimSpace(certData)) == 0 {
		return nil, nil, fmt.Errorf("证书内容为空")
	}
	if len(bytes.TrimSpace(keyData)) == 0 {
		return nil, nil, fmt.Errorf("私钥内容为空")
	}
	return certData, keyData, nil
}

func setCertificateMaterialFields(cert *repository.Certificate, parsed *x509.Certificate, certPath string, keyPath string) {
	now := time.Now()
	notAfter := parsed.NotAfter.UTC()

	cert.CertPath = certPath
	cert.KeyPath = keyPath
	cert.Certificate = ""
	cert.PrivateKey = ""
	cert.IssueDate = &now
	cert.ExpireDate = &notAfter
	cert.ExpiresAt = notAfter
	cert.Status = "active"
	cert.ErrorMessage = ""
}

// validateEmail validates and sanitizes email addresses
func validateEmail(email string) error {
	if email == "" {
		return nil // Empty email is allowed
	}
	if len(email) > 254 {
		return fmt.Errorf("email too long (max 254 characters)")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	// Check for shell metacharacters
	if strings.ContainsAny(email, ";|&$`<>(){}[]!*?~") {
		return fmt.Errorf("email contains invalid characters")
	}
	return nil
}

// validateDomain validates domain names
func validateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if len(domain) > 253 {
		return fmt.Errorf("domain too long (max 253 characters)")
	}
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain format")
	}
	return nil
}

// validateDNSProvider validates DNS provider names
func validateDNSProvider(provider string) error {
	if provider == "" {
		return nil
	}
	if !dnsProviderRegex.MatchString(provider) {
		return fmt.Errorf("invalid DNS provider format (alphanumeric and underscore only)")
	}
	if len(provider) > 50 {
		return fmt.Errorf("DNS provider name too long")
	}
	return nil
}

// validateEnvVars validates environment variable keys and values
func validateEnvVars(envVars map[string]string) error {
	for key, value := range envVars {
		if !envKeyRegex.MatchString(key) {
			return fmt.Errorf("invalid environment variable key: %s (must start with a letter, followed by letters, digits, or underscores)", key)
		}
		if len(key) > 100 {
			return fmt.Errorf("environment variable key too long: %s", key)
		}
		if len(value) > 1000 {
			return fmt.Errorf("environment variable value too long for key: %s", key)
		}
		// Check for shell metacharacters in values
		if strings.ContainsAny(value, ";|&$`<>(){}[]!*?~\n\r") {
			return fmt.Errorf("environment variable value contains invalid characters: %s", key)
		}
	}
	return nil
}

// Service provides certificate management operations.
type Service struct {
	certRepo       repository.CertificateRepository
	nodeRepo       repository.NodeRepository
	deploymentRepo repository.CertificateDeploymentRepository
	logger         logger.Logger
	certDir        string // 证书存储目录

	// 自动续期控制
	renewCtx    context.Context
	renewCancel context.CancelFunc
	renewWg     sync.WaitGroup

	// acme.sh 安装锁
	installMu sync.Mutex
}

// NewService creates a new certificate service.
func NewService(
	certRepo repository.CertificateRepository,
	nodeRepo repository.NodeRepository,
	deploymentRepo repository.CertificateDeploymentRepository,
	log logger.Logger,
	certDir string,
) *Service {
	return &Service{
		certRepo:       certRepo,
		nodeRepo:       nodeRepo,
		deploymentRepo: deploymentRepo,
		logger:         log,
		certDir:        certDir,
	}
}

// ApplyRequest represents a certificate application request.
type ApplyRequest struct {
	Domain      string
	Email       string
	Provider    string            // "letsencrypt" or "zerossl"
	Method      string            // "http" or "dns"
	DNSProvider string            // DNS provider for dns method, e.g., "dns_cf" for Cloudflare
	Webroot     string            // Webroot path for http method
	DNSEnv      map[string]string // DNS API credentials (e.g., CF_Token, CF_Account_ID)
	Wildcard    bool              // 是否申请泛域名证书
	AutoRenew   *bool             // 是否自动续期（nil 表示默认开启）
	NodeIDs     []int64           // 签发成功后自动关联的节点
}

// Apply applies for a new certificate using acme.sh.
func (s *Service) Apply(ctx context.Context, req *ApplyRequest) (*repository.Certificate, error) {
	s.logger.Info("申请证书",
		logger.F("domain", req.Domain),
		logger.F("provider", req.Provider),
		logger.F("method", req.Method))

	// SECURITY: Validate all user inputs to prevent command injection
	if err := validateDomain(req.Domain); err != nil {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("invalid domain: %v", err))
	}
	if err := validateEmail(req.Email); err != nil {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("invalid email: %v", err))
	}
	if err := validateDNSProvider(req.DNSProvider); err != nil {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("invalid DNS provider: %v", err))
	}
	if err := validateEnvVars(req.DNSEnv); err != nil {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("invalid DNS credentials: %v", err))
	}

	// 设置默认值
	if req.Provider == "" {
		req.Provider = "letsencrypt"
	}
	if req.Method == "" {
		req.Method = "http"
	}

	// Validate provider
	if req.Provider != "letsencrypt" && req.Provider != "zerossl" {
		return nil, apperrors.NewBadRequestError("provider must be 'letsencrypt' or 'zerossl'")
	}

	// Validate method
	if req.Method != "http" && req.Method != "dns" {
		return nil, apperrors.NewBadRequestError("method must be 'http' or 'dns'")
	}

	// 检查通配符域名
	if strings.HasPrefix(req.Domain, "*.") {
		s.logger.Info("检测到通配符域名，将使用 DNS 验证", logger.F("domain", req.Domain))
		req.Wildcard = true
		// 通配符域名必须使用 DNS 验证
		if req.Method != "dns" {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf("通配符域名（%s）只能使用 DNS 验证方式", req.Domain))
		}
	} else if req.Wildcard {
		// 如果用户勾选了泛域名选项，但域名不是 *. 开头，自动添加
		if !strings.HasPrefix(req.Domain, "*.") {
			req.Domain = "*." + req.Domain
			s.logger.Info("自动转换为泛域名", logger.F("domain", req.Domain))
		}
		// 泛域名必须使用 DNS 验证
		if req.Method != "dns" {
			return nil, apperrors.NewBadRequestError("泛域名证书只能使用 DNS 验证方式")
		}
	}

	// 验证请求参数
	if req.Method == "dns" {
		if req.DNSProvider == "" {
			return nil, apperrors.NewBadRequestError("DNS 验证方式需要指定 DNS 提供商，如: dns_cf (Cloudflare)")
		}
		// 检查 DNS API 凭证
		if len(req.DNSEnv) == 0 {
			return nil, apperrors.NewBadRequestError("DNS 验证方式需要提供 API 凭证")
		}
	}
	if req.Method == "http" {
		// 通配符域名只能使用 DNS 验证
		if strings.HasPrefix(req.Domain, "*.") || req.Wildcard {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf("通配符域名（%s）只能使用 DNS 验证方式", req.Domain))
		}
		if req.Webroot == "" {
			// 使用绝对路径，与前端保持一致
			req.Webroot = "/app/data/webroot"
		}
		// 检查 webroot 目录是否存在，如果不存在则创建
		if _, err := os.Stat(req.Webroot); os.IsNotExist(err) {
			s.logger.Warn("webroot 目录不存在，尝试创建",
				logger.F("webroot", req.Webroot))
			if err := os.MkdirAll(req.Webroot, 0755); err != nil {
				return nil, fmt.Errorf("创建 webroot 目录失败: %s, 错误: %w", req.Webroot, err)
			}
			s.logger.Info("webroot 目录创建成功", logger.F("webroot", req.Webroot))
		}
	}

	// 验证域名格式
	if !s.isValidDomain(req.Domain) {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("无效的域名格式: %s", req.Domain))
	}

	// 检查域名是否已存在证书
	existingCert, err := s.certRepo.GetByDomain(ctx, req.Domain)
	if err != nil && !apperrors.IsNotFound(err) {
		return nil, fmt.Errorf("查询证书记录失败: %w", err)
	}
	if err == nil && existingCert != nil {
		// 如果证书状态是 pending，不允许重复申请
		if existingCert.Status == "pending" {
			return nil, apperrors.New(apperrors.ErrCodeConflict, "该域名的证书正在申请中，请稍后再试")
		}
		// 如果证书状态是 active，提示用户
		if existingCert.Status == "active" {
			s.logger.Warn("域名已有有效证书，将覆盖", logger.F("domain", req.Domain))
		}
	}

	// HTTP 验证方式需要检查域名解析
	if req.Method == "http" {
		if err := s.checkDomainResolution(req.Domain); err != nil {
			s.logger.Warn("域名解析检查失败",
				logger.F("domain", req.Domain),
				logger.F("error", err.Error()))
			// 不阻止申请，只是警告
		}
	}

	// 创建证书记录
	autoRenew := true
	if req.AutoRenew != nil {
		autoRenew = *req.AutoRenew
	}

	estimatedExpire := time.Now().AddDate(0, 3, 0) // ACME 证书通常 90 天有效期
	var cert *repository.Certificate
	previousStatus := ""
	if existingCert != nil {
		// 复用现有记录，支持 failed/expired/active 状态下重新申请
		cert = existingCert
		previousStatus = existingCert.Status
		cert.Provider = req.Provider
		cert.AutoRenew = autoRenew
		// 对于已有有效证书，重签发失败时应保持可用状态
		if existingCert.Status == "active" {
			cert.Status = "active"
		} else {
			cert.Status = "pending"
		}
		cert.ErrorMessage = ""
		cert.ExpiresAt = estimatedExpire
		cert.ExpireDate = &estimatedExpire
		if err := s.certRepo.Update(ctx, cert); err != nil {
			return nil, fmt.Errorf("更新证书记录失败: %w", err)
		}
	} else {
		cert = &repository.Certificate{
			Domain:     req.Domain,
			Provider:   req.Provider,
			AutoRenew:  autoRenew,
			Status:     "pending",
			ExpiresAt:  estimatedExpire,
			ExpireDate: &estimatedExpire,
		}

		if err := s.certRepo.Create(ctx, cert); err != nil {
			if apperrors.IsConflict(err) {
				return nil, apperrors.New(apperrors.ErrCodeConflict, "该域名的证书已存在，请刷新后重试")
			}
			return nil, fmt.Errorf("创建证书记录失败: %w", err)
		}
	}

	// 异步申请证书。**必须** detach 到 context.Background()：原 ctx 是 HTTP
	// request 的 context，handler 返回 202 后立即被 cancel，acme.sh 还没来得
	// 及跑就因 "context canceled" 失败（出现在每次 retry 的几毫秒内）。
	go func() {
		applyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := s.ensureAcmeInstalled(applyCtx, req.Email); err != nil {
			s.logger.Error("准备 ACME 环境失败",
				logger.F("domain", req.Domain),
				logger.Err(err))
			s.updateApplyFailure(cert, previousStatus, err)
			return
		}

		// 跳过测试阶段，直接正式申请
		s.logger.Info("开始正式申请证书（跳过测试阶段）", logger.F("domain", req.Domain))

		// 正式申请（带重试）
		var lastErr error
		for i := 0; i < 3; i++ {
			if i > 0 {
				s.logger.Info("重试证书申请",
					logger.F("domain", req.Domain),
					logger.F("attempt", i+1))
				time.Sleep(time.Duration(i*5) * time.Second) // 递增延迟
			}

			if err := s.issueWithAcme(applyCtx, req, cert); err != nil {
				lastErr = err
				s.logger.Warn("证书申请失败",
					logger.F("domain", req.Domain),
					logger.F("attempt", i+1),
					logger.F("error", err.Error()))
				if !s.isRetryableApplyError(err) {
					s.logger.Info("检测到不可重试错误，停止重试",
						logger.F("domain", req.Domain),
						logger.F("attempt", i+1),
						logger.F("error", err.Error()))
					break
				}
				continue
			}

			// 成功
			if len(req.NodeIDs) > 0 {
				postIssueCtx, postIssueCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if err := s.assignCertificateToNodes(postIssueCtx, cert.ID, cert.Domain, req.NodeIDs); err != nil {
					s.logger.Warn("证书签发后自动关联节点失败",
						logger.F("domain", cert.Domain),
						logger.F("cert_id", cert.ID),
						logger.Err(err))
				} else if err := s.DeployToAssignedNodes(postIssueCtx, cert.ID); err != nil {
					s.logger.Warn("证书签发后自动部署到节点失败",
						logger.F("domain", cert.Domain),
						logger.F("cert_id", cert.ID),
						logger.Err(err))
				}
				postIssueCancel()
			}
			return
		}

		// 所有重试都失败
		s.logger.Error("证书申请最终失败",
			logger.F("domain", req.Domain),
			logger.F("error", lastErr.Error()))

		s.updateApplyFailure(cert, previousStatus, lastErr)
	}()

	return cert, nil
}

// isAcmeInstalled checks if acme.sh is installed.
func normalizeNodeTLSDomainFromCertificate(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if strings.HasPrefix(domain, "*.") {
		return ""
	}
	return domain
}

func (s *Service) assignCertificateToNodes(ctx context.Context, certID int64, certDomain string, nodeIDs []int64) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	autoFillDomain := normalizeNodeTLSDomainFromCertificate(certDomain)
	seen := make(map[int64]struct{}, len(nodeIDs))
	errorsList := make([]string, 0)
	successCount := 0

	for _, nodeID := range nodeIDs {
		if nodeID <= 0 {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}

		node, err := s.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("节点 %d 不存在", nodeID))
			continue
		}

		node.CertificateID = &certID
		if autoFillDomain != "" && strings.TrimSpace(node.TLSDomain) == "" {
			node.TLSDomain = autoFillDomain
		}

		if err := s.nodeRepo.Update(ctx, node); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("节点 %d 更新失败", nodeID))
			continue
		}

		successCount++
		s.logger.Info("证书已自动关联到节点",
			logger.F("cert_id", certID),
			logger.F("domain", certDomain),
			logger.F("node_id", nodeID),
			logger.F("node_name", node.Name))
	}

	if successCount == 0 && len(errorsList) > 0 {
		return fmt.Errorf("%s", strings.Join(errorsList, "; "))
	}
	if len(errorsList) > 0 {
		return fmt.Errorf("部分节点关联失败: %s", strings.Join(errorsList, "; "))
	}
	return nil
}

func (s *Service) isAcmeInstalled() bool {
	_, found := s.findAcmePath(true)
	return found
}

func (s *Service) findAcmePath(logMissing bool) (string, bool) {
	// 尝试多个可能的路径
	possiblePaths := []string{
		"/root/.acme.sh/acme.sh",
		"/home/app/.acme.sh/acme.sh",
	}

	// 也尝试从用户主目录获取
	if homeDir, err := os.UserHomeDir(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(homeDir, ".acme.sh", "acme.sh"))
	}

	// 检查所有可能的路径
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			s.logger.Info("找到 acme.sh", logger.F("path", path))
			return path, true
		}
	}

	if logMissing {
		s.logger.Warn("未找到 acme.sh，尝试的路径", logger.F("paths", possiblePaths))
	}

	return "", false
}

func (s *Service) ensureAcmeInstalled(ctx context.Context, email string) error {
	if _, found := s.findAcmePath(false); found {
		return nil
	}

	s.logger.Info("acme.sh 未安装，开始自动安装")

	s.installMu.Lock()
	defer s.installMu.Unlock()

	if _, found := s.findAcmePath(false); found {
		s.logger.Info("acme.sh 已被其他进程安装")
		return nil
	}

	installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if err := s.installAcme(installCtx, email); err != nil {
		s.logger.Error("acme.sh 自动安装失败", logger.Err(err))
		return fmt.Errorf("acme.sh 安装失败: %w", err)
	}

	s.logger.Info("acme.sh 安装成功")
	return nil
}

// installAcme automatically installs acme.sh.
func (s *Service) installAcme(ctx context.Context, email string) error {
	s.logger.Info("开始安装 acme.sh")

	// SECURITY: Validate email to prevent command injection
	if email != "" {
		if err := validateEmail(email); err != nil {
			return fmt.Errorf("invalid email for acme.sh installation: %w", err)
		}
	}

	installScript := filepath.Join(os.TempDir(), fmt.Sprintf("acme-install-%d.sh", time.Now().UnixNano()))
	defer os.Remove(installScript)

	if err := s.downloadAcmeInstallScript(ctx, "https://get.acme.sh", installScript); err != nil {
		s.logger.Error("acme.sh 安装脚本下载失败", logger.F("error", err.Error()))
		return fmt.Errorf("下载安装脚本失败: %w", err)
	}

	// SECURITY: Pass email as separate argument, not embedded in string
	installArgs := []string{installScript}
	if email != "" {
		// Email is validated above, safe to use
		installArgs = append(installArgs, fmt.Sprintf("email=%s", email))
	}

	installCmd := exec.CommandContext(ctx, "sh", installArgs...)
	installCmd.Env = os.Environ()
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		installCmd.Env = append(installCmd.Env, fmt.Sprintf("HOME=%s", homeDir))
	}
	installOutput, err := installCmd.CombinedOutput()
	if err != nil {
		s.logger.Error("acme.sh 安装失败",
			logger.F("error", err.Error()),
			logger.F("output", string(installOutput)))
		return fmt.Errorf("安装失败: %w", err)
	}

	s.logger.Info("acme.sh 安装完成")

	// 验证安装
	if _, found := s.findAcmePath(true); !found {
		return fmt.Errorf("安装完成但无法找到 acme.sh")
	}

	return nil
}

func (s *Service) downloadAcmeInstallScript(ctx context.Context, downloadURL, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return nil
}

func (s *Service) updateApplyFailure(cert *repository.Certificate, previousStatus string, err error) {
	parsedErr := s.parseAcmeError(err)
	if previousStatus == "active" {
		cert.Status = "active"
		cert.ErrorMessage = fmt.Sprintf("最近一次重签发失败: %s", parsedErr)
	} else {
		cert.Status = "failed"
		cert.ErrorMessage = parsedErr
	}

	if updateErr := s.certRepo.Update(context.Background(), cert); updateErr != nil {
		s.logger.Error("更新证书失败状态失败",
			logger.F("domain", cert.Domain),
			logger.Err(updateErr))
	}
}

// updateAcmeAccount updates the acme.sh account email.
func (s *Service) updateAcmeAccount(ctx context.Context, email string) error {
	if email == "" {
		return nil // 如果没有提供邮箱，跳过更新
	}

	// SECURITY: Validate email to prevent command injection
	if err := validateEmail(email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	acmePath := filepath.Join(homeDir, ".acme.sh", "acme.sh")

	// 更新账户邮箱 - email is now validated, safe to use
	cmd := exec.CommandContext(ctx, acmePath, "--update-account", "--accountemail", email)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Warn("更新 acme.sh 账户邮箱失败",
			logger.F("error", err.Error()),
			logger.F("output", string(output)))
		// 不返回错误，因为可能账户还未注册
		return nil
	}

	s.logger.Info("acme.sh 账户邮箱已更新", logger.F("email", email))
	return nil
}

// isValidDomain validates domain name format.
func (s *Service) isValidDomain(domain string) bool {
	// 支持通配符域名
	if strings.HasPrefix(domain, "*.") {
		domain = domain[2:] // 移除 *. 前缀
	}

	// 域名正则表达式
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(domain)
}

// checkDomainResolution checks if domain resolves to an IP address.
func (s *Service) checkDomainResolution(domain string) error {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return fmt.Errorf("域名解析失败: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("域名未解析到任何 IP 地址")
	}

	s.logger.Info("域名解析成功",
		logger.F("domain", domain),
		logger.F("ips", ips))

	return nil
}

// parseAcmeError parses acme.sh error output and returns user-friendly message.
func (s *Service) parseAcmeError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// 常见错误模式匹配
	patterns := map[string]string{
		"timeout":                              "申请超时，请检查网络连接",
		"acme.sh 安装失败":                         "acme.sh 自动安装失败，请检查服务器网络、HOME 目录权限或手动预装后重试",
		"下载安装脚本失败":                             "acme.sh 安装脚本下载失败，请检查服务器是否可访问 https://get.acme.sh",
		"安装完成但无法找到 acme.sh":                    "acme.sh 安装后未找到可执行文件，请检查 HOME 目录权限或手动安装",
		"dns problem":                          "DNS 解析问题，请检查域名是否正确解析",
		"connection refused":                   "连接被拒绝，请检查防火墙和端口配置",
		"too many certificates":                "证书申请次数过多，请稍后再试（Let's Encrypt 频率限制）",
		"rate limit":                           "触发频率限制，请等待后再试",
		"caa record":                           "CAA 记录阻止了证书申请，请检查 DNS CAA 记录",
		"invalid response":                     "验证失败，请检查域名解析和 webroot 配置",
		"verify error":                         "域名验证失败，请确保域名正确解析到本服务器",
		"create new order error":               "创建订单失败，请检查 ACME 服务器状态",
		"no eab credentials found":             "缺少 EAB 凭证（ZeroSSL 需要）",
		"the domain is in hsts preload":        "域名在 HSTS 预加载列表中，必须使用 HTTPS",
		"invalid domain":                       "DNS API 返回 invalid domain，请确认域名在对应 DNS 服务商账号中，且 Cloudflare Token 具备 Zone.DNS.Edit + Zone.Zone.Read 权限",
		"error adding txt record":              "DNS 验证失败：无法添加 TXT 记录，请检查 DNS API Token 权限与 Zone 配置",
		"com.cloudflare.api.account.zone.list": "Cloudflare Token 缺少读取 Zone 权限，请补充 Zone.Zone.Read",
		"invalidcontact":                       "证书邮箱无效或被 ACME 拒绝，请使用真实可用邮箱地址",
		"forbidden domain":                     "证书邮箱域名被 ACME 拒绝，请更换邮箱地址（不要使用示例域名）",
		"contact email has forbidden domain":   "证书邮箱域名被 ACME 拒绝，请更换邮箱地址（不要使用示例域名）",
	}

	for pattern, friendlyMsg := range patterns {
		if strings.Contains(errMsgLower, pattern) {
			return friendlyMsg
		}
	}

	// 返回原始错误（截断过长的输出）
	if len(errMsg) > 500 {
		return errMsg[:500] + "..."
	}
	return errMsg
}

// isRetryableApplyError determines whether certificate apply errors should be retried.
func (s *Service) isRetryableApplyError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	nonRetryablePatterns := []string{
		"invalid domain",
		"error adding txt record",
		"dns api 返回 invalid domain",
		"无法添加 txt 记录",
		"证书邮箱无效或被 acme 拒绝",
		"证书邮箱域名被 acme 拒绝",
		"forbidden domain",
		"invalidcontact",
		"缺少 dns 提供商配置",
		"缺少 dns api 凭证",
		"无效的域名格式",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	return true
}

// ensureAcmeAccountEmail ensures acme.sh account email is set to the provided email.
func (s *Service) ensureAcmeAccountEmail(ctx context.Context, acmePath, server, email string) error {
	if email == "" {
		return nil
	}

	// 先尝试注册账户（首次申请场景）
	registerArgs := []string{"--register-account", "--accountemail", email}
	if server != "" {
		registerArgs = append(registerArgs, "--server", server)
	}
	registerCmd := exec.CommandContext(ctx, acmePath, registerArgs...)
	registerOutput, registerErr := registerCmd.CombinedOutput()
	if registerErr == nil {
		s.logger.Info("acme.sh 账户邮箱已确认（register）", logger.F("email", email))
		return nil
	}

	registerOutStr := string(registerOutput)
	registerOutLower := strings.ToLower(registerOutStr)
	if strings.Contains(registerOutLower, "invalidcontact") ||
		strings.Contains(registerOutLower, "forbidden domain") {
		return apperrors.NewBadRequestError(fmt.Sprintf("证书邮箱无效或被拒绝: %s，请使用真实可用邮箱", email))
	}

	// 再尝试更新账户（已有账户场景）
	updateArgs := []string{"--update-account", "--accountemail", email}
	updateCmd := exec.CommandContext(ctx, acmePath, updateArgs...)
	updateOutput, updateErr := updateCmd.CombinedOutput()
	if updateErr == nil {
		s.logger.Info("acme.sh 账户邮箱已确认（update）", logger.F("email", email))
		return nil
	}

	updateOutStr := string(updateOutput)
	updateOutLower := strings.ToLower(updateOutStr)
	if strings.Contains(updateOutLower, "invalidcontact") ||
		strings.Contains(updateOutLower, "forbidden domain") {
		return apperrors.NewBadRequestError(fmt.Sprintf("证书邮箱无效或被拒绝: %s，请使用真实可用邮箱", email))
	}

	// 非邮箱类失败不阻断主流程，继续尝试申请
	s.logger.Warn("注册/更新 acme.sh 账户邮箱失败，继续申请",
		logger.F("email", email),
		logger.F("register_error", registerErr.Error()),
		logger.F("register_output", registerOutStr),
		logger.F("update_error", updateErr.Error()),
		logger.F("update_output", updateOutStr))
	return nil
}

// issueWithAcmeTest issues a test certificate to verify configuration.
func (s *Service) issueWithAcmeTest(ctx context.Context, req *ApplyRequest, cert *repository.Certificate) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	acmePath := filepath.Join(homeDir, ".acme.sh", "acme.sh")

	// 如果提供了邮箱，先尝试注册/更新账户
	if req.Email != "" {
		registerArgs := []string{
			"--register-account",
			"--server", "letsencrypt_test",
			"--accountemail", req.Email,
		}
		s.logger.Info("注册/更新测试账户", logger.F("email", req.Email))
		registerCmd := exec.CommandContext(ctx, acmePath, registerArgs...)
		registerOutput, registerErr := registerCmd.CombinedOutput()
		if registerErr != nil {
			outputStr := string(registerOutput)
			// 如果是邮箱相关错误，直接返回
			if strings.Contains(outputStr, "forbidden domain") || strings.Contains(outputStr, "invalidContact") {
				return fmt.Errorf("邮箱地址无效或被拒绝: %s，请使用真实的邮箱地址", req.Email)
			}
			s.logger.Warn("注册测试账户失败，继续申请",
				logger.F("error", registerErr.Error()),
				logger.F("output", outputStr))
		} else {
			s.logger.Info("测试账户注册成功")
		}
	}

	// 构建测试命令参数
	args := []string{
		"--issue",
		"--server", "letsencrypt_test", // 使用测试服务器
		"-d", req.Domain,
		"--keylength", "ec-256", // 使用 ECC 证书
	}

	// 添加 Email（如果提供）
	if req.Email != "" {
		args = append(args, "--accountemail", req.Email)
	}

	// 设置验证方式
	if req.Method == "dns" {
		args = append(args, "--dns", req.DNSProvider)
	} else {
		// HTTP 验证
		args = append(args, "-w", req.Webroot)
	}

	// 执行测试申请命令
	s.logger.Info("执行 acme.sh 测试申请", logger.F("args", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, acmePath, args...)

	// 设置 DNS API 环境变量
	if req.Method == "dns" && len(req.DNSEnv) > 0 {
		cmd.Env = os.Environ()
		for key, value := range req.DNSEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("acme.sh 测试申请失败: %s, output: %s", err.Error(), string(output))
	}

	s.logger.Info("测试证书申请成功", logger.F("domain", req.Domain))
	return nil
}

// issueWithAcme issues a certificate using acme.sh.
func (s *Service) issueWithAcme(ctx context.Context, req *ApplyRequest, cert *repository.Certificate) error {
	// 查找 acme.sh 路径
	acmePath, found := s.findAcmePath(true)
	if !found {
		return fmt.Errorf("未找到 acme.sh，请确保已正确安装")
	}

	s.logger.Info("使用 acme.sh", logger.F("path", acmePath))

	// 确保 ACME 账户邮箱与请求一致，避免沿用安装时的示例邮箱
	if err := s.ensureAcmeAccountEmail(ctx, acmePath, req.Provider, req.Email); err != nil {
		return err
	}

	// 构建申请命令参数
	args := []string{
		"--issue",
		"-d", req.Domain,
		"--keylength", "ec-256",
		"--force", // 强制申请，覆盖已存在的证书
	}

	// 通配符证书 *.example.com 不匹配根域 example.com。如果是从根域+wildcard
	// 申请的（req.Domain 形如 "*.foo"），同时加 -d foo，让证书的 SAN 列表既
	// 包含 *.foo 又包含 foo —— 浏览器访问 https://foo 不会再提示"证书域名
	// 不匹配 / 不安全"。
	if strings.HasPrefix(req.Domain, "*.") {
		args = append(args, "-d", strings.TrimPrefix(req.Domain, "*."))
	}

	// 添加邮箱（如果提供）
	if req.Email != "" {
		args = append(args, "--accountemail", req.Email)
	}

	// 设置验证方式
	if req.Method == "dns" {
		if req.DNSProvider == "" {
			return fmt.Errorf("DNS 验证需要指定 DNS 提供商")
		}
		args = append(args, "--dns", req.DNSProvider)
	} else {
		// HTTP 验证
		if req.Webroot == "" {
			req.Webroot = "/app/data/webroot"
		}
		args = append(args, "-w", req.Webroot)
	}

	// 设置服务器
	if req.Provider != "" {
		args = append(args, "--server", req.Provider)
	}

	s.logger.Info("开始申请证书",
		logger.F("domain", req.Domain),
		logger.F("method", req.Method),
		logger.F("wildcard", req.Wildcard),
		logger.F("dns_provider", req.DNSProvider),
		logger.F("args", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, acmePath, args...)

	// 设置 DNS API 环境变量
	if req.Method == "dns" && len(req.DNSEnv) > 0 {
		cmd.Env = os.Environ()
		for key, value := range req.DNSEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error("证书申请失败",
			logger.F("domain", req.Domain),
			logger.F("error", err.Error()),
			logger.F("output", string(output)))
		return fmt.Errorf("证书申请失败: %s", s.parseAcmeError(fmt.Errorf("%s", string(output))))
	}

	s.logger.Info("证书申请成功", logger.F("domain", req.Domain))

	certFile, keyFile, err := s.certificateStoragePaths(req.Domain)
	if err != nil {
		return err
	}

	installArgs := []string{
		"--installcert",
		"-d", req.Domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile,
		"--ecc",
	}

	cmd = exec.CommandContext(ctx, acmePath, installArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Error("安装证书失败",
			logger.F("error", err.Error()),
			logger.F("output", string(output)))
		return fmt.Errorf("安装证书失败: %w", err)
	}

	if err := os.Chmod(certFile, 0644); err != nil {
		s.logger.Warn("设置证书文件权限失败", logger.Err(err))
	}
	if err := os.Chmod(keyFile, 0600); err != nil {
		s.logger.Warn("设置私钥文件权限失败", logger.Err(err))
	}

	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("读取证书文件失败: %w", err)
	}
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("读取私钥文件失败: %w", err)
	}

	parsedCert, certData, keyData, err := validateCertificateMaterial(req.Domain, certData, keyData)
	if err != nil {
		return err
	}
	if certFile, keyFile, err = s.writeCertificateFiles(req.Domain, certData, keyData); err != nil {
		return err
	}

	setCertificateMaterialFields(cert, parsedCert, certFile, keyFile)

	return s.certRepo.Update(ctx, cert)
}

// Upload uploads a certificate manually.
func (s *Service) Upload(ctx context.Context, domain string, certData, keyData []byte) (*repository.Certificate, error) {
	s.logger.Info("上传证书", logger.F("domain", domain))

	domain = strings.TrimSpace(strings.ToLower(domain))
	parsedCert, certData, keyData, err := validateCertificateMaterial(domain, certData, keyData)
	if err != nil {
		return nil, err
	}
	if existingCert, err := s.certRepo.GetByDomain(ctx, domain); err == nil && existingCert != nil {
		return nil, apperrors.NewConflictError("certificate", "domain", domain)
	} else if err != nil && !apperrors.IsNotFound(err) {
		return nil, fmt.Errorf("查询证书记录失败: %w", err)
	}
	certFile, keyFile, err := s.writeCertificateFiles(domain, certData, keyData)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	notAfter := parsedCert.NotAfter.UTC()
	cert := &repository.Certificate{
		Domain:     domain,
		Provider:   "manual",
		CertPath:   certFile,
		KeyPath:    keyFile,
		IssueDate:  &now,
		ExpireDate: &notAfter,
		ExpiresAt:  notAfter,
		AutoRenew:  false,
		Status:     "active",
	}

	if err := s.certRepo.Create(ctx, cert); err != nil {
		return nil, fmt.Errorf("创建证书记录失败: %w", err)
	}

	s.logger.Info("证书上传成功", logger.F("domain", domain))
	return cert, nil
}

// UpdateMaterial replaces the stored PEM pair for an existing certificate.
func (s *Service) UpdateMaterial(ctx context.Context, id int64, certData, keyData []byte) (*repository.Certificate, error) {
	cert, err := s.certRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取证书失败: %w", err)
	}

	parsedCert, certData, keyData, err := validateCertificateMaterial(cert.Domain, certData, keyData)
	if err != nil {
		return nil, err
	}
	certFile, keyFile, err := s.writeCertificateFiles(cert.Domain, certData, keyData)
	if err != nil {
		return nil, err
	}

	cert.Provider = "manual"
	cert.AutoRenew = false
	setCertificateMaterialFields(cert, parsedCert, certFile, keyFile)
	if err := s.certRepo.Update(ctx, cert); err != nil {
		return nil, fmt.Errorf("更新证书记录失败: %w", err)
	}

	s.logger.Info("证书内容已更新",
		logger.F("domain", cert.Domain),
		logger.F("cert_id", cert.ID))
	return cert, nil
}

// Renew renews a certificate.
func (s *Service) Renew(ctx context.Context, id int64) error {
	cert, err := s.certRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取证书失败: %w", err)
	}

	if cert.Provider == "manual" {
		return fmt.Errorf("手动上传的证书不支持自动续期")
	}

	s.logger.Info("续期证书", logger.F("domain", cert.Domain))

	if err := s.ensureAcmeInstalled(ctx, ""); err != nil {
		return err
	}

	acmePath, found := s.findAcmePath(true)
	if !found {
		return fmt.Errorf("未找到 acme.sh，请确保已正确安装")
	}

	// 执行续期命令
	args := []string{
		"--renew",
		"-d", cert.Domain,
		"--ecc",
		"--force",
	}

	cmd := exec.CommandContext(ctx, acmePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("续期失败: %s, output: %s", err.Error(), string(output))
	}

	certFile, keyFile, err := s.certificateStoragePaths(cert.Domain)
	if err != nil {
		return err
	}

	installArgs := []string{
		"--installcert",
		"-d", cert.Domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile,
		"--ecc",
	}

	cmd = exec.CommandContext(ctx, acmePath, installArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("安装证书失败: %s, output: %s", err.Error(), string(output))
	}

	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("读取证书文件失败: %w", err)
	}
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("读取私钥文件失败: %w", err)
	}

	parsedCert, certData, keyData, err := validateCertificateMaterial(cert.Domain, certData, keyData)
	if err != nil {
		return err
	}
	if certFile, keyFile, err = s.writeCertificateFiles(cert.Domain, certData, keyData); err != nil {
		return err
	}

	setCertificateMaterialFields(cert, parsedCert, certFile, keyFile)

	if err := s.certRepo.Update(ctx, cert); err != nil {
		return fmt.Errorf("更新证书记录失败: %w", err)
	}

	s.logger.Info("证书续期成功", logger.F("domain", cert.Domain))
	return nil
}

// Delete deletes a certificate.
func (s *Service) Delete(ctx context.Context, id int64) error {
	cert, err := s.certRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取证书失败: %w", err)
	}

	if s.nodeRepo != nil {
		nodes, err := s.nodeRepo.List(ctx, nil)
		if err != nil {
			return fmt.Errorf("获取节点列表失败: %w", err)
		}
		for _, node := range nodes {
			if node.CertificateID == nil || *node.CertificateID != id {
				continue
			}
			node.CertificateID = nil
			if err := s.nodeRepo.Update(ctx, node); err != nil {
				return fmt.Errorf("解除节点证书关联失败: %w", err)
			}
		}
	}

	seenDirs := make(map[string]struct{})
	for _, materialPath := range []string{cert.CertPath, cert.KeyPath} {
		if strings.TrimSpace(materialPath) == "" {
			continue
		}
		certDir := filepath.Dir(materialPath)
		if _, exists := seenDirs[certDir]; exists {
			continue
		}
		seenDirs[certDir] = struct{}{}
		if filepath.Clean(certDir) == filepath.Clean(s.certDir) {
			s.logger.Warn("跳过删除证书目录：路径指向证书存储根目录",
				logger.F("cert_id", id),
				logger.F("path", certDir))
			continue
		}
		if !certificatePathWithinBase(certDir, s.certDir) {
			s.logger.Warn("跳过删除证书目录：路径不在证书存储目录内",
				logger.F("cert_id", id),
				logger.F("path", certDir))
			continue
		}
		if err := os.RemoveAll(certDir); err != nil {
			s.logger.Warn("删除证书文件失败",
				logger.F("path", certDir),
				logger.Err(err))
		}
	}

	if err := s.certRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除证书记录失败: %w", err)
	}

	s.logger.Info("证书删除成功", logger.F("domain", cert.Domain))
	return nil
}

// List lists all certificates.
func (s *Service) List(ctx context.Context) ([]*repository.Certificate, error) {
	return s.certRepo.List(ctx, 1000, 0)
}

// GetByID gets a certificate by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*repository.Certificate, error) {
	return s.certRepo.GetByID(ctx, id)
}

// GetByDomain gets a certificate by domain.
func (s *Service) GetByDomain(ctx context.Context, domain string) (*repository.Certificate, error) {
	return s.certRepo.GetByDomain(ctx, domain)
}

// UpdateAutoRenew updates auto-renew setting.
func (s *Service) UpdateAutoRenew(ctx context.Context, id int64, autoRenew bool) error {
	cert, err := s.certRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取证书失败: %w", err)
	}

	cert.AutoRenew = autoRenew
	if err := s.certRepo.Update(ctx, cert); err != nil {
		return fmt.Errorf("更新证书失败: %w", err)
	}

	s.logger.Info("更新自动续期设置",
		logger.F("domain", cert.Domain),
		logger.F("auto_renew", autoRenew))

	return nil
}

// CheckExpiring checks for expiring certificates and renews them if auto-renew is enabled.
func (s *Service) CheckExpiring(ctx context.Context) error {
	certs, err := s.certRepo.List(ctx, 1000, 0)
	if err != nil {
		return fmt.Errorf("获取证书列表失败: %w", err)
	}

	now := time.Now()
	renewThreshold := 30 * 24 * time.Hour // 30 天内过期

	for _, cert := range certs {
		if cert.ExpireDate == nil {
			continue
		}

		timeUntilExpiry := cert.ExpireDate.Sub(now)

		// 检查是否即将过期
		if timeUntilExpiry < renewThreshold && timeUntilExpiry > 0 {
			s.logger.Info("证书即将过期",
				logger.F("domain", cert.Domain),
				logger.F("days_left", int(timeUntilExpiry.Hours()/24)))

			// 如果启用了自动续期，则续期
			if cert.AutoRenew && cert.Provider != "manual" {
				s.logger.Info("自动续期证书", logger.F("domain", cert.Domain))
				if err := s.Renew(ctx, cert.ID); err != nil {
					s.logger.Error("自动续期失败",
						logger.F("domain", cert.Domain),
						logger.F("error", err.Error()))
				}
			}
		}
	}

	return nil
}

// GenerateSelfSigned generates a self-signed certificate for testing.
func (s *Service) GenerateSelfSigned(ctx context.Context, domain string) (*repository.Certificate, error) {
	s.logger.Info("生成自签名证书", logger.F("domain", domain))

	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// 生成证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("生成证书失败: %w", err)
	}

	// 编码证书
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// 编码私钥
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// SECURITY: Sanitize domain for safe path construction
	safeDomain, err := sanitizeDomainForPath(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain for path: %w", err)
	}

	// 保存文件
	certPath := filepath.Join(s.certDir, safeDomain)
	// Validate that certPath is within certDir (防止路径遍历)
	if !strings.HasPrefix(filepath.Clean(certPath), filepath.Clean(s.certDir)) {
		return nil, fmt.Errorf("path traversal attempt detected")
	}

	// 使用安全的目录权限 (0700)
	if err := os.MkdirAll(certPath, 0700); err != nil {
		return nil, fmt.Errorf("创建证书目录失败: %w", err)
	}

	certFile := filepath.Join(certPath, "fullchain.pem")
	keyFile := filepath.Join(certPath, "privkey.pem")

	// 证书文件权限 0644，私钥文件权限 0600
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		return nil, fmt.Errorf("保存证书文件失败: %w", err)
	}

	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		return nil, fmt.Errorf("保存私钥文件失败: %w", err)
	}

	// 创建证书记录
	now := time.Now()
	expireDate := now.Add(365 * 24 * time.Hour)

	cert := &repository.Certificate{
		Domain:     domain,
		Provider:   "self-signed",
		CertPath:   certFile,
		KeyPath:    keyFile,
		IssueDate:  &now,
		ExpireDate: &expireDate,
		ExpiresAt:  expireDate,
		AutoRenew:  false,
		Status:     "active",
	}

	if err := s.certRepo.Create(ctx, cert); err != nil {
		return nil, fmt.Errorf("创建证书记录失败: %w", err)
	}

	s.logger.Info("自签名证书生成成功", logger.F("domain", domain))
	return cert, nil
}

// StartAutoRenew 启动自动续期定时任务
func (s *Service) StartAutoRenew(ctx context.Context) error {
	s.renewCtx, s.renewCancel = context.WithCancel(ctx)

	s.renewWg.Add(1)
	go s.autoRenewLoop()

	s.logger.Info("证书自动续期服务已启动")
	return nil
}

// StopAutoRenew 停止自动续期定时任务
func (s *Service) StopAutoRenew() error {
	if s.renewCancel != nil {
		s.renewCancel()
	}

	// 等待 goroutine 结束
	done := make(chan struct{})
	go func() {
		s.renewWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("证书自动续期服务已停止")
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("停止自动续期服务超时")
	}
}

// autoRenewLoop 自动续期循环
func (s *Service) autoRenewLoop() {
	defer s.renewWg.Done()

	// 每天检查一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动时立即检查一次
	s.checkAndRenewCertificates()

	for {
		select {
		case <-s.renewCtx.Done():
			return
		case <-ticker.C:
			s.checkAndRenewCertificates()
		}
	}
}

// checkAndRenewCertificates 检查并续期证书
func (s *Service) checkAndRenewCertificates() {
	ctx := context.Background()

	certs, err := s.certRepo.GetAutoRenew(ctx)
	if err != nil {
		s.logger.Error("获取自动续期证书列表失败", logger.Err(err))
		return
	}

	now := time.Now()
	renewThreshold := 30 * 24 * time.Hour // 30 天内过期

	for _, cert := range certs {
		if cert.ExpireDate == nil {
			continue
		}

		timeUntilExpiry := cert.ExpireDate.Sub(now)

		// 检查是否即将过期
		if timeUntilExpiry < renewThreshold && timeUntilExpiry > 0 {
			daysLeft := int(timeUntilExpiry.Hours() / 24)
			s.logger.Info("证书即将过期，开始自动续期",
				logger.F("domain", cert.Domain),
				logger.F("days_left", daysLeft))

			if err := s.Renew(ctx, cert.ID); err != nil {
				s.logger.Error("自动续期失败",
					logger.F("domain", cert.Domain),
					logger.F("error", err.Error()))
			} else {
				s.logger.Info("自动续期成功", logger.F("domain", cert.Domain))

				// 续期成功后，部署到关联的节点
				if err := s.DeployToAssignedNodes(ctx, cert.ID); err != nil {
					s.logger.Error("部署证书到节点失败",
						logger.F("domain", cert.Domain),
						logger.F("error", err.Error()))
				}
			}
		}
	}
}

// DeployToNode 部署证书到指定节点
func (s *Service) DeployToNode(ctx context.Context, certID int64, nodeID int64) error {
	// 创建部署记录
	deployment := &repository.CertificateDeployment{
		CertificateID: certID,
		NodeID:        nodeID,
		Status:        "pending",
	}

	if err := s.deploymentRepo.Create(ctx, deployment); err != nil {
		return fmt.Errorf("创建部署记录失败: %w", err)
	}

	// 获取证书
	cert, err := s.certRepo.GetByID(ctx, certID)
	if err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("获取证书失败: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("获取证书失败: %w", err)
	}

	// 获取节点
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("获取节点失败: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("获取节点失败: %w", err)
	}

	s.logger.Info("开始部署证书到节点",
		logger.F("domain", cert.Domain),
		logger.F("node", node.Name))

	certData, keyData, err := readStoredCertificateMaterial(cert)
	if err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("读取证书材料失败: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("读取证书材料失败: %w", err)
	}
	if _, certData, keyData, err = validateCertificateMaterial(cert.Domain, certData, keyData); err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("证书材料无效: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("证书材料无效: %w", err)
	}

	// 通过 SSH 部署到节点
	if err := s.deployViaSSH(node, cert.Domain, certData, keyData); err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("SSH 部署失败: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("SSH 部署失败: %w", err)
	}

	// 更新部署记录为成功
	now := time.Now()
	deployment.Status = "success"
	deployment.Message = "部署成功"
	deployment.DeployedAt = &now
	s.deploymentRepo.Update(ctx, deployment)

	s.logger.Info("证书部署成功",
		logger.F("domain", cert.Domain),
		logger.F("node", node.Name))

	return nil
}

// DeployToAssignedNodes 部署证书到所有关联的节点
func (s *Service) DeployToAssignedNodes(ctx context.Context, certID int64) error {
	// 获取所有节点
	nodes, err := s.nodeRepo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("获取节点列表失败: %w", err)
	}

	// 找出关联此证书的节点
	assignedNodes := make([]*repository.Node, 0)
	for _, node := range nodes {
		if node.CertificateID != nil && *node.CertificateID == certID {
			assignedNodes = append(assignedNodes, node)
		}
	}

	if len(assignedNodes) == 0 {
		s.logger.Info("没有节点关联此证书", logger.F("cert_id", certID))
		return nil
	}

	s.logger.Info("开始部署证书到关联节点",
		logger.F("cert_id", certID),
		logger.F("node_count", len(assignedNodes)))

	// 并发部署到所有节点（限制并发数）
	const maxConcurrent = 5
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	errChan := make(chan error, len(assignedNodes))

	for _, node := range assignedNodes {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量
		go func(n *repository.Node) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放信号量
			if err := s.DeployToNode(ctx, certID, n.ID); err != nil {
				errChan <- fmt.Errorf("节点 %s 部署失败: %w", n.Name, err)
			}
		}(node)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分节点部署失败: %v", errors)
	}

	return nil
}

// deployViaSSH 通过 SSH 部署证书到节点
func (s *Service) deployViaSSH(node *repository.Node, domain string, certData, keyData []byte) error {
	// 确定 SSH 连接参数
	sshHost := node.SSHHost
	if sshHost == "" {
		sshHost = node.Address
	}

	sshPort := node.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}

	sshUser := node.SSHUser
	if sshUser == "" {
		sshUser = "root"
	}

	s.logger.Info("建立 SSH 连接",
		logger.F("host", sshHost),
		logger.F("port", sshPort),
		logger.F("user", sshUser))

	// 配置 SSH 认证
	var authMethods []ssh.AuthMethod

	// 优先使用密钥认证
	if node.SSHKeyPath != "" {
		keyData, err := os.ReadFile(node.SSHKeyPath)
		if err != nil {
			return fmt.Errorf("读取 SSH 私钥失败: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return fmt.Errorf("解析 SSH 私钥失败: %w", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// 密码认证
	sshPassword, err := nodepkg.DecryptSSHPassword(node.SSHPassword)
	if err != nil {
		return fmt.Errorf("解密 SSH 密码失败: %w", err)
	}
	if sshPassword != "" {
		authMethods = append(authMethods, ssh.Password(sshPassword))
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = sshPassword
			}
			return answers, nil
		}))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("未配置 SSH 认证方式")
	}

	// 建立 SSH 连接
	config := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: 使用已知主机密钥验证
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", sshHost, sshPort)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	s.logger.Info("SSH 连接成功")

	safeDomain, err := sanitizeDomainForPath(domain)
	if err != nil {
		return fmt.Errorf("invalid domain for remote path: %w", err)
	}

	certDir := "/etc/xray/certs/" + safeDomain
	if err := s.executeSSHCommand(client, fmt.Sprintf("mkdir -p %s", shellQuote(certDir))); err != nil {
		return fmt.Errorf("创建证书目录失败: %w", err)
	}

	certPath := fmt.Sprintf("%s/fullchain.pem", certDir)
	if err := s.uploadFileSSH(client, certPath, certData); err != nil {
		return fmt.Errorf("上传证书文件失败: %w", err)
	}

	keyPath := fmt.Sprintf("%s/privkey.pem", certDir)
	if err := s.uploadFileSSH(client, keyPath, keyData); err != nil {
		return fmt.Errorf("上传私钥文件失败: %w", err)
	}

	if err := s.executeSSHCommand(client, fmt.Sprintf("chmod 644 %s", shellQuote(certPath))); err != nil {
		return fmt.Errorf("设置证书权限失败: %w", err)
	}

	if err := s.executeSSHCommand(client, fmt.Sprintf("chmod 600 %s", shellQuote(keyPath))); err != nil {
		return fmt.Errorf("设置私钥权限失败: %w", err)
	}

	// 更新节点的 TLS 配置
	if err := s.executeSSHCommand(client, fmt.Sprintf(`
		# 备份当前配置
		if [ -f /etc/xray/config.json ]; then
			cp /etc/xray/config.json /etc/xray/config.json.backup.$(date +%%s)
		fi
	`)); err != nil {
		s.logger.Warn("备份配置失败", logger.Err(err))
	}

	// 重启 Xray 服务以应用新证书
	s.logger.Info("重启 Xray 服务")
	if err := s.executeSSHCommand(client, "systemctl restart xray || service xray restart"); err != nil {
		s.logger.Warn("重启 Xray 服务失败", logger.Err(err))
		// 不返回错误，因为证书已经部署成功
	}

	s.logger.Info("证书部署完成",
		logger.F("node", node.Name),
		logger.F("domain", domain))

	return nil
}

// executeSSHCommand 执行 SSH 命令
func (s *Service) executeSSHCommand(client *ssh.Client, command string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("命令执行失败: %w, output: %s", err, string(output))
	}

	return nil
}

// uploadFileSSH 通过 SSH 上传文件
func (s *Service) uploadFileSSH(client *ssh.Client, remotePath string, data []byte) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	// 使用 base64 编码传输文件内容
	encoded := base64.StdEncoding.EncodeToString(data)

	// 分块传输（每块 100KB）
	chunkSize := 100 * 1024
	totalChunks := (len(encoded) + chunkSize - 1) / chunkSize

	// 清空目标文件
	if err := s.executeSSHCommand(client, fmt.Sprintf("rm -f %s", remotePath)); err != nil {
		return err
	}

	// 分块上传
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}

		chunk := encoded[i:end]
		chunkNum := i/chunkSize + 1

		if chunkNum%10 == 0 || chunkNum == totalChunks {
			s.logger.Debug("上传进度",
				logger.F("chunk", chunkNum),
				logger.F("total", totalChunks))
		}

		// 使用 echo 和 base64 解码写入文件
		cmd := fmt.Sprintf("echo '%s' | base64 -d >> %s", chunk, remotePath)
		if err := s.executeSSHCommand(client, cmd); err != nil {
			return fmt.Errorf("上传第 %d 块失败: %w", chunkNum, err)
		}
	}

	return nil
}
