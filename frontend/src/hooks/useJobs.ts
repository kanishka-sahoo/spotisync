import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { batchApi, jobsApi } from '@/lib/api'
import { Batch } from '@/lib/types'

export function useBatches() {
  return useQuery({
    queryKey: ['batches'],
    queryFn: async () => {
      const response = await batchApi.getAll()
      return response as Batch[]
    },
  })
}

export function useBatch(id: string) {
  return useQuery({
    queryKey: ['batch', id],
    queryFn: async () => {
      const response = await batchApi.getById(id)
      return response as Batch
    },
    enabled: !!id,
  })
}

export function useCreateBatch() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (spotifyUrl: string) => {
      const response = await jobsApi.create(spotifyUrl)
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['batches'] })
    },
  })
}

export function useDeleteBatch() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const response = await batchApi.delete(id)
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['batches'] })
    },
  })
}

export function useRetryBatch() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const response = await batchApi.retry(id)
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['batch'] })
      queryClient.invalidateQueries({ queryKey: ['batches'] })
    },
  })
}

export function useResyncBatch() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const response = await batchApi.resync(id)
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['batch'] })
      queryClient.invalidateQueries({ queryKey: ['batches'] })
    },
  })
}

export function useRetryJob() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const response = await jobsApi.retry(id)
      return response
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['batch'] })
      queryClient.invalidateQueries({ queryKey: ['batches'] })
    },
  })
}
