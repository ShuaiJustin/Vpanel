package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/ip"
	"v/internal/logger"
)

var nodeCountryAliases = map[string]string{
	"au":                       "澳大利亚",
	"australia":                "澳大利亚",
	"ca":                       "加拿大",
	"canada":                   "加拿大",
	"cn":                       "中国",
	"china":                    "中国",
	"中国":                       "中国",
	"de":                       "德国",
	"germany":                  "德国",
	"fr":                       "法国",
	"france":                   "法国",
	"gb":                       "英国",
	"hk":                       "香港",
	"hong kong":                "香港",
	"hong kong sar":            "香港",
	"jp":                       "日本",
	"japan":                    "日本",
	"kr":                       "韩国",
	"korea, republic of":       "韩国",
	"netherlands":              "荷兰",
	"nl":                       "荷兰",
	"ru":                       "俄罗斯",
	"russia":                   "俄罗斯",
	"sg":                       "新加坡",
	"singapore":                "新加坡",
	"south korea":              "韩国",
	"tw":                       "台湾",
	"taiwan":                   "台湾",
	"uk":                       "英国",
	"united kingdom":           "英国",
	"united states":            "美国",
	"united states of america": "美国",
	"us":                       "美国",
	"俄罗斯":                      "俄罗斯",
	"台湾":                       "台湾",
	"德国":                       "德国",
	"新加坡":                      "新加坡",
	"日本":                       "日本",
	"法国":                       "法国",
	"澳大利亚":                     "澳大利亚",
	"美国":                       "美国",
	"英国":                       "英国",
	"荷兰":                       "荷兰",
	"韩国":                       "韩国",
	"香港":                       "香港",
}

var nodeLocationAliases = map[string]string{
	"ashburn":        "阿什本",
	"california":     "加州",
	"cupertino":      "库比蒂诺",
	"frankfurt":      "法兰克福",
	"fremont":        "弗里蒙特",
	"hong kong":      "香港",
	"london":         "伦敦",
	"los angeles":    "洛杉矶",
	"menlo park":     "门洛帕克",
	"milpitas":       "米尔皮塔斯",
	"mountain view":  "山景城",
	"osaka":          "大阪",
	"palo alto":      "帕洛阿尔托",
	"redwood city":   "红木城",
	"san jose":       "圣何塞",
	"santa clara":    "圣克拉拉",
	"seattle":        "西雅图",
	"seoul":          "首尔",
	"silicon valley": "硅谷",
	"singapore":      "新加坡",
	"sunnyvale":      "森尼韦尔",
	"taipei":         "台北",
	"tokyo":          "东京",
}

var siliconValleyLocationKeys = map[string]struct{}{
	"cupertino":      {},
	"fremont":        {},
	"menlo park":     {},
	"milpitas":       {},
	"mountain view":  {},
	"palo alto":      {},
	"redwood city":   {},
	"san jose":       {},
	"santa clara":    {},
	"silicon valley": {},
	"sunnyvale":      {},
}

// NodeNameSuggestionHandler provides naming suggestions for nodes.
type NodeNameSuggestionHandler struct {
	log            logger.Logger
	geoService     *ip.GeolocationService
	httpClient     *http.Client
	resolveIP      func(context.Context, string) (string, error)
	lookupExternal func(context.Context, string) (*ip.GeoInfo, error)
}

// NodeNameSuggestionResponse represents an automatic node-name suggestion.
type NodeNameSuggestionResponse struct {
	SuggestedName   string `json:"suggested_name"`
	SuggestedRegion string `json:"suggested_region,omitempty"`
	ResolvedIP      string `json:"resolved_ip,omitempty"`
	Country         string `json:"country,omitempty"`
	CountryCode     string `json:"country_code,omitempty"`
	Region          string `json:"region,omitempty"`
	City            string `json:"city,omitempty"`
	LocationLabel   string `json:"location_label,omitempty"`
	Source          string `json:"source"`
}

type ipWhoisLookupResponse struct {
	Success     bool    `json:"success"`
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Message     string  `json:"message"`
	Connection  struct {
		ISP string `json:"isp"`
	} `json:"connection"`
}

