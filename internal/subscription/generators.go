// Package subscription provides subscription link management functionality.
package subscription

import (
	"v/internal/database/repository"
	subgenerators "v/internal/subscription/generators"
)

// extractProxyInfo extracts proxy information from a repository.Proxy.
func extractProxyInfo(proxy *repository.Proxy) (name, server string, port int, settings map[string]interface{}) {
	info := subgenerators.ExtractProxyInfo(proxy)
	return info.Name, info.Server, info.Port, info.Settings
}

func toExternalGeneratorOptions(options *GeneratorOptions) *subgenerators.GeneratorOptions {
	if options == nil {
		return nil
	}
	return &subgenerators.GeneratorOptions{
		SubscriptionName:   options.SubscriptionName,
		ServerHost:         options.ServerHost,
		RenameTemplate:     options.RenameTemplate,
		IncludeProxyGroups: options.IncludeProxyGroups,
		UpdateInterval:     options.UpdateInterval,
	}
}

// generateV2rayN generates V2rayN format subscription content.
func generateV2rayN(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewV2rayNGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}

// generateClash generates Clash format subscription content.
func generateClash(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewClashGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}

// generateClashMeta generates Clash Meta/Mihomo format with extended protocol fields.
func generateClashMeta(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewClashMetaGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}

// generateShadowrocket generates Shadowrocket-specific share-link subscriptions.
func generateShadowrocket(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewShadowrocketGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}

// generateSurge generates Surge format subscription content.
func generateSurge(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewSurgeGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}

// generateQuantumultX generates Quantumult X format subscription content.
func generateQuantumultX(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewQuantumultXGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}

// generateSingbox generates Sing-box format subscription content.
func generateSingbox(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return subgenerators.NewSingboxGenerator().Generate(proxies, toExternalGeneratorOptions(options))
}
