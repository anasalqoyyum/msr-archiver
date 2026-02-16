import { mkdir, rename, writeFile } from 'node:fs/promises'
import { dirname } from 'node:path'

export class CompletedAlbumStore {
  private readonly completed = new Set<string>()

  constructor(private readonly path: string) {}

  async load(): Promise<void> {
    const file = Bun.file(this.path)
    if (!(await file.exists())) {
      return
    }

    const text = await file.text()
    const names = JSON.parse(text) as string[]
    for (const name of names) {
      this.completed.add(name)
    }
  }

  isCompleted(name: string): boolean {
    return this.completed.has(name)
  }

  all(): Set<string> {
    return new Set(this.completed)
  }

  async markCompleted(name: string): Promise<void> {
    if (this.completed.has(name)) {
      return
    }
    this.completed.add(name)
    const sorted = Array.from(this.completed).sort((a, b) => a.localeCompare(b))
    const dir = dirname(this.path)
    await mkdir(dir, { recursive: true })
    const tmpPath = `${this.path}.tmp.${Date.now()}`
    await writeFile(tmpPath, JSON.stringify(sorted), 'utf8')
    await rename(tmpPath, this.path)
  }
}
