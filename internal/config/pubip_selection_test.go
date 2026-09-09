package config

import (
	"testing"

	"github.com/qdm12/ddns-updater/pkg/publicip/dns"
	"github.com/stretchr/testify/assert"
)

func TestExpandDNSProvidersExplicitSelection(t *testing.T) {
	t.Parallel()

	providers := expandDNSProviders([]string{"cloudflare"})
	assert.Equal(t, []dns.Provider{"cloudflare"}, providers)
}

func TestExpandDNSProvidersAll(t *testing.T) {
	t.Parallel()

	providers := expandDNSProviders([]string{all})
	assert.ElementsMatch(t, dns.ListProviders(), providers)
}

func TestExpandDNSProvidersDeduplicates(t *testing.T) {
	t.Parallel()

	providers := expandDNSProviders([]string{"cloudflare", "cloudflare"})
	assert.Equal(t, []dns.Provider{"cloudflare"}, providers)
}
