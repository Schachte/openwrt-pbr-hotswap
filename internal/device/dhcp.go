package device

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type DHCPDiscoverer struct {
	LeasesPath    string
	FriendlyNames map[string]string
}

func NewDHCPDiscoverer(leasesPath string, friendlyNames map[string]string) *DHCPDiscoverer {
	if friendlyNames == nil {
		friendlyNames = make(map[string]string)
	}
	return &DHCPDiscoverer{
		LeasesPath:    leasesPath,
		FriendlyNames: friendlyNames,
	}
}

func (d *DHCPDiscoverer) Discover() ([]Device, error) {
	file, err := os.Open(d.LeasesPath)
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
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		expiry, _ := strconv.ParseInt(parts[0], 10, 64)
		mac := strings.ToUpper(parts[1])
		ip := parts[2]
		hostname := parts[3]

		if hostname == "*" {
			hostname = ""
		}

		friendlyName := hostname
		if fn, ok := d.FriendlyNames[ip]; ok {
			friendlyName = fn
		}

		devices = append(devices, Device{
			IP:           ip,
			MAC:          mac,
			Hostname:     hostname,
			FriendlyName: friendlyName,
			Source:       SourceDHCP,
			LeaseExpiry:  expiry,
		})
	}

	return devices, scanner.Err()
}
