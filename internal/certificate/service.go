// Package certificate provides TLS certificate management functionality.
package certificate

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
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
)

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
}

// Apply applies for a new certificate using acme.sh.
func (s *Service) Apply(ctx context.Context, req *ApplyRequest) (*repository.Certificate, error) {
	s.logger.Info("申请证书",
		logger.F("domain", req.Domain),
		logger.F("provider", req.Provider),
		logger.F("method", req.Method))

	// 检查 acme.sh 是否安装
	if !s.isAcmeInstalled() {
		return nil, fmt.Errorf("acme.sh 未安装，请先安装: curl https://get.acme.sh | sh")
	}

	// 设置默认值
	if req.Provider == "" {
		req.Provider = "letsencrypt"
	}
	if req.Method == "" {
		req.Method = "http"
	}

	// 验证请求参数
	if req.Method == "dns" {
		if req.DNSProvider == "" {
			return nil, fmt.Errorf("DNS 验证方式需要指定 DNS 提供商，如: dns_cf (Cloudflare)")
		}
		// 检查 DNS API 凭证
		if len(req.DNSEnv) == 0 {
			return nil, fmt.Errorf("DNS 验证方式需要提供 API 凭证")
		}
	}
	if req.Method == "http" {
		if req.Webroot == "" {
			req.Webroot = "/var/www/html" // 默认 webroot
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
		return nil, fmt.Errorf("无效的域名格式: %s", req.Domain)
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
	cert := &repository.Certificate{
		Domain:    req.Domain,
		Provider:  req.Provider,
		AutoRenew: true,
		Status:    "pending",
	}

	if err := s.certRepo.Create(ctx, cert); err != nil {
		return nil, fmt.Errorf("创建证书记录失败: %w", err)
	}

	// 异步申请证书（使用独立的 context）
	go func() {
		// 创建带超时的 context（30 分钟）
		applyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// 先测试申请
		if err := s.issueWithAcmeTest(applyCtx, req, cert); err != nil {
			s.logger.Error("测试证书申请失败",
				logger.F("domain", req.Domain),
				logger.F("error", err.Error()))
			
			cert.Status = "failed"
			cert.ErrorMessage = s.parseAcmeError(err)
			s.certRepo.Update(context.Background(), cert)
			return
		}

		s.logger.Info("测试证书申请成功，开始正式申请", logger.F("domain", req.Domain))

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
				continue
			}

			// 成功
			return
		}

		// 所有重试都失败
		s.logger.Error("证书申请最终失败",
			logger.F("domain", req.Domain),
			logger.F("error", lastErr.Error()))
		
		cert.Status = "failed"
		cert.ErrorMessage = s.parseAcmeError(lastErr)
		s.certRepo.Update(context.Background(), cert)
	}()

	return cert, nil
}

// isAcmeInstalled checks if acme.sh is installed.
func (s *Service) isAcmeInstalled() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	
	acmePath := filepath.Join(homeDir, ".acme.sh", "acme.sh")
	_, err = os.Stat(acmePath)
	return err == nil
}

// isValidDomain validates domain name format.
func (s *Service) isValidDomain(domain string) bool {
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
	
	// 常见错误模式匹配
	patterns := map[string]string{
		"Timeout":                           "申请超时，请检查网络连接",
		"DNS problem":                       "DNS 解析问题，请检查域名是否正确解析",
		"Connection refused":                "连接被拒绝，请检查防火墙和端口配置",
		"too many certificates":             "证书申请次数过多，请稍后再试（Let's Encrypt 频率限制）",
		"rate limit":                        "触发频率限制，请等待后再试",
		"CAA record":                        "CAA 记录阻止了证书申请，请检查 DNS CAA 记录",
		"Invalid response":                  "验证失败，请检查域名解析和 webroot 配置",
		"Verify error":                      "域名验证失败，请确保域名正确解析到本服务器",
		"Create new order error":            "创建订单失败，请检查 ACME 服务器状态",
		"No EAB credentials found":          "缺少 EAB 凭证（ZeroSSL 需要）",
		"The domain is in HSTS preload":     "域名在 HSTS 预加载列表中，必须使用 HTTPS",
	}

	for pattern, friendlyMsg := range patterns {
		if strings.Contains(errMsg, pattern) {
			return friendlyMsg
		}
	}

	// 返回原始错误（截断过长的输出）
	if len(errMsg) > 500 {
		return errMsg[:500] + "..."
	}
	return errMsg
}

