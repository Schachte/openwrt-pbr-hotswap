package config

import (
	"encoding/json"
	"os"
	"sync"
)

type VPNInterface struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type Config struct {
	mu         sync.RWMutex `json:"-"`
	configPath string       `json:"-"`

	ListenAddr string `json:"listen_addr"`

	VPNInterface string `json:"vpn_interface"`

	VPNInterfaces    []VPNInterface `json:"vpn_interfaces"`
	ActiveInterface  string         `json:"active_interface"`
	DefaultInterface string         `json:"default_interface"`

	DHCPLeasesPath string `json:"dhcp_leases_path"`
	EthersPath     string `json:"ethers_path"`
	HostsPath      string `json:"hosts_path"`
	DatabasePath   string `json:"database_path"`

	AccentColor string `json:"accent_color"`

	FriendlyNames map[string]string `json:"friendly_names"`
}

func (c *Config) GetAccentColor() string {
	if c.AccentColor != "" {
		return c.AccentColor
	}
	return "#f59e0b"
}

func (c *Config) GetActiveInterface() string {
	if c.ActiveInterface != "" {
		return c.ActiveInterface
	}
	// Fall back to first VPN interface (not default_interface which is typically br-lan)
	if len(c.VPNInterfaces) > 0 {
		return c.VPNInterfaces[0].Name
	}
	return c.VPNInterface
}

func (c *Config) GetDefaultInterface() string {
	if c.DefaultInterface != "" {
		return c.DefaultInterface
	}
	if len(c.VPNInterfaces) > 0 {
		return c.VPNInterfaces[0].Name
	}
	return c.VPNInterface
}

func (c *Config) GetVPNInterfaces() []VPNInterface {
	if len(c.VPNInterfaces) > 0 {
		return c.VPNInterfaces
	}

	if c.VPNInterface != "" {
		return []VPNInterface{{Name: c.VPNInterface, DisplayName: c.VPNInterface}}
	}
	return nil
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddr:     ":8080",
		VPNInterface:   "vpn_amsterdam",
		DHCPLeasesPath: "/tmp/dhcp.leases",
		EthersPath:     "/etc/ethers",
		HostsPath:      "/etc/hosts",
		DatabasePath:   "/apps/pbr-vpn-prefs.json",
		FriendlyNames:  make(map[string]string),
	}
}

func (c *Config) LoadFromFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, c); err != nil {
		return err
	}

	c.configPath = path
	return nil
}

func (c *Config) SaveToFile() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.configPath == "" {
		return nil
	}

	// Read existing config to preserve fields like VPNInterfaces
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(c.configPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	// Only update runtime-modifiable fields, preserve the rest
	existing["active_interface"] = c.ActiveInterface
	existing["accent_color"] = c.AccentColor

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0644)
}

func (c *Config) SetActiveInterface(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ActiveInterface = name
}
