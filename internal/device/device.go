package device

type DeviceSource string

const (
	SourceDHCP   DeviceSource = "dhcp"
	SourceStatic DeviceSource = "static"
)

type Device struct {
	IP           string       `json:"ip"`
	MAC          string       `json:"mac"`
	Hostname     string       `json:"hostname"`
	FriendlyName string       `json:"friendly_name"`
	CustomName   string       `json:"custom_name,omitempty"`
	Source       DeviceSource `json:"source"`
	VPNEnabled   bool         `json:"vpn_enabled"`
	VPNInterface string       `json:"vpn_interface,omitempty"`
	Favorite     bool         `json:"favorite"`
	Hidden       bool         `json:"hidden"`
	PolicyName   string       `json:"policy_name,omitempty"`
	LeaseExpiry  int64        `json:"lease_expiry,omitempty"`
}

func (d *Device) DisplayName() string {
	if d.CustomName != "" {
		return d.CustomName
	}
	if d.FriendlyName != "" {
		return d.FriendlyName
	}
	if d.Hostname != "" {
		return d.Hostname
	}
	return d.IP
}

type Discoverer interface {
	Discover() ([]Device, error)
}
