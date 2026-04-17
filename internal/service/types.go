// Package service provides business logic layer for both TUI and Web UI
package service

import (
	"fmt"

	"ember/internal/api"
)

// MediaItem represents a unified media item for UI consumption
type MediaItem struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	Year         int           `json:"year,omitempty"`
	SeriesID     string        `json:"seriesId,omitempty"`
	SeriesName   string        `json:"seriesName,omitempty"`
	SeasonID     string        `json:"seasonId,omitempty"`
	SeasonName   string        `json:"seasonName,omitempty"`
	ParentID     string        `json:"parentId,omitempty"`
	IndexNumber  int           `json:"indexNumber,omitempty"`
	Overview     string        `json:"overview,omitempty"`
	RunTimeTicks int64         `json:"runTimeTicks,omitempty"`
	ImageURL     string        `json:"imageUrl,omitempty"`     // 400px for TUI
	ImageURLs    []string      `json:"imageUrls,omitempty"`    // TUI fallback chain
	ImageURLHigh string        `json:"imageUrlHigh,omitempty"` // 800px for Web UI
	BackdropURL  string        `json:"backdropUrl,omitempty"`
	UserData     *UserData     `json:"userData,omitempty"`
	MediaSources []MediaSource `json:"mediaSources,omitempty"`
	Playable     bool          `json:"playable"`
	Browsable    bool          `json:"browsable"`
}

// UserData represents playback and favorite status
type UserData struct {
	PlaybackPositionTicks int64  `json:"playbackPositionTicks"`
	Played                bool   `json:"played"`
	IsFavorite            bool   `json:"isFavorite"`
	LastPlayedDate        string `json:"lastPlayedDate,omitempty"`
	PlaybackPositionPct   int    `json:"playbackPositionPct,omitempty"`
}

// MediaSource represents a playable media source
type MediaSource struct {
	ID        string `json:"id"`
	Container string `json:"container"`
	Protocol  string `json:"protocol,omitempty"`
}

// MediaDetail stores additional media information
type MediaDetail struct {
	ItemID      string         `json:"itemId"`
	SourceID    string         `json:"sourceId"`
	Container   string         `json:"container"`
	Subtitles   []SubtitleInfo `json:"subtitles"`
	CachedAt    string         `json:"cachedAt,omitempty"`
	PositionSec int64          `json:"positionSec,omitempty"`
	DurationSec int64          `json:"durationSec,omitempty"`
	UpdatedAt   string         `json:"updatedAt,omitempty"`
}

// SubtitleInfo represents a subtitle stream
type SubtitleInfo struct {
	Index      int    `json:"index"`
	Language   string `json:"language"`
	Title      string `json:"title,omitempty"`
	IsExternal bool   `json:"isExternal"`
	IsDefault  bool   `json:"isDefault"`
	Codec      string `json:"codec,omitempty"`
}

