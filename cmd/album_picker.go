package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"msr-archiver/internal/api"
	"msr-archiver/internal/model"
	"msr-archiver/internal/state"
)

const (
	albumListHeight  = 12
	detailListHeight = 12
)

type pickerPhase int

const (
	pickerPhaseSelect pickerPhase = iota
	pickerPhaseReview
	pickerPhaseDetail
)

type albumSongsLoadedMsg struct {
	albumIdx int
	songs    []model.Song
	err      error
}

type albumPickerModel struct {
	ctx      context.Context
	api      *api.Client
	albums   []model.Album
	selected map[int]struct{}

	completed map[int]bool
	filtered  []int
	cursor    int

	filterInput textinput.Model
	filtering   bool

	phase        pickerPhase
	reviewCursor int

	detailAlbumIdx int
	detailSongs    []model.Song
	detailErr      string
	detailLoading  bool
	detailOffset   int

	songsCache   map[int][]model.Song
	songErrCache map[int]string

	done    bool
	aborted bool
	err     error
}

func chooseAlbumsInteractively(
	ctx context.Context,
	albums []model.Album,
	store *state.Store,
	apiClient *api.Client,
) ([]model.Album, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect stdin: %w", err)
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return nil, fmt.Errorf("interactive selection requires a terminal; use --albums instead")
	}

	picker := newAlbumPickerModel(ctx, albums, store, apiClient)
	finalModel, err := tea.NewProgram(picker).Run()
	if err != nil {
		return nil, fmt.Errorf("run interactive album selector: %w", err)
	}

	final := finalModel.(*albumPickerModel)
	if final.aborted {
		return nil, fmt.Errorf("interactive selection aborted")
	}

	selectedIndexes := selectedIndexesFromSet(final.selected)
	return albumsFromIndexes(albums, selectedIndexes)
}

func newAlbumPickerModel(ctx context.Context, albums []model.Album, store *state.Store, apiClient *api.Client) *albumPickerModel {
	filter := textinput.New()
	filter.Prompt = "/"

	completed := make(map[int]bool, len(albums))
	for i, album := range albums {
		completed[i] = store != nil && store.IsCompleted(album.Name)
	}

	m := &albumPickerModel{
		ctx:          ctx,
		api:          apiClient,
		albums:       albums,
		selected:     make(map[int]struct{}),
		completed:    completed,
		filterInput:  filter,
		phase:        pickerPhaseSelect,
		songsCache:   make(map[int][]model.Song),
		songErrCache: make(map[int]string),
	}
	m.rebuildFiltered()
	return m
}

func (m *albumPickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *albumPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case albumSongsLoadedMsg:
		if msg.err != nil {
			m.songErrCache[msg.albumIdx] = msg.err.Error()
		} else {
			m.songsCache[msg.albumIdx] = msg.songs
			delete(m.songErrCache, msg.albumIdx)
		}

		if m.phase == pickerPhaseDetail && m.detailAlbumIdx == msg.albumIdx {
			m.detailLoading = false
			if msg.err != nil {
				m.detailErr = msg.err.Error()
				m.detailSongs = nil
			} else {
				m.detailErr = ""
				m.detailSongs = msg.songs
			}
			m.detailOffset = 0
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}

		switch m.phase {
		case pickerPhaseSelect:
			return m.updateSelect(msg)
		case pickerPhaseReview:
			return m.updateReview(msg)
		case pickerPhaseDetail:
			return m.updateDetail(msg)
		}
	}

	return m, nil
}

func (m *albumPickerModel) View() string {
	switch m.phase {
	case pickerPhaseReview:
		return m.reviewView()
	case pickerPhaseDetail:
		return m.detailView()
	default:
		return m.selectionView()
	}
}

func (m *albumPickerModel) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.String() {
		case "enter", "esc":
			m.filtering = false
			m.filterInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.rebuildFiltered()
			return m, cmd
		}
	}

	switch msg.String() {
	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, nil
	case "esc":
		if strings.TrimSpace(m.filterInput.Value()) != "" {
			m.filterInput.SetValue("")
			m.rebuildFiltered()
		}
	case "up", "k":
		if len(m.filtered) > 0 && m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "ctrl+u", "pgup":
		if len(m.filtered) > 0 {
			step := max(1, albumListHeight/2)
			m.cursor = max(0, m.cursor-step)
		}
	case "ctrl+d", "pgdown":
		if len(m.filtered) > 0 {
			step := max(1, albumListHeight/2)
			m.cursor = min(len(m.filtered)-1, m.cursor+step)
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}
	case "x", " ":
		idx, ok := m.currentAlbumIndex()
		if ok {
			m.toggleSelection(idx)
		}
	case "ctrl+a":
		m.toggleSelectAllFiltered()
	case "d":
		return m.showCurrentAlbumDetails()
	case "enter":
		m.phase = pickerPhaseReview
		m.reviewCursor = 0
	}

	return m, nil
}

