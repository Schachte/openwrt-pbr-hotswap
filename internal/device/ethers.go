package device

import (
	"bufio"
	"os"
	"strings"
)

type StaticDiscoverer struct {
	EthersPath    string
	HostsPath     string
	FriendlyNames map[string]string
}

func NewStaticDiscoverer(ethersPath, hostsPath string, friendlyNames map[string]string) *StaticDiscoverer {
	if friendlyNames == nil {
		friendlyNames = make(map[string]string)
	}
	return &StaticDiscoverer{
		EthersPath:    ethersPath,
		HostsPath:     hostsPath,
		FriendlyNames: friendlyNames,
	}
}

func (s *StaticDiscoverer) Discover() ([]Device, error) {

	hostnames := s.parseHosts()

	file, err := os.Open(s.EthersPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Device{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var devices []Device
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		mac := strings.ToUpper(parts[0])
		ipOrHost := parts[1]

		ip := ipOrHost
		hostname := ""

		if !isIPAddress(ipOrHost) {

			hostname = ipOrHost
			if resolvedIP, ok := hostnames[hostname]; ok {
				ip = resolvedIP
			} else {

				continue
			}
		} else {

			hostname = s.getHostnameForIP(hostnames, ip)
		}

		friendlyName := hostname
		if fn, ok := s.FriendlyNames[ip]; ok {
			friendlyName = fn
		}

		devices = append(devices, Device{
			IP:           ip,
			MAC:          mac,
			Hostname:     hostname,
			FriendlyName: friendlyName,
			Source:       SourceStatic,
		})
	}

	return devices, scanner.Err()
}

func (s *StaticDiscoverer) parseHosts() map[string]string {

	hostnames := make(map[string]string)

	file, err := os.Open(s.HostsPath)
	if err != nil {
		return hostnames
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			ip := parts[0]

			for _, hostname := range parts[1:] {

				if ip == "127.0.0.1" || ip == "::1" {
					continue
				}

				if _, exists := hostnames[ip]; !exists {
					hostnames[ip] = hostname
				}

				hostnames[hostname] = ip
			}
		}
	}

	return hostnames
}

func (s *StaticDiscoverer) getHostnameForIP(hostnames map[string]string, ip string) string {
	if hostname, ok := hostnames[ip]; ok {
		return hostname
	}
	return ""
}

func isIPAddress(s string) bool {

	if strings.Contains(s, ".") {

		parts := strings.Split(s, ".")
		if len(parts) == 4 {
			return true
		}
	}
	if strings.Contains(s, ":") {

		return true
	}
	return false
}
