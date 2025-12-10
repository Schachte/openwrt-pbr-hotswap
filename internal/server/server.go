package server

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/schachte/pbr-vpn/internal/config"
	"github.com/schachte/pbr-vpn/internal/device"
	"github.com/schachte/pbr-vpn/internal/pbr"
	"github.com/schachte/pbr-vpn/internal/store"
)

type Server struct {
	config     *config.Config
	discoverer device.Discoverer
	pbrManager pbr.Manager
	store      *store.Store
	templates  *template.Template
	staticFS   embed.FS
	version    string
	mux        *http.ServeMux
}

func New(cfg *config.Config, discoverer device.Discoverer, pbrManager pbr.Manager, st *store.Store, templateFS embed.FS, staticFS embed.FS, version string) *Server {
	s := &Server{
		config:     cfg,
		discoverer: discoverer,
		pbrManager: pbrManager,
		store:      st,
		staticFS:   staticFS,
		version:    version,
		mux:        http.NewServeMux(),
	}

	tmpl, err := template.ParseFS(templateFS, "web/templates/*.html")
	if err != nil {
		tmpl = template.Must(template.New("index.html").Parse(fallbackTemplate))
	}
	s.templates = tmpl

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {

	staticSub, _ := fs.Sub(s.staticFS, "web/static")
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	s.mux.HandleFunc("/favicon.ico", s.handleFavicon)

	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/devices", s.handleListDevices)
	s.mux.HandleFunc("/api/toggle", s.handleToggleDevice)
	s.mux.HandleFunc("/api/favorite", s.handleFavorite)
	s.mux.HandleFunc("/api/rename", s.handleRename)
	s.mux.HandleFunc("/api/interfaces", s.handleListInterfaces)
	s.mux.HandleFunc("/api/interface", s.handleSetInterface)
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	data, err := s.staticFS.ReadFile("web/static/favicon.ico")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/x-icon")
	w.Write(data)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	s.mux.ServeHTTP(w, r)
}

const fallbackTemplate = `<!DOCTYPE html>
<html>
<head><title>OpenWRT PBR VPN Control</title></head>
<body>
<h1>OpenWRT PBR VPN Control</h1>
<p>Template loading error. Please check that web/templates/index.html exists.</p>
</body>
</html>`