// NewNodeNameSuggestionHandler creates a handler for node name suggestions.
func NewNodeNameSuggestionHandler(log logger.Logger, geoService *ip.GeolocationService) *NodeNameSuggestionHandler {
	handler := &NodeNameSuggestionHandler{
		log:        log,
		geoService: geoService,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	handler.resolveIP = handler.defaultResolveIP
	handler.lookupExternal = handler.lookupGeolocationExternally
	return handler
}

// Suggest returns a generated node name based on region and IP geolocation.
func (h *NodeNameSuggestionHandler) Suggest(c *gin.Context) {
	address := strings.TrimSpace(c.Query("address"))
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address is required"})
		return
	}

	response := h.suggestNodeName(c.Request.Context(), address, c.Query("region"))
	c.JSON(http.StatusOK, response)
}

func (h *NodeNameSuggestionHandler) suggestNodeName(ctx context.Context, address, preferredRegion string) *NodeNameSuggestionResponse {
	normalizedAddress := normalizeNodeAddress(address)
	if normalizedAddress == "" {
		normalizedAddress = strings.TrimSpace(address)
	}

	response := &NodeNameSuggestionResponse{
		Source: "fallback",
	}

	resolvedIP, err := h.resolveIP(ctx, normalizedAddress)
	if err != nil && h.log != nil {
		h.log.Warn("解析节点地址失败，使用回退命名规则", logger.Err(err), logger.F("address", normalizedAddress))
	}
	response.ResolvedIP = resolvedIP

	var geoInfo *ip.GeoInfo
	if resolvedIP != "" {
		geoInfo, response.Source = h.lookupGeolocation(ctx, resolvedIP)
	}

	countryLabel := ""
	locationLabel := ""
	if geoInfo != nil {
		countryLabel = localizeCountry(geoInfo.Country, geoInfo.CountryCode)
		response.Country = countryLabel
		response.CountryCode = strings.TrimSpace(geoInfo.CountryCode)
		response.Region = localizeLocation(geoInfo.Region)
		response.City = localizeLocation(geoInfo.City)
		locationLabel = pickLocationLabel(countryLabel, geoInfo.Region, geoInfo.City)
		response.LocationLabel = joinUniqueLabels(countryLabel, locationLabel)
	}

	manualRegion := localizeCountry(preferredRegion, "")
	if manualRegion == "" {
		manualRegion = strings.TrimSpace(preferredRegion)
	}
	if manualRegion != "" {
		response.SuggestedRegion = manualRegion
	} else {
		response.SuggestedRegion = countryLabel
	}

	response.SuggestedName = buildSuggestedNodeName(manualRegion, countryLabel, locationLabel, resolvedIP, normalizedAddress)
	return response
}

func (h *NodeNameSuggestionHandler) lookupGeolocation(ctx context.Context, ipStr string) (*ip.GeoInfo, string) {
	if h.geoService != nil {
		info, err := h.geoService.LookupLocal(ctx, ipStr)
		if err == nil && hasGeolocationDetails(info) {
			return info, "local"
		}
		if err != nil && h.log != nil {
			h.log.Warn("本地节点地理信息查询失败", logger.Err(err), logger.F("ip", ipStr))
		}
	}

	if h.lookupExternal == nil {
		return nil, "fallback"
	}

	info, err := h.lookupExternal(ctx, ipStr)
	if err != nil {
		if h.log != nil {
			h.log.Warn("外部节点地理信息查询失败", logger.Err(err), logger.F("ip", ipStr))
		}
		return nil, "fallback"
	}
	if !hasGeolocationDetails(info) {
		return nil, "fallback"
	}

	if h.geoService != nil {
		_ = h.geoService.Cache(ctx, info)
	}
	return info, "external"
}

func (h *NodeNameSuggestionHandler) defaultResolveIP(ctx context.Context, address string) (string, error) {
	address = normalizeNodeAddress(address)
	if address == "" {
		return "", fmt.Errorf("empty address")
	}

	if parsedIP := net.ParseIP(address); parsedIP != nil {
		return parsedIP.String(), nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, address)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		if addr.IP.IsGlobalUnicast() {
			return addr.IP.String(), nil
		}
	}
	if len(addrs) == 0 || addrs[0].IP == nil {
		return "", fmt.Errorf("no IP found for %s", address)
	}
	return addrs[0].IP.String(), nil
}

func (h *NodeNameSuggestionHandler) lookupGeolocationExternally(ctx context.Context, ipStr string) (*ip.GeoInfo, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://ipwho.is/"+url.PathEscape(ipStr),
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := h.httpClient.Do(request)
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

	return &ip.GeoInfo{
		IP:          ipStr,
		Country:     payload.Country,
		CountryCode: payload.CountryCode,
		Region:      payload.Region,
		City:        payload.City,
		Latitude:    payload.Latitude,
		Longitude:   payload.Longitude,
		ISP:         payload.Connection.ISP,
	}, nil
}

