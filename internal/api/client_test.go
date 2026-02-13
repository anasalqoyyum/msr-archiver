package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(handler roundTripFunc) *Client {
	httpClient := &http.Client{Transport: handler}
	return New(httpClient)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestGetAlbumsSuccess(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/albums" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if req.Header.Get("Accept") != "application/json" {
			t.Fatalf("missing accept header")
		}
		return response(200, `{"data":[{"cid":"a1","name":"Album","coverUrl":"https://x","artistes":["AA"]}]}`), nil
	})

	albums, err := client.GetAlbums(context.Background())
	if err != nil {
		t.Fatalf("GetAlbums failed: %v", err)
	}
	if len(albums) != 1 || albums[0].CID != "a1" || albums[0].Name != "Album" {
		t.Fatalf("unexpected album payload: %+v", albums)
	}
}

func TestGetAlbumSongsSuccess(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/album/album-1/detail" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return response(200, `{"data":{"songs":[{"cid":"s1","name":"Song","artistes":["A"]}]}}`), nil
	})

	songs, err := client.GetAlbumSongs(context.Background(), "album-1")
	if err != nil {
		t.Fatalf("GetAlbumSongs failed: %v", err)
	}
	if len(songs) != 1 || songs[0].CID != "s1" {
		t.Fatalf("unexpected songs payload: %+v", songs)
	}
}

func TestGetSongDetailSuccess(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/song/song-1" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return response(200, `{"data":{"lyricUrl":"https://lyrics","sourceUrl":"https://audio"}}`), nil
	})

	detail, err := client.GetSongDetail(context.Background(), "song-1")
	if err != nil {
		t.Fatalf("GetSongDetail failed: %v", err)
	}
	if detail.SourceURL != "https://audio" || detail.LyricURL != "https://lyrics" {
		t.Fatalf("unexpected song detail: %+v", detail)
	}
}

func TestGetJSONStatusError(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return response(500, `{"data":[]}`), nil
	})

	_, err := client.GetAlbums(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestGetJSONDecodeError(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return response(200, `{invalid json`), nil
	})

	_, err := client.GetAlbums(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
}
