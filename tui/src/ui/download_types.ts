export type AlbumDownloadStatus = 'queued' | 'running' | 'completed' | 'skipped' | 'failed'

export interface DownloadSummary {
  totalAlbums: number
  completedAlbums: number
  skippedAlbums: number
  failedAlbums: number
  durationMs: number
}

export type DownloadEvent =
  | { type: 'session-start'; totalAlbums: number; workers: number }
  | { type: 'album-state'; albumCid: string; albumName: string; status: AlbumDownloadStatus; message?: string }
  | { type: 'album-songs-total'; albumCid: string; totalSongs: number }
  | { type: 'song-start'; albumCid: string; songName: string; track: number; total: number }
  | { type: 'song-progress'; albumCid: string; songName: string; track: number; total: number; percent: number }
  | { type: 'song-done'; albumCid: string; songName: string; track: number; total: number }
  | { type: 'log'; level: 'info' | 'warn' | 'error'; message: string }
  | { type: 'session-finished'; summary: DownloadSummary }
