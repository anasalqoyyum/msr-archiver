package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"msr-archiver/internal/model"
)

const baseURL = "https://monster-siren.hypergryph.com/api"

// Client wraps calls to Monster Siren API.
type Client struct {
	httpClient *http.Client
}

// New creates an API client.
func New(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

type apiResp[T any] struct {
	Data T `json:"data"`
}

type albumDetail struct {
	Songs []model.Song `json:"songs"`
}

// GetAlbums returns all albums.
func (c *Client) GetAlbums(ctx context.Context) ([]model.Album, error) {
	url := fmt.Sprintf("%s/albums", baseURL)
	var out apiResp[[]model.Album]
	if err := c.getJSON(ctx, url, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetAlbumSongs returns songs for an album.
func (c *Client) GetAlbumSongs(ctx context.Context, albumCID string) ([]model.Song, error) {
	url := fmt.Sprintf("%s/album/%s/detail", baseURL, albumCID)
	var out apiResp[albumDetail]
	if err := c.getJSON(ctx, url, &out); err != nil {
		return nil, err
	}
	return out.Data.Songs, nil
}

// GetSongDetail returns the source and lyric URLs for a song.
func (c *Client) GetSongDetail(ctx context.Context, songCID string) (model.SongDetail, error) {
	url := fmt.Sprintf("%s/song/%s", baseURL, songCID)
	var out apiResp[model.SongDetail]
	if err := c.getJSON(ctx, url, &out); err != nil {
		return model.SongDetail{}, err
	}
	return out.Data, nil
}

func (c *Client) getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request %s: unexpected status %d", url, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}

	return nil
}
