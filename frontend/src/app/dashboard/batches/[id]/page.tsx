'use client'

import { useParams } from 'next/navigation'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useBatch, useRetryBatch, useRetryJob } from '@/hooks/useJobs'
import { Job } from '@/lib/types'
import { formatDistanceToNow } from 'date-fns'
import { ArrowLeft, Clock, CheckCircle, XCircle, RefreshCw, Music, FileText, Image, MinusCircle } from 'lucide-react'
import Link from 'next/link'

function getStatusIcon(status: string) {
  switch (status) {
    case 'completed':
      return <CheckCircle className="h-4 w-4 text-green-500" />
    case 'failed':
      return <XCircle className="h-4 w-4 text-red-500" />
    case 'running':
      return <RefreshCw className="h-4 w-4 text-blue-500 animate-spin" />
    default:
      return <Clock className="h-4 w-4 text-gray-500" />
  }
}

function getGranularStatusIcon(status?: string) {
  switch (status) {
    case 'completed':
      return <CheckCircle className="h-3 w-3 text-green-500" />
    case 'failed':
      return <XCircle className="h-3 w-3 text-red-500" />
    case 'not_found':
      return <MinusCircle className="h-3 w-3 text-yellow-500" />
    case 'downloading':
    case 'fetching':
      return <RefreshCw className="h-3 w-3 text-blue-500 animate-spin" />
    default:
      return <Clock className="h-3 w-3 text-gray-500" />
  }
}

function getStatusBadgeVariant(status: string): 'success' | 'destructive' | 'secondary' | 'default' {
  switch (status) {
    case 'completed':
      return 'success'
    case 'failed':
      return 'destructive'
    case 'running':
      return 'default'
    default:
      return 'secondary'
  }
}

export default function BatchDetailPage() {
  const params = useParams()
  const batchId = params.id as string
  const { data: batch, isLoading: batchLoading } = useBatch(batchId)
  const retryBatch = useRetryBatch()
  const retryJob = useRetryJob()

  if (batchLoading) {
    return <div className="text-white">Loading...</div>
  }

  if (!batch) {
    return <div className="text-white">Batch not found</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link href="/dashboard/batches">
          <Button variant="ghost" size="sm">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Batches
          </Button>
        </Link>
        <h1 className="text-3xl font-bold text-white">{batch.name}</h1>
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

      <div className="grid gap-4 md:grid-cols-3">
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Progress
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">{batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0}%</div>
            <Progress value={batch.total_jobs > 0 ? Math.round((batch.completed_jobs / batch.total_jobs) * 100) : 0} className="mt-2 h-2" />
          </CardContent>
        </Card>
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Spotify Type
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white uppercase">
              {batch.spotify_type}
            </div>
          </CardContent>
        </Card>
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">
              Created
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-white">
              {formatDistanceToNow(new Date(batch.created_at), { addSuffix: true })}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card className="border-gray-700 bg-gray-900">
        <CardHeader>
          <CardTitle className="text-white">Jobs ({batch.jobs?.length || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border border-gray-700">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-700 bg-gray-800">
                  <th className="p-4 text-left text-sm font-medium text-gray-400">
                    Status
                  </th>
                  <th className="p-4 text-left text-sm font-medium text-gray-400">
                    Title
                  </th>
                  <th className="p-4 text-left text-sm font-medium text-gray-400">
                    Artist
                  </th>
                  <th className="p-4 text-left text-sm font-medium text-gray-400">
                    Album
                  </th>
                  <th className="p-4 text-left text-sm font-medium text-gray-400">
                    Details
                  </th>
                  <th className="p-4 text-left text-sm font-medium text-gray-400">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {batch.jobs?.map((job: Job) => (
                  <tr
                    key={job.id}
                    className="border-b border-gray-700 last:border-0"
                  >
                    <td className="p-4">
                      <div className="flex items-center gap-2">
                        {getStatusIcon(job.status)}
                        <Badge variant={getStatusBadgeVariant(job.status)}>
                          {job.status}
                        </Badge>
                      </div>
                    </td>
                    <td className="p-4 text-white">{job.track_name || 'N/A'}</td>
                    <td className="p-4 text-gray-400">{job.artist_name || 'N/A'}</td>
                    <td className="p-4 text-gray-400">{job.album_name || 'N/A'}</td>
                    <td className="p-4">
                      <div className="flex items-center gap-3">
                        {/* Song status */}
                        <div className="flex items-center gap-1" title={`Song: ${job.song_status || 'pending'}`}>
                          <Music className="h-4 w-4 text-gray-400" />
                          {getGranularStatusIcon(job.song_status)}
                        </div>
                        {/* Lyrics status */}
                        <div className="flex items-center gap-1" title={`Lyrics: ${job.lyrics_status || 'pending'}`}>
                          <FileText className="h-4 w-4 text-gray-400" />
                          {getGranularStatusIcon(job.lyrics_status)}
                        </div>
                        {/* Cover status */}
                        <div className="flex items-center gap-1" title={`Cover: ${job.cover_status || 'pending'}`}>
                          <Image className="h-4 w-4 text-gray-400" />
                          {getGranularStatusIcon(job.cover_status)}
                        </div>
                        {/* Overall progress bar */}
                        <div className="flex items-center gap-2 ml-2">
                          <Progress value={job.progress} className="h-2 w-16" />
                          <span className="text-xs text-gray-400">{job.progress}%</span>
                        </div>
                      </div>
                    </td>
                    <td className="p-4">
                      {job.status === 'failed' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => retryJob.mutate(job.id)}
                          disabled={retryJob.isPending}
                        >
                          <RefreshCw className="h-4 w-4" />
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {batch.status === 'failed' && (
        <div className="flex justify-end">
          <Button
            onClick={() => retryBatch.mutate(batch.id)}
            disabled={retryBatch.isPending}
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            Retry Batch
          </Button>
        </div>
      )}
    </div>
  )
}
