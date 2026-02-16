export interface Album {
  cid: string
  name: string
  coverUrl: string
  artistes: string[]
}

export interface Song {
  cid: string
  name: string
  artistes: string[]
}

export interface SongDetail {
  lyricUrl: string
  sourceUrl: string
}

export interface AppConfig {
  outputDir: string
  workers: number
  httpTimeoutMs: number
  albums: string
  chooseAlbums: boolean
  refreshAlbums: boolean
  albumCachePath: string
  albumCacheTtlMs: number
}
