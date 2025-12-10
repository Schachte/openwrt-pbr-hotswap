package device

import (
	"sort"

	"github.com/schachte/pbr-vpn/internal/pbr"
)

type AggregatedDiscoverer struct {
	sources    []Discoverer
	pbrManager pbr.Manager
}

func NewAggregatedDiscoverer(pbrManager pbr.Manager, sources ...Discoverer) *AggregatedDiscoverer {
	return &AggregatedDiscoverer{
		sources:    sources,
		pbrManager: pbrManager,
	}
}

func (a *AggregatedDiscoverer) Discover() ([]Device, error) {
	deviceMap := make(map[string]Device)

	for _, src := range a.sources {
		devices, err := src.Discover()
		if err != nil {

			continue
		}

		for _, d := range devices {

			if existing, ok := deviceMap[d.IP]; ok {
				if existing.Source == SourceStatic {
					continue
				}
			}
			deviceMap[d.IP] = d
		}
	}

	var policyByIP map[string]pbr.Policy
	if a.pbrManager != nil {
		policies, err := a.pbrManager.ListPolicies()
		if err == nil {
			policyByIP = make(map[string]pbr.Policy)
			for _, p := range policies {
				policyByIP[p.SrcAddr] = p
			}
		}
	}

	devices := make([]Device, 0, len(deviceMap))
	for _, d := range deviceMap {
		if policyByIP != nil {
			if policy, ok := policyByIP[d.IP]; ok {
				d.VPNEnabled = policy.Enabled
				d.PolicyName = policy.Name
				d.VPNInterface = policy.Interface
			}
		}
		devices = append(devices, d)
	}

	sort.Slice(devices, func(i, j int) bool {
		return compareIPs(devices[i].IP, devices[j].IP)
	})

	return devices, nil
}

func compareIPs(a, b string) bool {

	partsA := splitIP(a)
	partsB := splitIP(b)

	for i := 0; i < len(partsA) && i < len(partsB); i++ {
		if partsA[i] != partsB[i] {
			return partsA[i] < partsB[i]
		}
	}
	return len(partsA) < len(partsB)
}

func splitIP(ip string) []int {
	var parts []int
	current := 0
	for _, c := range ip {
		if c == '.' {
			parts = append(parts, current)
			current = 0
		} else if c >= '0' && c <= '9' {
			current = current*10 + int(c-'0')
		}
	}
	parts = append(parts, current)
	return parts
}
