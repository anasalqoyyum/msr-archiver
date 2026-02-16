import { useKeyboard, useRenderer, useTerminalDimensions } from '@opentui/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ApiClient } from '../lib/api'
import { listWindow } from '../lib/selection'
import type { Album, Song } from '../lib/types'
import { emitInlineCover, ensureTmuxPassthrough } from './cover_art'
import type { DownloadEvent, DownloadSummary } from './download_types'
import type { AlbumArtCapability } from './image_mode'

interface AppProps {
  albums: Album[]
  completedAlbumNames: Set<string>
  apiClient: ApiClient
  albumArt: AlbumArtCapability
  onAbort: () => void
  onClose: () => void
  onStartDownload: (selected: Album[], emit: (event: DownloadEvent) => void) => Promise<DownloadSummary>
}

type PickerPhase = 'select' | 'confirm' | 'downloading' | 'finished'

interface AlbumDownloadView {
  cid: string
  name: string
  status: 'queued' | 'running' | 'completed' | 'skipped' | 'failed'
  totalSongs: number
  doneSongs: number
  currentSong: string
  currentSongPercent: number
  message: string
}

const MIN_ALBUM_LIST_HEIGHT = 6
const NON_LIST_LAYOUT_OVERHEAD = 14
const SONG_PREVIEW_HEIGHT = 18
const INLINE_IMAGE_ID = 777
const INLINE_IMAGE_WIDTH = 30
const INLINE_IMAGE_HEIGHT = 12
const REVIEW_PREVIEW_LIMIT = 10
const DOWNLOAD_LOG_LINES = 8
const DOWNLOAD_LOG_BOX_HEIGHT = 11
const SPINNER_FRAMES = ['|', '/', '-', '\\']

