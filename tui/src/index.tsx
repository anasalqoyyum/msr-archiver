import { createCliRenderer } from '@opentui/core'
import { createRoot } from '@opentui/react'
import { mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import React from 'react'
import type { Album, AppConfig, Song } from './lib/types'
import { ApiClient } from './lib/api'
import { loadAlbumCache, saveAlbumCache, shouldUseCachedAlbums } from './lib/cache'
import { parseConfig, printHelp } from './lib/config'
import { Downloader } from './lib/downloader'
import { applyMetadata, convertToPng, ensureFFmpeg, removeIfExists } from './lib/ffmpeg'
import { runPool } from './lib/pool'
import { withRetry } from './lib/retry'
import { makeValidName } from './lib/sanitize'
import { selectAlbumsByQuery } from './lib/selection'
import { CompletedAlbumStore } from './lib/state'
import { App } from './ui/App'
import type { DownloadEvent, DownloadSummary } from './ui/download_types'
import { detectAlbumArtCapability } from './ui/image_mode'

function logInfo(message: string): void {
  process.stderr.write(`${message}\n`)
}

async function main() {
  const args = process.argv.slice(2)
  if (args.includes('--help') || args.includes('-h')) {
    printHelp()
    return
  }

  const cfg = parseConfig(args)
  await ensureFFmpeg()
  await mkdir(cfg.outputDir, { recursive: true })

  const apiClient = new ApiClient(cfg.httpTimeoutMs)
  const downloader = new Downloader()
  const store = new CompletedAlbumStore(join(cfg.outputDir, 'completed_albums.json'))
  await store.load()

  const albums = await loadAlbums(cfg, apiClient)
  const interactiveMode = cfg.albums.trim() === '' && cfg.chooseAlbums
  if (interactiveMode) {
    await runPicker(cfg, albums, store, apiClient, downloader)
    return
  }

  const selected = await chooseAlbums(cfg, albums)

  if (selected.length === 0) {
    logInfo('No albums selected; exiting.')
    return
  }

  logInfo(`Selected ${selected.length}/${albums.length} albums.`)
  const summary = await downloadAlbums(cfg, selected, apiClient, downloader, store, { printLogs: true })
  if (summary.failedAlbums > 0) {
    logInfo(`Finished with failures. completed=${summary.completedAlbums}, skipped=${summary.skippedAlbums}, failed=${summary.failedAlbums}`)
  } else {
    logInfo('All albums processed successfully.')
  }
}

async function loadAlbums(cfg: AppConfig, apiClient: ApiClient): Promise<Album[]> {
  const cached = await loadAlbumCache(cfg.albumCachePath)

  if (cached && !cfg.refreshAlbums && shouldUseCachedAlbums(cached.fetchedAt, cfg.albumCacheTtlMs, new Date())) {
    logInfo(`Loaded ${cached.albums.length} albums from cache: ${cfg.albumCachePath}`)
    return cached.albums
  }

  try {
    logInfo('Fetching album catalog from API...')
    const albums = await withRetry(3, () => apiClient.getAlbums())
    await saveAlbumCache(cfg.albumCachePath, albums)
    return albums
  } catch (error) {
    if (cached) {
      console.warn(`Fetch failed; using cached catalog (${cached.albums.length} albums): ${(error as Error).message}`)
      return cached.albums
    }
    throw error
  }
}

async function chooseAlbums(cfg: AppConfig, albums: Album[]): Promise<Album[]> {
  if (albums.length === 0) {
    return []
  }

  if (cfg.albums.trim() !== '') {
    return selectAlbumsByQuery(albums, cfg.albums)
  }

  if (!cfg.chooseAlbums) {
    return albums
  }

  return albums
}

async function runPicker(
  cfg: AppConfig,
  albums: Album[],
  store: CompletedAlbumStore,
  apiClient: ApiClient,
  downloader: Downloader,
): Promise<void> {
  const renderer = await createCliRenderer({ exitOnCtrlC: false, useMouse: true })
  const root = createRoot(renderer)

  let lastCopiedSelection = ''
  const onSelection = (selection: { getSelectedText: () => string }) => {
    const text = selection.getSelectedText()
    if (!text) {
      lastCopiedSelection = ''
      return
    }
    if (text === lastCopiedSelection) {
      return
    }

    const copied = renderer.copyToClipboardOSC52(text)
    if (copied) {
      lastCopiedSelection = text
    }
  }
  renderer.on('selection', onSelection)

  try {
    await new Promise<void>((resolve, reject) => {
      root.render(
        React.createElement(App, {
          albums,
          completedAlbumNames: store.all(),
          apiClient,
          albumArt: detectAlbumArtCapability(),
          onAbort: () => reject(new Error('interactive selection aborted')),
          onClose: () => resolve(),
          onStartDownload: async (selected: Album[], emit: (event: DownloadEvent) => void): Promise<DownloadSummary> => {
            return downloadAlbums(cfg, selected, apiClient, downloader, store, { emit, printLogs: false })
          },
        }),
      )
    })
  } finally {
    renderer.off('selection', onSelection)
    renderer.destroy()
  }
}

async function downloadAlbums(
  cfg: AppConfig,
  albums: Album[],
  apiClient: ApiClient,
  downloader: Downloader,
  store: CompletedAlbumStore,
  options: { emit?: (event: DownloadEvent) => void; printLogs?: boolean } = {},
): Promise<DownloadSummary> {
  const started = Date.now()
  const emit = options.emit
  const printLogs = options.printLogs ?? true

  emit?.({ type: 'session-start', totalAlbums: albums.length, workers: cfg.workers })
  for (const album of albums) {
    emit?.({ type: 'album-state', albumCid: album.cid, albumName: album.name, status: 'queued', message: 'Queued' })
  }

  let completedAlbums = 0
  let skippedAlbums = 0
  let failedAlbums = 0

  const jobs = albums.map(album => async () => {
    try {
      const result = await processAlbum(cfg, album, apiClient, downloader, store, emit, printLogs)
      if (result === 'skipped') {
        skippedAlbums += 1
      } else {
        completedAlbums += 1
      }
    } catch (error) {
      failedAlbums += 1
      const message = `[${album.name}] Failed album: ${(error as Error).message}`
      emit?.({ type: 'album-state', albumCid: album.cid, albumName: album.name, status: 'failed', message })
      emitLog(emit, printLogs, 'error', message)
    }
  })
  await runPool(cfg.workers, jobs)

  const summary: DownloadSummary = {
    totalAlbums: albums.length,
    completedAlbums,
    skippedAlbums,
    failedAlbums,
    durationMs: Date.now() - started,
  }
  emit?.({ type: 'session-finished', summary })
  return summary
}

async function processAlbum(
  cfg: AppConfig,
  album: Album,
  apiClient: ApiClient,
  downloader: Downloader,
  store: CompletedAlbumStore,
  emit?: (event: DownloadEvent) => void,
  printLogs = true,
): Promise<'completed' | 'skipped'> {
  if (store.isCompleted(album.name)) {
    emit?.({ type: 'album-state', albumCid: album.cid, albumName: album.name, status: 'skipped', message: 'Already downloaded' })
    emitLog(emit, printLogs, 'info', `Skipping completed album: ${album.name}`)
    return 'skipped'
  }

  const started = Date.now()
  emit?.({ type: 'album-state', albumCid: album.cid, albumName: album.name, status: 'running', message: 'Preparing album' })
  emitLog(emit, printLogs, 'info', `[${album.name}] Starting album download`)

  const albumDir = join(cfg.outputDir, makeValidName(album.name))
  await mkdir(albumDir, { recursive: true })

  const coverJpg = join(albumDir, 'cover.jpg')
  const coverPng = join(albumDir, 'cover.png')
  await withRetry(3, () => downloader.downloadToFile(album.coverUrl, coverJpg))
  await convertToPng(coverJpg, coverPng)
  await removeIfExists(coverJpg)

  const songs = await withRetry(3, () => apiClient.getAlbumSongs(album.cid))
  emit?.({ type: 'album-songs-total', albumCid: album.cid, totalSongs: songs.length })
  emitLog(emit, printLogs, 'info', `[${album.name}] Found ${songs.length} songs`)

  for (let i = 0; i < songs.length; i += 1) {
    const song = songs[i]
    await processSong(albumDir, album, songs, i, song, apiClient, downloader, coverPng, emit, printLogs)
  }

  await store.markCompleted(album.name)
  emit?.({ type: 'album-state', albumCid: album.cid, albumName: album.name, status: 'completed', message: 'Completed' })
  emitLog(emit, printLogs, 'info', `[${album.name}] Completed album in ${Math.round((Date.now() - started) / 1000)}s`)
  return 'completed'
}

async function processSong(
  albumDir: string,
  album: Album,
  songs: Song[],
  index: number,
  song: Song,
  apiClient: ApiClient,
  downloader: Downloader,
  coverPng: string,
  emit?: (event: DownloadEvent) => void,
  printLogs = true,
): Promise<void> {
  const track = index + 1
  const total = songs.length
  emit?.({ type: 'song-start', albumCid: album.cid, songName: song.name, track, total })
  emitLog(emit, printLogs, 'info', `[${album.name}] [${track}/${total}] Resolving ${song.name}`)

  const detail = await withRetry(3, () => apiClient.getSongDetail(song.cid))

  let lyricPath = ''
  if (detail.lyricUrl) {
    lyricPath = join(albumDir, `${makeValidName(song.name)}.lrc`)
    await withRetry(3, () => downloader.downloadToFile(detail.lyricUrl, lyricPath))
  }

  let lastBucket = -1
  const { songPath, fileType, stats } = await withRetry(3, () =>
    downloader.downloadSong(albumDir, song.name, detail.sourceUrl, undefined, p => {
      if (p.totalBytes > 0) {
        const progress = Math.floor((p.bytesWritten * 100) / p.totalBytes)
        emit?.({ type: 'song-progress', albumCid: album.cid, songName: song.name, track, total, percent: progress })
        const bucket = Math.floor(progress / 10)
        if (bucket > lastBucket || progress === 100) {
          emitLog(
            emit,
            printLogs,
            'info',
            `[${album.name}] [${track}/${total}] Downloading ${song.name}: ${progress}% (${formatBytes(p.bytesWritten)}/${formatBytes(p.totalBytes)})`,
          )
          lastBucket = bucket
        }
      }
    }),
  )

  await applyMetadata({
    filePath: songPath,
    fileType,
    album: album.name,
    title: song.name,
    albumArtists: album.artistes,
    artists: song.artistes,
    trackNumber: track,
    coverPath: coverPng,
    lyricPath,
  })

  emit?.({ type: 'song-done', albumCid: album.cid, songName: song.name, track, total })
  emitLog(
    emit,
    printLogs,
    'info',
    `[${album.name}] [${track}/${total}] Finished ${song.name} (${fileType}, ${formatBytes(stats.bytesWritten)}, ${formatRate(stats.bytesWritten, stats.durationMs)})`,
  )
}

function emitLog(
  emit: ((event: DownloadEvent) => void) | undefined,
  printLogs: boolean,
  level: 'info' | 'warn' | 'error',
  message: string,
): void {
  emit?.({ type: 'log', level, message })
  if (printLogs) {
    logInfo(message)
  }
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(1)} ${units[idx]}`
}

function formatRate(bytes: number, durationMs: number): string {
  if (durationMs <= 0) return 'n/a'
  const bytesPerSecond = (bytes / durationMs) * 1000
  return `${formatBytes(Math.round(bytesPerSecond))}/s`
}

main().catch(error => {
  const message = (error as Error).message
  if (message.includes('ambiguous') && message.includes('album query')) {
    console.error(message)
    return
  }

  if (message.includes('not found') && message.includes('album query')) {
    const query = message.match(/\"(.+)\"/)?.[1] ?? ''
    console.error(message)
    if (query) {
      console.error(`Hint: try a more specific CID; one candidate command is --albums \"${query}\"`)
    }
    return
  }

  if (message.includes('interactive selection aborted')) {
    console.error('Selection aborted.')
    return
  }

  console.error(`Error: ${message}`)
})
