package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/monitor"
	"v/internal/node"
	"v/pkg/errors"
)

type nodeDNSProvider struct {
	client  *http.Client
	baseURL string
	token   string
	zoneID  string
	writes  chan struct{}
}

func (h *NodeHandler) WithCloudflareNodeDNS() *NodeHandler {
	token := strings.TrimSpace(os.Getenv("CF_Token"))
	zoneID := strings.TrimSpace(os.Getenv("CF_Zone_ID"))
	if token != "" && zoneID != "" {
		h.nodeDNS = &nodeDNSProvider{
			client: h.httpClient, baseURL: "https://api.cloudflare.com/client/v4",
			token: token, zoneID: zoneID, writes: make(chan struct{}, 1),
		}
	}
	return h
}

type nodeDNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
}

func (p *nodeDNSProvider) request(ctx context.Context, method, path string, record *nodeDNSRecord, result any) error {
	var body []byte
	if record != nil {
		body, _ = json.Marshal(record)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+"/zones/"+url.PathEscape(p.zoneID)+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	if record != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("invalid Cloudflare response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 300 || !envelope.Success {
		return fmt.Errorf("Cloudflare DNS API request failed (HTTP %d)", resp.StatusCode)
	}
	if result != nil {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("invalid Cloudflare result (HTTP %d)", resp.StatusCode)
		}
	}
	return nil
}

func (p *nodeDNSProvider) ensure(ctx context.Context, n *node.Node, ip netip.Addr) (string, error) {
	var zone struct {
		Name string `json:"name"`
	}
	if err := p.request(ctx, http.MethodGet, "", nil, &zone); err != nil {
		return "", err
	}
	if !node.ValidateDomain(zone.Name) || zone.Name == "localhost" {
		return "", fmt.Errorf("Cloudflare DNS zone name is invalid")
	}
	name := fmt.Sprintf("node-%d.%s", n.ID, strings.ToLower(zone.Name))
	if n.AccessDomain != "" && n.AccessDomain != name {
		return "", errors.NewConflictError("DNS record", "name", n.AccessDomain)
	}
	record := nodeDNSRecord{
		Type: "A", Name: name, Content: ip.String(), TTL: 1,
		Comment: fmt.Sprintf("V Panel node %d (%s)", n.ID, n.CreatedAt.UTC().Format(time.RFC3339Nano)),
	}
	if ip.Is6() {
		record.Type = "AAAA"
	}
	var existing []nodeDNSRecord
	path := "/dns_records?name=" + url.QueryEscape(name)
	if err := p.request(ctx, http.MethodGet, path, nil, &existing); err != nil {
		return "", err
	}
	method, writePath := http.MethodPost, "/dns_records"
	if len(existing) != 0 {
		if len(existing) != 1 || existing[0].ID == "" || existing[0].Comment != record.Comment ||
			existing[0].Proxied || !strings.EqualFold(existing[0].Name, name) ||
			(existing[0].Type != "A" && existing[0].Type != "AAAA") {
			return "", errors.NewConflictError("DNS record", "name", name)
		}
		if existing[0].matches(record, ip) {
			return name, nil
		}
		method, writePath = http.MethodPut, "/dns_records/"+url.PathEscape(existing[0].ID)
	}
	if err := p.request(ctx, method, writePath, &record, nil); err != nil {
		// The provider can accept the write before the response is lost, or
		// another retry can create the same record first.
		if lookupErr := p.request(ctx, http.MethodGet, path, nil, &existing); lookupErr == nil &&
			len(existing) == 1 && existing[0].Comment == record.Comment && existing[0].matches(record, ip) {
			return name, nil
		}
		return "", err
	}
	return name, nil
}

func (r nodeDNSRecord) matches(expected nodeDNSRecord, ip netip.Addr) bool {
	current, err := netip.ParseAddr(r.Content)
	return err == nil && current.Unmap() == ip && r.Type == expected.Type &&
		strings.EqualFold(r.Name, expected.Name) && !r.Proxied
}

var nonPublicNodeDNSNetworks = [...]netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("3fff::/20"),
}

