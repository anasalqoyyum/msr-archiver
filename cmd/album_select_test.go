package main

import (
	"strings"
	"testing"

	"msr-archiver/internal/model"
)

func TestAlbumsFromIndexes(t *testing.T) {
	albums := []model.Album{{CID: "a1", Name: "First"}, {CID: "a2", Name: "Second"}, {CID: "a3", Name: "Third"}}

	got, err := albumsFromIndexes(albums, []int{0, 2})
	if err != nil {
		t.Fatalf("albumsFromIndexes failed: %v", err)
	}

	want := []model.Album{{CID: "a1", Name: "First"}, {CID: "a3", Name: "Third"}}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i].CID != want[i].CID {
			t.Fatalf("unexpected indexes: got=%v want=%v", got, want)
		}
	}
}

func TestAlbumsFromIndexesEmptySelection(t *testing.T) {
	albums := []model.Album{{CID: "a1", Name: "First"}, {CID: "a2", Name: "Second"}}

	_, err := albumsFromIndexes(albums, nil)
	if err == nil {
		t.Fatalf("expected empty selection error")
	}
}

func TestAlbumsFromIndexesOutOfBounds(t *testing.T) {
	albums := []model.Album{{CID: "a1", Name: "First"}}
	_, err := albumsFromIndexes(albums, []int{1})
	if err == nil {
		t.Fatalf("expected out-of-bounds error")
	}
}

func TestSelectedIndexesFromSetSorted(t *testing.T) {
	selected := map[int]struct{}{4: {}, 1: {}, 3: {}}
	got := selectedIndexesFromSet(selected)
	want := []int{1, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected sorted indexes: got=%v want=%v", got, want)
		}
	}
}

func TestAlbumOptionLabelMarksDownloaded(t *testing.T) {
	album := model.Album{CID: "a1", Name: "First"}
	label := albumOptionLabel(1, album, true)
	if label != "  1. First (a1) [downloaded]" {
		t.Fatalf("unexpected label: %q", label)
	}
}

func TestAlbumOptionLabelWithoutDownloadedMarker(t *testing.T) {
	album := model.Album{CID: "a1", Name: "First"}
	label := albumOptionLabel(1, album, false)
	if label != "  1. First (a1)" {
		t.Fatalf("unexpected label: %q", label)
	}
}

func TestBuildSelectedAlbumsPreview(t *testing.T) {
	albums := []model.Album{{CID: "a1", Name: "First"}, {CID: "b2", Name: "Second"}, {CID: "c3", Name: "Third"}}

	preview := buildSelectedAlbumsPreview(albums, []int{0, 2}, 16)
	if !strings.Contains(preview, "Selected 2 album(s):") {
		t.Fatalf("missing selection count in preview: %q", preview)
	}
	if !strings.Contains(preview, "1. First (a1)") {
		t.Fatalf("missing first album line in preview: %q", preview)
	}
	if !strings.Contains(preview, "2. Third (c3)") {
		t.Fatalf("missing second album line in preview: %q", preview)
	}
}

func TestBuildSelectedAlbumsPreviewTruncates(t *testing.T) {
	albums := []model.Album{{CID: "a1", Name: "First"}, {CID: "b2", Name: "Second"}, {CID: "c3", Name: "Third"}}

	preview := buildSelectedAlbumsPreview(albums, []int{0, 1, 2}, 2)
	if !strings.Contains(preview, "... and 1 more") {
		t.Fatalf("expected truncation summary in preview: %q", preview)
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
