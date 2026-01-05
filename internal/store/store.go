package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type DevicePrefs struct {
	MAC        string `json:"mac"`
	CustomName string `json:"custom_name"`
	Favorite   bool   `json:"favorite"`
	Hidden     bool   `json:"hidden"`
}

type storeData struct {
	Devices map[string]*DevicePrefs `json:"devices"`
}

type Store struct {
	path string
	data storeData
	mu   sync.RWMutex
}

func New(filePath string) (*Store, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	s := &Store{
		path: filePath,
		data: storeData{Devices: make(map[string]*DevicePrefs)},
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.data)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) GetDevicePrefs(mac string) (*DevicePrefs, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if p, ok := s.data.Devices[mac]; ok {
		return p, nil
	}
	return &DevicePrefs{MAC: mac}, nil
}

func (s *Store) GetAllPrefs() (map[string]*DevicePrefs, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*DevicePrefs)
	for k, v := range s.data.Devices {
		result[k] = v
	}
	return result, nil
}

func (s *Store) SetFavorite(mac string, favorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.data.Devices[mac]
	if !ok {
		p = &DevicePrefs{MAC: mac}
		s.data.Devices[mac] = p
	}
	p.Favorite = favorite
	return s.save()
}

func (s *Store) SetCustomName(mac string, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.data.Devices[mac]
	if !ok {
		p = &DevicePrefs{MAC: mac}
		s.data.Devices[mac] = p
	}
	p.CustomName = name
	return s.save()
}

func (s *Store) SetHidden(mac string, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.data.Devices[mac]
	if !ok {
		p = &DevicePrefs{MAC: mac}
		s.data.Devices[mac] = p
	}
	p.Hidden = hidden
	return s.save()
}

func (s *Store) Close() error {
	return nil
}
