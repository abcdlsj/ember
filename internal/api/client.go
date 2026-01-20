package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"ember/internal/logging"
)

const (
	clientName  = "Ember"
	deviceName  = "Go"
	deviceID    = "ember-go-001"
	version     = "1.0.0"
	httpTimeout = 15 * time.Second
)

type Client struct {
	Server  string
	UserID  string
	Token   string
	http    *http.Client
	Latency time.Duration
}

type MediaItem struct {
	ID           string        `json:"Id"`
	Name         string        `json:"Name"`
	Type         string        `json:"Type"`
	Year         int           `json:"ProductionYear,omitempty"`
	Overview     string        `json:"Overview,omitempty"`
	SeriesID     string        `json:"SeriesId,omitempty"`
	SeriesName   string        `json:"SeriesName,omitempty"`
	SeasonID     string        `json:"SeasonId,omitempty"`
	SeasonName   string        `json:"SeasonName,omitempty"`
	ParentID     string        `json:"ParentId,omitempty"`
	IndexNumber  int           `json:"IndexNumber,omitempty"`
	RunTimeTicks int64         `json:"RunTimeTicks,omitempty"`
	MediaSources []MediaSource `json:"MediaSources,omitempty"`
	ImageTags    ImageTags     `json:"ImageTags,omitempty"`
	UserData     *UserData     `json:"UserData,omitempty"`
}

func (m *MediaItem) imageID() string {
	if m.ImageTags.Primary != "" {
		return m.ID
	}
	if m.SeriesID != "" {
		return m.SeriesID
	}
	if m.SeasonID != "" {
		return m.SeasonID
	}
	if m.ParentID != "" {
		return m.ParentID
	}
	return m.ID
}

type UserData struct {
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	Played                bool   `json:"Played"`
	IsFavorite            bool   `json:"IsFavorite"`
	LastPlayedDate        string `json:"LastPlayedDate,omitempty"`
}

type ImageTags struct {
	Primary string `json:"Primary,omitempty"`
}

type MediaSource struct {
	ID           string        `json:"Id"`
	Container    string        `json:"Container"`
	MediaStreams []MediaStream `json:"MediaStreams,omitempty"`
}

type MediaStream struct {
	Type         string `json:"Type"`
	Index        int    `json:"Index"`
	Language     string `json:"Language,omitempty"`
	Title        string `json:"Title,omitempty"`
	DisplayTitle string `json:"DisplayTitle,omitempty"`
	IsExternal   bool   `json:"IsExternal"`
	IsDefault    bool   `json:"IsDefault"`
	Codec        string `json:"Codec,omitempty"`
}

type ItemsResponse struct {
	Items      []MediaItem `json:"Items"`
	TotalCount int         `json:"TotalRecordCount"`
}

type AuthResponse struct {
	User        AuthUser `json:"User"`
	AccessToken string   `json:"AccessToken"`
}

type AuthUser struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