func (m *albumPickerModel) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.reviewCursor > 0 {
			m.reviewCursor--
		}
	case "down", "j":
		if m.reviewCursor < 1 {
			m.reviewCursor++
		}
	case "b", "esc":
		m.phase = pickerPhaseSelect
	case "d":
		return m.showCurrentAlbumDetails()
	case "s":
		if len(m.selected) > 0 {
			m.done = true
			return m, tea.Quit
		}
	case "enter":
		if m.reviewCursor == 0 {
			if len(m.selected) == 0 {
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
		m.phase = pickerPhaseSelect
	}

	return m, nil
}

func (m *albumPickerModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "esc", "q":
		m.phase = pickerPhaseSelect
		m.detailLoading = false
		return m, nil
	case "x", " ":
		if m.detailAlbumIdx >= 0 && m.detailAlbumIdx < len(m.albums) {
			m.toggleSelection(m.detailAlbumIdx)
		}
		return m, nil
	case "n":
		return m.moveDetail(1)
	case "p":
		return m.moveDetail(-1)
	case "r":
		if m.api != nil && m.detailAlbumIdx >= 0 && m.detailAlbumIdx < len(m.albums) {
			return m.openAlbumDetails(m.detailAlbumIdx, true)
		}
	}

	if m.detailLoading {
		return m, nil
	}

	maxOffset := max(0, len(m.detailSongs)-detailListHeight)
	switch msg.String() {
	case "up", "k":
		if m.detailOffset > 0 {
			m.detailOffset--
		}
	case "down", "j":
		if m.detailOffset < maxOffset {
			m.detailOffset++
		}
	case "ctrl+u", "pgup":
		step := max(1, detailListHeight/2)
		m.detailOffset = max(0, m.detailOffset-step)
	case "ctrl+d", "pgdown":
		step := max(1, detailListHeight/2)
		m.detailOffset = min(maxOffset, m.detailOffset+step)
	}

	return m, nil
}

func (m *albumPickerModel) rebuildFiltered() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.filtered = m.filtered[:0]

	for idx, album := range m.albums {
		if query == "" {
			m.filtered = append(m.filtered, idx)
			continue
		}
		if strings.Contains(strings.ToLower(album.Name), query) || strings.Contains(strings.ToLower(album.CID), query) {
			m.filtered = append(m.filtered, idx)
		}
	}

	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.cursor = min(m.cursor, len(m.filtered)-1)
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *albumPickerModel) currentAlbumIndex() (int, bool) {
	if len(m.filtered) == 0 {
		return 0, false
	}
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return 0, false
	}
	return m.filtered[m.cursor], true
}

func (m *albumPickerModel) toggleSelection(idx int) {
	if _, exists := m.selected[idx]; exists {
		delete(m.selected, idx)
		return
	}
	m.selected[idx] = struct{}{}
}

func (m *albumPickerModel) toggleSelectAllFiltered() {
	if len(m.filtered) == 0 {
		return
	}

	allSelected := true
	for _, idx := range m.filtered {
		if _, ok := m.selected[idx]; !ok {
			allSelected = false
			break
		}
	}

	if allSelected {
		for _, idx := range m.filtered {
			delete(m.selected, idx)
		}
		return
	}

	for _, idx := range m.filtered {
		m.selected[idx] = struct{}{}
	}
}

func (m *albumPickerModel) moveDetail(step int) (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 || step == 0 {
		return m, nil
	}

	pos := 0
	for i, idx := range m.filtered {
		if idx == m.detailAlbumIdx {
			pos = i
			break
		}
	}

	nextPos := pos + step
	if nextPos < 0 {
		nextPos = 0
	}
	if nextPos >= len(m.filtered) {
		nextPos = len(m.filtered) - 1
	}

	m.cursor = nextPos
	return m.openAlbumDetails(m.filtered[nextPos], false)
}

func (m *albumPickerModel) showCurrentAlbumDetails() (tea.Model, tea.Cmd) {
	idx, ok := m.currentAlbumIndex()
	if !ok {
		return m, nil
	}
	return m.openAlbumDetails(idx, false)
}

func (m *albumPickerModel) openAlbumDetails(idx int, forceRefetch bool) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.albums) {
		return m, nil
	}

	m.phase = pickerPhaseDetail
	m.detailAlbumIdx = idx
	m.detailOffset = 0

	if !forceRefetch {
		if songs, ok := m.songsCache[idx]; ok {
			m.detailLoading = false
			m.detailSongs = songs
			m.detailErr = ""
			return m, nil
		}
		if errMsg, ok := m.songErrCache[idx]; ok {
			m.detailLoading = false
			m.detailSongs = nil
			m.detailErr = errMsg
			return m, nil
		}
	} else {
		delete(m.songErrCache, idx)
		delete(m.songsCache, idx)
	}

	if m.api == nil {
		m.detailLoading = false
		m.detailSongs = nil
		m.detailErr = "album detail API client is unavailable"
		return m, nil
	}

	album := m.albums[idx]
	m.detailLoading = true
	m.detailSongs = nil
	m.detailErr = ""
	return m, fetchAlbumSongsCmd(m.ctx, m.api, album.CID, idx)
}