// MediaList represents a paginated list of media items
type MediaList struct {
	Items    []MediaItem `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	HasMore  bool        `json:"hasMore"`
}

// StreamInfo represents streaming information for playback
type StreamInfo struct {
	ItemID        string         `json:"itemId"`
	Name          string         `json:"name"`
	SeriesID      string         `json:"seriesId,omitempty"`
	SeriesName    string         `json:"seriesName,omitempty"`
	Type          string         `json:"type"`
	StreamURL     string         `json:"streamUrl"`
	PosterURL     string         `json:"posterUrl,omitempty"`
	Container     string         `json:"container,omitempty"`
	Duration      int64          `json:"duration,omitempty"`
	PositionSec   int64          `json:"positionSec,omitempty"`
	Subtitles     []SubtitleInfo `json:"subtitles,omitempty"`
	IsFavorite    bool           `json:"isFavorite"`
	MediaSourceID string         `json:"mediaSourceId,omitempty"`
}

// ServerInfo represents a configured server
type ServerInfo struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	IsActive bool   `json:"isActive"`
	Prefix   string `json:"prefix,omitempty"`
	Latency  int64  `json:"latency,omitempty"`
}

// ServerStatus represents current connection status
type ServerStatus struct {
	Connected    bool        `json:"connected"`
	Server       *ServerInfo `json:"server,omitempty"`
	Latency      int64       `json:"latency,omitempty"`
	MpvAvailable bool        `json:"mpvAvailable"`
	Error        string      `json:"error,omitempty"`
}

// PlaybackRequest represents a playback reporting request
type PlaybackRequest struct {
	Type          string `json:"type"` // start, progress, stop
	ItemID        string `json:"itemId"`
	PositionTicks int64  `json:"positionTicks"`
}

// SearchQuery represents search parameters
type SearchQuery struct {
	Query        string `json:"query"`
	Limit        int    `json:"limit"`
	Page         int    `json:"page,omitempty"`
	ParentID     string `json:"parentId,omitempty"`
	ItemType     string `json:"itemType,omitempty"`     // movie, series, episode
	PlayedFilter string `json:"playedFilter,omitempty"` // played, unplayed
	FavoriteOnly bool   `json:"favoriteOnly,omitempty"`
	Year         int    `json:"year,omitempty"`
}

// Pagination represents pagination parameters
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// FavoriteResult represents the result of a favorite toggle
type FavoriteResult struct {
	IsFavorite bool `json:"isFavorite"`
}

// PlayRequest represents a play request for MPV
type PlayRequest struct {
	ItemID string `json:"itemId"`
}

// PlaySeriesRequest represents a play series request for MPV
type PlaySeriesRequest struct {
	SeriesID       string `json:"seriesId"`
	StartEpisodeID string `json:"startEpisodeId,omitempty"`
}

// PlayResult represents the result of a play operation
type PlayResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// EpisodePlaylist represents a playlist of episodes for series playback
type EpisodePlaylist struct {
	SeriesID   string            `json:"seriesId"`
	SeriesName string            `json:"seriesName"`
	Episodes   []PlaylistEpisode `json:"episodes"`
}

// PlaylistEpisode represents a single episode in a playlist
type PlaylistEpisode struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Index     int    `json:"index"`
	StreamURL string `json:"streamUrl"`
}

// convertAPIItem converts an API MediaItem to service MediaItem
func convertAPIItem(item api.MediaItem, imageBaseURL, token string) MediaItem {
	imageURLs := buildImageCandidateURLs(item, imageBaseURL, token, 400)
	imageURL := firstImageURL(imageURLs)
	imageURLHigh := firstImageURL(buildImageCandidateURLs(item, imageBaseURL, token, 800))
	backdropURL := buildBackdropURL(item, imageBaseURL, token)

	playable := item.Type == "Movie" || item.Type == "Episode" || item.Type == "Video"
	browsable := item.Type == "Series" || item.Type == "Season" ||
		item.Type == "CollectionFolder" || item.Type == "Folder" || item.Type == "BoxSet"

	var userData *UserData
	if item.UserData != nil {
		pct := 0
		if item.RunTimeTicks > 0 && item.UserData.PlaybackPositionTicks > 0 {
			pct = int(float64(item.UserData.PlaybackPositionTicks) / float64(item.RunTimeTicks) * 100)
		}
		userData = &UserData{
			PlaybackPositionTicks: item.UserData.PlaybackPositionTicks,
			Played:                item.UserData.Played,
			IsFavorite:            item.UserData.IsFavorite,
			LastPlayedDate:        item.UserData.LastPlayedDate,
			PlaybackPositionPct:   pct,
		}
	}

	var mediaSources []MediaSource
	for _, ms := range item.MediaSources {
		mediaSources = append(mediaSources, MediaSource{
			ID:        ms.ID,
			Container: ms.Container,
		})
	}

	return MediaItem{
		ID:           item.ID,
		Name:         item.Name,
		Type:         item.Type,
		Year:         item.Year,
		SeriesID:     item.SeriesID,
		SeriesName:   item.SeriesName,
		SeasonID:     item.SeasonID,
		SeasonName:   item.SeasonName,
		ParentID:     item.ParentID,
		IndexNumber:  item.IndexNumber,
		Overview:     item.Overview,
		RunTimeTicks: item.RunTimeTicks,
		ImageURL:     imageURL,
		ImageURLs:    imageURLs,
		ImageURLHigh: imageURLHigh,
		BackdropURL:  backdropURL,
		UserData:     userData,
		MediaSources: mediaSources,
		Playable:     playable,
		Browsable:    browsable,
	}
}

func buildImageCandidateURLs(item api.MediaItem, imageBaseURL, token string, width int) []string {
	var urls []string

	if item.ImageTags.Primary != "" {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.ID, "Primary", width, token))
	}
	if item.ImageTags.Thumb != "" {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.ID, "Thumb", width, token))
	}
	if item.SeriesPrimaryImageTag != "" && item.SeriesID != "" {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.SeriesID, "Primary", width, token))
	}
	if item.SeasonID != "" {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.SeasonID, "Primary", width, token))
	}
	if item.ParentID != "" {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.ParentID, "Primary", width, token))
	}
	if len(item.BackdropImageTags) > 0 {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.ID, "Backdrop", width, token))
	}
	if item.SeriesID != "" {
		urls = appendUniqueImageURL(urls, buildImageURL(imageBaseURL, item.SeriesID, "Backdrop", width, token))
	}

	return urls
}

func buildBackdropURL(item api.MediaItem, imageBaseURL, token string) string {
	if len(item.BackdropImageTags) > 0 {
		return buildImageURL(imageBaseURL, item.ID, "Backdrop", 800, token)
	}
	if item.SeriesID != "" {
		return buildImageURL(imageBaseURL, item.SeriesID, "Backdrop", 800, token)
	}
	return ""
}

func buildImageURL(imageBaseURL, itemID, imageType string, width int, token string) string {
	return fmt.Sprintf("%s/emby/Items/%s/Images/%s?maxWidth=%d&api_key=%s",
		imageBaseURL, itemID, imageType, width, token)
}

func appendUniqueImageURL(urls []string, url string) []string {
	if url == "" {
		return urls
	}
	for _, existing := range urls {
		if existing == url {
			return urls
		}
	}
	return append(urls, url)
}

func firstImageURL(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}
