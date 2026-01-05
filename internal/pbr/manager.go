package pbr

import (
	"fmt"
	"strings"
)

type Manager interface {
	ListPolicies() ([]Policy, error)
	GetPolicyForIP(ip string) (*Policy, error)
	AddPolicy(ip string, deviceName string) error
	RemovePolicy(index int) error
	TogglePolicy(ip string, deviceName string) (enabled bool, err error)
	SwapInterface(ip string, deviceName string) error
	Reload() error
	SetInterface(name string)
	GetInterface() string
}

type UCIManager struct {
	VPNInterface string
	executor     CommandExecutor
}

func NewUCIManager(vpnInterface string) *UCIManager {
	return &UCIManager{
		VPNInterface: vpnInterface,
		executor:     &DefaultExecutor{},
	}
}

func (m *UCIManager) WithExecutor(e CommandExecutor) *UCIManager {
	m.executor = e
	return m
}

func (m *UCIManager) ListPolicies() ([]Policy, error) {
	output, err := m.executor.Execute("uci", "show", "pbr")
	if err != nil {

		if strings.Contains(string(output), "Entry not found") {
			return []Policy{}, nil
		}
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	return ParseUCIOutput(string(output))
}

func (m *UCIManager) GetPolicyForIP(ip string) (*Policy, error) {
	policies, err := m.ListPolicies()
	if err != nil {
		return nil, err
	}

	// First try to find policy for current interface
	for _, p := range policies {
		if p.SrcAddr == ip && p.Interface == m.VPNInterface {
			return &p, nil
		}
	}
	// If not found, find any VPN policy for this IP (for toggling off)
	for _, p := range policies {
		if p.SrcAddr == ip && strings.HasPrefix(p.Interface, "vpn_") {
			return &p, nil
		}
	}
	return nil, nil
}

func (m *UCIManager) AddPolicy(ip string, deviceName string) error {

	safeName := sanitizePolicyName(deviceName)
	if safeName == "" {
		safeName = strings.ReplaceAll(ip, ".", "_")
	}
	policyName := fmt.Sprintf("%s_VPN", safeName)

	commands := [][]string{
		{"uci", "add", "pbr", "policy"},
		{"uci", "set", fmt.Sprintf("pbr.@policy[-1].name=%s", policyName)},
		{"uci", "set", fmt.Sprintf("pbr.@policy[-1].src_addr=%s", ip)},
		{"uci", "set", fmt.Sprintf("pbr.@policy[-1].interface=%s", m.VPNInterface)},
		{"uci", "set", "pbr.@policy[-1].enabled=1"},
		{"uci", "commit", "pbr"},
	}

	for _, cmd := range commands {
		if _, err := m.executor.Execute(cmd[0], cmd[1:]...); err != nil {
			return fmt.Errorf("failed to execute %v: %w", cmd, err)
		}
	}

	return m.Reload()
}

func (m *UCIManager) RemovePolicy(index int) error {
	_, err := m.executor.Execute("uci", "delete", fmt.Sprintf("pbr.@policy[%d]", index))
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	if _, err := m.executor.Execute("uci", "commit", "pbr"); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return m.Reload()
}

func (m *UCIManager) RemovePolicyByName(name string) error {
	// Find the policy index by iterating through policies
	policies, err := m.ListPolicies()
	if err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}

	for _, p := range policies {
		if p.Name == name {
			return m.RemovePolicy(p.Index)
		}
	}

	return fmt.Errorf("policy with name '%s' not found", name)
}

func (m *UCIManager) TogglePolicy(ip string, deviceName string) (enabled bool, err error) {
	policy, err := m.GetPolicyForIP(ip)
	if err != nil {
		return false, err
	}

	if policy == nil {
		// No policy exists, create one
		if err := m.AddPolicy(ip, deviceName); err != nil {
			return false, err
		}
		return true, nil
	}

	// Policy exists - remove it (toggle off)
	// Use index directly since we just fetched fresh policy data
	if err := m.RemovePolicy(policy.Index); err != nil {
		return false, err
	}
	return false, nil
}

// SwapInterface removes existing policy and creates new one for current interface
func (m *UCIManager) SwapInterface(ip string, deviceName string) error {
	policy, err := m.GetPolicyForIP(ip)
	if err != nil {
		return err
	}

	if policy == nil {
		return m.AddPolicy(ip, deviceName)
	}

	// Remove old policy
	if err := m.RemovePolicy(policy.Index); err != nil {
		return err
	}

	// Add new policy for current interface
	return m.AddPolicy(ip, deviceName)
}

func (m *UCIManager) Reload() error {
	_, err := m.executor.Execute("/etc/init.d/pbr", "reload")
	if err != nil {
		_, err = m.executor.Execute("/etc/init.d/pbr", "restart")
	}
	return err
}

func (m *UCIManager) SetInterface(name string) {
	m.VPNInterface = name
}

func (m *UCIManager) GetInterface() string {
	return m.VPNInterface
}

func sanitizePolicyName(name string) string {
	if name == "" {
		return ""
	}

	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		case r == ' ':
			return '_'
		default:
			return -1
		}
	}, name)

	return strings.Trim(safe, "_")
}
