package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

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

type Data struct {
	Server       string                 `json:"server"`
	UserID       string                 `json:"user_id"`
	Token        string                 `json:"token"`
	Items        map[string]ItemMeta    `json:"items,omitempty"`
	MediaDetails map[string]MediaDetail `json:"media_details,omitempty"`
}

var (
	homeDir, _ = os.UserHomeDir()
)

func init() {
	if homeDir != "" {
		_ = os.MkdirAll(homeDir+"/.ember", 0755)
	}
}

type Store struct {
	path string
	data Data
}

func New() (*Store, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	dir := filepath.Dir(exe)

	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}

	s := &Store{
		path: filepath.Join(homeDir, ".ember", "data.json"),
	}
	s.load()
	return s, nil
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
	}
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) GetToken(server string) (userID, token string) {
	if s.data.Server == server {
		return s.data.UserID, s.data.Token
	}
	return "", ""
}

func (s *Store) SetToken(server, userID, token string) {
	s.data.Server = server
	s.data.UserID = userID
	s.data.Token = token
	s.save()
}

func (s *Store) SetItemMeta(meta ItemMeta) {
	if s.data.Items == nil {
		s.data.Items = make(map[string]ItemMeta)
	}
	s.data.Items[meta.ItemID] = meta
	s.save()
}

func (s *Store) GetItemMeta(itemID string) (ItemMeta, bool) {
	if s.data.Items == nil {
		return ItemMeta{}, false
	}
	meta, ok := s.data.Items[itemID]
	return meta, ok
}

func (s *Store) GetMediaDetail(itemID string) (MediaDetail, bool) {
	if s.data.MediaDetails == nil {
		return MediaDetail{}, false
	}
	detail, ok := s.data.MediaDetails[itemID]
	return detail, ok
}

func (s *Store) SetMediaDetail(detail MediaDetail) {
	if s.data.MediaDetails == nil {
		s.data.MediaDetails = make(map[string]MediaDetail)
	}
	detail.CachedAt = time.Now().Format(time.RFC3339)
	s.data.MediaDetails[detail.ItemID] = detail
	s.save()
}

func (s *Store) UpdatePlaybackPosition(itemID string, positionSec, durationSec int64) {
	if s.data.MediaDetails == nil {
		s.data.MediaDetails = make(map[string]MediaDetail)
	}
	detail, ok := s.data.MediaDetails[itemID]
	if !ok {
		detail = MediaDetail{ItemID: itemID}
	}
	detail.PositionSec = positionSec
	detail.DurationSec = durationSec
	detail.UpdatedAt = time.Now().Format(time.RFC3339)
	s.data.MediaDetails[itemID] = detail
	s.save()
}

func (s *Store) GetPlaybackPosition(itemID string) int64 {
	if s.data.MediaDetails == nil {
		return 0
	}
	if detail, ok := s.data.MediaDetails[itemID]; ok {
		return detail.PositionSec
	}
	return 0
}
