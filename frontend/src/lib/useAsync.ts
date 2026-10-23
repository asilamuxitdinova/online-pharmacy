import { useCallback, useEffect, useRef, useState, type DependencyList } from 'react'
import { ApiError } from './api'

export interface Async<T> {
  data: T | undefined
  error: ApiError | undefined
  loading: boolean
  reload: () => void
  setData: (v: T) => void
}

/** Runs fn when deps change; keeps the previous data while reloading. */
export function useAsync<T>(fn: (signal: AbortSignal) => Promise<T>, deps: DependencyList): Async<T> {
  const [data, setData] = useState<T>()
  const [error, setError] = useState<ApiError>()
  const [loading, setLoading] = useState(true)
  const [tick, setTick] = useState(0)
  const fnRef = useRef(fn)
  fnRef.current = fn

  useEffect(() => {
    const ctrl = new AbortController()
    setLoading(true)
    setError(undefined)
    fnRef
      .current(ctrl.signal)
      .then((d) => !ctrl.signal.aborted && setData(d))
      .catch((e) => {
        if (ctrl.signal.aborted || (e as Error).name === 'AbortError') return
        setError(e instanceof ApiError ? e : new ApiError({ status: 0, code: 'unexpected', message: String(e) }))
      })
      .finally(() => !ctrl.signal.aborted && setLoading(false))
    return () => ctrl.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick])

  const reload = useCallback(() => setTick((t) => t + 1), [])
  return { data, error, loading, reload, setData }
}

/** Polls fn every `ms` while `active` is true. */
export function useInterval(fn: () => void, ms: number, active = true) {
  const ref = useRef(fn)
  ref.current = fn
  useEffect(() => {
    if (!active) return
    const id = window.setInterval(() => ref.current(), ms)
    return () => window.clearInterval(id)
  }, [ms, active])
}
