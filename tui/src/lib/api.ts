import type { Album, Song, SongDetail } from './types'

const BASE_URL = 'https://monster-siren.hypergryph.com/api'

interface ApiResponse<T> {
  data: T
}

interface AlbumDetailResponse {
  songs: Song[]
}

export class ApiClient {
  constructor(private readonly timeoutMs: number) {}

  async getAlbums(signal?: AbortSignal): Promise<Album[]> {
    const out = await this.getJSON<ApiResponse<Album[]>>(`${BASE_URL}/albums`, signal)
    return out.data
  }

  async getAlbumSongs(albumCid: string, signal?: AbortSignal): Promise<Song[]> {
    const out = await this.getJSON<ApiResponse<AlbumDetailResponse>>(`${BASE_URL}/album/${albumCid}/detail`, signal)
    return out.data.songs
  }

  async getSongDetail(songCid: string, signal?: AbortSignal): Promise<SongDetail> {
    const out = await this.getJSON<ApiResponse<SongDetail>>(`${BASE_URL}/song/${songCid}`, signal)
    return out.data
  }

  private async getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), this.timeoutMs)
    const abort = () => controller.abort()
    signal?.addEventListener('abort', abort)

    try {
      const resp = await fetch(url, {
        headers: { Accept: 'application/json' },
        signal: controller.signal,
      })
      if (!resp.ok) {
        throw new Error(`request ${url} failed with status ${resp.status}`)
      }
      return (await resp.json()) as T
    } catch (error) {
      throw new Error(`request ${url}: ${(error as Error).message}`)
    } finally {
      clearTimeout(timeout)
      signal?.removeEventListener('abort', abort)
    }
  }
}
