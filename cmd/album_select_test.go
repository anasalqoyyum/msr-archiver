package main

import (
	"testing"

	"msr-archiver/internal/model"
)

func TestParseAlbumIndexesMixed(t *testing.T) {
	got, err := parseAlbumIndexes("1,3-4,2", 5)
	if err != nil {
		t.Fatalf("parseAlbumIndexes failed: %v", err)
	}

	want := []int{0, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected indexes: got=%v want=%v", got, want)
		}
	}
}

func TestParseAlbumIndexesAll(t *testing.T) {
	got, err := parseAlbumIndexes("all", 3)
	if err != nil {
		t.Fatalf("parseAlbumIndexes failed: %v", err)
	}

	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected indexes: got=%v want=%v", got, want)
		}
	}
}

func TestParseAlbumIndexesOutOfBounds(t *testing.T) {
	_, err := parseAlbumIndexes("4", 3)
	if err == nil {
		t.Fatalf("expected out-of-bounds error")
	}
}

func TestSelectAlbumsByQuery(t *testing.T) {
	albums := []model.Album{
		{CID: "a1", Name: "First Light"},
		{CID: "a2", Name: "Second Dawn"},
		{CID: "a3", Name: "Third Night"},
	}

	got, err := selectAlbumsByQuery(albums, "a2,third")
	if err != nil {
		t.Fatalf("selectAlbumsByQuery failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(got))
	}
	if got[0].CID != "a2" || got[1].CID != "a3" {
		t.Fatalf("unexpected album selection: %+v", got)
	}
}

func TestSelectAlbumsByQueryAmbiguous(t *testing.T) {
	albums := []model.Album{
		{CID: "a1", Name: "Alpha"},
		{CID: "a2", Name: "Alpha Remix"},
	}

	_, err := selectAlbumsByQuery(albums, "alp")
	if err == nil {
		t.Fatalf("expected ambiguous query error")
	}
}
