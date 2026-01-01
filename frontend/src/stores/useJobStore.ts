import { create } from 'zustand'
import { Job } from '@/lib/types'

interface JobState {
  jobs: Job[]
  addJob: (job: Job) => void
  updateJob: (id: string, updates: Partial<Job>) => void
  removeJob: (id: string) => void
  clearJobs: () => void
}

export const useJobStore = create<JobState>((set) => ({
  jobs: [],
  addJob: (job) =>
    set((state) => ({
      jobs: [...state.jobs, job],
    })),
  updateJob: (id, updates) =>
    set((state) => ({
      jobs: state.jobs.map((job) =>
        job.id === id ? { ...job, ...updates } : job
      ),
    })),
  removeJob: (id) =>
    set((state) => ({
      jobs: state.jobs.filter((job) => job.id !== id),
    })),
  clearJobs: () => set({ jobs: [] }),
}))