func New(server string) *Client {
	return &Client{
		Server: server,
		http: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

func baseParams(limit int) url.Values {
	return url.Values{
		"Limit":            {fmt.Sprintf("%d", limit)},
		"Fields":           {"Overview,MediaSources,ProductionYear"},
		"ImageTypeLimit":   {"1"},
		"EnableImageTypes": {"Primary"},
	}
}

func (c *Client) authHeader() string {
	h := fmt.Sprintf(`MediaBrowser Client="%s", Device="%s", DeviceId="%s", Version="%s"`,
		clientName, deviceName, deviceID, version)
	if c.Token != "" {
		h += fmt.Sprintf(`, Token="%s"`, c.Token)
	}
	return h
}

func (c *Client) request(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.Server+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Emby-Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.http.Do(req)
	c.Latency = time.Since(start)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if logging.IsEnabled() {
		logging.HTTP(method, c.Server+endpoint, resp.StatusCode, string(respBody))
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) Login(username, password string) error {
	body := map[string]string{
		"Username": username,
		"Pw":       password,
	}

	data, err := c.request(context.Background(), "POST", "/emby/Users/AuthenticateByName", body)
	if err != nil {
		return err
	}

	var resp AuthResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	c.UserID = resp.User.ID
	c.Token = resp.AccessToken
	return nil
}

func (c *Client) VerifyToken() bool {
	if c.UserID == "" || c.Token == "" {
		return false
	}
	_, err := c.request(context.Background(), "GET", "/emby/Users/"+c.UserID, nil)
	return err == nil
}

func (c *Client) getItems(endpoint string) ([]MediaItem, error) {
	data, err := c.request(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp ItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) GetLibraries() ([]MediaItem, error) {
	return c.getItems("/emby/Users/" + c.UserID + "/Views")
}

func (c *Client) GetLatest(limit int) ([]MediaItem, error) {
	params := baseParams(limit)
	endpoint := fmt.Sprintf("/emby/Users/%s/Items/Latest?%s", c.UserID, params.Encode())

	data, err := c.request(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var items []MediaItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) GetResume(limit int) ([]MediaItem, error) {
	params := baseParams(limit)
	endpoint := fmt.Sprintf("/emby/Users/%s/Items/Resume?%s", c.UserID, params.Encode())
	return c.getItems(endpoint)
}

func (c *Client) GetItems(parentID string, start, limit int) ([]MediaItem, int, error) {
	params := baseParams(limit)
	params.Set("Recursive", "true")
	params.Set("SortBy", "SortName")
	params.Set("SortOrder", "Ascending")
	params.Set("StartIndex", fmt.Sprintf("%d", start))
	if parentID != "" {
		params.Set("ParentId", parentID)
	}

	endpoint := fmt.Sprintf("/emby/Users/%s/Items?%s", c.UserID, params.Encode())
	data, err := c.request(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}

	var resp ItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Items, resp.TotalCount, nil
}

func (c *Client) Search(query string, limit int) ([]MediaItem, error) {
	params := baseParams(limit)
	params.Set("Recursive", "true")
	params.Set("SearchTerm", query)
	params.Set("Fields", "Overview,MediaSources,ProductionYear,UserData")

	endpoint := fmt.Sprintf("/emby/Users/%s/Items?%s", c.UserID, params.Encode())
	return c.getItems(endpoint)
}

func (c *Client) GetItem(itemID string) (*MediaItem, error) {
	params := url.Values{
		"Fields": {"MediaSources,Overview,UserData"},
	}

	endpoint := fmt.Sprintf("/emby/Users/%s/Items/%s?%s", c.UserID, itemID, params.Encode())
	data, err := c.request(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var item MediaItem
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *Client) GetSeasons(seriesID string) ([]MediaItem, error) {
	endpoint := fmt.Sprintf("/emby/Shows/%s/Seasons?UserId=%s", seriesID, c.UserID)
	return c.getItems(endpoint)
}

func (c *Client) GetEpisodes(seriesID, seasonID string) ([]MediaItem, error) {
	params := url.Values{
		"UserId":   {c.UserID},
		"SeasonId": {seasonID},
		"Fields":   {"MediaSources,Overview"},
	}
	endpoint := fmt.Sprintf("/emby/Shows/%s/Episodes?%s", seriesID, params.Encode())
	return c.getItems(endpoint)
}

func (c *Client) StreamURL(itemID, sourceID, container string) string {
	return fmt.Sprintf("%s/emby/Videos/%s/stream.%s?MediaSourceId=%s&api_key=%s&Static=true",
		c.Server, itemID, container, sourceID, c.Token)
}

func (c *Client) ImageURL(item MediaItem, width int) string {
	imageID := item.imageID()
	return fmt.Sprintf("%s/emby/Items/%s/Images/Primary?maxWidth=%d&api_key=%s",
		c.Server, imageID, width, c.Token)
}

func (c *Client) ImageURLByID(itemID string, width int) string {
	return fmt.Sprintf("%s/emby/Items/%s/Images/Primary?maxWidth=%d&api_key=%s",
		c.Server, itemID, width, c.Token)
}

func (c *Client) SubtitleURL(itemID, sourceID string, index int) string {
	return fmt.Sprintf("%s/emby/Videos/%s/%s/Subtitles/%d/Stream.srt?api_key=%s",
		c.Server, itemID, sourceID, index, c.Token)
}

func (c *Client) Ping() time.Duration {
	start := time.Now()
	c.request(context.Background(), "GET", "/emby/System/Info/Public", nil)
	return time.Since(start)
}

func (c *Client) playbackBody(itemID, mediaSourceID, playSessionID string, positionTicks int64) map[string]any {
	return map[string]any{
		"ItemId":        itemID,
		"MediaSourceId": mediaSourceID,
		"PlaySessionId": playSessionID,
		"PositionTicks": positionTicks,
	}
}

func (c *Client) ReportPlaybackStart(itemID, mediaSourceID, playSessionID string, positionTicks int64) error {
	body := c.playbackBody(itemID, mediaSourceID, playSessionID, positionTicks)
	body["CanSeek"] = true
	body["PlayMethod"] = "DirectStream"
	_, err := c.request(context.Background(), "POST", "/emby/Sessions/Playing", body)
	return err
}

func (c *Client) ReportPlaybackProgress(itemID, mediaSourceID, playSessionID string, positionTicks int64, isPaused bool) error {
	body := c.playbackBody(itemID, mediaSourceID, playSessionID, positionTicks)
	body["CanSeek"] = true
	body["PlayMethod"] = "DirectStream"
	body["IsPaused"] = isPaused
	_, err := c.request(context.Background(), "POST", "/emby/Sessions/Playing/Progress", body)
	return err
}

func (c *Client) ReportPlaybackStopped(itemID, mediaSourceID, playSessionID string, positionTicks int64) error {
	body := c.playbackBody(itemID, mediaSourceID, playSessionID, positionTicks)
	_, err := c.request(context.Background(), "POST", "/emby/Sessions/Playing/Stopped", body)
	return err
}

func (c *Client) AddFavorite(itemID string) error {
	endpoint := fmt.Sprintf("/emby/Users/%s/FavoriteItems/%s", c.UserID, itemID)
	_, err := c.request(context.Background(), "POST", endpoint, nil)
	return err
}

func (c *Client) RemoveFavorite(itemID string) error {
	endpoint := fmt.Sprintf("/emby/Users/%s/FavoriteItems/%s", c.UserID, itemID)
	_, err := c.request(context.Background(), "DELETE", endpoint, nil)
	return err
}

func (c *Client) GetFavorites(limit int) ([]MediaItem, error) {
	params := baseParams(limit)
	params.Set("Recursive", "true")
	params.Set("Fields", "Overview,MediaSources,ProductionYear,UserData")
	params.Set("Filters", "IsFavorite")
	params.Set("SortBy", "DatePlayed")
	params.Set("SortOrder", "Descending")
	params.Set("IncludeItemTypes", "Movie,Series,Episode")

	endpoint := fmt.Sprintf("/emby/Users/%s/Items?%s", c.UserID, params.Encode())
	return c.getItems(endpoint)
}

func (c *Client) GetResumeItems(limit int) ([]MediaItem, error) {
	params := baseParams(limit)
	params.Set("Recursive", "true")
	params.Set("Fields", "Overview,MediaSources,ProductionYear,UserData")
	params.Set("Filters", "IsResumable")
	params.Set("SortBy", "DatePlayed")
	params.Set("SortOrder", "Descending")
	params.Set("IncludeItemTypes", "Movie,Episode")

	endpoint := fmt.Sprintf("/emby/Users/%s/Items?%s", c.UserID, params.Encode())
	return c.getItems(endpoint)
}
