import type { Album } from './types'

export function selectAlbumsByQuery(albums: Album[], raw: string): Album[] {
  const queries = raw
    .split(',')
    .map(s => s.trim())
    .filter(Boolean)
  if (queries.length === 0) {
    throw new Error('no valid album query provided')
  }

  const seen = new Set<string>()
  const selected: Album[] = []
  for (const query of queries) {
    if (query.toLowerCase() === 'all') {
      return albums
    }
    const album = resolveAlbumQuery(albums, query)
    if (!seen.has(album.cid)) {
      seen.add(album.cid)
      selected.push(album)
    }
  }
  return selected
}

export function resolveAlbumQuery(albums: Album[], query: string): Album {
  const q = query.trim().toLowerCase()
  if (!q) {
    throw new Error('empty album query')
  }

  const exact = albums.filter(a => a.cid.toLowerCase() === q || a.name.toLowerCase() === q)
  if (exact.length === 1) {
    return exact[0]
  }
  if (exact.length > 1) {
    throw new Error(`album query \"${query}\" matched multiple albums exactly; use CID`)
  }

  const contains = albums.filter(a => a.cid.toLowerCase().includes(q) || a.name.toLowerCase().includes(q))
  if (contains.length === 1) {
    return contains[0]
  }
  if (contains.length > 1) {
    const labels = contains.map(a => `${a.name} (${a.cid})`).sort((a, b) => a.localeCompare(b))
    throw new Error(`album query \"${query}\" is ambiguous: ${labels.join(', ')}`)
  }

  throw new Error(`album query \"${query}\" not found`)
}

export function listWindow(total: number, cursor: number, size: number): [number, number] {
  if (total <= 0) return [0, 0]
  if (size <= 0 || total <= size) return [0, total]

  let start = cursor - Math.floor(size / 2)
  if (start < 0) start = 0
  if (start + size > total) start = total - size
  return [start, start + size]
}

export function formatBytes(bytes: number): string {
  const unit = 1024
  if (bytes < unit) return `${bytes} B`
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let value = bytes
  let idx = 0
  while (value >= unit && idx < units.length - 1) {
    value /= unit
    idx += 1
  }
  return `${value.toFixed(1)} ${units[idx]}`
}

export function formatRate(bytes: number, durationMs: number): string {
  if (durationMs <= 0) return 'n/a'
  const perSec = Math.round((bytes / durationMs) * 1000)
  return `${formatBytes(perSec)}/s`
}
