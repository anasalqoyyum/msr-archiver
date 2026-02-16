export type AlbumArtMode = 'kitty-inline' | 'unsupported'

export interface AlbumArtCapability {
  mode: AlbumArtMode
  reason: string
  inlineProtocol: 'kitty' | null
}

export function detectAlbumArtCapability(): AlbumArtCapability {
  const term = process.env.TERM?.toLowerCase() ?? ''
  const termProgram = process.env.TERM_PROGRAM?.toLowerCase() ?? ''
  const kittyLike = term.includes('kitty') || term.includes('ghostty') || termProgram.includes('ghostty')

  if (kittyLike) {
    return {
      mode: 'kitty-inline',
      reason: 'Kitty graphics protocol appears available (Kitty/Ghostty-compatible terminal detected).',
      inlineProtocol: 'kitty',
    }
  }

  return {
    mode: 'unsupported',
    reason: 'Kitty protocol unsupported in this terminal/session; cover image rendering is disabled.',
    inlineProtocol: null,
  }
}
