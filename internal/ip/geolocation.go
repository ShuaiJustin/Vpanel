package ip

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
	"gorm.io/gorm"
)

// GeolocationService provides IP geolocation lookup functionality.
type GeolocationService struct {
	db         *gorm.DB
	reader     *geoip2.Reader
	cacheTTL   time.Duration
	httpClient *http.Client
	mu         sync.RWMutex
}

// GeolocationConfig holds configuration for the geolocation service.
type GeolocationConfig struct {
	DatabasePath string        // Path to MaxMind GeoLite2 database file
	CacheTTL     time.Duration // Cache TTL duration
}

// DefaultGeolocationConfig returns default configuration.
func DefaultGeolocationConfig() *GeolocationConfig {
	return &GeolocationConfig{
		DatabasePath: "data/GeoLite2-City.mmdb",
		CacheTTL:     24 * time.Hour,
	}
}

// NewGeolocationService creates a new GeolocationService instance.
func NewGeolocationService(db *gorm.DB, config *GeolocationConfig) (*GeolocationService, error) {
	if config == nil {
		config = DefaultGeolocationConfig()
	}

	var reader *geoip2.Reader
	var err error

	// Try to open the GeoIP database if path is provided
	if config.DatabasePath != "" {
		reader, err = geoip2.Open(config.DatabasePath)
		if err != nil {
			// Log warning but don't fail - service can work without GeoIP database
			reader = nil
		}
	}

	return &GeolocationService{
		db:       db,
		reader:   reader,
		cacheTTL: config.CacheTTL,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
	}, nil
}

// Close closes the GeoIP database reader.
func (g *GeolocationService) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.reader != nil {
		return g.reader.Close()
	}
	return nil
}

// LookupLocal looks up geolocation using only cache and the local GeoIP database.
// When local data is found it is persisted into the cache for later reuse.
func (g *GeolocationService) LookupLocal(ctx context.Context, ipStr string) (*GeoInfo, error) {
	return g.lookupLocal(ctx, ipStr, true)
}

// LookupFast looks up geolocation using only cached data and the local GeoIP database.
// It never performs network requests or cache writes, making it safe for response paths.
func (g *GeolocationService) LookupFast(ctx context.Context, ipStr string) (*GeoInfo, error) {
	return g.lookupLocal(ctx, ipStr, false)
}

func (g *GeolocationService) lookupLocal(ctx context.Context, ipStr string, persistCache bool) (*GeoInfo, error) {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return &GeoInfo{}, nil
	}

	cached, err := g.getFromCache(ctx, ipStr)
	if err == nil && cached != nil {
		return cached, nil
	}

	info, err := g.lookupFromDatabase(ipStr)
	if err != nil {
		return nil, err
	}
	if persistCache && hasGeolocationDetails(info) {
		_ = g.saveToCache(ctx, info)
	}
	return info, nil
}

// Lookup looks up geolocation information for an IP address.
func (g *GeolocationService) Lookup(ctx context.Context, ipStr string) (*GeoInfo, error) {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return &GeoInfo{}, nil
	}

	info, err := g.LookupLocal(ctx, ipStr)
	if err != nil {
		return nil, err
	}
	if hasGeolocationDetails(info) || !isExternalLookupCandidate(ipStr) {
		return info, nil
	}

	// Fall back to external lookup when local GeoIP is unavailable or has no details.
	externalInfo, err := g.lookupExternally(ctx, ipStr)
	if err == nil && hasGeolocationDetails(externalInfo) {
		_ = g.saveToCache(ctx, externalInfo)
		return externalInfo, nil
	}

	return info, nil
}

// LookupBatch looks up geolocation for multiple IPs.
func (g *GeolocationService) LookupBatch(ctx context.Context, ips []string) (map[string]*GeoInfo, error) {
	results := make(map[string]*GeoInfo)

	for _, ip := range ips {
		info, err := g.Lookup(ctx, ip)
		if err != nil {
			// Continue with other IPs even if one fails
			continue
		}
		results[ip] = info
	}

	return results, nil
}

// Cache stores a geolocation result in the local cache for later reuse.
func (g *GeolocationService) Cache(ctx context.Context, info *GeoInfo) error {
	if info == nil || info.IP == "" {
		return nil
	}
	return g.saveToCache(ctx, info)
}

// lookupFromDatabase performs the actual GeoIP lookup.
func (g *GeolocationService) lookupFromDatabase(ipStr string) (*GeoInfo, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	info := &GeoInfo{IP: ipStr}

	if g.reader == nil {
		// Return empty info if no database is available.
		return info, nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return info, nil
	}

	record, err := g.reader.City(ip)
	if err != nil {
		return info, nil
	}

	info.Country = record.Country.Names["en"]
	info.CountryCode = record.Country.IsoCode
	info.City = record.City.Names["en"]
	if len(record.Subdivisions) > 0 {
		info.Region = record.Subdivisions[0].Names["en"]
	}
	info.Latitude = record.Location.Latitude
	info.Longitude = record.Location.Longitude
	// Note: ISP info requires GeoIP2 ISP database, not available in GeoLite2 City.

	return info, nil
}

