'use client'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useBatches, useDeleteBatch, useRetryBatch } from '@/hooks/useJobs'
import { Batch } from '@/lib/types'
import { formatDistanceToNow } from 'date-fns'
import { Trash2, RefreshCw, Clock, Plus } from 'lucide-react'
import Link from 'next/link'
import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { CreateBatchForm } from '@/components/jobs/create-batch-form'

export default function BatchesPage() {
  const { data: batches, isLoading } = useBatches()
  const deleteBatch = useDeleteBatch()
  const retryBatch = useRetryBatch()
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)

  const handleDelete = async (id: string) => {
    setDeletingId(id)
    try {
      await deleteBatch.mutateAsync(id)
    } finally {
      setDeletingId(null)
    }
  }

  const handleRetry = async (id: string) => {
    await retryBatch.mutateAsync(id)
  }

  if (isLoading) {
    return <div className="text-white">Loading...</div>
  }

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Header with responsive title and button */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <h1 className="text-2xl sm:text-3xl font-bold text-white">Batches</h1>
        <Dialog open={isCreateModalOpen} onOpenChange={setIsCreateModalOpen}>
          <DialogTrigger asChild>
            <Button className="w-full sm:w-auto min-h-[44px]">
              <Plus className="mr-2 h-4 w-4" />
              New Batch
            </Button>
          </DialogTrigger>
          <DialogContent className="w-[95vw] max-w-[600px] max-h-[90vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Create New Sync Batch</DialogTitle>
            </DialogHeader>
            <CreateBatchForm
              onSuccess={() => setIsCreateModalOpen(false)}
              onCancel={() => setIsCreateModalOpen(false)}
              inModal
            />
          </DialogContent>
        </Dialog>
      </div>

      {/* Batches list - responsive layout */}
      <Card className="border-gray-700 bg-gray-900">
        <CardHeader>
          <CardTitle className="text-white">All Batches</CardTitle>
        </CardHeader>
        <CardContent>
          {batches?.length === 0 ? (
            <div className="text-center text-gray-400 py-8 text-sm">
              No batches yet. Create your first sync batch to get started.
            </div>
          ) : (
            <div className="space-y-3 sm:space-y-4">
              {batches?.map((batch: Batch) => (
                <Link
                  key={batch.id}
                  href={`/dashboard/batches/${batch.id}`}
                  className="flex flex-col lg:flex-row lg:items-center gap-3 sm:gap-4 rounded-lg border border-gray-700 p-3 sm:p-4 hover:bg-gray-800/50 transition-colors block"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-white truncate">{batch.name}</span>
                      <Badge
                        variant={
                          batch.status === 'completed'
                            ? 'success'
                            : batch.status === 'failed'
                            ? 'destructive'
                            : 'secondary'
                        }
                      >
                        {batch.status}
                      </Badge>
                    </div>
                    <div className="mt-2 flex items-center gap-3 sm:gap-4 lg:gap-6 text-xs sm:text-sm text-gray-400 flex-wrap">
                      <span className="flex items-center gap-1">
                        <Clock className="h-4 w-4 flex-shrink-0" />
                        <span className="hidden sm:inline">{formatDistanceToNow(new Date(batch.created_at), { addSuffix: true })}</span>
                        <span className="sm:hidden">{new Date(batch.created_at).toLocaleDateString()}</span>
                      </span>
                      <span className="uppercase">{batch.spotify_type}</span>
                      <span className="hidden sm:inline">
                        {batch.completed_jobs}/{batch.total_jobs} jobs
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 sm:gap-4">
                    <div className="flex-1 sm:w-32">
                      <Progress value={batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0} className="h-2" />
                      <div className="mt-1 flex justify-between sm:justify-end text-xs text-gray-400">
                        <span className="sm:hidden">{batch.completed_jobs}/{batch.total_jobs} jobs</span>
                        <span>{batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0}%</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1 sm:gap-2">
                      {batch.status === 'failed' && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-11 w-11 sm:h-9 sm:w-9"
                          onClick={(e) => { e.preventDefault(); handleRetry(batch.id); }}
                          disabled={retryBatch.isPending}
                          aria-label="Retry batch"
                        >
                          <RefreshCw className="h-4 w-4" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-11 w-11 sm:h-9 sm:w-9"
                        onClick={(e) => { e.preventDefault(); handleDelete(batch.id); }}
                        disabled={deletingId === batch.id}
                        aria-label="Delete batch"
                      >
                        <Trash2 className="h-4 w-4 text-red-500" />
                      </Button>
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
