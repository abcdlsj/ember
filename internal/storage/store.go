package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	UserID   string `json:"user_id,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (s *Server) Prefix() string {
	if idx := strings.Index(s.Name, " "); idx > 0 {
		return s.Name[:idx]
	}
	return s.Name
}

type ItemMeta struct {
	ItemID     string `json:"item_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	SeriesID   string `json:"series_id,omitempty"`
	SeriesName string `json:"series_name,omitempty"`
	SeasonID   string `json:"season_id,omitempty"`
	SeasonName string `json:"season_name,omitempty"`
}

type SubtitleInfo struct {
	Index      int    `json:"index"`
	Language   string `json:"language"`
	Title      string `json:"title"`
	IsExternal bool   `json:"is_external"`
	Codec      string `json:"codec"`
}

type MediaDetail struct {
	ItemID      string         `json:"item_id"`
	SourceID    string         `json:"source_id"`
	Container   string         `json:"container"`
	Subtitles   []SubtitleInfo `json:"subtitles"`
	CachedAt    string         `json:"cached_at"`
	PositionSec int64          `json:"position_sec,omitempty"`
	DurationSec int64          `json:"duration_sec,omitempty"`
	UpdatedAt   string         `json:"updated_at,omitempty"`
}

type ServerConfig struct {
	Servers      []Server `json:"servers,omitempty"`
	ActiveServer int      `json:"active_server"`
}

type ServerData struct {
	Items        map[string]ItemMeta    `json:"items,omitempty"`
	MediaDetails map[string]MediaDetail `json:"media_details,omitempty"`
}

var (
	homeDir, _ = os.UserHomeDir()
	configDir  string
)

func init() {
	if homeDir != "" {
		configDir = filepath.Join(homeDir, ".ember")
		_ = os.MkdirAll(configDir, 0755)
	}
}

type Store struct {
	configPath string
	config     ServerConfig
	dataPath   string
	data       ServerData
}

func (s *Store) validServerIndex(idx int) bool {
	return idx >= 0 && idx < len(s.config.Servers)
}

func (s *Store) ensureItemsMap() {
	if s.data.Items == nil {
		s.data.Items = make(map[string]ItemMeta)
	}
}

func (s *Store) ensureMediaDetailsMap() {
	if s.data.MediaDetails == nil {
		s.data.MediaDetails = make(map[string]MediaDetail)
	}
}

func New() (*Store, error) {
	s := &Store{
		configPath: filepath.Join(configDir, "servers.json"),
	}
	s.loadConfig()
	s.loadDataForActiveServer()
	return s, nil
}

func (s *Store) loadConfig() {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.config)
}

func (s *Store) saveConfig() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0644)
}

func (s *Store) loadDataForActiveServer() {
	srv := s.GetActiveServer()
	if srv == nil {
		s.dataPath = ""
		s.data = ServerData{}
		return
	}

	prefix := srv.Prefix()
	s.dataPath = filepath.Join(configDir, "data_"+prefix+".json")

	data, err := os.ReadFile(s.dataPath)
	if err != nil {
		s.data = ServerData{}
		return
	}
	json.Unmarshal(data, &s.data)
}

func (s *Store) saveData() error {
	if s.dataPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.dataPath, data, 0644)
}

func (s *Store) SetItemMeta(meta ItemMeta) {
	s.ensureItemsMap()
	s.data.Items[meta.ItemID] = meta
	s.saveData()
}

func (s *Store) GetItemMeta(itemID string) (ItemMeta, bool) {
	meta, ok := s.data.Items[itemID]
	return meta, ok
}

func (s *Store) GetMediaDetail(itemID string) (MediaDetail, bool) {
	detail, ok := s.data.MediaDetails[itemID]
	return detail, ok
}

func (s *Store) SetMediaDetail(detail MediaDetail) {
	s.ensureMediaDetailsMap()
	detail.CachedAt = time.Now().Format(time.RFC3339)
	s.data.MediaDetails[detail.ItemID] = detail
	s.saveData()
}

func (s *Store) UpdatePlaybackPosition(itemID string, positionSec, durationSec int64) {
	s.ensureMediaDetailsMap()
	detail := s.data.MediaDetails[itemID]
	detail.ItemID = itemID
	detail.PositionSec = positionSec
	detail.DurationSec = durationSec
	detail.UpdatedAt = time.Now().Format(time.RFC3339)
	s.data.MediaDetails[itemID] = detail
	s.saveData()
}

func (s *Store) GetPlaybackPosition(itemID string) int64 {
	return s.data.MediaDetails[itemID].PositionSec
}

func (s *Store) GetServers() []Server {
	return s.config.Servers
}

func (s *Store) AddServer(srv Server) {
	s.config.Servers = append(s.config.Servers, srv)
	s.saveConfig()
}

func (s *Store) UpdateServer(idx int, srv Server) {
	if !s.validServerIndex(idx) {
		return
	}
	s.config.Servers[idx] = srv
	s.saveConfig()
}

func (s *Store) DeleteServer(idx int) {
	if !s.validServerIndex(idx) {
		return
	}
	s.config.Servers = append(s.config.Servers[:idx], s.config.Servers[idx+1:]...)
	s.config.ActiveServer = max(0, min(s.config.ActiveServer, len(s.config.Servers)-1))
	s.saveConfig()
	s.loadDataForActiveServer()
}

func (s *Store) GetActiveServer() *Server {
	if len(s.config.Servers) == 0 {
		return nil
	}
	if s.config.ActiveServer < 0 || s.config.ActiveServer >= len(s.config.Servers) {
		s.config.ActiveServer = 0
	}
	return &s.config.Servers[s.config.ActiveServer]
}

func (s *Store) SetActiveServer(idx int) {
	if !s.validServerIndex(idx) {
		return
	}
	s.config.ActiveServer = idx
	s.saveConfig()
	s.loadDataForActiveServer()
}

func (s *Store) GetActiveServerIndex() int {
	return s.config.ActiveServer
}

func (s *Store) SaveServerToken(idx int, userID, token string) {
	if !s.validServerIndex(idx) {
		return
	}
	prefix := s.config.Servers[idx].Prefix()
	for i := range s.config.Servers {
		if s.config.Servers[i].Prefix() == prefix {
			s.config.Servers[i].UserID = userID
			s.config.Servers[i].Token = token
		}
	}
	s.saveConfig()
}
