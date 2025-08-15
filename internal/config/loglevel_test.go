//nolint:errcheck
package config

import (
	"os"
	"testing"
)

func TestResolveLogLevel(t *testing.T) {
	// Save and restore environment
	oldEnv := os.Getenv("DUCK_LOG_LEVEL")
	defer func() {
		if oldEnv != "" {
			os.Setenv("DUCK_LOG_LEVEL", oldEnv)
		} else {
			os.Unsetenv("DUCK_LOG_LEVEL")
		}
	}()

	tests := []struct {
		name        string
		cliLogLevel string
		envLogLevel string
		cfg         *DuckConf
		want        string
	}{
		{
			name:        "CLI flag takes precedence",
			cliLogLevel: "debug",
			envLogLevel: "warn",
			cfg:         &DuckConf{Settings: &Settings{LogLevel: "error"}},
			want:        "debug",
		},
		{
			name:        "Environment variable when no CLI flag",
			cliLogLevel: "",
			envLogLevel: "warn",
			cfg:         &DuckConf{Settings: &Settings{LogLevel: "error"}},
			want:        "warn",
		},
		{
			name:        "Config file when no CLI flag or env",
			cliLogLevel: "",
			envLogLevel: "",
			cfg:         &DuckConf{Settings: &Settings{LogLevel: "error"}},
			want:        "error",
		},
		{
			name:        "Default when nothing set",
			cliLogLevel: "",
			envLogLevel: "",
			cfg:         nil,
			want:        "info",
		},
		{
			name:        "Default when config has no settings",
			cliLogLevel: "",
			envLogLevel: "",
			cfg:         &DuckConf{Settings: nil},
			want:        "info",
		},
		{
			name:        "Default when settings has empty logLevel",
			cliLogLevel: "",
			envLogLevel: "",
			cfg:         &DuckConf{Settings: &Settings{LogLevel: ""}},
			want:        "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envLogLevel != "" {
				os.Setenv("DUCK_LOG_LEVEL", tt.envLogLevel)
			} else {
				os.Unsetenv("DUCK_LOG_LEVEL")
			}

			got := ResolveLogLevel(tt.cliLogLevel, tt.cfg)
			if got != tt.want {
				t.Errorf("ResolveLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSettingsValidation(t *testing.T) {
	tests := []struct {
		name     string
		settings *Settings
		wantErr  bool
	}{
		{
			name:     "nil settings is valid",
			settings: nil,
			wantErr:  false,
		},
		{
			name:     "empty settings is valid",
			settings: &Settings{},
			wantErr:  false,
		},
		{
			name:     "valid error level",
			settings: &Settings{LogLevel: "error"},
			wantErr:  false,
		},
		{
			name:     "valid warn level",
			settings: &Settings{LogLevel: "warn"},
			wantErr:  false,
		},
		{
			name:     "valid info level",
			settings: &Settings{LogLevel: "info"},
			wantErr:  false,
		},
		{
			name:     "valid debug level",
			settings: &Settings{LogLevel: "debug"},
			wantErr:  false,
		},
		{
			name:     "invalid log level",
			settings: &Settings{LogLevel: "invalid"},
			wantErr:  true,
		},
		{
			name:     "empty string log level is valid",
			settings: &Settings{LogLevel: ""},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSettings(tt.settings)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSettingsGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		settings *Settings
		want     string
	}{
		{
			name:     "nil settings returns default",
			settings: nil,
			want:     "info",
		},
		{
			name:     "empty logLevel returns default",
			settings: &Settings{LogLevel: ""},
			want:     "info",
		},
		{
			name:     "configured logLevel is returned",
			settings: &Settings{LogLevel: "debug"},
			want:     "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.settings.GetLogLevel()
			if got != tt.want {
				t.Errorf("Settings.GetLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSettingsGetCacheDir(t *testing.T) {
	tests := []struct {
		name     string
		settings *Settings
		want     string
	}{
		{
			name:     "nil settings returns default",
			settings: nil,
			want:     ".duck/objects",
		},
		{
			name:     "empty cacheDir returns default",
			settings: &Settings{CacheDir: ""},
			want:     ".duck/objects",
		},
		{
			name:     "configured cacheDir is returned",
			settings: &Settings{CacheDir: "/custom/cache"},
			want:     "/custom/cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.settings.GetCacheDir()
			if got != tt.want {
				t.Errorf("Settings.GetCacheDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSettingsIsLocked(t *testing.T) {
	tests := []struct {
		name     string
		settings *Settings
		want     bool
	}{
		{
			name:     "nil settings returns false",
			settings: nil,
			want:     false,
		},
		{
			name:     "default locked is false",
			settings: &Settings{},
			want:     false,
		},
		{
			name:     "explicitly set locked true",
			settings: &Settings{Locked: true},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.settings.IsLocked()
			if got != tt.want {
				t.Errorf("Settings.IsLocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
