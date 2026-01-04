import { useQueryClient } from '@tanstack/react-query'
import { PlaylistProgressPayload } from '@/lib/types'
import { useEffect, useState } from 'react'

export function usePlaylistProgress(batchId: string) {
  const queryClient = useQueryClient()
  const [progress, setProgress] = useState<PlaylistProgressPayload | null>(null)

  useEffect(() => {
    // Subscribe to query data changes
    const unsubscribe = queryClient.getQueryCache().subscribe((event) => {
      if (
        event.type === 'updated' &&
        event.query.queryKey[0] === 'playlistProgress' &&
        event.query.queryKey[1] === batchId
      ) {
        const data = event.query.state.data as PlaylistProgressPayload | undefined
        if (data) {
          setProgress(data)
          
          // Clear progress after completion (100%)
          if (data.progress === 100) {
            setTimeout(() => {
              setProgress(null)
              queryClient.removeQueries({ queryKey: ['playlistProgress', batchId] })
            }, 2000)
          }
        }
      }
    })

    // Check for existing progress data
    const existingData = queryClient.getQueryData<PlaylistProgressPayload>(['playlistProgress', batchId])
    if (existingData) {
      setProgress(existingData)
    }

    return () => {
      unsubscribe()
    }
  }, [batchId, queryClient])

  return progress
}
