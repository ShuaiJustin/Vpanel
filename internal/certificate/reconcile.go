package certificate

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

var errNoTLSConsumers = errors.New("无启用的 TLS 代理，无需 TLS 验证")

const (
	deploymentConfirmTimeout = 5 * time.Minute
	deploymentRetryMin       = 5 * time.Minute
	deploymentRetryMax       = 6 * time.Hour
	// Agents report every 30s; ten missed reports mean the agent is gone.
	nodeAgentSilence = 5 * time.Minute
)

// nodeAgentReporting tells whether the node's agent can still apply and
// confirm a certificate. Without it a redeploy can never be verified.
func nodeAgentReporting(node *repository.Node, now time.Time) bool {
	return node.LastSeenAt != nil && now.Sub(*node.LastSeenAt) <= nodeAgentSilence
}

// deploymentRetryDelay waits as long as the deployment has already been
// failing (5m, 10m, 20m, ... capped at 6h), so a node that stays broken costs a
// few SSH attempts per day instead of one every five minutes.
func deploymentRetryDelay(d *repository.CertificateDeployment) time.Duration {
	return min(max(d.UpdatedAt.Sub(d.CreatedAt), deploymentRetryMin), deploymentRetryMax)
}

func certificateExpiry(cert *repository.Certificate) time.Time {
	if cert.ExpireDate != nil {
		return *cert.ExpireDate
	}
	return cert.ExpiresAt
}

func certificateAlertBucket(expiresAt, now time.Time) (string, int) {
	if expiresAt.IsZero() {
		return "", 0
	}
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return "expired", 0
	}
	for _, days := range []int{1, 7, 14, 30} {
		if remaining <= time.Duration(days)*24*time.Hour {
			return "expiring", days
		}
	}
	return "", 0
}

func (s *Service) scanCertificateAlerts(ctx context.Context, now time.Time) {
	for offset := 0; ; offset += 1000 {
		certs, err := s.certRepo.List(ctx, 1000, offset)
		if err != nil {
			s.logger.Error("扫描证书到期状态失败", logger.Err(err))
			return
		}
		for _, cert := range certs {
			expires := certificateExpiry(cert)
			level, bucket := certificateAlertBucket(expires, now)
			if level == "" {
				continue
			}
			key := fmt.Sprintf("certificate:%d:%s:%s:%d", cert.ID, expires.UTC().Format(time.RFC3339), level, bucket)
			s.notifyCertificateAlertWithKey(cert, level, "请检查自动续期和节点证书应用状态", key)
		}
		if len(certs) < 1000 {
			return
		}
	}
}

func (s *Service) reconcileCertificateDeployments(ctx context.Context) {
	if s.nodeRepo == nil || s.deploymentRepo == nil {
		return
	}
	nodes, err := s.nodeRepo.List(ctx, nil)
	if err != nil {
		s.logger.Error("扫描证书部署状态失败", logger.Err(err))
		return
	}
	for _, node := range nodes {
		if ctx.Err() != nil {
			return
		}
		if node.CertificateID == nil {
			continue
		}
		deployment, err := s.deploymentRepo.GetLatestByNodeAndCert(ctx, node.ID, *node.CertificateID)
		if err != nil || deployment == nil || deployment.Status == "success" {
			continue
		}
		// A silent agent can neither apply nor confirm the certificate. The node
		// is already marked for config sync, so it picks the certificate up from
		// its generated config as soon as the agent reports again.
		if !nodeAgentReporting(node, time.Now()) {
			if s.agentSilentAt == nil {
				s.agentSilentAt = make(map[int64]time.Time)
			}
			s.agentSilentAt[node.ID] = time.Now()
			continue
		}
		cert, err := s.certRepo.GetByID(ctx, *node.CertificateID)
		if err != nil {
			continue
		}
		verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		verifyErr := s.verifyAppliedCertificate(verifyCtx, node, cert, deployment.CreatedAt)
		cancel()
		if verifyErr == nil || errors.Is(verifyErr, errNoTLSConsumers) {
			now := time.Now()
			deployment.Status, deployment.Message, deployment.DeployedAt = "success", "节点证书已应用并通过 TLS 验证", &now
			if errors.Is(verifyErr, errNoTLSConsumers) {
				deployment.Message = "节点配置已同步；无启用的 TLS 代理，无需 TLS 验证"
			}
			if err := s.deploymentRepo.Update(ctx, deployment); err != nil {
				s.logger.Error("保存证书应用确认失败", logger.Err(err))
			}
			continue
		}
		// UpdatedAt is the end of the last attempt; CreatedAt stays at the first
		// one because retries reuse the row. An agent that just came back gets
		// the full window to sync before the deployment counts as failed.
		waitingSince := deployment.UpdatedAt
		if silentAt := s.agentSilentAt[node.ID]; silentAt.After(waitingSince) {
			waitingSince = silentAt
		}
		if deployment.Status == "pending" && time.Since(waitingSince) >= deploymentConfirmTimeout {
			deployment.Status = "failed"
			deployment.Message = "节点未确认应用新证书: " + verifyErr.Error()
			if err := s.deploymentRepo.Update(ctx, deployment); err != nil {
				s.logger.Error("保存证书应用失败状态失败", logger.Err(err))
				continue
			}
			s.notifyCertificateAlertWithKey(cert, "deployment_failed", deployment.Message,
				fmt.Sprintf("certificate-deployment:%d:%d:%s:%s", cert.ID, node.ID, certificateExpiry(cert).UTC().Format(time.RFC3339), time.Now().UTC().Format("2006-01-02")))
		} else if deployment.Status == "failed" && time.Since(deployment.UpdatedAt) >= deploymentRetryDelay(deployment) {
			if err := s.runDeployment(ctx, deployment); err != nil {
				s.logger.Error("重试证书下发失败", logger.F("node_id", node.ID), logger.Err(err))
			}
		}
	}
}

