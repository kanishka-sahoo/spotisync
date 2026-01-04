'use client'

import { useState } from 'react'
import { useParams } from 'next/navigation'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useBatch, useRetryBatch, useCheckPlaylist, useSyncPlaylist, useRetryJob } from '@/hooks/useJobs'
import { usePlaylistProgress } from '@/hooks/usePlaylistProgress'
import { Job, PlaylistStatus } from '@/lib/types'
import { formatDistanceToNow } from 'date-fns'
import { ArrowLeft, Clock, CheckCircle, XCircle, RefreshCw, Music, FileText, Image, MinusCircle, ExternalLink, ListMusic, Search } from 'lucide-react'
import Link from 'next/link'
import { useToast } from '@/hooks/useToast'

// Validation function to check if URL is safe
function isValidHttpUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    // Only allow http/https protocols
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

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

function getPlaylistStatusBadgeVariant(status?: PlaylistStatus): 'success' | 'destructive' | 'secondary' | 'default' | 'warning' {
  switch (status) {
    case 'completed':
      return 'success'
    case 'failed':
      return 'destructive'
    case 'creating':
      return 'warning'
    default:
      return 'secondary'
  }
}

function getPlaylistStatusMessage(batch: { playlist_status?: PlaylistStatus, playlist_message?: string, tracks_found?: number, tracks_failed?: number }): string {
  if (!batch.playlist_status || batch.playlist_status === 'pending') {
    return 'Playlist creation will begin automatically when all downloads complete.'
  }
  if (batch.playlist_status === 'creating') {
    return batch.playlist_message || 'Searching for tracks in Navidrome...'
  }
  if (batch.playlist_status === 'completed') {
    const found = batch.tracks_found || 0
    const failed = batch.tracks_failed || 0
    return `Playlist created with ${found} tracks found and ${failed} tracks not found in Navidrome.`
  }
  if (batch.playlist_status === 'failed') {
    return batch.playlist_message || 'Failed to create playlist in Navidrome.'
  }
  return ''
}