func nodePublicIP(address string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(address)
	ip = ip.Unmap()
	if err != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.Zone() != "" {
		return ip, false
	}
	for _, prefix := range nonPublicNodeDNSNetworks {
		if prefix.Contains(ip) {
			return ip, false
		}
	}
	return ip, true
}

func (h *NodeHandler) provisionNodeDNS(ctx context.Context, n *node.Node) (string, error) {
	if n.AccessDomain != "" && !n.AccessDNSManaged {
		return n.AccessDomain, nil
	}
	if h.nodeDNS == nil {
		return "", errors.NewBadRequestError("面板尚未配置自动 DNS")
	}
	ip, ok := nodePublicIP(n.Address)
	if !ok {
		return "", errors.NewBadRequestError("请先保存节点的公网 IPv4 或 IPv6 地址")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	// One panel process serializes provider changes and rechecks the address
	// after taking the slot. Database updates still compare the saved address.
	if h.nodeDNS.writes != nil {
		select {
		case h.nodeDNS.writes <- struct{}{}:
			defer func() { <-h.nodeDNS.writes }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	saved, err := h.nodeService.GetByID(ctx, n.ID)
	if err != nil {
		return "", err
	}
	if saved.Address != n.Address || saved.AccessDomain != n.AccessDomain || saved.AccessDNSManaged != n.AccessDNSManaged {
		return "", errors.NewConflictError("node", "id", n.ID)
	}
	domain, err := h.nodeDNS.ensure(ctx, saved, ip)
	if err != nil {
		return "", err
	}
	if err := h.nodeService.SetGeneratedAccessDomain(ctx, saved, domain); err != nil {
		return "", err
	}
	return domain, nil
}

func (h *NodeHandler) CreateAccessDomain(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点 ID 无效"})
		return
	}
	n, err := h.nodeService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到节点"})
		return
	}
	domain, err := h.provisionNodeDNS(c.Request.Context(), n)
	if err != nil {
		h.logger.Warn("Failed to configure node access DNS", logger.F("node_id", id), logger.Err(err))
		status, message := http.StatusBadGateway, "DNS 创建失败，请检查 Cloudflare 配置后重试"
		if errors.IsConflict(err) {
			status, message = http.StatusConflict, "已有同名 DNS 记录或节点配置已变化，请刷新后检查"
		} else if appErr, ok := err.(*errors.AppError); ok && appErr.Code == errors.ErrCodeBadRequest {
			status, message = http.StatusBadRequest, appErr.Message
		}
		c.JSON(status, gin.H{"error": message})
		return
	}
	if n.AccessDomain == "" || n.AccessDNSManaged {
		emitAudit(c, h.auditService, monitor.AuditEntry{Action: monitor.ActionNodeUpdate, ResourceType: monitor.ResourceNode, ResourceID: strconv.FormatInt(id, 10), Details: map[string]any{"access_domain": domain}})
	}
	c.JSON(http.StatusOK, gin.H{"access_domain": domain, "access_dns_managed": n.AccessDomain == "" || n.AccessDNSManaged})
}

// resolveNodeListIPs resolves each hostname once, with a shared deadline for the page.
func resolveNodeListIPs(ctx context.Context, nodes []*NodeResponse) {
	byHost := make(map[string][]*NodeResponse)
	for _, n := range nodes {
		if ip, err := netip.ParseAddr(n.Address); err == nil {
			n.ResolvedIPs = []string{ip.Unmap().String()}
		} else {
			byHost[n.Address] = append(byHost[n.Address], n)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	jobs := make(chan string, len(byHost))
	for host := range byHost {
		jobs <- host
	}
	close(jobs)

	var workers sync.WaitGroup
	for range min(8, len(byHost)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for host := range jobs {
				if ctx.Err() != nil {
					return
				}
				ips := resolveNodeIPs(ctx, host)
				for _, n := range byHost[host] {
					n.ResolvedIPs = ips
				}
			}
		}()
	}
	workers.Wait()
}

func resolveNodeIPs(ctx context.Context, address string) []string {
	if ip, err := netip.ParseAddr(address); err == nil {
		return []string{ip.Unmap().String()}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", address)
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{}, len(ips))
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		value := ip.Unmap().String()
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}