export function App(props: AppProps) {
  const renderer = useRenderer()
  const { width, height } = useTerminalDimensions()

  const [phase, setPhase] = useState<PickerPhase>('select')
  const [filtering, setFiltering] = useState(false)
  const [filterValue, setFilterValue] = useState('')
  const [cursor, setCursor] = useState(0)
  const [selected, setSelected] = useState<Set<number>>(new Set())

  const [songsByAlbum, setSongsByAlbum] = useState<Record<number, Song[]>>({})
  const [songErrors, setSongErrors] = useState<Record<number, string>>({})
  const [loadingAlbumIdx, setLoadingAlbumIdx] = useState<number | null>(null)

  const [inlineStateByAlbum, setInlineStateByAlbum] = useState<Record<number, 'success' | 'failed'>>({})
  const [inlineNotice, setInlineNotice] = useState('')

  const [downloadAlbumOrder, setDownloadAlbumOrder] = useState<string[]>([])
  const [downloadByAlbum, setDownloadByAlbum] = useState<Record<string, AlbumDownloadView>>({})
  const [downloadLogs, setDownloadLogs] = useState<string[]>([])
  const [downloadSummary, setDownloadSummary] = useState<DownloadSummary | null>(null)
  const [downloadWorkers, setDownloadWorkers] = useState(0)
  const [downloadError, setDownloadError] = useState('')
  const [spinnerTick, setSpinnerTick] = useState(0)

  const requestToken = useRef(0)
  const inlineRenderedByAlbum = useRef<Set<number>>(new Set())

  useEffect(() => {
    if (props.albumArt.inlineProtocol === 'kitty') {
      renderer.disableStdoutInterception()
    }
  }, [props.albumArt.inlineProtocol, renderer])

  useEffect(() => {
    if (props.albumArt.inlineProtocol === 'kitty' && process.env.TMUX) {
      void ensureTmuxPassthrough()
    }
  }, [props.albumArt.inlineProtocol])

  useEffect(() => {
    if (phase !== 'downloading') {
      return
    }
    const timer = setInterval(() => setSpinnerTick(prev => prev + 1), 120)
    return () => clearInterval(timer)
  }, [phase])

  const filtered = useMemo(() => {
    const query = filterValue.trim().toLowerCase()
    return props.albums
      .map((album, idx) => ({ album, idx }))
      .filter(({ album }) => {
        if (!query) return true
        return album.name.toLowerCase().includes(query) || album.cid.toLowerCase().includes(query)
      })
      .map(({ idx }) => idx)
  }, [filterValue, props.albums])

  useEffect(() => {
    if (filtered.length === 0) {
      setCursor(0)
      return
    }
    if (cursor >= filtered.length) {
      setCursor(filtered.length - 1)
    }
  }, [cursor, filtered])

  const focusedAlbumIdx = filtered.length > 0 ? filtered[cursor] : -1
  const focusedAlbum = focusedAlbumIdx >= 0 ? props.albums[focusedAlbumIdx] : null

  const inlinePosition = useMemo(() => {
    const margin = 2
    const col = Math.max(1, width - INLINE_IMAGE_WIDTH - margin)
    const row = Math.max(1, height - INLINE_IMAGE_HEIGHT - margin)
    return { row, col }
  }, [height, width])

  const albumListHeight = useMemo(() => {
    return Math.max(MIN_ALBUM_LIST_HEIGHT, height - NON_LIST_LAYOUT_OVERHEAD)
  }, [height])

  useEffect(() => {
    if (phase !== 'select') {
      return
    }
    if (!focusedAlbum || focusedAlbumIdx < 0) {
      return
    }
    if (songsByAlbum[focusedAlbumIdx] || songErrors[focusedAlbumIdx]) {
      return
    }

    requestToken.current += 1
    const token = requestToken.current
    setLoadingAlbumIdx(focusedAlbumIdx)

    props.apiClient
      .getAlbumSongs(focusedAlbum.cid)
      .then(songs => {
        if (token !== requestToken.current) return
        setSongsByAlbum((prev: Record<number, Song[]>) => ({ ...prev, [focusedAlbumIdx]: songs }))
        setSongErrors((prev: Record<number, string>) => {
          const next = { ...prev }
          delete next[focusedAlbumIdx]
          return next
        })
      })
      .catch(error => {
        if (token !== requestToken.current) return
        setSongErrors((prev: Record<number, string>) => ({ ...prev, [focusedAlbumIdx]: (error as Error).message }))
      })
      .finally(() => {
        if (token !== requestToken.current) return
        setLoadingAlbumIdx(null)
      })
  }, [focusedAlbum, focusedAlbumIdx, phase, props.apiClient, songErrors, songsByAlbum])

  useEffect(() => {
    if (phase !== 'select') {
      return
    }
    if (!focusedAlbum || focusedAlbumIdx < 0) {
      return
    }
    if (props.albumArt.inlineProtocol !== 'kitty') {
      return
    }
    if (inlineRenderedByAlbum.current.has(focusedAlbumIdx)) {
      return
    }

    inlineRenderedByAlbum.current.add(focusedAlbumIdx)
    setInlineNotice('Auto rendering inline cover via kitty protocol...')

    void emitInlineCover(focusedAlbum.coverUrl, INLINE_IMAGE_WIDTH, INLINE_IMAGE_HEIGHT, INLINE_IMAGE_ID, inlinePosition)
      .then(() => {
        setInlineStateByAlbum(prev => ({ ...prev, [focusedAlbumIdx]: 'success' }))
        setInlineNotice('Auto inline cover rendered at bottom-right.')
      })
      .catch(error => {
        inlineRenderedByAlbum.current.delete(focusedAlbumIdx)
        setInlineStateByAlbum(prev => ({ ...prev, [focusedAlbumIdx]: 'failed' }))
        setInlineNotice(`Inline render failed: ${(error as Error).message}`)
      })
  }, [focusedAlbum, focusedAlbumIdx, inlinePosition, phase, props.albumArt.inlineProtocol])

  const pushLogLine = useCallback((line: string) => {
    setDownloadLogs(prev => {
      const next = [...prev, line]
      if (next.length <= 120) return next
      return next.slice(next.length - 120)
    })
  }, [])

  const onDownloadEvent = useCallback(
    (event: DownloadEvent) => {
      switch (event.type) {
        case 'session-start':
          setDownloadWorkers(event.workers)
          pushLogLine(`[info] Starting ${event.totalAlbums} album(s) with ${event.workers} worker(s)`) 
          return
        case 'album-state':
          setDownloadByAlbum(prev => {
            const current = prev[event.albumCid] ?? {
              cid: event.albumCid,
              name: event.albumName,
              status: 'queued',
              totalSongs: 0,
              doneSongs: 0,
              currentSong: '',
              currentSongPercent: 0,
              message: '',
            }
            return {
              ...prev,
              [event.albumCid]: {
                ...current,
                name: event.albumName,
                status: event.status,
                message: event.message ?? current.message,
              },
            }
          })
          return
        case 'album-songs-total':
          setDownloadByAlbum(prev => {
            const current = prev[event.albumCid]
            if (!current) return prev
            return {
              ...prev,
              [event.albumCid]: {
                ...current,
                totalSongs: event.totalSongs,
                doneSongs: Math.min(current.doneSongs, event.totalSongs),
              },
            }
          })
          return
        case 'song-start':
          setDownloadByAlbum(prev => {
            const current = prev[event.albumCid]
            if (!current) return prev
            return {
              ...prev,
              [event.albumCid]: {
                ...current,
                status: 'running',
                totalSongs: event.total,
                doneSongs: Math.max(current.doneSongs, event.track - 1),
                currentSong: event.songName,
                currentSongPercent: 0,
              },
            }
          })
          return
        case 'song-progress':
          setDownloadByAlbum(prev => {
            const current = prev[event.albumCid]
            if (!current) return prev
            return {
              ...prev,
              [event.albumCid]: {
                ...current,
                status: 'running',
                totalSongs: event.total,
                doneSongs: Math.max(current.doneSongs, event.track - 1),
                currentSong: event.songName,
                currentSongPercent: event.percent,
              },
            }
          })
          return
        case 'song-done':
          setDownloadByAlbum(prev => {
            const current = prev[event.albumCid]
            if (!current) return prev
            return {
              ...prev,
              [event.albumCid]: {
                ...current,
                totalSongs: event.total,
                doneSongs: Math.max(current.doneSongs, event.track),
                currentSong: event.songName,
                currentSongPercent: 100,
              },
            }
          })
          return
        case 'log':
          pushLogLine(`[${event.level}] ${event.message}`)
          return
        case 'session-finished':
          setDownloadSummary(event.summary)
      }
    },
    [pushLogLine],
  )

  const beginDownload = useCallback(() => {
    const ordered = Array.from(selected).sort((a: number, b: number) => a - b)
    if (ordered.length === 0) {
      setInlineNotice('Pick at least one album before starting.')
      setPhase('select')
      return
    }

    const picked = ordered.map(idx => props.albums[idx])
    const initialState: Record<string, AlbumDownloadView> = {}
    for (const album of picked) {
      initialState[album.cid] = {
        cid: album.cid,
        name: album.name,
        status: 'queued',
        totalSongs: 0,
        doneSongs: 0,
        currentSong: '',
        currentSongPercent: 0,
        message: 'Queued',
      }
    }

    setDownloadAlbumOrder(picked.map(a => a.cid))
    setDownloadByAlbum(initialState)
    setDownloadLogs([])
    setDownloadSummary(null)
    setDownloadWorkers(0)
    setDownloadError('')
    setPhase('downloading')

    void props
      .onStartDownload(picked, onDownloadEvent)
      .then(summary => {
        setDownloadSummary(summary)
        setPhase('finished')
      })
      .catch(error => {
        setDownloadError((error as Error).message)
        setPhase('finished')
      })
  }, [onDownloadEvent, props, selected])

  useKeyboard(event => {
    const isEnter = event.name === 'enter' || event.name === 'return'

    if (event.ctrl && event.name === 'c') {
      if (phase === 'downloading') {
        pushLogLine('[warn] Cannot abort while downloads are active. Wait for completion.')
        return
      }
      props.onAbort()
      return
    }

    if (phase === 'finished') {
      if (isEnter || event.name === 'q' || event.name === 'escape') {
        props.onClose()
      }
      return
    }

    if (phase === 'downloading') {
      return
    }

    if (phase === 'confirm') {
      if (event.name === 'escape' || event.name === 'b' || event.name === 'n') {
        setPhase('select')
        return
      }
      if (isEnter || event.name === 's' || event.name === 'y') {
        beginDownload()
      }
      return
    }

    if (filtering) {
      if (event.name === 'escape' || isEnter) {
        setFiltering(false)
      }
      return
    }

    if (event.name === '/') {
      setFiltering(true)
      return
    }

    if (event.name === 'escape') {
      if (filterValue.trim() !== '') {
        setFilterValue('')
      }
      return
    }

    if (event.name === 'up' || event.name === 'k') {
      setCursor((c: number) => Math.max(0, c - 1))
      return
    }
    if (event.name === 'down' || event.name === 'j') {
      setCursor((c: number) => Math.min(Math.max(0, filtered.length - 1), c + 1))
      return
    }
    if (event.name === 'pageup' || (event.ctrl && event.name === 'u')) {
      setCursor((c: number) => Math.max(0, c - Math.max(1, Math.floor(albumListHeight / 2))))
      return
    }
    if (event.name === 'pagedown' || (event.ctrl && event.name === 'd')) {
      setCursor((c: number) => Math.min(Math.max(0, filtered.length - 1), c + Math.max(1, Math.floor(albumListHeight / 2))))
      return
    }
    if (event.name === 'home' || event.name === 'g') {
      setCursor(0)
      return
    }
    if (event.name === 'end') {
      setCursor(Math.max(0, filtered.length - 1))
      return
    }

    if (event.ctrl && event.name === 'a') {
      if (filtered.length === 0) return
      setSelected((prev: Set<number>) => {
        const next = new Set(prev)
        const allSelected = filtered.every(idx => next.has(idx))
        if (allSelected) {
          for (const idx of filtered) next.delete(idx)
        } else {
          for (const idx of filtered) next.add(idx)
        }
        return next
      })
      return
    }

    if (event.name === 'space' || event.name === 'x') {
      if (focusedAlbumIdx < 0) return
      setSelected((prev: Set<number>) => {
        const next = new Set(prev)
        if (next.has(focusedAlbumIdx)) next.delete(focusedAlbumIdx)
        else next.add(focusedAlbumIdx)
        return next
      })
      return
    }

    if (isEnter) {
      if (selected.size === 0) {
        setInlineNotice('Select at least one album before continuing.')
        return
      }
      setPhase('confirm')
      return
    }

    if (event.name === 'i') {
      if (!focusedAlbum) {
        setInlineNotice('No focused album to render.')
        return
      }
      if (props.albumArt.inlineProtocol !== 'kitty') {
        setInlineNotice('Kitty protocol unsupported; cover rendering disabled.')
        return
      }

      setInlineNotice('Sending inline cover image sequence...')
      void emitInlineCover(focusedAlbum.coverUrl, INLINE_IMAGE_WIDTH, INLINE_IMAGE_HEIGHT, INLINE_IMAGE_ID, inlinePosition)
        .then(() => {
          setInlineStateByAlbum(prev => ({ ...prev, [focusedAlbumIdx]: 'success' }))
          setInlineNotice('Inline cover rendered at bottom-right.')
        })
        .catch(error => {
          setInlineStateByAlbum(prev => ({ ...prev, [focusedAlbumIdx]: 'failed' }))
          setInlineNotice(`Inline cover failed: ${(error as Error).message}`)
        })
    }
  })

  const [start, end] = listWindow(filtered.length, cursor, albumListHeight)
  const albumListText = useMemo(() => {
    if (filtered.length === 0) {
      return 'No albums match current filter.'
    }

    const lines: string[] = []
    for (let pos = start; pos < end; pos += 1) {
      const albumIdx = filtered[pos]
      const album = props.albums[albumIdx]
      const order = pos + 1
      const cursorMark = pos === cursor ? '>' : ' '
      const checked = selected.has(albumIdx) ? 'x' : ' '
      const completed = props.completedAlbumNames.has(album.name) ? ' [downloaded]' : ''
      lines.push(`${cursorMark} [${checked}] ${order}. ${album.name} (${album.cid})${completed}`)
    }

    if (end < filtered.length) {
      lines.push(`... ${filtered.length - end} more album(s)`)
    }

    return lines.join('\n')
  }, [cursor, end, filtered, props.albums, props.completedAlbumNames, selected, start])

  const selectedPreviewText = useMemo(() => {
    const ordered = Array.from(selected).sort((a: number, b: number) => a - b)
    if (ordered.length === 0) return '(none)'

    const lines = ordered.slice(0, REVIEW_PREVIEW_LIMIT).map((idx, i) => {
      const album = props.albums[idx]
      return `${i + 1}. ${album.name} (${album.cid})`
    })
    if (ordered.length > REVIEW_PREVIEW_LIMIT) {
      lines.push(`... ${ordered.length - REVIEW_PREVIEW_LIMIT} more selected album(s)`)
    }
    return lines.join('\n')
  }, [props.albums, selected])

  const previewSongs = focusedAlbumIdx >= 0 ? songsByAlbum[focusedAlbumIdx] : undefined
  const previewError = focusedAlbumIdx >= 0 ? songErrors[focusedAlbumIdx] : undefined
  const previewLoading = loadingAlbumIdx === focusedAlbumIdx
  const inlineState = focusedAlbumIdx >= 0 ? inlineStateByAlbum[focusedAlbumIdx] : undefined

  const spinner = SPINNER_FRAMES[spinnerTick % SPINNER_FRAMES.length]
  const sortedDownloadAlbums = useMemo(() => {
    const statusPriority: Record<AlbumDownloadView['status'], number> = {
      running: 0,
      failed: 1,
      queued: 2,
      completed: 3,
      skipped: 4,
    }
    return downloadAlbumOrder
      .map((cid, order) => ({ order, data: downloadByAlbum[cid] }))
      .filter(entry => Boolean(entry.data))
      .sort((a, b) => {
        const pa = statusPriority[a.data.status]
        const pb = statusPriority[b.data.status]
        if (pa !== pb) return pa - pb
        return a.order - b.order
      })
      .map(entry => entry.data)
  }, [downloadAlbumOrder, downloadByAlbum])

  const doneAlbumsCount = useMemo(() => {
    return sortedDownloadAlbums.filter(a => a.status === 'completed' || a.status === 'skipped' || a.status === 'failed').length
  }, [sortedDownloadAlbums])

  const runningAlbumsCount = useMemo(() => {
    return sortedDownloadAlbums.filter(a => a.status === 'running').length
  }, [sortedDownloadAlbums])

  const downloadHeaderBar = renderProgressBar(
    downloadAlbumOrder.length === 0 ? 0 : doneAlbumsCount / downloadAlbumOrder.length,
    Math.max(18, Math.min(52, width - 36)),
    phase === 'downloading',
    spinnerTick,
  )

  const downloadListText = useMemo(() => {
    if (sortedDownloadAlbums.length === 0) {
      return 'Waiting for jobs...'
    }

    const rows = Math.max(6, Math.floor(height * 0.33))
    const lines: string[] = []

    for (let i = 0; i < Math.min(rows, sortedDownloadAlbums.length); i += 1) {
      const album = sortedDownloadAlbums[i]
      const shortName = truncate(album.name, 30)
      const songInfo = album.totalSongs > 0 ? `${album.doneSongs}/${album.totalSongs}` : '--/--'
      const songProgress = album.totalSongs > 0 ? album.doneSongs / album.totalSongs : album.currentSongPercent / 100
      const songBar = renderProgressBar(songProgress, 14, album.status === 'running', spinnerTick)
      const status = statusTag(album.status)
      const currentSong = album.currentSong ? ` | ${truncate(album.currentSong, 22)} ${Math.round(album.currentSongPercent)}%` : ''
      lines.push(`${status} ${shortName} | songs ${songInfo} ${songBar}${currentSong}`)
    }

    if (sortedDownloadAlbums.length > rows) {
      lines.push(`... ${sortedDownloadAlbums.length - rows} more album(s)`)
    }
    return lines.join('\n')
  }, [height, sortedDownloadAlbums, spinnerTick])

  const downloadLogsText = useMemo(() => {
    if (downloadLogs.length === 0) {
      return 'No logs yet.'
    }
    return downloadLogs.slice(Math.max(0, downloadLogs.length - DOWNLOAD_LOG_LINES)).join('\n')
  }, [downloadLogs])

  if (phase === 'confirm') {
    return (
      <box flexDirection="column" width="100%" height="100%" padding={1}>
        <box border title="Confirm Download" padding={1} marginBottom={1}>
          <text>{selected.size} album(s) selected. Start download now?</text>
          <text>After start, downloads continue in this TUI with live progress.</text>
        </box>

        <box border title="Selected Albums" padding={1} flexGrow={1} marginBottom={1}>
          <text>{selectedPreviewText}</text>
        </box>

        <box border title="Actions" padding={1}>
          <text>Enter / s / y: Start download</text>
          <text>Esc / b / n: Back to selection</text>
        </box>
      </box>
    )
  }

  if (phase === 'downloading' || phase === 'finished') {
    return (
      <box flexDirection="column" width="100%" height="100%" padding={1}>
        <box border title="Download Progress" padding={1} marginBottom={1}>
          <text>
            {phase === 'downloading' ? `${spinner} Downloading` : 'Done'} | workers {downloadWorkers || '--'} | albums {doneAlbumsCount}/
            {downloadAlbumOrder.length} | active {runningAlbumsCount}
          </text>
          <text>{downloadHeaderBar}</text>
          {downloadSummary ? (
            <text>
              Completed {downloadSummary.completedAlbums}, skipped {downloadSummary.skippedAlbums}, failed {downloadSummary.failedAlbums} in{' '}
              {Math.round(downloadSummary.durationMs / 1000)}s
            </text>
          ) : null}
          {downloadError ? <text>Error: {downloadError}</text> : null}
          {phase === 'finished' ? <text>Press Enter, q, or Esc to exit.</text> : null}
        </box>

        <box border title="Albums" padding={1} flexGrow={1} marginBottom={1}>
          <text>{downloadListText}</text>
        </box>

        <box border title="Logs" padding={1} height={DOWNLOAD_LOG_BOX_HEIGHT}>
          <text>{downloadLogsText}</text>
        </box>
      </box>
    )
  }

  return (
    <box flexDirection="column" width="100%" height="100%" padding={1}>
      <box border title="msr-archiver-tui" padding={1} marginBottom={1}>
        <text>
          <strong>Select albums</strong> ({props.albums.length} total, {selected.size} selected) | Enter continue | Ctrl+C exit
        </text>
      </box>

      <box flexDirection="row" gap={1} flexGrow={1}>
        <box border title="Albums" width="55%" padding={1} flexDirection="column">
          {filtering ? (
            <box marginBottom={1}>
              <input value={filterValue} onChange={setFilterValue} focused placeholder="Type album name or CID" width="100%" />
            </box>
          ) : (
            <box marginBottom={1}>
              <text>
                Filter: {filterValue.trim() === '' ? '(none)' : `/${filterValue}`} | / edit | Esc clear | x/Space toggle | Ctrl+A all
              </text>
            </box>
          )}

          <text>{albumListText}</text>
        </box>

        <box border title="Songs Preview" width="45%" padding={1} flexDirection="column">
          {!focusedAlbum ? (
            <text>No album focused.</text>
          ) : (
            <>
              <text>
                <strong>{focusedAlbum.name}</strong>
              </text>
              <text>CID: {focusedAlbum.cid}</text>
              <text>Artists: {focusedAlbum.artistes.join(', ') || '(unknown)'}</text>
              {inlineState === 'failed' ? <text>Inline cover failed for this album.</text> : null}
              <text>
                {props.albumArt.inlineProtocol === 'kitty'
                  ? 'Auto inline render enabled for Kitty/Ghostty. Press i to rerender.'
                  : 'Image rendering disabled: terminal does not support kitty protocol.'}
              </text>
              {inlineNotice ? <text>{inlineNotice}</text> : null}
              <text>Cover URL: {focusedAlbum.coverUrl}</text>
              <text> </text>

              <text>
                <strong>Songs</strong>
              </text>

              {previewLoading ? <text>Loading songs...</text> : null}
              {previewError ? <text>Failed to load songs: {previewError}</text> : null}
              {!previewLoading && !previewError && previewSongs && previewSongs.length === 0 ? (
                <text>No songs found for this album.</text>
              ) : null}

              {!previewLoading && !previewError && previewSongs ? (
                <box flexDirection="column">
                  {previewSongs.slice(0, SONG_PREVIEW_HEIGHT).map((song: Song, idx: number) => (
                    <text key={song.cid}>
                      {idx + 1}. {song.name}
                    </text>
                  ))}
                  {previewSongs.length > SONG_PREVIEW_HEIGHT ? <text>... {previewSongs.length - SONG_PREVIEW_HEIGHT} more song(s)</text> : null}
                </box>
              ) : null}
            </>
          )}
        </box>
      </box>
    </box>
  )
}

function truncate(input: string, max: number): string {
  if (input.length <= max) return input
  if (max <= 3) return input.slice(0, max)
  return `${input.slice(0, max - 3)}...`
}

function renderProgressBar(ratio: number, width: number, animated: boolean, tick: number): string {
  const safeWidth = Math.max(8, width)
  const clamped = Number.isFinite(ratio) ? Math.max(0, Math.min(1, ratio)) : 0
  const filled = Math.round(clamped * safeWidth)

  const cells: string[] = Array.from({ length: safeWidth }, (_, i) => (i < filled ? '=' : '-'))
  if (animated && filled < safeWidth) {
    const pulse = Math.max(filled, (tick % safeWidth) + 1) - 1
    if (pulse >= 0 && pulse < safeWidth) {
      cells[pulse] = '>'
    }
  }

  const percent = Math.round(clamped * 100)
  return `[${cells.join('')}] ${percent}%`
}

function statusTag(status: AlbumDownloadView['status']): string {
  if (status === 'running') return '[>]'
  if (status === 'completed') return '[ok]'
  if (status === 'skipped') return '[sk]'
  if (status === 'failed') return '[!!]'
  return '[..]'
}
