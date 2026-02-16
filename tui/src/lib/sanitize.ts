const REPLACEMENTS: Array<[string, string]> = [
  [':', '_'],
  ['/', '_'],
  ['<', '_'],
  ['>', '_'],
  ["'", '_'],
  ['\\', '_'],
  ['|', '_'],
  ['?', '_'],
  ['*', '_'],
  [' ', '_'],
]

export function makeValidName(name: string): string {
  let out = name
  for (const [needle, replacement] of REPLACEMENTS) {
    out = out.split(needle).join(replacement)
  }
  return out
}