// issueWithAcmeTest issues a test certificate to verify configuration.
func (s *Service) issueWithAcmeTest(ctx context.Context, req *ApplyRequest, cert *repository.Certificate) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	acmePath := filepath.Join(homeDir, ".acme.sh", "acme.sh")

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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	acmePath := filepath.Join(homeDir, ".acme.sh", "acme.sh")

	// 设置默认 CA
	setCAArgs := []string{"--set-default-ca", "--server", req.Provider}
	cmd := exec.CommandContext(ctx, acmePath, setCAArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Warn("设置默认 CA 失败", 
			logger.F("error", err.Error()),
			logger.F("output", string(output)))
	}

	// 构建正式申请命令参数
	args := []string{
		"--issue",
		"-d", req.Domain,
		"--keylength", "ec-256", // 使用 ECC 证书
		"--force",               // 强制更新（覆盖测试证书）
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

	// 执行申请命令
	s.logger.Info("执行 acme.sh 正式申请", logger.F("args", strings.Join(args, " ")))
	
	cmd = exec.CommandContext(ctx, acmePath, args...)
	
	// 设置 DNS API 环境变量
	if req.Method == "dns" && len(req.DNSEnv) > 0 {
		cmd.Env = os.Environ()
		for key, value := range req.DNSEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
			// 不记录敏感信息到日志
			s.logger.Debug("设置 DNS 环境变量", logger.F("key", key))
		}
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("acme.sh 申请失败: %s, output: %s", err.Error(), string(output))
	}

	s.logger.Info("证书申请成功", logger.F("domain", req.Domain))

	// 安装证书到指定目录
	certPath := filepath.Join(s.certDir, req.Domain)
	if err := os.MkdirAll(certPath, 0750); err != nil {
		return fmt.Errorf("创建证书目录失败: %w", err)
	}

	certFile := filepath.Join(certPath, "fullchain.pem")
	keyFile := filepath.Join(certPath, "privkey.pem")

	installArgs := []string{
		"--installcert",
		"-d", req.Domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile,
		"--ecc",
	}

	cmd = exec.CommandContext(ctx, acmePath, installArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("安装证书失败: %s, output: %s", err.Error(), string(output))
	}

	// 设置私钥文件权限为 0600（仅所有者可读写）
	if err := os.Chmod(keyFile, 0600); err != nil {
		s.logger.Warn("设置私钥文件权限失败", logger.Err(err))
	}

	// 读取证书信息
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("读取证书文件失败: %w", err)
	}

	// 解析证书获取过期时间
	block, _ := pem.Decode(certData)
	if block == nil {
		return fmt.Errorf("解析证书失败")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("解析证书失败: %w", err)
	}

	// 更新证书记录
	now := time.Now()
	cert.CertPath = certFile
	cert.KeyPath = keyFile
	cert.IssueDate = &now
	cert.ExpireDate = &x509Cert.NotAfter
	cert.Status = "active"
	cert.ErrorMessage = ""

	return s.certRepo.Update(context.Background(), cert)
}

