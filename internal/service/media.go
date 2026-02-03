package service

import (
	"fmt"
	"time"

	"ember/internal/api"
	"ember/internal/player"
	"ember/internal/storage"
)

// MediaService provides unified business logic for media operations
type MediaService struct {
	client *api.Client
	store  *storage.Store
}

// NewMediaService creates a new media service instance
func NewMediaService(client *api.Client, store *storage.Store) *MediaService {
	return &MediaService{
		client: client,
		store:  store,
	}
}

// SetClient updates the underlying API client (used when switching servers)
func (s *MediaService) SetClient(client *api.Client) {
	s.client = client
}

// Client returns the current API client
func (s *MediaService) Client() *api.Client {
	return s.client
}

// Store returns the storage instance
func (s *MediaService) Store() *storage.Store {
	return s.store
}

// ==================== Media List Operations ====================

// GetResume returns items to resume watching
func (s *MediaService) GetResume(limit int) (*MediaList, error) {
	if limit <= 0 {
		limit = 20
	}
	
	items, err := s.client.GetResumeItems(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get resume items: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    len(items),
		Page:     0,
		PageSize: limit,
		HasMore:  false,
	}, nil
}

// GetFavorites returns favorite items
func (s *MediaService) GetFavorites(limit int) (*MediaList, error) {
	if limit <= 0 {
		limit = 50
	}
	
	items, err := s.client.GetFavorites(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorites: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    len(items),
		Page:     0,
		PageSize: limit,
		HasMore:  false,
	}, nil
}