func hasGeolocationDetails(info *ip.GeoInfo) bool {
	if info == nil {
		return false
	}
	return strings.TrimSpace(info.Country) != "" ||
		strings.TrimSpace(info.Region) != "" ||
		strings.TrimSpace(info.City) != ""
}

func normalizeNodeAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	if strings.Contains(address, "://") {
		if parsedURL, err := url.Parse(address); err == nil && parsedURL.Hostname() != "" {
			return parsedURL.Hostname()
		}
	}

	if host, _, err := net.SplitHostPort(address); err == nil {
		return strings.Trim(host, "[]")
	}

	return strings.Trim(address, "[]")
}

func buildSuggestedNodeName(manualRegion, detectedCountry, detectedLocation, resolvedIP, address string) string {
	var parts []string

	switch {
	case manualRegion != "":
		parts = append(parts, manualRegion)
		if detectedCountry != "" && sameRegionLabel(manualRegion, detectedCountry) && detectedLocation != "" {
			parts = append(parts, detectedLocation)
		}
	case detectedCountry != "":
		parts = append(parts, detectedCountry)
		if detectedLocation != "" {
			parts = append(parts, detectedLocation)
		}
	case detectedLocation != "":
		parts = append(parts, detectedLocation)
	}

	if len(parts) == 0 {
		return "节点-" + fallbackAddressLabel(resolvedIP, address)
	}

	suffix := shortNodeNameSuffix(resolvedIP, address)
	if suffix != "" {
		parts = append(parts, suffix)
	}
	return joinUniqueLabels(parts...)
}

func fallbackAddressLabel(resolvedIP, address string) string {
	if parsedIP := net.ParseIP(strings.TrimSpace(resolvedIP)); parsedIP != nil {
		return sanitizeAddressLabel(parsedIP.String())
	}
	return sanitizeAddressLabel(address)
}

func shortNodeNameSuffix(resolvedIP, address string) string {
	if parsedIP := net.ParseIP(strings.TrimSpace(resolvedIP)); parsedIP != nil {
		if ipv4 := parsedIP.To4(); ipv4 != nil {
			return fmt.Sprintf("%d", ipv4[3])
		}

		segments := strings.Split(parsedIP.String(), ":")
		for i := len(segments) - 1; i >= 0; i-- {
			segment := strings.TrimLeft(segments[i], "0")
			if segment != "" {
				return segment
			}
		}
		return "0"
	}

	return sanitizeAddressLabel(address)
}

func sanitizeAddressLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "node"
	}

	var builder strings.Builder
	lastHyphen := false
	for _, char := range value {
		isAllowed := (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9')
		if isAllowed {
			builder.WriteRune(char)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "node"
	}
	if len(result) > 48 {
		return result[:48]
	}
	return result
}

func joinUniqueLabels(labels ...string) string {
	seen := make(map[string]struct{}, len(labels))
	ordered := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key := normalizeRegionKey(label)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ordered = append(ordered, label)
	}
	return strings.Join(ordered, "-")
}

func sameRegionLabel(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return normalizeRegionKey(left) == normalizeRegionKey(right)
}

func normalizeRegionKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func localizeCountry(country, countryCode string) string {
	if label, ok := nodeCountryAliases[normalizeRegionKey(countryCode)]; ok {
		return label
	}
	if label, ok := nodeCountryAliases[normalizeRegionKey(country)]; ok {
		return label
	}
	return strings.TrimSpace(country)
}

func localizeLocation(value string) string {
	if label, ok := nodeLocationAliases[normalizeRegionKey(value)]; ok {
		return label
	}
	return strings.TrimSpace(value)
}

func pickLocationLabel(countryLabel, region, city string) string {
	cityKey := normalizeRegionKey(city)
	if countryLabel == "美国" {
		if _, ok := siliconValleyLocationKeys[cityKey]; ok {
			return "硅谷"
		}
	}

	cityLabel := localizeLocation(city)
	if cityLabel != "" {
		return cityLabel
	}

	regionLabel := localizeLocation(region)
	if regionLabel != "" && !sameRegionLabel(regionLabel, countryLabel) {
		return regionLabel
	}
	return ""
}
