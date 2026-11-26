package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
)

func TestNodeAccessDomain(t *testing.T) {
	handler, repo, _ := newNodeCertificateAutoMatchHandler(t)
	var records = make(map[string]nodeDNSRecord)
	failWrite := false
	writeCount := 0
	cloudflare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		if r.URL.Path == "/zones/test-zone" {
			_ = json.NewEncoder(w).Encode(gin.H{"success": true, "result": gin.H{"name": "example.com"}})
			return
		}
		require.True(t, strings.HasPrefix(r.URL.Path, "/zones/test-zone/dns_records"))
		if r.Method == http.MethodGet {
			result := []nodeDNSRecord{}
			if record, ok := records[r.URL.Query().Get("name")]; ok {
				result = append(result, record)
			}
			_ = json.NewEncoder(w).Encode(gin.H{"success": true, "result": result})
			return
		}
		require.Contains(t, []string{http.MethodPost, http.MethodPut}, r.Method)
		writeCount++
		if failWrite {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"success":false}`))
			return
		}
		var record nodeDNSRecord
		require.NoError(t, json.NewDecoder(r.Body).Decode(&record))
		if r.Method == http.MethodPut {
			record.ID = records[record.Name].ID
			require.Equal(t, "/zones/test-zone/dns_records/"+record.ID, r.URL.Path)
		} else {
			record.ID = record.Name
			require.Equal(t, "/zones/test-zone/dns_records", r.URL.Path)
		}
		records[record.Name] = record
		_ = json.NewEncoder(w).Encode(gin.H{"success": true, "result": record})
	}))
	defer cloudflare.Close()
	handler.nodeDNS = &nodeDNSProvider{client: cloudflare.Client(), baseURL: cloudflare.URL, token: "test-token", zoneID: "test-zone"}

	router := gin.New()
	router.POST("/nodes", handler.Create)
	router.GET("/nodes/:id", handler.Get)
	router.PUT("/nodes/:id", handler.Update)
	router.POST("/nodes/:id/access-domain", handler.CreateAccessDomain)
	send := func(method, path, payload string, status int) []byte {
		req := httptest.NewRequest(method, path, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, req)
		require.Equal(t, status, result.Code, result.Body.String())
		return result.Body.Bytes()
	}

	created := send(http.MethodPost, "/nodes", `{"name":"node-v6","address":"2607:8700:5501:774c::2","auto_dns":true}`, http.StatusCreated)
	var added NodeWithTokenResponse
	require.NoError(t, json.Unmarshal(created, &added))
	require.Equal(t, "node-1.example.com", added.AccessDomain)
	require.True(t, added.AccessDNSManaged)
	require.Equal(t, "AAAA", records[added.AccessDomain].Type)
	require.Equal(t, "2607:8700:5501:774c::2", records[added.AccessDomain].Content)
	require.False(t, records[added.AccessDomain].Proxied)
	require.Equal(t, 1, records[added.AccessDomain].TTL)
	require.NotEmpty(t, records[added.AccessDomain].Comment)
	send(http.MethodPost, "/nodes/1/access-domain", "", http.StatusOK)
	require.Equal(t, 1, writeCount)
	saved, err := repo.GetByID(t.Context(), added.ID)
	require.NoError(t, err)
	require.Equal(t, "2607:8700:5501:774c::2", saved.Address)
	require.Equal(t, added.AccessDomain, saved.AccessDomain)
	saved.AccessDomain = ""
	saved.Region = "us"
	require.NoError(t, repo.Update(t.Context(), saved))
	saved, err = repo.GetByID(t.Context(), added.ID)
	require.NoError(t, err)
	require.Equal(t, added.AccessDomain, saved.AccessDomain)

	failWrite = true
	failed := send(http.MethodPost, "/nodes", `{"name":"node-v4","address":"174.137.54.225","auto_dns":true}`, http.StatusCreated)
	require.Contains(t, string(failed), "dns_warning")
	failWrite = false
	records["node-2.example.com"] = nodeDNSRecord{Type: "A", Name: "node-2.example.com", Content: "192.0.2.3"}
	send(http.MethodPost, "/nodes/2/access-domain", "", http.StatusConflict)
	require.Equal(t, "192.0.2.3", records["node-2.example.com"].Content)
	delete(records, "node-2.example.com")
	send(http.MethodPost, "/nodes/2/access-domain", "", http.StatusOK)
	require.Equal(t, "A", records["node-2.example.com"].Type)
	require.Equal(t, "174.137.54.225", records["node-2.example.com"].Content)

	send(http.MethodPost, "/nodes", `{"name":"manual","address":"192.0.2.4","auto_dns":true,"access_domain":"manual.example.com"}`, http.StatusCreated)
	require.Equal(t, 3, writeCount)
	require.Error(t, repo.SetGeneratedAccessDomain(t.Context(), 3, "192.0.2.4", "", "node-3.example.com", false))

	require.NoError(t, repo.Create(t.Context(), &repository.Node{Name: "legacy", Address: "localhost", Token: "legacy-token"}))
	detail := send(http.MethodGet, "/nodes/4", "", http.StatusOK)
	require.Contains(t, string(detail), `"address":"localhost"`)
	require.Contains(t, string(detail), `"resolved_ips":[`)
	require.Contains(t, string(detail), `"127.0.0.1"`)
	send(http.MethodPost, "/nodes/4/access-domain", "", http.StatusBadRequest)

	// Changing address families updates the panel-owned record in place.
	updated := send(http.MethodPut, "/nodes/1", `{"address":"174.137.54.226"}`, http.StatusOK)
	require.NoError(t, json.Unmarshal(updated, &added))
	require.Equal(t, "node-1.example.com", added.AccessDomain)
	require.True(t, added.AccessDNSManaged)
	require.Equal(t, "A", records[added.AccessDomain].Type)
	require.Equal(t, "174.137.54.226", records[added.AccessDomain].Content)

	failWrite = true
	updated = send(http.MethodPut, "/nodes/1", `{"address":"2607:8700:5501:774c::3"}`, http.StatusOK)
	var pending NodeResponse
	require.NoError(t, json.Unmarshal(updated, &pending))
	require.Equal(t, "2607:8700:5501:774c::3", pending.Address)
	require.Empty(t, pending.AccessDomain)
	require.True(t, pending.AccessDNSManaged)
	require.NotEmpty(t, pending.DNSWarning)
	saved, err = repo.GetByID(t.Context(), 1)
	require.NoError(t, err)
	require.Empty(t, saved.AccessDomain)
	require.Equal(t, "174.137.54.226", records["node-1.example.com"].Content)
	failWrite = false
	send(http.MethodPost, "/nodes/1/access-domain", "", http.StatusOK)
	require.Equal(t, "AAAA", records["node-1.example.com"].Type)
	require.Equal(t, pending.Address, records["node-1.example.com"].Content)

	// The editor can repair a missing owned record without changing the IP.
	delete(records, "node-1.example.com")
	send(http.MethodPost, "/nodes/1/access-domain", "", http.StatusOK)
	require.Equal(t, pending.Address, records["node-1.example.com"].Content)

	// An automatic sync must leave externally managed DNS alone.
	manualRecord := records["node-1.example.com"]
	manualRecord.Comment = "External DNS"
	manualRecord.Content = "2607:8700:5501:774c::4"
	records[manualRecord.Name] = manualRecord
	send(http.MethodPost, "/nodes/1/access-domain", "", http.StatusConflict)
	require.Equal(t, manualRecord, records[manualRecord.Name])

	// An explicit manual domain bypasses automatic sync on later IP changes.
	updated = send(http.MethodPut, "/nodes/2", `{"address":"174.137.54.227","access_domain":"custom.example.com"}`, http.StatusOK)
	var manual NodeResponse
	require.NoError(t, json.Unmarshal(updated, &manual))
	require.Equal(t, "custom.example.com", manual.AccessDomain)
	require.False(t, manual.AccessDNSManaged)
	require.Equal(t, "174.137.54.225", records["node-2.example.com"].Content)

	// A binding from the old release has no ownership proof, even if its
	// name looks generated. Require an explicit change before replacing its IP.
	require.NoError(t, repo.Create(t.Context(), &repository.Node{
		Name: "previous-release", Address: "174.137.54.228", AccessDomain: "node-5.example.com", Token: "legacy-dns-token",
	}))
	legacyRecord := nodeDNSRecord{ID: "legacy-id", Name: "node-5.example.com", Type: "A", Content: "174.137.54.228"}
	records[legacyRecord.Name] = legacyRecord
	send(http.MethodPut, "/nodes/5", `{"address":"174.137.54.229"}`, http.StatusBadRequest)
	saved, err = repo.GetByID(t.Context(), 5)
	require.NoError(t, err)
	require.Equal(t, "174.137.54.228", saved.Address)
	require.Equal(t, legacyRecord.Name, saved.AccessDomain)
	send(http.MethodPut, "/nodes/5", `{"address":"174.137.54.229","access_domain":""}`, http.StatusOK)
	send(http.MethodPost, "/nodes/5/access-domain", "", http.StatusConflict)
	require.Equal(t, legacyRecord, records[legacyRecord.Name])
}

func TestNodeDNSRejectsNonPublicAddresses(t *testing.T) {
	for _, address := range []string{"localhost", "10.0.0.1", "100.64.0.5", "192.0.2.1", "2001:db8::1", "fe80::1"} {
		_, ok := nodePublicIP(address)
		require.False(t, ok, address)
	}
}

func TestNodeListResolvesIPsOnRequest(t *testing.T) {
	handler, repo, _ := newNodeCertificateAutoMatchHandler(t)
	for _, n := range []*repository.Node{
		{Name: "DNS node", Address: "localhost", Token: "test-dns"},
		{Name: "IPv6 node", Address: "2001:0db8::2", Token: "test-ipv6"},
	} {
		require.NoError(t, repo.Create(t.Context(), n))
	}

	router := gin.New()
	router.GET("/nodes", handler.List)
	list := func(query string) map[string][]string {
		req := httptest.NewRequest(http.MethodGet, "/nodes"+query, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var response struct {
			Nodes []NodeResponse `json:"nodes"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response.Nodes, 2)
		ips := make(map[string][]string)
		for _, n := range response.Nodes {
			ips[n.Address] = n.ResolvedIPs
		}
		return ips
	}

	resolved := list("?resolve_ips=true")
	require.Contains(t, resolved["localhost"], "127.0.0.1")
	require.Equal(t, []string{"2001:db8::2"}, resolved["2001:0db8::2"])
	require.Empty(t, list("")["localhost"])
}

func TestNodeListIncludesRegionsOutsideCurrentPageAndFilters(t *testing.T) {
	handler, repo, _ := newNodeCertificateAutoMatchHandler(t)
	for i, region := range []string{"us", "hk", "hk", ""} {
		require.NoError(t, repo.Create(t.Context(), &repository.Node{
			Name: region, Address: "localhost", Region: region, Token: string(rune('a' + i)),
		}))
	}
	router := gin.New()
	router.GET("/nodes", handler.List)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nodes?region=us&limit=1&include_regions=true", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var response struct {
		Nodes   []NodeResponse `json:"nodes"`
		Regions []string       `json:"regions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Nodes, 1)
	require.Equal(t, "us", response.Nodes[0].Region)
	require.Equal(t, []string{"hk", "us"}, response.Regions)
}
