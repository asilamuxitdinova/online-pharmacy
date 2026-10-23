import { motion } from 'motion/react'
import { useEffect, useState } from 'react'
import { get } from '../lib/api'
import type { Job } from '../lib/types'
import { ErrorNote, StatusBadge } from './ui'
import { ApiError } from '../lib/api'

/** Polls a job until it finishes, honouring Retry-After's spirit (every 1.5s). */
export function useJob(location: string | null) {
  const [job, setJob] = useState<Job | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  useEffect(() => {
    if (!location) return
    let stop = false
    let timer = 0
    const tick = async () => {
      try {
        const j = await get<Job>(location)
        if (stop) return
        setJob(j)
        if (j.status === 'running') timer = window.setTimeout(tick, 1500)
      } catch (e) {
        if (!stop) setError(e as ApiError)
      }
    }
    void tick()
    return () => {
      stop = true
      window.clearTimeout(timer)
    }
  }, [location])
  return { job, error }
}

export function JobBar({ job, error }: { job: Job | null; error: ApiError | null }) {
  if (error) return <ErrorNote error={error} />
  if (!job) return null
  return (
    <div className="stack" style={{ '--gap': '8px' } as React.CSSProperties}>
      <div className="spread small">
        <span className="mono">{job.id}</span>
        <StatusBadge status={job.status} />
      </div>
      <div style={{ height: 10, borderRadius: 5, background: 'var(--bg-2)', overflow: 'hidden' }}>
        <motion.div
          style={{ height: '100%', borderRadius: 5 }}
          animate={{ width: `${job.progress}%`, background: job.status === 'failed' ? 'var(--bad)' : job.status === 'succeeded' ? 'var(--mint)' : 'var(--sun)' }}
          transition={{ type: 'spring', stiffness: 80, damping: 18 }}
        />
      </div>
      {job.status === 'running' && <span className="tiny muted">{job.progress}% · about {job.estimated_seconds_left}s left · polling GET /jobs/{'{id}'}</span>}
      {job.error && <div className="error-note">{job.error.message}</div>}
    </div>
  )
}
