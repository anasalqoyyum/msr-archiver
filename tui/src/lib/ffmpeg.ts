import { rename, rm } from 'node:fs/promises'

export async function ensureFFmpeg(): Promise<void> {
  if (!Bun.which('ffmpeg')) {
    throw new Error('ffmpeg is required but unavailable in PATH')
  }
}

export async function wavToFlac(wavPath: string, flacPath: string): Promise<void> {
  await runFFmpeg(['-y', '-i', wavPath, '-vn', '-compression_level', '12', flacPath], 'wav->flac conversion failed')
}

export async function convertToPng(srcPath: string, dstPath: string): Promise<void> {
  await runFFmpeg(['-y', '-i', srcPath, dstPath], 'cover conversion to png failed')
}

export interface MetadataInput {
  filePath: string
  fileType: '.mp3' | '.flac'
  album: string
  title: string
  albumArtists: string[]
  artists: string[]
  trackNumber: number
  coverPath: string
  lyricPath: string
}

export async function applyMetadata(input: MetadataInput): Promise<void> {
  let lyrics = ''
  if (input.lyricPath) {
    lyrics = await Bun.file(input.lyricPath).text()
  }

  const args: string[] = ['-y', '-i', input.filePath]
  const coverEnabled = Boolean(input.coverPath)
  if (coverEnabled) {
    args.push('-i', input.coverPath)
  }

  args.push('-map', '0:a')
  if (coverEnabled) {
    args.push('-map', '1:v')
  }
  args.push('-c:a', 'copy')

  if (coverEnabled) {
    if (input.fileType === '.mp3') {
      args.push(
        '-c:v',
        'mjpeg',
        '-id3v2_version',
        '3',
        '-disposition:v',
        'attached_pic',
        '-metadata:s:v',
        'title=Cover',
        '-metadata:s:v',
        'comment=Cover (front)',
      )
    } else {
      args.push('-c:v', 'png', '-disposition:v', 'attached_pic')
    }
  }

  const albumArtists = input.albumArtists.join('')
  const artists = input.artists.join('')

  args.push(
    '-metadata',
    `album=${input.album}`,
    '-metadata',
    `title=${input.title}`,
    '-metadata',
    `album_artist=${albumArtists}`,
    '-metadata',
    `albumartist=${albumArtists}`,
    '-metadata',
    `artist=${artists}`,
    '-metadata',
    `track=${input.trackNumber}`,
  )

  if (lyrics) {
    args.push('-metadata', `lyrics=${lyrics}`)
  }

  const tmpPath = `${input.filePath}.tmp-metadata${input.fileType}`
  args.push(tmpPath)
  await runFFmpeg(args, `metadata write failed for ${input.filePath}`)

  await rename(tmpPath, input.filePath)
}

export async function removeIfExists(path: string): Promise<void> {
  await rm(path, { force: true })
}

async function runFFmpeg(args: string[], errorPrefix: string): Promise<void> {
  const proc = Bun.spawn({
    cmd: ['ffmpeg', ...args],
    stdout: 'ignore',
    stderr: 'pipe',
  })
  const stderr = await new Response(proc.stderr).text()
  const code = await proc.exited
  if (code !== 0) {
    throw new Error(`${errorPrefix}: ${stderr.trim()}`)
  }
}
