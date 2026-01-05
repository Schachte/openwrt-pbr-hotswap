package server

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/schachte/pbr-vpn/internal/device"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type DeviceListResponse struct {
	Devices []device.Device `json:"devices"`
}

type ToggleResponse struct {
	Success    bool          `json:"success"`
	Device     device.Device `json:"device,omitempty"`
	Message    string        `json:"message"`
	VPNEnabled bool          `json:"vpn_enabled"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type IndexData struct {
	VPNInterface        string
	VPNInterfaceDisplay string
	DefaultInterface    string
	VPNInterfaces       []InterfaceInfo
	ServiceRunning      bool
	Devices             []device.Device
	HiddenCount         int
	VisibleCount        int
	LastUpdated         string
	Version             string
	AccentColor         string
	ClientIP            string
}

type InterfaceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Active      bool   `json:"active"`
}

type FavoriteResponse struct {
	Success  bool   `json:"success"`
	MAC      string `json:"mac"`
	Favorite bool   `json:"favorite"`
}

type RenameResponse struct {
	Success    bool   `json:"success"`
	MAC        string `json:"mac"`
	CustomName string `json:"custom_name"`
}

type HiddenResponse struct {
	Success bool   `json:"success"`
	MAC     string `json:"mac"`
	Hidden  bool   `json:"hidden"`
}

type InterfacesResponse struct {
	Success    bool            `json:"success"`
	Interfaces []InterfaceInfo `json:"interfaces"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	devices, err := s.discoverer.Discover()
	if err != nil {
		log.Printf("Error discovering devices: %v", err)
		devices = []device.Device{}
	}

	devices = s.enrichDevices(devices)

	hiddenCount := 0
	for _, d := range devices {
		if d.Hidden {
			hiddenCount++
		}
	}
	visibleCount := len(devices) - hiddenCount

	interfaces := s.buildInterfaceList()

	activeInterface := s.config.GetActiveInterface()
	activeDisplayName := activeInterface
	for _, iface := range interfaces {
		if iface.Active {
			if iface.DisplayName != "" {
				activeDisplayName = iface.DisplayName
			}
			break
		}
	}

	data := IndexData{
		VPNInterface:        activeInterface,
		VPNInterfaceDisplay: activeDisplayName,
		DefaultInterface:    s.config.GetDefaultInterface(),
		VPNInterfaces:       interfaces,
		ServiceRunning:      true,
		Devices:             devices,
		HiddenCount:         hiddenCount,
		VisibleCount:        visibleCount,
		LastUpdated:         time.Now().Format("2006-01-02 15:04:05"),
		Version:             s.version,
		AccentColor:         s.config.GetAccentColor(),
		ClientIP:            getClientIP(r),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Template rendering error", http.StatusInternalServerError)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   s.version,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.discoverer.Discover()
	if err != nil {
		log.Printf("Error discovering devices: %v", err)
		writeError(w, "Failed to discover devices", http.StatusInternalServerError)
		return
	}

	resp := DeviceListResponse{Devices: devices}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleToggleDevice(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		writeError(w, "IP address required", http.StatusBadRequest)
		return
	}

	if !isValidIP(ip) {
		writeError(w, "Invalid IP address format", http.StatusBadRequest)
		return
	}

	devices, _ := s.discoverer.Discover()
	var deviceName string
	for _, d := range devices {
		if d.IP == ip {
			if d.FriendlyName != "" {
				deviceName = d.FriendlyName
			} else if d.Hostname != "" {
				deviceName = d.Hostname
			}
			break
		}
	}

	enabled, err := s.pbrManager.TogglePolicy(ip, deviceName)
	if err != nil {
		log.Printf("Error toggling policy for %s: %v", ip, err)
		writeError(w, "Failed to toggle VPN routing: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var updatedDevice device.Device
	devices, _ = s.discoverer.Discover()
	for _, d := range devices {
		if d.IP == ip {
			updatedDevice = d
			break
		}
	}

	message := "VPN routing disabled"
	if enabled {
		message = "VPN routing enabled"
	}
	message += " for " + ip

	resp := ToggleResponse{
		Success:    true,
		Device:     updatedDevice,
		Message:    message,
		VPNEnabled: enabled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Success: false,
		Error:   message,
	})
}

func isValidIP(ip string) bool {
	parts := 0
	current := ""
	for _, c := range ip {
		if c == '.' {
			if current == "" || len(current) > 3 {
				return false
			}
			parts++
			current = ""
		} else if c >= '0' && c <= '9' {
			current += string(c)
		} else {
			return false
		}
	}
	if current == "" || len(current) > 3 {
		return false
	}
	return parts == 3
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) handleFavorite(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	if mac == "" {
		writeError(w, "MAC address required", http.StatusBadRequest)
		return
	}

	favoriteStr := r.URL.Query().Get("favorite")
	favorite := favoriteStr == "true" || favoriteStr == "1"

	if err := s.store.SetFavorite(mac, favorite); err != nil {
		log.Printf("Error setting favorite for %s: %v", mac, err)
		writeError(w, "Failed to set favorite", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FavoriteResponse{
		Success:  true,
		MAC:      mac,
		Favorite: favorite,
	})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	if mac == "" {
		writeError(w, "MAC address required", http.StatusBadRequest)
		return
	}

	name := r.URL.Query().Get("name")

	if err := s.store.SetCustomName(mac, name); err != nil {
		log.Printf("Error renaming device %s: %v", mac, err)
		writeError(w, "Failed to rename device", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RenameResponse{
		Success:    true,
		MAC:        mac,
		CustomName: name,
	})
}

func (s *Server) handleHidden(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	if mac == "" {
		writeError(w, "MAC address required", http.StatusBadRequest)
		return
	}

	hiddenStr := r.URL.Query().Get("hidden")
	hidden := hiddenStr == "true" || hiddenStr == "1"

	if err := s.store.SetHidden(mac, hidden); err != nil {
		log.Printf("Error setting hidden for %s: %v", mac, err)
		writeError(w, "Failed to set hidden", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HiddenResponse{
		Success: true,
		MAC:     mac,
		Hidden:  hidden,
	})
}

func (s *Server) handleListInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces := s.buildInterfaceList()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(InterfacesResponse{
		Success:    true,
		Interfaces: interfaces,
	})
}

func (s *Server) handleSetInterface(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, "Interface name required", http.StatusBadRequest)
		return
	}

	found := false
	for _, iface := range s.config.GetVPNInterfaces() {
		if iface.Name == name {
			found = true
			break
		}
	}
	if !found {
		writeError(w, "Invalid interface name", http.StatusBadRequest)
		return
	}

	s.config.SetActiveInterface(name)
	s.pbrManager.SetInterface(name)

	if err := s.config.SaveToFile(); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"interface": name,
	})
}

