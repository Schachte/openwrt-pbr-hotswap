package config

import (
	"encoding/json"
	"os"
)

type VPNInterface struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type Config struct {
	ListenAddr string `json:"listen_addr"`

	VPNInterface string `json:"vpn_interface"`

	VPNInterfaces   []VPNInterface `json:"vpn_interfaces"`
	ActiveInterface string         `json:"active_interface"`

	DHCPLeasesPath string `json:"dhcp_leases_path"`
	EthersPath     string `json:"ethers_path"`
	HostsPath      string `json:"hosts_path"`
	DatabasePath   string `json:"database_path"`

	FriendlyNames map[string]string `json:"friendly_names"`
}

func (c *Config) GetActiveInterface() string {
	if c.ActiveInterface != "" {
		return c.ActiveInterface
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
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, c)
}