func fetchAlbumSongsCmd(ctx context.Context, apiClient *api.Client, albumCID string, albumIdx int) tea.Cmd {
	return func() tea.Msg {
		songs, err := withRetryResult(ctx, 3, func() ([]model.Song, error) {
			return apiClient.GetAlbumSongs(ctx, albumCID)
		})
		return albumSongsLoadedMsg{albumIdx: albumIdx, songs: songs, err: err}
	}
}

func (m *albumPickerModel) selectionView() string {
	lines := []string{
		fmt.Sprintf("Select albums (%d total, %d selected)", len(m.albums), len(m.selected)),
	}

	if m.filtering {
		lines = append(lines,
			"Filter mode: type album name/CID, then Enter or Esc to apply.",
			m.filterInput.View(),
		)
	} else if q := strings.TrimSpace(m.filterInput.Value()); q != "" {
		lines = append(lines, fmt.Sprintf("Active filter: /%s (press / to edit, Esc to clear)", q))
	} else {
		lines = append(lines, "Filter: press / to search by album name or CID")
	}

	lines = append(lines, "")
	if len(m.filtered) == 0 {
		lines = append(lines, "No albums match the current filter.")
	} else {
		start, end := listWindow(len(m.filtered), m.cursor, albumListHeight)
		for pos := start; pos < end; pos++ {
			albumIdx := m.filtered[pos]
			cursor := " "
			if pos == m.cursor {
				cursor = ">"
			}
			checked := " "
			if _, ok := m.selected[albumIdx]; ok {
				checked = "x"
			}

			label := albumOptionLabel(albumIdx+1, m.albums[albumIdx], m.completed[albumIdx])
			lines = append(lines, fmt.Sprintf("%s [%s] %s", cursor, checked, label))
		}
		if end < len(m.filtered) {
			lines = append(lines, fmt.Sprintf("... %d more album(s)", len(m.filtered)-end))
		}
	}

	lines = append(lines, "")
	if m.filtering {
		lines = append(lines, "Keys: type filter | Enter/Esc apply | Ctrl+C exit")
	} else {
		lines = append(lines, "Keys: j/k move | x toggle | Ctrl+A select all | / filter | d details | Enter review | Ctrl+C exit")
	}

	return strings.Join(lines, "\n")
}

func (m *albumPickerModel) reviewView() string {
	selectedIndexes := selectedIndexesFromSet(m.selected)
	preview := buildSelectedAlbumsPreview(m.albums, selectedIndexes, 8)

	startLabel := "Start download"
	if len(selectedIndexes) == 0 {
		startLabel = "Start download (select at least one album)"
	}
	options := []string{startLabel, "Back to selection"}

	lines := []string{
		"Review selected albums",
		preview,
		"",
	}
	for i, option := range options {
		cursor := " "
		if i == m.reviewCursor {
			cursor = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %s", cursor, option))
	}
	lines = append(lines, "", "Keys: j/k move | Enter choose | b back | d details")

	return strings.Join(lines, "\n")
}

func (m *albumPickerModel) detailView() string {
	if m.detailAlbumIdx < 0 || m.detailAlbumIdx >= len(m.albums) {
		return "Album details unavailable. Press Esc to go back."
	}

	album := m.albums[m.detailAlbumIdx]
	status := "not downloaded"
	if m.completed[m.detailAlbumIdx] {
		status = "downloaded"
	}

	lines := []string{
		fmt.Sprintf("Album details: %s", album.Name),
		fmt.Sprintf("CID: %s | Status: %s", album.CID, status),
	}
	if len(album.Artistes) > 0 {
		lines = append(lines, fmt.Sprintf("Artists: %s", strings.Join(album.Artistes, ", ")))
	}
	lines = append(lines, "")

	if m.detailLoading {
		lines = append(lines, "Loading songs from API...")
		lines = append(lines, "", "Keys: n/p next/prev album | x toggle | Esc/B back")
		return strings.Join(lines, "\n")
	}

	if m.detailErr != "" {
		lines = append(lines, fmt.Sprintf("Failed to load songs: %s", m.detailErr))
		lines = append(lines, "", "Keys: r retry | n/p next/prev album | x toggle | Esc/B back")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, fmt.Sprintf("Songs (%d):", len(m.detailSongs)))
	if len(m.detailSongs) == 0 {
		lines = append(lines, "No songs found for this album.")
		lines = append(lines, "", "Keys: n/p next/prev album | x toggle | Esc/B back")
		return strings.Join(lines, "\n")
	}

	end := min(len(m.detailSongs), m.detailOffset+detailListHeight)
	for i := m.detailOffset; i < end; i++ {
		lines = append(lines, fmt.Sprintf("%3d. %s", i+1, m.detailSongs[i].Name))
	}
	if end < len(m.detailSongs) {
		lines = append(lines, fmt.Sprintf("... %d more song(s)", len(m.detailSongs)-end))
	}

	lines = append(lines, "", "Keys: j/k scroll | r refetch | n/p next/prev album | x toggle | Esc/B back")
	return strings.Join(lines, "\n")
}

func listWindow(total, cursor, size int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if size <= 0 || total <= size {
		return 0, total
	}

	start := cursor - size/2
	if start < 0 {
		start = 0
	}
	if start+size > total {
		start = total - size
	}
	return start, start + size
}