export default function BatchDetailPage() {
  const params = useParams()
  const batchId = params.id as string
  const { data: batch, isLoading: batchLoading } = useBatch(batchId)
  const progress = usePlaylistProgress(batchId)
  const retryBatch = useRetryBatch()
  const retryJob = useRetryJob()
  const [checkResult, setCheckResult] = useState<string | null>(null)
  const { toast } = useToast()

  const checkPlaylist = useCheckPlaylist({
    onSuccess: (data) => {
      if (data && data.found) {
        setCheckResult('Playlist found in Navidrome')
        toast({
          title: 'Success',
          description: 'Playlist found in Navidrome',
          variant: 'success'
        })
      } else {
        setCheckResult('Playlist not found in Navidrome')
        toast({
          title: 'Warning',
          description: 'Playlist not found in Navidrome',
          variant: 'destructive'
        })
      }
    },
    onError: () => {
      setCheckResult('Error checking playlist')
      toast({
        title: 'Error',
        description: 'Failed to check playlist',
        variant: 'destructive'
      })
    }
  })

  const syncPlaylist = useSyncPlaylist({
    onSuccess: () => {
      toast({
        title: 'Success',
        description: 'Playlist synced successfully',
        variant: 'success'
      })
    },
    onError: () => {
      toast({
        title: 'Error',
        description: 'Failed to sync playlist',
        variant: 'destructive'
      })
    }
  })

  if (batchLoading) {
    return <div className="text-white">Loading...</div>
  }

  if (!batch) {
    return <div className="text-white">Batch not found</div>
  }

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Header - responsive layout */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
        <Link href="/dashboard/batches">
          <Button variant="ghost" size="sm" className="min-h-[44px] w-full sm:w-auto">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Batches
          </Button>
        </Link>
        <div className="flex items-center gap-2 flex-wrap">
          <h1 className="text-xl sm:text-2xl lg:text-3xl font-bold text-white break-words">{batch.name}</h1>
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
      </div>

      {/* Stats cards - responsive grid */}
      <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
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

      {batch.spotify_type === 'playlist' && (
        <Card className="border-gray-700 bg-gray-900">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <Music className="h-4 w-4" />
              Navidrome Playlist
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Badge variant={getPlaylistStatusBadgeVariant(batch.playlist_status)}>
                  {batch.playlist_status || 'pending'}
                </Badge>
                {batch.playlist_status === 'creating' && (
                  <RefreshCw className="h-4 w-4 text-blue-500 animate-spin" />
                )}
              </div>
              <p className="text-sm text-gray-400">{getPlaylistStatusMessage(batch)}</p>
              {(batch.tracks_found !== undefined || batch.tracks_failed !== undefined) && (
                <div className="flex items-center gap-4 text-sm">
                  <span className="text-green-500">
                    {batch.tracks_found || 0} tracks found
                  </span>
                  <span className="text-red-500">
                    {batch.tracks_failed || 0} not found
                  </span>
                </div>
              )}
              {batch.playlist_id && (
                <div>
                  {isValidHttpUrl(batch.playlist_id) ? (
                    <a
                      href={batch.playlist_id}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-sm text-blue-400 hover:text-blue-300"
                    >
                      Open in Navidrome <ExternalLink className="h-3 w-3" />
                    </a>
                  ) : (
                    <span className="text-sm text-gray-400">
                      {batch.playlist_id}
                    </span>
                  )}
                </div>
              )}
               {(batch.status === 'completed' || batch.completed_jobs > 0) && (
                <div className="mt-4 pt-4 border-t border-gray-700">
                  <div className="flex flex-col sm:flex-row gap-2">
                    { (batch.playlist_status === 'pending' || batch.playlist_status === 'failed') && (
                      <Button
                        onClick={() => checkPlaylist.mutate(batch.id)}
                        disabled={checkPlaylist.isPending || syncPlaylist.isPending || (progress !== null)}
                        variant="outline"
                        size="sm"
                        className="w-full sm:w-auto min-h-[44px]"
                      >
                        <Search className={`mr-2 h-4 w-4 ${checkPlaylist.isPending ? 'animate-spin' : ''}`} />
                        {checkPlaylist.isPending ? 'Checking...' : 'Check Playlist'}
                      </Button>
                    )}
                    { (batch.status === 'completed' || batch.completed_jobs > 0) && (
                      <Button
                        onClick={() => syncPlaylist.mutate(batch.id)}
                        disabled={checkPlaylist.isPending || syncPlaylist.isPending || (progress !== null)}
                        variant="outline"
                        size="sm"
                        className="w-full sm:w-auto min-h-[44px]"
                      >
                        <RefreshCw className={`mr-2 h-4 w-4 ${syncPlaylist.isPending ? 'animate-spin' : ''}`} />
                        {syncPlaylist.isPending ? 'Syncing...' : 'Sync Playlist'}
                      </Button>
                    )}
                  </div>

                  {/* Real-time progress indicator for both operations */}
                  {progress && (
                    <div className="mt-4 space-y-2 animate-in fade-in duration-200">
                      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 sm:gap-2 text-sm">
                        <span className="text-gray-300 break-words flex-1">
                          {progress.message}
                        </span>
                        <div className="flex items-center gap-2">
                          {progress.current_track > 0 && progress.total_tracks > 0 && (
                            <span className="text-gray-500 text-xs whitespace-nowrap">
                              {progress.current_track}/{progress.total_tracks}
                            </span>
                          )}
                          <span className="text-gray-400 font-medium whitespace-nowrap">
                            {progress.progress}%
                          </span>
                        </div>
                      </div>
                      <Progress value={progress.progress} className="h-2" />
                    </div>
                  )}
                </div>
              )}
              {checkResult && <p className="text-sm text-gray-400 mt-2">{checkResult}</p>}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Jobs table - responsive with horizontal scroll on mobile */}
      <Card className="border-gray-700 bg-gray-900">
        <CardHeader>
          <CardTitle className="text-white">Jobs ({batch.jobs?.length || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {/* Desktop table view */}
          <div className="hidden lg:block rounded-md border border-gray-700 overflow-x-auto">
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
                    Playlist
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
                        {batch.playlist_status === 'completed' && (
                          job.in_playlist ? (
                            <ListMusic className="h-4 w-4 text-green-500" />
                          ) : (
                            <span className="text-gray-400">-</span>
                          )
                        )}
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

          {/* Mobile card view */}
          <div className="lg:hidden space-y-3">
            {batch.jobs?.map((job: Job) => (
              <div
                key={job.id}
                className="rounded-lg border border-gray-700 p-3 space-y-3"
              >
                {/* Status and action row */}
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {getStatusIcon(job.status)}
                    <Badge variant={getStatusBadgeVariant(job.status)}>
                      {job.status}
                    </Badge>
                  </div>
                  {job.status === 'failed' && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-11 w-11"
                      onClick={() => retryJob.mutate(job.id)}
                      disabled={retryJob.isPending}
                      aria-label="Retry job"
                    >
                      <RefreshCw className="h-4 w-4" />
                    </Button>
                  )}
                </div>

                {/* Track info */}
                <div className="space-y-1">
                  <div className="text-white font-medium text-sm">{job.track_name || 'N/A'}</div>
                  <div className="text-gray-400 text-xs">{job.artist_name || 'N/A'}</div>
                  <div className="text-gray-500 text-xs">{job.album_name || 'N/A'}</div>
                </div>

                {/* Details row */}
                <div className="flex items-center justify-between">
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
                    {batch.playlist_status === 'completed' && job.in_playlist && (
                      <ListMusic className="h-4 w-4 text-green-500" />
                    )}
                  </div>
                </div>

                {/* Progress bar */}
                <div className="flex items-center gap-2">
                  <Progress value={job.progress} className="h-2 flex-1" />
                  <span className="text-xs text-gray-400 min-w-[3rem] text-right">{job.progress}%</span>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {batch.status === 'failed' && (
        <div className="flex justify-end">
          <Button
            onClick={() => retryBatch.mutate(batch.id)}
            disabled={retryBatch.isPending}
            className="w-full sm:w-auto min-h-[44px]"
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            Retry Batch
          </Button>
        </div>
      )}
    </div>
  )
}
