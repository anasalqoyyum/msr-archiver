export async function withRetry<T>(attempts: number, fn: () => Promise<T>, signal?: AbortSignal): Promise<T> {
  const maxAttempts = Math.max(1, attempts)
  let lastError: unknown

  for (let i = 1; i <= maxAttempts; i += 1) {
    if (signal?.aborted) {
      throw new Error('operation canceled')
    }
    try {
      return await fn()
    } catch (error) {
      lastError = error
      if (i === maxAttempts) {
        break
      }
      await sleep(i * 400, signal)
    }
  }

  throw lastError
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms)
    const onAbort = () => {
      clearTimeout(timer)
      reject(new Error('operation canceled'))
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}
