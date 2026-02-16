import { createWriteStream } from 'node:fs'
import { mkdir, rename, rm } from 'node:fs/promises'
import { dirname } from 'node:path'
import { wavToFlac } from './ffmpeg'
import { makeValidName } from './sanitize'

export interface ProgressUpdate {
  bytesWritten: number
  totalBytes: number
}

export interface FileDownloadResult {
  contentType: string
  bytesWritten: number
  durationMs: number
}

export class Downloader {
  async downloadToFile(
    url: string,
    dstPath: string,
    signal?: AbortSignal,
    onProgress?: (p: ProgressUpdate) => void,
  ): Promise<FileDownloadResult> {
    const started = Date.now()
    const resp = await fetch(url, { signal })
    if (!resp.ok) {
      throw new Error(`download ${url} failed with status ${resp.status}`)
    }
    if (!resp.body) {
      throw new Error(`download ${url} returned empty body`)
    }

    await mkdir(dirname(dstPath), { recursive: true })

    const totalBytes = Number(resp.headers.get('content-length') ?? '-1')
    const writer = createWriteStream(dstPath)
    const reader = resp.body.getReader()

    let bytesWritten = 0
    let lastProgress = 0
    while (true) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      writer.write(value)
      bytesWritten += value.length
      const now = Date.now()
      if (onProgress && (lastProgress === 0 || now - lastProgress >= 700)) {
        onProgress({ bytesWritten, totalBytes })
        lastProgress = now
      }
    }
    await new Promise<void>((resolve, reject) => {
      writer.end(() => resolve())
      writer.on('error', reject)
    })

    if (onProgress) {
      onProgress({ bytesWritten, totalBytes })
    }

    return {
      contentType: resp.headers.get('content-type') ?? '',
      bytesWritten,
      durationMs: Date.now() - started,
    }
  }

  async downloadSong(
    dir: string,
    songName: string,
    sourceUrl: string,
    signal?: AbortSignal,
    onProgress?: (p: ProgressUpdate) => void,
  ): Promise<{ songPath: string; fileType: '.mp3' | '.flac'; stats: FileDownloadResult }> {
    const base = `${dir}/${makeValidName(songName)}`
    const wavPath = `${base}.wav`
    const mp3Path = `${base}.mp3`

    const stats = await this.downloadToFile(sourceUrl, wavPath, signal, onProgress)
    if (stats.contentType.toLowerCase().includes('audio/mpeg')) {
      await rename(wavPath, mp3Path)
      return { songPath: mp3Path, fileType: '.mp3', stats }
    }

    const flacPath = `${base}.flac`
    await wavToFlac(wavPath, flacPath)
    await rm(wavPath, { force: true })
    return { songPath: flacPath, fileType: '.flac', stats }
  }
}