// getFromCache retrieves geolocation info from cache.
func (g *GeolocationService) getFromCache(ctx context.Context, ipStr string) (*GeoInfo, error) {
	var cache GeoCache
	err := g.db.WithContext(ctx).Where("ip = ?", ipStr).First(&cache).Error
	if err != nil {
		return nil, err
	}

	// Check if cache is still valid.
	if !cache.IsCacheValid(g.cacheTTL) {
		return nil, gorm.ErrRecordNotFound
	}

	return &GeoInfo{
		IP:          cache.IP,
		Country:     cache.Country,
		CountryCode: cache.CountryCode,
		Region:      cache.Region,
		City:        cache.City,
		Latitude:    cache.Latitude,
		Longitude:   cache.Longitude,
		ISP:         cache.ISP,
	}, nil
}

// saveToCache saves geolocation info to cache.
func (g *GeolocationService) saveToCache(ctx context.Context, info *GeoInfo) error {
	if info == nil || info.IP == "" || !hasGeolocationDetails(info) {
		return nil
	}

	cache := GeoCache{
		IP:          info.IP,
		Country:     info.Country,
		CountryCode: info.CountryCode,
		Region:      info.Region,
		City:        info.City,
		Latitude:    info.Latitude,
		Longitude:   info.Longitude,
		ISP:         info.ISP,
		CachedAt:    time.Now(),
	}

	// Upsert cache entry.
	return g.db.WithContext(ctx).Save(&cache).Error
}

// CleanupExpiredCache removes expired cache entries.
func (g *GeolocationService) CleanupExpiredCache(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-g.cacheTTL)
	result := g.db.WithContext(ctx).
		Where("cached_at < ?", cutoff).
		Delete(&GeoCache{})
	return result.RowsAffected, result.Error
}

// CheckGeoRestriction checks if an IP is allowed based on geo restrictions.
func (g *GeolocationService) CheckGeoRestriction(ctx context.Context, ipStr string, allowedCountries, blockedCountries []string) (*GeoCheckResult, error) {
	info, err := g.Lookup(ctx, ipStr)
	if err != nil {
		return &GeoCheckResult{
			Allowed: true, // Allow if lookup fails
			Reason:  "geolocation lookup failed",
		}, nil
	}

	result := &GeoCheckResult{
		Country:     info.Country,
		CountryCode: info.CountryCode,
		City:        info.City,
	}

	// If no restrictions configured, allow.
	if len(allowedCountries) == 0 && len(blockedCountries) == 0 {
		result.Allowed = true
		return result, nil
	}

	// Check blocked countries first.
	for _, blocked := range blockedCountries {
		if info.CountryCode == blocked {
			result.Allowed = false
			result.Reason = "country is blocked"
			return result, nil
		}
	}

	// If allowed countries list is specified, check if country is in it.
	if len(allowedCountries) > 0 {
		for _, allowed := range allowedCountries {
			if info.CountryCode == allowed {
				result.Allowed = true
				return result, nil
			}
		}
		result.Allowed = false
		result.Reason = "country is not in allowed list"
		return result, nil
	}

	result.Allowed = true
	return result, nil
}

// ReloadDatabase reloads the GeoIP database.
func (g *GeolocationService) ReloadDatabase(path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.reader != nil {
		g.reader.Close()
	}

	reader, err := geoip2.Open(path)
	if err != nil {
		return err
	}

	g.reader = reader
	return nil
}

// IsAvailable checks if the GeoIP database is available.
func (g *GeolocationService) IsAvailable() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.reader != nil
}

type ipWhoisLookupResponse struct {
	Success     bool    `json:"success"`
	Message     string  `json:"message"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Connection  struct {
		ISP string `json:"isp"`
	} `json:"connection"`
}

func (g *GeolocationService) lookupExternally(ctx context.Context, ipStr string) (*GeoInfo, error) {
	if g.httpClient == nil {
		return nil, fmt.Errorf("http client is not configured")
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://ipwho.is/"+url.PathEscape(ipStr),
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := g.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", response.StatusCode)
	}

	var payload ipWhoisLookupResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		if payload.Message == "" {
			payload.Message = "lookup failed"
		}
		return nil, fmt.Errorf("%s", payload.Message)
	}

	return &GeoInfo{
		IP:          ipStr,
		Country:     payload.Country,
		CountryCode: strings.ToUpper(strings.TrimSpace(payload.CountryCode)),
		Region:      payload.Region,
		City:        payload.City,
		Latitude:    payload.Latitude,
		Longitude:   payload.Longitude,
		ISP:         payload.Connection.ISP,
	}, nil
}

func isExternalLookupCandidate(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func hasGeolocationDetails(info *GeoInfo) bool {
	if info == nil {
		return false
	}
	return strings.TrimSpace(info.Country) != "" ||
		strings.TrimSpace(info.CountryCode) != "" ||
		strings.TrimSpace(info.Region) != "" ||
		strings.TrimSpace(info.City) != ""
}
