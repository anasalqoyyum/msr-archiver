import { mkdir, rename, writeFile } from 'node:fs/promises'
import { dirname } from 'node:path'
import type { Album } from './types'

interface CachePayload {
  fetchedAt: string
  albums: Album[]
}

export async function loadAlbumCache(path: string): Promise<{ albums: Album[]; fetchedAt: Date } | null> {
  const file = Bun.file(path)
  if (!(await file.exists())) {
    return null
  }

  const content = await file.text()
  const payload = JSON.parse(content) as CachePayload
  const fetchedAt = payload.fetchedAt ? new Date(payload.fetchedAt) : new Date(0)
  if (!Array.isArray(payload.albums)) {
    throw new Error('invalid album cache payload')
  }
  return { albums: payload.albums, fetchedAt }
}

export async function saveAlbumCache(path: string, albums: Album[]): Promise<void> {
  const payload: CachePayload = {
    fetchedAt: new Date().toISOString(),
    albums,
  }
  const dir = dirname(path)
  await mkdir(dir, { recursive: true })
  const tmpPath = `${path}.tmp.${Date.now()}`
  await writeFile(tmpPath, JSON.stringify(payload, null, 2), 'utf8')
  await rename(tmpPath, path)
}

export function shouldUseCachedAlbums(fetchedAt: Date, ttlMs: number, now: Date): boolean {
  if (ttlMs <= 0) return true
  if (Number.isNaN(fetchedAt.getTime())) return false
  return now.getTime() - fetchedAt.getTime() <= ttlMs
}
