import { availableParallelism } from 'node:os'
import { join } from 'node:path'
import type { AppConfig } from './types'

const DEFAULT_OUTPUT = './MonsterSiren'

function defaultWorkers(): number {
  try {
    return Math.max(2, availableParallelism())
  } catch {
    return 2
  }
}

function parseBool(value: string): boolean {
  const normalized = value.trim().toLowerCase()
  return !(normalized === '0' || normalized === 'false' || normalized === 'no')
}

function parseDurationToMs(raw: string): number {
  const value = raw.trim().toLowerCase()
  if (!value) {
    throw new Error('empty duration')
  }
  const m = value.match(/^(\d+)(ms|s|m|h)$/)
  if (!m) {
    throw new Error(`invalid duration: ${raw}`)
  }
  const amount = Number(m[1])
  const unit = m[2]
  if (unit === 'ms') return amount
  if (unit === 's') return amount * 1000
  if (unit === 'm') return amount * 60 * 1000
  return amount * 60 * 60 * 1000
}

function parseArgValue(args: string[], key: string): string | undefined {
  const eq = args.find(a => a.startsWith(`${key}=`))
  if (eq) return eq.slice(key.length + 1)
  const idx = args.indexOf(key)
  if (idx >= 0 && idx < args.length - 1) return args[idx + 1]
  return undefined
}

export function parseConfig(args: string[] = process.argv.slice(2)): AppConfig {
  const workersRaw = parseArgValue(args, '--workers')
  const timeoutRaw = parseArgValue(args, '--http-timeout')
  const albums = parseArgValue(args, '--albums') ?? ''
  const outputDir = parseArgValue(args, '--output') ?? DEFAULT_OUTPUT

  const chooseAlbumsRaw = parseArgValue(args, '--choose-albums')
  const refreshAlbumsRaw = parseArgValue(args, '--refresh-albums')
  const albumCachePathRaw = parseArgValue(args, '--album-cache')
  const albumCacheTtlRaw = parseArgValue(args, '--album-cache-ttl')

  const workers = workersRaw ? Math.max(1, Number(workersRaw)) : defaultWorkers()
  if (!Number.isFinite(workers)) {
    throw new Error(`invalid --workers value: ${workersRaw}`)
  }

  const httpTimeoutMs = timeoutRaw ? parseDurationToMs(timeoutRaw) : 2 * 60 * 1000
  const chooseAlbums = chooseAlbumsRaw ? parseBool(chooseAlbumsRaw) : true
  const refreshAlbums = refreshAlbumsRaw ? parseBool(refreshAlbumsRaw) : false
  const albumCacheTtlMs = albumCacheTtlRaw ? parseDurationToMs(albumCacheTtlRaw) : 24 * 60 * 60 * 1000
  const albumCachePath = albumCachePathRaw && albumCachePathRaw.trim() !== '' ? albumCachePathRaw : join(outputDir, 'albums_cache.json')

  return {
    outputDir,
    workers,
    httpTimeoutMs,
    albums,
    chooseAlbums,
    refreshAlbums,
    albumCachePath,
    albumCacheTtlMs,
  }
}

export function printHelp(): void {
  const lines = [
    'msr-archiver-tui (Bun + OpenTUI)',
    '',
    'Flags:',
    '  --output <path>            Output directory (default: ./MonsterSiren)',
    '  --workers <n>              Concurrent album workers (default: max(2, CPU))',
    '  --http-timeout <dur>       HTTP timeout (default: 2m)',
    '  --albums <csv>             Album names/CIDs (comma-separated)',
    '  --choose-albums <bool>     Interactive picker mode (default: true)',
    '  --refresh-albums <bool>    Force refresh album cache from API',
    '  --album-cache <path>       Album cache path (default: <output>/albums_cache.json)',
    '  --album-cache-ttl <dur>    Cache max age before refresh (default: 24h)',
    '',
    'Duration format: 500ms, 10s, 2m, 1h',
  ]
  console.log(lines.join('\n'))
}
