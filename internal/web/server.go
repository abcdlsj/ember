package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"ember/internal/service"
)

//go:embed templates/* static/*
var content embed.FS

// Server represents the web server
type Server struct {
	svc    *service.MediaService
	router *http.ServeMux
}

// New creates a new web server
func New(svc *service.MediaService) *Server {
	s := &Server{
		svc:    svc,
		router: http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Static files
	s.router.Handle("/static/", http.FileServer(http.FS(content)))

	// HTML template
	s.router.HandleFunc("/", s.handleIndex)

	// API routes
	s.router.HandleFunc("/api/status", s.handleStatus)
	s.router.HandleFunc("/api/resume", s.handleResume)
	s.router.HandleFunc("/api/favorites", s.handleFavorites)
	s.router.HandleFunc("/api/libraries", s.handleLibraries)
	s.router.HandleFunc("/api/items", s.handleItems)
	s.router.HandleFunc("/api/seasons", s.handleSeasons)
	s.router.HandleFunc("/api/episodes", s.handleEpisodes)
	s.router.HandleFunc("/api/search", s.handleSearch)
	s.router.HandleFunc("/api/stream", s.handleStream)
	s.router.HandleFunc("/api/playback", s.handlePlayback)
	s.router.HandleFunc("/api/favorite", s.handleFavorite)

	// MPV playback routes
	s.router.HandleFunc("/api/play", s.handlePlayMPV)
	s.router.HandleFunc("/api/play-series", s.handlePlaySeriesMPV)
	s.router.HandleFunc("/api/playlist", s.handleGetPlaylist)

	// Server management
	s.router.HandleFunc("/api/servers", s.handleServers)
	s.router.HandleFunc("/api/servers/", s.handleServerDetail)
	s.router.HandleFunc("/api/servers/ping", s.handlePingServers)
}

// Run starts the web server
func (s *Server) Run(addr string) error {
	fmt.Printf("Web server starting on http://%s\n", addr)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, nil)
}

// ==================== Media Handlers ====================

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.svc.GetServerStatus()
	respondJSON(w, status)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.GetResume(50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, list)
}

func (s *Server) handleFavorites(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.GetFavorites(50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, list)
}

func (s *Server) handleLibraries(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.GetLibraries()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, list)
}

func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parentId")
	page := 0
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	list, err := s.svc.GetItems(parentID, page, 20)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, list)
}

func (s *Server) handleSeasons(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("seriesId")
	if seriesID == "" {
		respondError(w, http.StatusBadRequest, "seriesId required")
		return
	}

	list, err := s.svc.GetSeasons(seriesID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, list)
}

func (s *Server) handleEpisodes(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("seriesId")
	seasonID := r.URL.Query().Get("seasonId")

	if seriesID == "" || seasonID == "" {
		respondError(w, http.StatusBadRequest, "seriesId and seasonId required")
		return
	}

	list, err := s.svc.GetEpisodes(seriesID, seasonID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, list)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respondJSON(w, &service.MediaList{Items: []service.MediaItem{}, Total: 0})
		return
	}

	list, err := s.svc.Search(query, 50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, list)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	itemID := r.URL.Query().Get("itemId")
	if itemID == "" {
		respondError(w, http.StatusBadRequest, "itemId required")
		return
	}

	streamInfo, err := s.svc.GetStreamInfo(itemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, streamInfo)
}

func (s *Server) handlePlayback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.PlaybackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.ReportPlayback(req); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) handleFavorite(w http.ResponseWriter, r *http.Request) {
	itemID := r.URL.Query().Get("itemId")
	if itemID == "" {
		respondError(w, http.StatusBadRequest, "itemId required")
		return
	}

	result, err := s.svc.ToggleFavorite(itemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, result)
}

// ==================== MPV Playback Handlers ====================

func (s *Server) handlePlayMPV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.PlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.ItemID == "" {
		respondError(w, http.StatusBadRequest, "itemId required")
		return
	}

	result, err := s.svc.PlayWithMPV(req.ItemID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, result)
}

func (s *Server) handlePlaySeriesMPV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req service.PlaySeriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.SeriesID == "" {
		respondError(w, http.StatusBadRequest, "seriesId required")
		return
	}

	result, err := s.svc.PlaySeriesWithMPV(req.SeriesID, req.StartEpisodeID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, result)
}

func (s *Server) handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("seriesId")
	if seriesID == "" {
		respondError(w, http.StatusBadRequest, "seriesId required")
		return
	}

	playlist, err := s.svc.GetSeriesPlaylist(seriesID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, playlist)
}

// ==================== Server Management Handlers ====================

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.serversList(w, r)
	case http.MethodPost:
		s.serverAdd(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) serversList(w http.ResponseWriter, r *http.Request) {
	servers := s.svc.GetServers()
	active := s.svc.GetActiveServer()

	respondJSON(w, map[string]interface{}{
		"servers": servers,
		"active":  active,
	})
}

func (s *Server) serverAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.AddServer(req.Name, req.URL, req.Username, req.Password); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) handleServerDetail(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/servers/"):]

	if path == "ping" {
		return
	}

	idx, err := strconv.Atoi(path)
	if err != nil {
		// Handle /{id}/activate
		if len(path) > 9 && path[len(path)-9:] == "/activate" {
			idx, _ = strconv.Atoi(path[:len(path)-9])
			s.serverActivate(w, r, idx)
			return
		}
		respondError(w, http.StatusBadRequest, "Invalid server index")
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.serverUpdate(w, r, idx)
	case http.MethodDelete:
		s.serverDelete(w, r, idx)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) serverUpdate(w http.ResponseWriter, r *http.Request, idx int) {
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.svc.UpdateServer(idx, req.Name, req.URL, req.Username, req.Password); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) serverDelete(w http.ResponseWriter, r *http.Request, idx int) {
	if err := s.svc.DeleteServer(idx); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) serverActivate(w http.ResponseWriter, r *http.Request, idx int) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.svc.ActivateServer(idx); err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	respondJSON(w, map[string]bool{"success": true})
}

func (s *Server) handlePingServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	servers := s.svc.GetServers()
	results := make([]service.ServerInfo, len(servers))

	for i, srv := range servers {
		latency := s.svc.PingServer(srv.URL)
		results[i] = srv
		results[i].Latency = latency
	}

	respondJSON(w, map[string]interface{}{"servers": results})
}

// ==================== Helpers ====================

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	respondJSON(w, map[string]string{"error": message})
}

