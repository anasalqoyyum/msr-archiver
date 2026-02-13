package model

// Album is returned by the albums endpoint.
type Album struct {
	CID      string   `json:"cid"`
	Name     string   `json:"name"`
	CoverURL string   `json:"coverUrl"`
	Artistes []string `json:"artistes"`
}

// Song is returned by the album detail endpoint.
type Song struct {
	CID      string   `json:"cid"`
	Name     string   `json:"name"`
	Artistes []string `json:"artistes"`
}

// SongDetail contains URLs for lyric and source assets.
type SongDetail struct {
	LyricURL  string `json:"lyricUrl"`
	SourceURL string `json:"sourceUrl"`
}