func (s *Service) verifyAppliedCertificate(ctx context.Context, node *repository.Node, cert *repository.Certificate, queuedAt time.Time) error {
	if node.SyncStatus != repository.NodeSyncStatusSynced || node.SyncedAt == nil || node.SyncedAt.Before(queuedAt) || !node.XrayRunning {
		return fmt.Errorf("Agent 尚未确认配置同步且 Xray 运行")
	}
	if s.proxyRepo == nil {
		return fmt.Errorf("代理证书验证未配置")
	}
	certPEM, _, err := readStoredCertificateMaterial(cert)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("证书内容无法解析")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	proxies, err := s.proxyRepo.GetByNodeID(ctx, node.ID)
	if err != nil {
		return err
	}
	tlsConsumers, verified := 0, 0
	for _, proxy := range proxies {
		security, _ := proxy.Settings["security"].(string)
		tlsEnabled := strings.EqualFold(security, "tls") || (security == "" && strings.EqualFold(proxy.Protocol, "trojan"))
		if security == "" {
			v := fmt.Sprint(proxy.Settings["tls"])
			tlsEnabled = tlsEnabled || v == "true" || v == "1" || v == "tls"
		}
		if !tlsEnabled {
			continue
		}
		tlsConsumers++
		serverName := node.TLSDomain
		for _, k := range []string{"sni", "server_name", "tls_domain"} {
			if value, ok := proxy.Settings[k].(string); ok && value != "" {
				serverName = value
				break
			}
		}
		if err := leaf.VerifyHostname(serverName); err != nil {
			continue
		} // A different inbound certificate is not this deployment's consumer.
		address := net.JoinHostPort(node.Address, fmt.Sprint(proxy.Port))
		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("端口 %d: %w", proxy.Port, err)
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: serverName, InsecureSkipVerify: true}) // #nosec G402 -- exact certificate bytes and hostname checked below.
		err = tlsConn.HandshakeContext(ctx)
		if err == nil {
			peers := tlsConn.ConnectionState().PeerCertificates
			if len(peers) == 0 || !bytes.Equal(peers[0].Raw, leaf.Raw) {
				err = fmt.Errorf("节点仍提供不同证书")
			}
			if !time.Now().Before(leaf.NotAfter) || time.Now().Before(leaf.NotBefore) {
				err = fmt.Errorf("节点证书不在有效期内")
			}
		}
		conn.Close()
		if err != nil {
			return fmt.Errorf("端口 %d: %w", proxy.Port, err)
		}
		verified++
	}
	if tlsConsumers > 0 && verified == 0 {
		return fmt.Errorf("未找到使用该证书的 TLS 端口，无法确认应用")
	}
	if tlsConsumers == 0 {
		return errNoTLSConsumers
	}
	return nil
}
