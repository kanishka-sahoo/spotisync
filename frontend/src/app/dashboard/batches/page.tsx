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
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold text-white">Batches</h1>
        <Dialog open={isCreateModalOpen} onOpenChange={setIsCreateModalOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              New Batch
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[600px]">
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

      <Card className="border-gray-700 bg-gray-900">
        <CardHeader>
          <CardTitle className="text-white">All Batches</CardTitle>
        </CardHeader>
        <CardContent>
          {batches?.length === 0 ? (
            <div className="text-center text-gray-400 py-8">
              No batches yet. Create your first sync batch to get started.
            </div>
          ) : (
            <div className="space-y-4">
              {batches?.map((batch: Batch) => (
                <Link
                  key={batch.id}
                  href={`/dashboard/batches/${batch.id}`}
                  className="flex items-center justify-between rounded-lg border border-gray-700 p-4 hover:bg-gray-800/50 transition-colors block"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-white">{batch.name}</span>
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
                    <div className="mt-2 flex items-center gap-6 text-sm text-gray-400">
                      <span className="flex items-center gap-1">
                        <Clock className="h-4 w-4" />
                        {formatDistanceToNow(new Date(batch.created_at), { addSuffix: true })}
                      </span>
                      <span className="uppercase">{batch.spotify_type}</span>
                      <span>
                        {batch.completed_jobs}/{batch.total_jobs} jobs
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="w-32">
                      <Progress value={batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0} className="h-2" />
                      <div className="mt-1 text-right text-xs text-gray-400">
                        {batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0}%
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {batch.status === 'failed' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => { e.preventDefault(); handleRetry(batch.id); }}
                          disabled={retryBatch.isPending}
                        >
                          <RefreshCw className="h-4 w-4" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={(e) => { e.preventDefault(); handleDelete(batch.id); }}
                        disabled={deletingId === batch.id}
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
