// Package service provides business logic layer for both TUI and Web UI
package service

import (
	"fmt"

	"ember/internal/api"
)

// MediaItem represents a unified media item for UI consumption
type MediaItem struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Year            int        `json:"year,omitempty"`
	SeriesID        string     `json:"seriesId,omitempty"`
	SeriesName      string     `json:"seriesName,omitempty"`
	SeasonID        string     `json:"seasonId,omitempty"`
	SeasonName      string     `json:"seasonName,omitempty"`
	ParentID        string     `json:"parentId,omitempty"`
	IndexNumber     int        `json:"indexNumber,omitempty"`
	Overview        string     `json:"overview,omitempty"`
	RunTimeTicks    int64      `json:"runTimeTicks,omitempty"`
	ImageURL        string     `json:"imageUrl,omitempty"`
	BackdropURL     string     `json:"backdropUrl,omitempty"`
	UserData        *UserData  `json:"userData,omitempty"`
	MediaSources    []MediaSource `json:"mediaSources,omitempty"`
	Playable        bool       `json:"playable"`
	Browsable       bool       `json:"browsable"`
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
	ID         string `json:"id"`
	Container  string `json:"container"`
	Protocol   string `json:"protocol,omitempty"`
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
	Items      []MediaItem `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	HasMore    bool        `json:"hasMore"`
}

// StreamInfo represents streaming information for playback
type StreamInfo struct {
	ItemID       string         `json:"itemId"`
	Name         string         `json:"name"`
	StreamURL    string         `json:"streamUrl"`
	PosterURL    string         `json:"posterUrl,omitempty"`
	Container    string         `json:"container,omitempty"`
	Duration     int64          `json:"duration,omitempty"`
	PositionSec  int64          `json:"positionSec,omitempty"`
	Subtitles    []SubtitleInfo `json:"subtitles,omitempty"`
	IsFavorite   bool           `json:"isFavorite"`
	MediaSourceID string        `json:"mediaSourceId,omitempty"`
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
	Query    string `json:"query"`
	Limit    int    `json:"limit"`
	ParentID string `json:"parentId,omitempty"`
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

// convertAPIItem converts an API MediaItem to service MediaItem
func convertAPIItem(item api.MediaItem, imageBaseURL, token string) MediaItem {
	imageURL := ""
	if item.ImageTags.Primary != "" || item.SeriesID != "" || item.SeasonID != "" {
		imageID := item.ID
		if item.ImageTags.Primary == "" {
			if item.SeriesID != "" {
				imageID = item.SeriesID
			} else if item.SeasonID != "" {
				imageID = item.SeasonID
			} else if item.ParentID != "" {
				imageID = item.ParentID
			}
		}
		imageURL = fmt.Sprintf("%s/emby/Items/%s/Images/Primary?maxWidth=400&api_key=%s",
			imageBaseURL, imageID, token)
	}

	backdropURL := ""
	if item.SeriesID != "" {
		backdropURL = fmt.Sprintf("%s/emby/Items/%s/Images/Backdrop?maxWidth=800&api_key=%s",
			imageBaseURL, item.SeriesID, token)
	}

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
		BackdropURL:  backdropURL,
		UserData:     userData,
		MediaSources: mediaSources,
		Playable:     playable,
		Browsable:    browsable,
	}
}