func (s *Server) enrichDevices(devices []device.Device) []device.Device {
	prefs, err := s.store.GetAllPrefs()
	if err != nil {
		log.Printf("Error getting device prefs: %v", err)
		return devices
	}

	ifaceDisplayNames := make(map[string]string)
	for _, iface := range s.config.GetVPNInterfaces() {
		if iface.DisplayName != "" {
			ifaceDisplayNames[iface.Name] = iface.DisplayName
		} else {
			ifaceDisplayNames[iface.Name] = iface.Name
		}
	}

	var favorites, others []device.Device
	for i := range devices {
		if p, ok := prefs[devices[i].MAC]; ok {
			devices[i].CustomName = p.CustomName
			devices[i].Favorite = p.Favorite
			devices[i].Hidden = p.Hidden
		}
		if devices[i].VPNInterface != "" {
			if displayName, ok := ifaceDisplayNames[devices[i].VPNInterface]; ok {
				devices[i].VPNInterfaceDisplay = displayName
			} else {
				devices[i].VPNInterfaceDisplay = devices[i].VPNInterface
			}
		}
		if devices[i].Favorite {
			favorites = append(favorites, devices[i])
		} else {
			others = append(others, devices[i])
		}
	}

	return append(favorites, others...)
}

func (s *Server) buildInterfaceList() []InterfaceInfo {
	active := s.config.GetActiveInterface()
	var interfaces []InterfaceInfo
	for _, iface := range s.config.GetVPNInterfaces() {
		interfaces = append(interfaces, InterfaceInfo{
			Name:        iface.Name,
			DisplayName: iface.DisplayName,
			Active:      iface.Name == active,
		})
	}
	return interfaces
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Server restarting...",
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		// Try OpenWRT init.d restart first
		if _, err := os.Stat("/etc/init.d/pbr-vpn"); err == nil {
			exec.Command("/etc/init.d/pbr-vpn", "restart").Start()
		} else {
			// Fallback: exit and let service manager restart
			os.Exit(0)
		}
	}()
}

func (s *Server) handleSwapInterface(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		writeError(w, "IP address required", http.StatusBadRequest)
		return
	}

	if !isValidIP(ip) {
		writeError(w, "Invalid IP address format", http.StatusBadRequest)
		return
	}

	devices, _ := s.discoverer.Discover()
	var deviceName string
	for _, d := range devices {
		if d.IP == ip {
			if d.FriendlyName != "" {
				deviceName = d.FriendlyName
			} else if d.Hostname != "" {
				deviceName = d.Hostname
			}
			break
		}
	}

	err := s.pbrManager.SwapInterface(ip, deviceName)
	if err != nil {
		log.Printf("Error swapping interface for %s: %v", ip, err)
		writeError(w, "Failed to swap interface: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var updatedDevice device.Device
	devices, _ = s.discoverer.Discover()
	for _, d := range devices {
		if d.IP == ip {
			updatedDevice = d
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"device":  updatedDevice,
		"message": "Interface swapped for " + ip,
	})
}

func (s *Server) handlePublicIP(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://ip-check-perf.radar.cloudflare.com/api/info")
	if err != nil {
		writeError(w, "Failed to fetch public IP", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var cfResp struct {
		IP      string `json:"ip_address"`
		Country string `json:"country"`
		City    string `json:"city"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		writeError(w, "Failed to parse response", http.StatusInternalServerError)
		return
	}

	// Return in a consistent format
	result := map[string]string{
		"ip":           cfResp.IP,
		"country_code": cfResp.Country,
		"country":      cfResp.Country,
		"city":         cfResp.City,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
