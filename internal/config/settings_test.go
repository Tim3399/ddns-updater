package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Settings_String(t *testing.T) {
	t.Parallel()

	var defaultSettings Config
	defaultSettings.SetDefaults()

	s := defaultSettings.String()

	expected := `Settings summary:
├── HTTP client
|   └── Timeout: 20s
├── Update
|   ├── Period: 5m0s
|   └── Cooldown: 5m0s
├── Public IP fetching
|   ├── HTTP enabled: yes
|   ├── HTTP IP providers
|   |   └── all
|   ├── HTTP IPv4 providers
|   |   └── all
|   ├── HTTP IPv6 providers
|   |   └── all
|   ├── DNS enabled: yes
|   ├── DNS timeout: 3s
|   └── DNS over TLS providers
|       └── all
├── Resolver: use Go default resolver
├── Server
|   ├── Listening address: :8000
|   └── Root URL: /
├── Health
|   └── Server is disabled
├── Paths
|   ├── Data directory: ./data
|   ├── Config file: ` + filepath.Join("data", "config.json") + `
|   ├── Persist environment config: true
|   └── Umask: system default
├── Backup: disabled
└── Logger
    ├── Level: INFO
    └── Caller: hidden`
	assert.Equal(t, expected, s)
}

func Test_Config_SetDefaults(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		config        Config
		includeConfig bool
	}{
		"config_from_file": {
			includeConfig: true,
		},
		"config_from_environment_persisted": {
			config: Config{Paths: Paths{
				ConfigPersist:     new(true),
				environmentConfig: true,
			}},
			includeConfig: true,
		},
		"config_from_environment_not_persisted": {
			config: Config{Paths: Paths{
				ConfigPersist:     new(false),
				environmentConfig: true,
			}},
			includeConfig: false,
		},
		"include_config_already_set": {
			config: Config{
				Paths: Paths{
					ConfigPersist:     new(false),
					environmentConfig: true,
				},
				Backup: Backup{IncludeConfig: new(true)},
			},
			includeConfig: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testCase.config.SetDefaults()

			assert.Equal(t, testCase.includeConfig, *testCase.config.Backup.IncludeConfig)
		})
	}
}

func Test_Config_Validate(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		config     Config
		errMessage string
	}{
		"default_settings": {},
		"config_not_persisted_and_not_backed_up": {
			config: Config{Paths: Paths{
				ConfigPersist:     new(false),
				environmentConfig: true,
			}},
		},
		"config_not_persisted_and_backed_up": {
			config: Config{
				Paths: Paths{
					ConfigPersist:     new(false),
					environmentConfig: true,
				},
				Backup: Backup{IncludeConfig: new(true)},
			},
			errMessage: "BACKUP_INCLUDE_CONFIG cannot be enabled when CONFIG is set and CONFIG_PERSIST is false",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testCase.config.SetDefaults()

			err := testCase.config.Validate()

			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}
			assert.NoError(t, err)
		})
	}
}
