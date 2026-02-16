export async function runPool(workers: number, jobs: Array<() => Promise<void>>): Promise<void> {
  if (jobs.length === 0) {
    return
  }
  const size = Math.max(1, workers)
  let cursor = 0
  const errors: Error[] = []

  const runWorker = async () => {
    while (cursor < jobs.length) {
      const current = cursor
      cursor += 1
      try {
        await jobs[current]()
      } catch (error) {
        errors.push(error as Error)
      }
    }
  }

  await Promise.all(Array.from({ length: size }, () => runWorker()))

  if (errors.length > 0) {
    throw new AggregateError(errors, 'one or more jobs failed')
  }
}
