package main

import (
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/schachte/pbr-vpn/internal/config"
	"github.com/schachte/pbr-vpn/internal/device"
	"github.com/schachte/pbr-vpn/internal/pbr"
	"github.com/schachte/pbr-vpn/internal/server"
	"github.com/schachte/pbr-vpn/internal/store"
)

//go:embed web/templates/*
var templateFS embed.FS

//go:embed web/static/*
var staticFS embed.FS

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {

	listenAddr := flag.String("listen", ":8080", "HTTP listen address (e.g., :8080 or 0.0.0.0:8080)")
	vpnInterface := flag.String("vpn-interface", "vpn_amsterdam", "VPN interface name for PBR routing")
	dhcpPath := flag.String("dhcp-leases", "/tmp/dhcp.leases", "Path to DHCP leases file")
	ethersPath := flag.String("ethers", "/etc/ethers", "Path to /etc/ethers file")
	hostsPath := flag.String("hosts", "/etc/hosts", "Path to /etc/hosts file")
	configPath := flag.String("config", "", "Path to JSON config file (optional, overrides other flags)")
	tlsEnabled := flag.Bool("tls", true, "Enable TLS/HTTPS (default: true)")
	tlsCert := flag.String("tls-cert", "/etc/pbr-vpn/cert.pem", "Path to TLS certificate file")
	tlsKey := flag.String("tls-key", "/etc/pbr-vpn/key.pem", "Path to TLS private key file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("pbr-vpn %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	cfg := config.DefaultConfig()

	cfg.ListenAddr = *listenAddr
	cfg.VPNInterface = *vpnInterface
	cfg.DHCPLeasesPath = *dhcpPath
	cfg.EthersPath = *ethersPath
	cfg.HostsPath = *hostsPath
	cfg.TLSEnabled = *tlsEnabled
	cfg.TLSCertPath = *tlsCert
	cfg.TLSKeyPath = *tlsKey

	if *configPath != "" {
		if err := cfg.LoadFromFile(*configPath); err != nil {
			log.Fatalf("Failed to load config from %s: %v", *configPath, err)
		}
		log.Printf("Loaded configuration from %s", *configPath)
	}

	log.Printf("PBR-VPN %s (built %s)", Version, BuildTime)
	log.Printf("VPN Interface: %s", cfg.GetActiveInterface())
	log.Printf("DHCP Leases: %s", cfg.DHCPLeasesPath)
	log.Printf("Ethers File: %s", cfg.EthersPath)
	log.Printf("Database: %s", cfg.DatabasePath)
	if cfg.TLSEnabled {
		log.Printf("TLS: enabled (cert: %s, key: %s)", cfg.TLSCertPath, cfg.TLSKeyPath)
		log.Printf("Starting HTTPS server on %s", cfg.ListenAddr)
	} else {
		log.Printf("Starting HTTP server on %s", cfg.ListenAddr)
	}

	st, err := store.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer st.Close()

	pbrManager := pbr.NewUCIManager(cfg.GetActiveInterface())

	// Reload PBR in background to not block server startup
	go func() {
		if err := pbrManager.Reload(); err != nil {
			log.Printf("Warning: failed to reload PBR on startup: %v", err)
		} else {
			log.Printf("PBR service reloaded on startup")
		}
	}()

	dhcpDiscoverer := device.NewDHCPDiscoverer(cfg.DHCPLeasesPath, cfg.FriendlyNames)
	staticDiscoverer := device.NewStaticDiscoverer(cfg.EthersPath, cfg.HostsPath, cfg.FriendlyNames)
	discoverer := device.NewAggregatedDiscoverer(pbrManager, dhcpDiscoverer, staticDiscoverer)

	srv := server.New(cfg, discoverer, pbrManager, st, templateFS, staticFS, Version)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		os.Exit(0)
	}()

	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" || cfg.TLSKeyPath == "" {
			log.Fatalf("TLS enabled but certificate or key path not specified")
		}

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		httpServer := &http.Server{
			Addr:      cfg.ListenAddr,
			Handler:   srv,
			TLSConfig: tlsConfig,
		}

		if err := httpServer.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath); err != nil {
			log.Fatalf("HTTPS server error: %v", err)
		}
	} else {
		if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}
}
