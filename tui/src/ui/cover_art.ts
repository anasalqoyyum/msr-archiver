const ESC_RE = new RegExp('\\u001b', 'g')

function moveCursor(row: number, col: number): string {
  return `\u001b[${row};${col}H`
}

function wrapForTmuxPassthrough(seq: string): string {
  if (!process.env.TMUX) {
    return seq
  }
  return `\u001bPtmux;${seq.replace(ESC_RE, '\u001b\u001b')}\u001b\\`
}

function writeControl(seq: string): void {
  process.stdout.write(wrapForTmuxPassthrough(seq))
}

export async function ensureTmuxPassthrough(): Promise<void> {
  if (!process.env.TMUX || !Bun.which('tmux')) {
    return
  }

  const proc = Bun.spawn({
    cmd: ['tmux', 'set', '-p', 'allow-passthrough', 'on'],
    stdout: 'ignore',
    stderr: 'ignore',
  })
  await proc.exited
}

async function fetchCoverBytes(coverUrl: string): Promise<Uint8Array> {
  const resp = await fetch(coverUrl)
  if (!resp.ok) {
    throw new Error(`cover fetch failed: ${resp.status}`)
  }
  return new Uint8Array(await resp.arrayBuffer())
}

export async function emitInlineCover(
  coverUrl: string,
  widthCells: number,
  heightCells: number,
  imageId?: number,
  position?: { row: number; col: number },
): Promise<void> {
  const bytes = await fetchCoverBytes(coverUrl)
  const base64 = Buffer.from(bytes).toString('base64')

  if (position) {
    const row = Math.max(1, Math.floor(position.row))
    const col = Math.max(1, Math.floor(position.col))
    writeControl(moveCursor(row, col))
  }

  const id = Math.max(1, imageId ?? 1)
  writeControl(`\u001b_Ga=d,d=I,i=${id}\u001b\\`)

  const chunkSize = 4096
  for (let offset = 0; offset < base64.length; offset += chunkSize) {
    const chunk = base64.slice(offset, offset + chunkSize)
    const hasMore = offset + chunkSize < base64.length
    const params =
      offset === 0
        ? `a=T,i=${id},f=100,t=d,c=${Math.max(8, widthCells)},r=${Math.max(4, heightCells)},m=${hasMore ? 1 : 0}`
        : `m=${hasMore ? 1 : 0}`
    writeControl(`\u001b_G${params};${chunk}\u001b\\`)
  }
}
