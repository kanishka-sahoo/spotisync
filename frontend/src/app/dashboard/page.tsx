'use client'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { FolderSync, CheckCircle, XCircle, Clock, Activity, Plus } from 'lucide-react'
import { useBatches } from '@/hooks/useJobs'
import { Batch } from '@/lib/types'
import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { CreateBatchForm } from '@/components/jobs/create-batch-form'
import { Button } from '@/components/ui/button'

export default function DashboardPage() {
  const { data: batches, isLoading } = useBatches()
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)

  const stats = {
    total: batches?.length || 0,
    completed: batches?.filter((b: Batch) => b.status === 'completed').length || 0,
    failed: batches?.filter((b: Batch) => b.status === 'failed').length || 0,
    processing: batches?.filter((b: Batch) => b.status === 'processing').length || 0,
  }

  const recentBatches = batches?.slice(0, 5) || []

  if (isLoading) {
    return <div className="text-white">Loading...</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold text-white">Dashboard</h1>
        <Dialog open={isCreateModalOpen} onOpenChange={setIsCreateModalOpen}>
          <DialogTrigger asChild>
            <Button className="bg-spotify-green hover:bg-spotify-green/90 text-black">
              <Plus className="mr-2 h-4 w-4" />
              New Sync
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[500px]">
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

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Total Batches
            </CardTitle>
            <FolderSync className="h-4 w-4 text-spotify-green" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{stats.total}</div>
          </CardContent>
        </Card>
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Completed
            </CardTitle>
            <CheckCircle className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{stats.completed}</div>
          </CardContent>
        </Card>
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Failed
            </CardTitle>
            <XCircle className="h-4 w-4 text-red-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{stats.failed}</div>
          </CardContent>
        </Card>
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Processing
            </CardTitle>
            <Activity className="h-4 w-4 text-yellow-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{stats.processing}</div>
          </CardContent>
        </Card>
      </div>

      <Card className="border-gray-700 bg-gray-900">
        <CardHeader>
          <CardTitle className="text-white">Recent Batches</CardTitle>
        </CardHeader>
        <CardContent>
          {recentBatches.length === 0 ? (
            <div className="text-center text-gray-400 py-8">
              No batches yet. Create your first sync batch to get started.
            </div>
          ) : (
            <div className="space-y-4">
              {recentBatches.map((batch: Batch) => (
                <div
                  key={batch.id}
                  className="flex items-center justify-between rounded-lg border border-gray-700 p-4"
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
                    <div className="mt-2 flex items-center gap-4 text-sm text-gray-400">
                      <span className="flex items-center gap-1">
                        <Clock className="h-4 w-4" />
                        {new Date(batch.created_at).toLocaleDateString()}
                      </span>
                      <span>
                        {batch.spotify_type.toUpperCase()}
                      </span>
                    </div>
                  </div>
                  <div className="w-32">
                    <Progress value={batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0} className="h-2" />
                    <div className="mt-1 text-right text-xs text-gray-400">
                      {batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0}%
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
