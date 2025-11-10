package vpn

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the VPN configuration file structure
type Config struct {
	// Application settings
	Database  DatabaseConfig `yaml:"database"`
	Storage   StorageConfig  `yaml:"storage"`

	// VPN settings
	VPNs []VPNConfig `yaml:"vpns"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Path string `yaml:"path"` // Path to SQLite database file
}

// StorageConfig contains storage configuration
type StorageConfig struct {
	MediaDir string `yaml:"media_dir"` // Root directory for downloaded media
}

// VPNConfig represents a single VPN configuration
type VPNConfig struct {
	Name         string            `yaml:"name"`
	Type         string            `yaml:"type"` // "openvpn", "wireguard", "socks5"
	Enabled      bool              `yaml:"enabled"`
	Priority     int               `yaml:"priority"` // Higher priority VPNs are preferred
	MaxBandwidth int64             `yaml:"max_bandwidth,omitempty"`

	// OpenVPN specific
	OpenVPN *OpenVPNConfig `yaml:"openvpn,omitempty"`

	// WireGuard specific
	WireGuard *WireGuardConfig `yaml:"wireguard,omitempty"`

	// SOCKS5 specific
	SOCKS5 *SOCKS5Config `yaml:"socks5,omitempty"`

	// Connection options
	Options ConnectionOptions `yaml:"options,omitempty"`
}

// OpenVPNConfig contains OpenVPN-specific configuration
type OpenVPNConfig struct {
	ConfigFile string `yaml:"config_file"` // Path to .ovpn file
	Username   string `yaml:"username,omitempty"`
	Password   string `yaml:"password,omitempty"`
	AuthFile   string `yaml:"auth_file,omitempty"` // Path to auth file with user/pass
}

// WireGuardConfig contains WireGuard-specific configuration
type WireGuardConfig struct {
	ConfigFile string `yaml:"config_file"` // Path to .conf file
	Interface  string `yaml:"interface"`   // Interface name (e.g., wg0)
}

// SOCKS5Config contains SOCKS5 proxy configuration
type SOCKS5Config struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// ConnectionOptions contains general connection options
type ConnectionOptions struct {
	AutoConnect     bool   `yaml:"auto_connect"`      // Auto-connect on startup
	RetryAttempts   int    `yaml:"retry_attempts"`    // Number of retry attempts
	RetryDelay      int    `yaml:"retry_delay"`       // Delay between retries (seconds)
	HealthCheckURL  string `yaml:"health_check_url"`  // URL to check connection health
	HealthCheckInterval int `yaml:"health_check_interval"` // Health check interval (seconds)
}

// LoadConfig loads VPN configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// validateConfig validates the VPN configuration
func validateConfig(config *Config) error {
	// Validate database config
	if config.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}

	// Validate storage config
	if config.Storage.MediaDir == "" {
		return fmt.Errorf("storage media_dir is required")
	}

	// VPNs are optional - can run without VPNs
	if len(config.VPNs) == 0 {
		return nil // No VPNs is valid
	}

	for i, vpn := range config.VPNs {
		if vpn.Name == "" {
			return fmt.Errorf("VPN #%d: name is required", i+1)
		}

		switch vpn.Type {
		case "openvpn":
			if vpn.OpenVPN == nil {
				return fmt.Errorf("VPN %s: openvpn configuration is required", vpn.Name)
			}
			if vpn.OpenVPN.ConfigFile == "" {
				return fmt.Errorf("VPN %s: openvpn config_file is required", vpn.Name)
			}
		case "wireguard":
			if vpn.WireGuard == nil {
				return fmt.Errorf("VPN %s: wireguard configuration is required", vpn.Name)
			}
			if vpn.WireGuard.ConfigFile == "" {
				return fmt.Errorf("VPN %s: wireguard config_file is required", vpn.Name)
			}
			if vpn.WireGuard.Interface == "" {
				return fmt.Errorf("VPN %s: wireguard interface is required", vpn.Name)
			}
		case "socks5":
			if vpn.SOCKS5 == nil {
				return fmt.Errorf("VPN %s: socks5 configuration is required", vpn.Name)
			}
			if vpn.SOCKS5.Host == "" {
				return fmt.Errorf("VPN %s: socks5 host is required", vpn.Name)
			}
			if vpn.SOCKS5.Port == 0 {
				return fmt.Errorf("VPN %s: socks5 port is required", vpn.Name)
			}
		default:
			return fmt.Errorf("VPN %s: unknown type '%s' (must be openvpn, wireguard, or socks5)", vpn.Name, vpn.Type)
		}
	}

	return nil
}

// SaveConfig saves the configuration to a YAML file
func SaveConfig(config *Config, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