// Upload uploads a certificate manually.
func (s *Service) Upload(ctx context.Context, domain string, certData, keyData []byte) (*repository.Certificate, error) {
	s.logger.Info("上传证书", logger.F("domain", domain))

	// 验证证书格式
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("无效的证书格式")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析证书失败: %w", err)
	}

	// 验证私钥格式
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return nil, fmt.Errorf("无效的私钥格式")
	}

	// 保存证书文件
	certPath := filepath.Join(s.certDir, domain)
	if err := os.MkdirAll(certPath, 0755); err != nil {
		return nil, fmt.Errorf("创建证书目录失败: %w", err)
	}

	certFile := filepath.Join(certPath, "fullchain.pem")
	keyFile := filepath.Join(certPath, "privkey.pem")

	if err := os.WriteFile(certFile, certData, 0644); err != nil {
		return nil, fmt.Errorf("保存证书文件失败: %w", err)
	}

	if err := os.WriteFile(keyFile, keyData, 0600); err != nil {
		return nil, fmt.Errorf("保存私钥文件失败: %w", err)
	}

	// 创建证书记录
	now := time.Now()
	cert := &repository.Certificate{
		Domain:      domain,
		Provider:    "manual",
		CertPath:    certFile,
		KeyPath:     keyFile,
		IssueDate:   &now,
		ExpireDate:  &x509Cert.NotAfter,
		AutoRenew:   false,
		Status:      "active",
	}

	if err := s.certRepo.Create(ctx, cert); err != nil {
		return nil, fmt.Errorf("创建证书记录失败: %w", err)
	}

	s.logger.Info("证书上传成功", logger.F("domain", domain))
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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	acmePath := filepath.Join(homeDir, ".acme.sh", "acme.sh")

	// 执行续期命令
	args := []string{
		"--renew",
		"-d", cert.Domain,
		"--ecc",
		"--force",
	}

	cmd := exec.Command(acmePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("续期失败: %s, output: %s", err.Error(), string(output))
	}

	// 重新安装证书
	certFile := cert.CertPath
	keyFile := cert.KeyPath

	installArgs := []string{
		"--installcert",
		"-d", cert.Domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile,
		"--ecc",
	}

	cmd = exec.Command(acmePath, installArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("安装证书失败: %s, output: %s", err.Error(), string(output))
	}

	// 读取新证书信息
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("读取证书文件失败: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return fmt.Errorf("解析证书失败")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("解析证书失败: %w", err)
	}

	// 更新证书记录
	now := time.Now()
	cert.IssueDate = &now
	cert.ExpireDate = &x509Cert.NotAfter

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

	// 删除证书文件
	if cert.CertPath != "" {
		certDir := filepath.Dir(cert.CertPath)
		if err := os.RemoveAll(certDir); err != nil {
			s.logger.Warn("删除证书文件失败", logger.F("error", err.Error()))
		}
	}

	// 删除数据库记录
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

	// 保存文件
	certPath := filepath.Join(s.certDir, domain)
	if err := os.MkdirAll(certPath, 0755); err != nil {
		return nil, fmt.Errorf("创建证书目录失败: %w", err)
	}

	certFile := filepath.Join(certPath, "fullchain.pem")
	keyFile := filepath.Join(certPath, "privkey.pem")

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
		Domain:      domain,
		Provider:    "self-signed",
		CertPath:    certFile,
		KeyPath:     keyFile,
		IssueDate:   &now,
		ExpireDate:  &expireDate,
		AutoRenew:   false,
		Status:      "active",
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
	
	// 读取证书文件
	certData, err := os.ReadFile(cert.CertPath)
	if err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("读取证书文件失败: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("读取证书文件失败: %w", err)
	}
	
	keyData, err := os.ReadFile(cert.KeyPath)
	if err != nil {
		deployment.Status = "failed"
		deployment.Message = fmt.Sprintf("读取私钥文件失败: %v", err)
		s.deploymentRepo.Update(ctx, deployment)
		return fmt.Errorf("读取私钥文件失败: %w", err)
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
	
	// 并发部署到所有节点
	var wg sync.WaitGroup
	errChan := make(chan error, len(assignedNodes))
	
	for _, node := range assignedNodes {
		wg.Add(1)
		go func(n *repository.Node) {
			defer wg.Done()
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
	if node.SSHPassword != "" {
		authMethods = append(authMethods, ssh.Password(node.SSHPassword))
		authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = node.SSHPassword
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
	
	// 创建证书目录
	certDir := fmt.Sprintf("/etc/xray/certs/%s", domain)
	if err := s.executeSSHCommand(client, fmt.Sprintf("mkdir -p %s", certDir)); err != nil {
		return fmt.Errorf("创建证书目录失败: %w", err)
	}
	
	// 上传证书文件
	certPath := fmt.Sprintf("%s/fullchain.pem", certDir)
	if err := s.uploadFileSSH(client, certPath, certData); err != nil {
		return fmt.Errorf("上传证书文件失败: %w", err)
	}
	
	// 上传私钥文件
	keyPath := fmt.Sprintf("%s/privkey.pem", certDir)
	if err := s.uploadFileSSH(client, keyPath, keyData); err != nil {
		return fmt.Errorf("上传私钥文件失败: %w", err)
	}
	
	// 设置文件权限
	if err := s.executeSSHCommand(client, fmt.Sprintf("chmod 644 %s", certPath)); err != nil {
		return fmt.Errorf("设置证书权限失败: %w", err)
	}
	
	if err := s.executeSSHCommand(client, fmt.Sprintf("chmod 600 %s", keyPath)); err != nil {
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