// GetLibraries returns media libraries
func (s *MediaService) GetLibraries() (*MediaList, error) {
	items, err := s.client.GetLibraries()
	if err != nil {
		return nil, fmt.Errorf("failed to get libraries: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    len(items),
		Page:     0,
		PageSize: len(items),
		HasMore:  false,
	}, nil
}

// GetItems returns items in a parent folder with pagination
func (s *MediaService) GetItems(parentID string, page, pageSize int) (*MediaList, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 0 {
		page = 0
	}
	
	items, total, err := s.client.GetItems(parentID, page*pageSize, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  (page+1)*pageSize < total,
	}, nil
}

// GetSeasons returns seasons for a series
func (s *MediaService) GetSeasons(seriesID string) (*MediaList, error) {
	items, err := s.client.GetSeasons(seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seasons: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    len(items),
		Page:     0,
		PageSize: len(items),
		HasMore:  false,
	}, nil
}

// GetEpisodes returns episodes for a season
func (s *MediaService) GetEpisodes(seriesID, seasonID string) (*MediaList, error) {
	items, err := s.client.GetEpisodes(seriesID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    len(items),
		Page:     0,
		PageSize: len(items),
		HasMore:  false,
	}, nil
}

// Search performs a search query
func (s *MediaService) Search(query string, limit int) (*MediaList, error) {
	if limit <= 0 {
		limit = 50
	}
	
	items, err := s.client.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	
	return &MediaList{
		Items:    s.convertItems(items),
		Total:    len(items),
		Page:     0,
		PageSize: limit,
		HasMore:  false,
	}, nil
}

// GetItem returns a single item by ID
func (s *MediaService) GetItem(itemID string) (*MediaItem, error) {
	item, err := s.client.GetItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	
	converted := s.convertItem(*item)
	return &converted, nil
}

// ==================== Playback Operations ====================

// GetStreamInfo returns streaming information for an item
func (s *MediaService) GetStreamInfo(itemID string) (*StreamInfo, error) {
	item, err := s.client.GetItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	
	if len(item.MediaSources) == 0 {
		return nil, fmt.Errorf("no media source available")
	}
	
	ms := item.MediaSources[0]
	
	// Get saved playback position
	positionSec := s.store.GetPlaybackPosition(itemID)
	
	// Build subtitle list
	subtitles := []SubtitleInfo{}
	for _, stream := range ms.MediaStreams {
		if stream.Type == "Subtitle" {
			subtitles = append(subtitles, SubtitleInfo{
				Index:      stream.Index,
				Language:   stream.Language,
				Title:      stream.Title,
				IsExternal: stream.IsExternal,
				IsDefault:  stream.IsDefault,
				Codec:      stream.Codec,
			})
		}
	}
	
	isFav := item.UserData != nil && item.UserData.IsFavorite
	
	return &StreamInfo{
		ItemID:        itemID,
		Name:          item.Name,
		SeriesID:      item.SeriesID,
		SeriesName:    item.SeriesName,
		Type:          item.Type,
		StreamURL:     s.client.StreamURL(itemID, ms.ID, ms.Container),
		PosterURL:     s.client.ImageURLByID(itemID, 800),
		Container:     ms.Container,
		Duration:      item.RunTimeTicks,
		PositionSec:   positionSec,
		Subtitles:     subtitles,
		IsFavorite:    isFav,
		MediaSourceID: ms.ID,
	}, nil
}

// ReportPlayback reports playback progress to the server
func (s *MediaService) ReportPlayback(req PlaybackRequest) error {
	sessionID := generateSessionID()
	
	switch req.Type {
	case "start":
		return s.client.ReportPlaybackStart(req.ItemID, "", sessionID, req.PositionTicks)
	case "progress":
		return s.client.ReportPlaybackProgress(req.ItemID, "", sessionID, req.PositionTicks, false)
	case "stop":
		err := s.client.ReportPlaybackStopped(req.ItemID, "", sessionID, req.PositionTicks)
		if err == nil {
			// Save to local storage
			durationSec := int64(0)
			if item, e := s.client.GetItem(req.ItemID); e == nil {
				durationSec = item.RunTimeTicks / 10000000
			}
			s.store.UpdatePlaybackPosition(req.ItemID, req.PositionTicks/10000000, durationSec)
		}
		return err
	default:
		return fmt.Errorf("unknown playback type: %s", req.Type)
	}
}

// ==================== Favorite Operations ====================

// ToggleFavorite toggles favorite status for an item
func (s *MediaService) ToggleFavorite(itemID string) (*FavoriteResult, error) {
	item, err := s.client.GetItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	
	isFav := item.UserData != nil && item.UserData.IsFavorite
	
	if isFav {
		err = s.client.RemoveFavorite(itemID)
	} else {
		err = s.client.AddFavorite(itemID)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to toggle favorite: %w", err)
	}
	
	return &FavoriteResult{IsFavorite: !isFav}, nil
}

// ==================== Server Management ====================

// GetServers returns all configured servers
func (s *MediaService) GetServers() []ServerInfo {
	servers := s.store.GetServers()
	activeIdx := s.store.GetActiveServerIndex()
	
	result := make([]ServerInfo, len(servers))
	for i, srv := range servers {
		result[i] = ServerInfo{
			Index:    i,
			Name:     srv.Name,
			URL:      srv.URL,
			Username: srv.Username,
			IsActive: i == activeIdx,
			Prefix:   srv.Prefix(),
		}
	}
	
	return result
}

// GetActiveServer returns the currently active server
func (s *MediaService) GetActiveServer() *ServerInfo {
	idx := s.store.GetActiveServerIndex()
	servers := s.GetServers()
	if idx < 0 || idx >= len(servers) {
		return nil
	}
	return &servers[idx]
}

// AddServer adds a new server configuration
func (s *MediaService) AddServer(name, url, username, password string) error {
	srv := storage.Server{
		Name:     name,
		URL:      url,
		Username: username,
		Password: password,
	}
	
	// Test connection
	client := api.New(srv.URL)
	if err := client.Login(username, password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	
	srv.UserID = client.UserID
	srv.Token = client.Token
	
	s.store.AddServer(srv)
	
	// If first server, activate it and update service client
	if len(s.store.GetServers()) == 1 {
		s.store.SetActiveServer(0)
		s.client = client
	}
	
	return nil
}

// UpdateServer updates an existing server
func (s *MediaService) UpdateServer(index int, name, url, username, password string) error {
	servers := s.store.GetServers()
	if index < 0 || index >= len(servers) {
		return fmt.Errorf("server not found")
	}
	
	srv := servers[index]
	srv.Name = name
	srv.URL = url
	srv.Username = username
	if password != "" {
		srv.Password = password
	}
	
	s.store.UpdateServer(index, srv)
	return nil
}

// DeleteServer removes a server
func (s *MediaService) DeleteServer(index int) error {
	servers := s.store.GetServers()
	if index < 0 || index >= len(servers) {
		return fmt.Errorf("server not found")
	}
	
	s.store.DeleteServer(index)
	return nil
}

// ActivateServer switches to a different server
func (s *MediaService) ActivateServer(index int) error {
	servers := s.store.GetServers()
	if index < 0 || index >= len(servers) {
		return fmt.Errorf("server not found")
	}
	
	s.store.SetActiveServer(index)
	srv := s.store.GetActiveServer()
	
	// Update client
	client := api.New(srv.URL)
	client.UserID = srv.UserID
	client.Token = srv.Token
	
	// Verify token
	if !client.VerifyToken() {
		if err := client.Login(srv.Username, srv.Password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		s.store.SaveServerToken(index, client.UserID, client.Token)
	}
	
	s.client = client
	return nil
}

// PingServer tests latency for a specific server
func (s *MediaService) PingServer(url string) int64 {
	client := api.New(url)
	return client.Ping().Milliseconds()
}

// GetServerStatus returns current connection status
func (s *MediaService) GetServerStatus() *ServerStatus {
	srv := s.store.GetActiveServer()
	status := &ServerStatus{
		MpvAvailable: s.IsMpvAvailable(),
	}
	
	if srv != nil {
		status.Server = &ServerInfo{
			Name:     srv.Name,
			URL:      srv.URL,
			Username: srv.Username,
			Prefix:   srv.Prefix(),
		}
		status.Connected = s.client.VerifyToken()
		status.Latency = s.client.Latency.Milliseconds()
	}
	
	return status
}

// IsMpvAvailable checks if mpv player is available
func (s *MediaService) IsMpvAvailable() bool {
	return player.Available()
}

// ==================== Helper Methods ====================

func (s *MediaService) convertItems(items []api.MediaItem) []MediaItem {
	result := make([]MediaItem, len(items))
	for i, item := range items {
		result[i] = s.convertItem(item)
	}
	return result
}

func (s *MediaService) convertItem(item api.MediaItem) MediaItem {
	return convertAPIItem(item, s.client.Server, s.client.Token)
}

func generateSessionID() string {
	return fmt.Sprintf("ember-%d", time.Now().Unix())
}

// GetItemRaw returns the raw API item (for advanced use cases)
func (s *MediaService) GetItemRaw(itemID string) (*api.MediaItem, error) {
	return s.client.GetItem(itemID)
}

// ==================== MPV Playback Operations ====================

// PlayWithMPV plays a single item with MPV
func (s *MediaService) PlayWithMPV(itemID string) (*PlayResult, error) {
	if !player.Available() {
		return nil, fmt.Errorf("mpv player not available")
	}

	item, err := s.client.GetItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if len(item.MediaSources) == 0 {
		return nil, fmt.Errorf("no media source available")
	}

	ms := item.MediaSources[0]
	streamURL := s.client.StreamURL(itemID, ms.ID, ms.Container)

	// Build subtitle URLs
	var subtitleURLs []string
	for _, stream := range ms.MediaStreams {
		if stream.Type == "Subtitle" && stream.IsExternal {
			subURL := fmt.Sprintf("%s/emby/Videos/%s/%s/Subtitles/%d/Stream.%s?api_key=%s",
				s.client.Server, itemID, ms.ID, stream.Index, stream.Codec, s.client.Token)
			subtitleURLs = append(subtitleURLs, subURL)
		}
	}

	// Get saved playback position
	positionSec := s.store.GetPlaybackPosition(itemID)

	// Start playback in a goroutine so it doesn't block
	go func() {
		result := player.Play(streamURL, item.Name, subtitleURLs, positionSec)
		if result.Err != nil {
			return
		}
		// Save playback position after MPV closes
		durationSec := int64(0)
		if item.RunTimeTicks > 0 {
			durationSec = item.RunTimeTicks / 10000000
		}
		s.store.UpdatePlaybackPosition(itemID, result.PositionSec, durationSec)
	}()

	return &PlayResult{Success: true, Message: "Playback started in MPV"}, nil
}

// GetSeriesPlaylist returns all episodes in a series for playlist playback
func (s *MediaService) GetSeriesPlaylist(seriesID string) (*EpisodePlaylist, error) {
	// Get series info
	series, err := s.client.GetItem(seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	// Get all seasons
	seasons, err := s.client.GetSeasons(seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get seasons: %w", err)
	}

	var allEpisodes []PlaylistEpisode
	for _, season := range seasons {
		episodes, err := s.client.GetEpisodes(seriesID, season.ID)
		if err != nil {
			continue
		}
		for _, ep := range episodes {
			if len(ep.MediaSources) == 0 {
				continue
			}
			ms := ep.MediaSources[0]
			streamURL := s.client.StreamURL(ep.ID, ms.ID, ms.Container)
			allEpisodes = append(allEpisodes, PlaylistEpisode{
				ItemID:    ep.ID,
				Name:      ep.Name,
				Index:     ep.IndexNumber,
				StreamURL: streamURL,
			})
		}
	}

	return &EpisodePlaylist{
		SeriesID:   seriesID,
		SeriesName: series.Name,
		Episodes:   allEpisodes,
	}, nil
}

// PlaySeriesWithMPV plays a series with MPV from a specific episode
func (s *MediaService) PlaySeriesWithMPV(seriesID, startEpisodeID string) (*PlayResult, error) {
	if !player.Available() {
		return nil, fmt.Errorf("mpv player not available")
	}

	playlist, err := s.GetSeriesPlaylist(seriesID)
	if err != nil {
		return nil, err
	}

	if len(playlist.Episodes) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	// Build URL list and find start index
	var urls []string
	startIndex := 0
	for i, ep := range playlist.Episodes {
		urls = append(urls, ep.StreamURL)
		if ep.ItemID == startEpisodeID {
			startIndex = i
		}
	}

	// Get position for the start episode
	positionSec := int64(0)
	if startIndex < len(playlist.Episodes) {
		positionSec = s.store.GetPlaybackPosition(playlist.Episodes[startIndex].ItemID)
	}

	// Start playback in a goroutine
	go func() {
		result := player.PlayMultiple(urls, playlist.SeriesName, nil, positionSec, startIndex)
		if result.Err != nil {
			return
		}
		// Save playback position for the last played episode
		if startIndex < len(playlist.Episodes) {
			s.store.UpdatePlaybackPosition(playlist.Episodes[startIndex].ItemID, result.PositionSec, 0)
		}
	}()

	return &PlayResult{Success: true, Message: fmt.Sprintf("Started playing %s from episode %d", playlist.SeriesName, startIndex+1)}, nil
}
