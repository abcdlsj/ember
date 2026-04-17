package service

import (
	"fmt"
	"time"

	"ember/internal/api"
	"ember/internal/player"
	"ember/internal/storage"
)

type MediaService struct {
	client *api.Client
	store  *storage.Store
}

func NewMediaService(client *api.Client, store *storage.Store) *MediaService {
	return &MediaService{
		client: client,
		store:  store,
	}
}

func (s *MediaService) SetClient(client *api.Client) {
	s.client = client
}

func (s *MediaService) Store() *storage.Store {
	return s.store
}
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

func (s *MediaService) Search(query string, limit int) (*MediaList, error) {
	return s.SearchWithOptions(SearchQuery{
		Query: query,
		Limit: limit,
	})
}

func (s *MediaService) SearchWithOptions(q SearchQuery) (*MediaList, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Page < 0 {
		q.Page = 0
	}

	var itemTypes []string
	switch q.ItemType {
	case "movie":
		itemTypes = append(itemTypes, "Movie")
	case "series":
		itemTypes = append(itemTypes, "Series")
	case "episode":
		itemTypes = append(itemTypes, "Episode")
	}

	items, total, err := s.client.SearchWithOptions(api.SearchOptions{
		Query:        q.Query,
		Start:        q.Page * q.Limit,
		Limit:        q.Limit,
		ItemTypes:    itemTypes,
		PlayedFilter: q.PlayedFilter,
		FavoriteOnly: q.FavoriteOnly,
		Year:         q.Year,
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &MediaList{
		Items:    s.convertItems(items),
		Total:    total,
		Page:     q.Page,
		PageSize: q.Limit,
		HasMore:  (q.Page+1)*q.Limit < total,
	}, nil
}

func (s *MediaService) GetHistory(page, pageSize int) (*MediaList, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	items, total, err := s.client.GetHistory(page*pageSize, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	return &MediaList{
		Items:    s.convertItems(items),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  (page+1)*pageSize < total,
	}, nil
}

func (s *MediaService) GetItem(itemID string) (*MediaItem, error) {
	item, err := s.client.GetItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	converted := s.convertItem(*item)
	return &converted, nil
}

func (s *MediaService) GetStreamInfo(itemID string) (*StreamInfo, error) {
	item, err := s.client.GetItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return s.GetStreamInfoForItem(s.convertItem(*item))
}

func (s *MediaService) GetStreamInfoForItem(item MediaItem) (*StreamInfo, error) {
	if len(item.MediaSources) == 0 {
		return nil, fmt.Errorf("no media source available")
	}

	ms := item.MediaSources[0]
	isFav := item.UserData != nil && item.UserData.IsFavorite
	subtitleURLs := make([]string, 0, len(ms.Subtitles))
	for _, subtitle := range ms.Subtitles {
		if !subtitle.IsExternal {
			continue
		}
		subtitleURLs = append(subtitleURLs, s.client.SubtitleURL(item.ID, ms.ID, subtitle.Index, subtitle.Codec))
	}

	return &StreamInfo{
		ItemID:        item.ID,
		Name:          item.Name,
		SeriesID:      item.SeriesID,
		SeriesName:    item.SeriesName,
		Type:          item.Type,
		StreamURL:     s.client.StreamURL(item.ID, ms.ID, ms.Container),
		PosterURL:     s.client.ImageURLByID(item.ID, 800),
		Container:     ms.Container,
		Duration:      item.RunTimeTicks,
		PositionSec:   s.playbackPosition(item),
		Subtitles:     ms.Subtitles,
		SubtitleURLs:  subtitleURLs,
		IsFavorite:    isFav,
		MediaSourceID: ms.ID,
	}, nil
}

func (s *MediaService) playbackPosition(item MediaItem) int64 {
	positionSec := s.store.GetPlaybackPosition(item.ID)
	if positionSec > 0 {
		return positionSec
	}
	if item.UserData == nil || item.UserData.PlaybackPositionTicks <= 0 {
		return 0
	}
	return item.UserData.PlaybackPositionTicks / 10000000
}

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

func (s *MediaService) SetFavorite(itemID string, favorite bool) (*FavoriteResult, error) {
	var err error
	if favorite {
		err = s.client.AddFavorite(itemID)
	} else {
		err = s.client.RemoveFavorite(itemID)
	}
	if err != nil {
		state, statusErr := s.client.IsFavorite(itemID)
		if statusErr == nil && state == favorite {
			return &FavoriteResult{IsFavorite: favorite}, nil
		}
		if favorite {
			return nil, fmt.Errorf("failed to add favorite: %w", err)
		}
		return nil, fmt.Errorf("failed to remove favorite: %w", err)
	}

	finalState, statusErr := s.client.IsFavorite(itemID)
	if statusErr != nil {
		return &FavoriteResult{IsFavorite: favorite}, nil
	}
	return &FavoriteResult{IsFavorite: finalState}, nil
}

func (s *MediaService) ToggleFavorite(itemID string) (*FavoriteResult, error) {
	isFav, err := s.client.IsFavorite(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite status: %w", err)
	}

	return s.SetFavorite(itemID, !isFav)
}

func (s *MediaService) ReportPlaybackStart(itemID, mediaSourceID, sessionID string, positionSec int64) error {
	return s.client.ReportPlaybackStart(itemID, mediaSourceID, sessionID, positionSec*10_000_000)
}

func (s *MediaService) ReportPlaybackStopped(itemID, mediaSourceID, sessionID string, positionSec, durationTicks int64) error {
	s.store.UpdatePlaybackPosition(itemID, positionSec, durationTicks/10_000_000)
	return s.client.ReportPlaybackStopped(itemID, mediaSourceID, sessionID, positionSec*10_000_000)
}

func (s *MediaService) BuildContinuousPlayback(item MediaItem) (*ContinuousPlaybackPlan, error) {
	seriesID := item.SeriesID
	seasonID := item.SeasonID
	if seasonID == "" {
		seasonID = item.ParentID
	}
	if seriesID == "" || seasonID == "" {
		return nil, fmt.Errorf("missing season info")
	}

	episodes, err := s.client.GetEpisodes(seriesID, seasonID)
	if err != nil {
		return nil, err
	}
	if len(episodes) == 0 {
		return nil, fmt.Errorf("no episodes found")
	}

	startIndex := 0
	for i, ep := range episodes {
		if ep.ID == item.ID {
			startIndex = i
			break
		}
	}

	urls := make([]string, 0, len(episodes)-startIndex)
	var currentItem MediaItem
	currentSet := false
	for i := startIndex; i < len(episodes); i++ {
		epFull, err := s.client.GetItem(episodes[i].ID)
		if err != nil || len(epFull.MediaSources) == 0 {
			continue
		}

		ms := epFull.MediaSources[0]
		urls = append(urls, s.client.StreamURL(epFull.ID, ms.ID, ms.Container))
		if !currentSet {
			currentItem = s.convertItem(*epFull)
			currentSet = true
		}
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no playable episodes found")
	}
	if !currentSet {
		return nil, fmt.Errorf("no playable item found")
	}

	streamInfo, err := s.GetStreamInfoForItem(currentItem)
	if err != nil {
		return nil, err
	}

	title := item.SeriesName
	if title == "" {
		title = item.Name
	}

	return &ContinuousPlaybackPlan{
		Title:       title,
		StartIndex:  0,
		URLs:        urls,
		CurrentItem: currentItem,
		StreamInfo:  streamInfo,
	}, nil
}

func (s *MediaService) ResolveSeason(item MediaItem) (*MediaList, string, string, error) {
	seriesID := item.SeriesID
	seasonID := item.SeasonID
	if seriesID == "" {
		fullItem, err := s.client.GetItem(item.ID)
		if err != nil {
			return nil, "", "", fmt.Errorf("no series info")
		}
		seriesID = fullItem.SeriesID
		seasonID = fullItem.SeasonID
		if seasonID == "" {
			seasonID = fullItem.ParentID
		}
	}

	if seriesID == "" || seasonID == "" {
		return nil, "", "", fmt.Errorf("no season info")
	}

	list, err := s.GetEpisodes(seriesID, seasonID)
	if err != nil {
		return nil, "", "", err
	}

	return list, seriesID, seasonID, nil
}

func (s *MediaService) ResolveSeries(item MediaItem) (*MediaList, string, error) {
	seriesID := item.SeriesID
	if seriesID == "" && item.ParentID != "" {
		seriesID = item.ParentID
	}
	if seriesID == "" {
		fullItem, err := s.client.GetItem(item.ID)
		if err != nil {
			return nil, "", fmt.Errorf("no series info")
		}
		seriesID = fullItem.SeriesID
		if seriesID == "" {
			seriesID = fullItem.ParentID
		}
	}

	if seriesID == "" {
		return nil, "", fmt.Errorf("no series info")
	}

	list, err := s.GetSeasons(seriesID)
	if err != nil {
		return nil, "", err
	}

	return list, seriesID, nil
}

func (s *MediaService) GetMediaDetail(itemID string) (*storage.MediaDetail, error) {
	if cached, ok := s.store.GetMediaDetail(itemID); ok {
		detail := cached
		return &detail, nil
	}

	item, err := s.client.GetItem(itemID)
	if err != nil || len(item.MediaSources) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	ms := item.MediaSources[0]
	detail := storage.MediaDetail{
		ItemID:    itemID,
		SourceID:  ms.ID,
		Container: ms.Container,
	}
	for _, stream := range ms.MediaStreams {
		if stream.Type != "Subtitle" {
			continue
		}
		detail.Subtitles = append(detail.Subtitles, storage.SubtitleInfo{
			Index:      stream.Index,
			Language:   stream.Language,
			Title:      stream.Title,
			IsExternal: stream.IsExternal,
			Codec:      stream.Codec,
		})
	}
	s.store.SetMediaDetail(detail)
	return &detail, nil
}

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

func (s *MediaService) GetActiveServer() *ServerInfo {
	idx := s.store.GetActiveServerIndex()
	servers := s.GetServers()
	if idx < 0 || idx >= len(servers) {
		return nil
	}
	return &servers[idx]
}

func (s *MediaService) AddServer(name, url, username, password string) error {
	srv := storage.Server{
		Name:     name,
		URL:      url,
		Username: username,
		Password: password,
	}

	client := api.New(srv.URL)
	if err := client.Login(username, password); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	srv.UserID = client.UserID
	srv.Token = client.Token

	s.store.AddServer(srv)

	if len(s.store.GetServers()) == 1 {
		s.store.SetActiveServer(0)
		s.client = client
	}

	return nil
}

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

func (s *MediaService) DeleteServer(index int) error {
	servers := s.store.GetServers()
	if index < 0 || index >= len(servers) {
		return fmt.Errorf("server not found")
	}

	s.store.DeleteServer(index)
	return nil
}

func (s *MediaService) ActivateServer(index int) error {
	servers := s.store.GetServers()
	if index < 0 || index >= len(servers) {
		return fmt.Errorf("server not found")
	}

	s.store.SetActiveServer(index)
	srv := s.store.GetActiveServer()

	client := api.New(srv.URL)
	client.UserID = srv.UserID
	client.Token = srv.Token

	if !client.VerifyToken() {
		if err := client.Login(srv.Username, srv.Password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		s.store.SaveServerToken(index, client.UserID, client.Token)
	}

	s.client = client
	return nil
}

func (s *MediaService) PingServer(url string) int64 {
	client := api.New(url)
	return client.Ping().Milliseconds()
}

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

func (s *MediaService) IsMpvAvailable() bool {
	return player.Available()
}

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

	var subtitleURLs []string
	for _, stream := range ms.MediaStreams {
		if stream.Type == "Subtitle" && stream.IsExternal {
			subURL := fmt.Sprintf("%s/emby/Videos/%s/%s/Subtitles/%d/Stream.%s?api_key=%s",
				s.client.Server, itemID, ms.ID, stream.Index, stream.Codec, s.client.Token)
			subtitleURLs = append(subtitleURLs, subURL)
		}
	}

	positionSec := s.store.GetPlaybackPosition(itemID)

	go func() {
		result := player.Play(streamURL, item.Name, subtitleURLs, positionSec)
		if result.Err != nil {
			return
		}
		durationSec := int64(0)
		if item.RunTimeTicks > 0 {
			durationSec = item.RunTimeTicks / 10000000
		}
		s.store.UpdatePlaybackPosition(itemID, result.PositionSec, durationSec)
	}()

	return &PlayResult{Success: true, Message: "Playback started in MPV"}, nil
}

func (s *MediaService) GetSeriesPlaylist(seriesID string) (*EpisodePlaylist, error) {
	series, err := s.client.GetItem(seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

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

	var urls []string
	startIndex := 0
	for i, ep := range playlist.Episodes {
		urls = append(urls, ep.StreamURL)
		if ep.ItemID == startEpisodeID {
			startIndex = i
		}
	}

	positionSec := int64(0)
	if startIndex < len(playlist.Episodes) {
		positionSec = s.store.GetPlaybackPosition(playlist.Episodes[startIndex].ItemID)
	}

	go func() {
		result := player.PlayMultiple(urls, playlist.SeriesName, nil, positionSec, startIndex)
		if result.Err != nil {
			return
		}
		if startIndex < len(playlist.Episodes) {
			s.store.UpdatePlaybackPosition(playlist.Episodes[startIndex].ItemID, result.PositionSec, 0)
		}
	}()

	return &PlayResult{Success: true, Message: fmt.Sprintf("Started playing %s from episode %d", playlist.SeriesName, startIndex+1)}, nil
}
