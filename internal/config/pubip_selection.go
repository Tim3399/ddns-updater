package config

import "github.com/qdm12/ddns-updater/pkg/publicip/dns"

func expandDNSProviders(providerStrings []string) (providers []dns.Provider) {
	uniqueProviders := make(map[string]struct{}, len(providerStrings))
	for _, provider := range providerStrings {
		if provider != all {
			uniqueProviders[provider] = struct{}{}
			continue
		}

		for _, provider := range dns.ListProviders() {
			uniqueProviders[string(provider)] = struct{}{}
		}
	}

	providers = make([]dns.Provider, 0, len(uniqueProviders))
	for providerString := range uniqueProviders {
		providers = append(providers, dns.Provider(providerString))
	}
	return providers
}
