'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Loader2, Link as LinkIcon, Music, Clock, ArrowLeft, Eye } from 'lucide-react'
import { useCreateBatch } from '@/hooks/useJobs'
import { useToast } from '@/hooks/useToast'
import { previewApi, PreviewResponse } from '@/lib/api'
import Image from 'next/image'

interface CreateBatchFormProps {
  onSuccess?: () => void
  onCancel?: () => void
  inModal?: boolean
}

// Format duration from milliseconds to mm:ss
function formatDuration(ms: number): string {
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.floor((ms % 60000) / 1000)
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
}

export function CreateBatchForm({ onSuccess, onCancel, inModal = false }: CreateBatchFormProps) {
  const router = useRouter()
  const { toast } = useToast()
  const [spotifyUrl, setSpotifyUrl] = useState('')
  const [isPreviewLoading, setIsPreviewLoading] = useState(false)
  const [previewData, setPreviewData] = useState<PreviewResponse | null>(null)
  const createBatch = useCreateBatch()

  const handlePreview = async () => {
    if (!spotifyUrl.trim()) {
      toast({
        title: 'Missing URL',
        description: 'Please enter a Spotify URL',
        variant: 'destructive',
      })
      return
    }

    // Basic URL validation
    const urlPattern = /^(https?:\/\/)?(open\.)?spotify\.com\/(track|album|playlist|artist)\/[a-zA-Z0-9]+/
    if (!urlPattern.test(spotifyUrl)) {
      toast({
        title: 'Invalid URL',
        description: 'Please enter a valid Spotify URL (track, album, playlist, or artist)',
        variant: 'destructive',
      })
      return
    }

    setIsPreviewLoading(true)
    try {
      const data = await previewApi.preview(spotifyUrl)
      setPreviewData(data)
    } catch (error: any) {
      toast({
        title: 'Preview failed',
        description: error.response?.data || error.message || 'Failed to fetch preview',
        variant: 'destructive',
      })
    } finally {
      setIsPreviewLoading(false)
    }
  }

  const handleCreateBatch = async () => {
    try {
      const result = await createBatch.mutateAsync(spotifyUrl)

      toast({
        title: 'Batch created',
        description: `Started syncing ${previewData?.total_tracks || 0} tracks`,
        variant: 'success',
      })

      // Navigate to the batch detail page
      if (result.batch_id) {
        router.push(`/dashboard/batches/${result.batch_id}`)
      } else if (result.id) {
        router.push(`/dashboard/batches/${result.id}`)
      } else {
        router.push('/dashboard')
      }

      if (onSuccess) {
        onSuccess()
      }
    } catch (error: any) {
      toast({
        title: 'Error creating batch',
        description: error.response?.data?.error || 'Failed to create sync batch',
        variant: 'destructive',
      })
    }
  }

  const handleBack = () => {
    setPreviewData(null)
  }

  // Preview step - show tracks
  if (previewData) {
    const previewContent = (
      <>
        <div className="flex items-start gap-4 mb-4">
          {previewData.cover_url && (
            <div className="relative h-20 w-20 flex-shrink-0 overflow-hidden rounded-lg">
              <Image
                src={previewData.cover_url}
                alt={previewData.name}
                fill
                className="object-cover"
                unoptimized
              />
            </div>
          )}
          <div className="flex-1 min-w-0">
            <h3 className="text-white font-semibold truncate">{previewData.name}</h3>
            <p className="text-gray-400 text-sm">
              {previewData.type.charAt(0).toUpperCase() + previewData.type.slice(1)} • {previewData.total_tracks} tracks
            </p>
          </div>
        </div>

        {/* Track list */}
        <div className="max-h-80 overflow-y-auto rounded-lg border border-gray-700">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-gray-800 text-gray-400">
              <tr>
                <th className="px-3 py-2 text-left font-medium">#</th>
                <th className="px-3 py-2 text-left font-medium">Title</th>
                <th className="px-3 py-2 text-left font-medium hidden sm:table-cell">Artist</th>
                <th className="px-3 py-2 text-right font-medium">
                  <Clock className="h-4 w-4 inline" />
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-700">
              {previewData.tracks.map((track, index) => (
                <tr key={track.id} className="hover:bg-gray-800/50">
                  <td className="px-3 py-2 text-gray-500">{index + 1}</td>
                  <td className="px-3 py-2 text-white truncate max-w-[200px]">
                    <div className="flex items-center gap-2">
                      <Music className="h-4 w-4 text-gray-500 flex-shrink-0" />
                      <span className="truncate">{track.name}</span>
                    </div>
                    <div className="text-xs text-gray-500 sm:hidden truncate">
                      {track.artist}
                    </div>
                  </td>
                  <td className="px-3 py-2 text-gray-400 truncate max-w-[150px] hidden sm:table-cell">
                    {track.artist}
                  </td>
                  <td className="px-3 py-2 text-gray-400 text-right whitespace-nowrap">
                    {formatDuration(track.duration_ms)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Actions */}
        <div className="flex gap-4 justify-between pt-2">
          <Button
            type="button"
            variant="outline"
            onClick={handleBack}
            disabled={createBatch.isPending}
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
          <div className="flex gap-2">
            {onCancel && (
              <Button
                type="button"
                variant="ghost"
                onClick={onCancel}
                disabled={createBatch.isPending}
              >
                Cancel
              </Button>
            )}
            <Button
              onClick={handleCreateBatch}
              disabled={createBatch.isPending}
            >
              {createBatch.isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating...
                </>
              ) : (
                `Sync ${previewData.total_tracks} Tracks`
              )}
            </Button>
          </div>
        </div>
      </>
    )

    if (inModal) {
      return <div className="space-y-4">{previewContent}</div>
    }

    return (
      <Card className="border-gray-700 bg-gray-900 w-full max-w-2xl">
        <CardHeader>
          <div className="flex items-start gap-4">
            {previewData.cover_url && (
              <div className="relative h-20 w-20 flex-shrink-0 overflow-hidden rounded-lg">
                <Image
                  src={previewData.cover_url}
                  alt={previewData.name}
                  fill
                  className="object-cover"
                  unoptimized
                />
              </div>
            )}
            <div className="flex-1 min-w-0">
              <CardTitle className="text-white truncate">{previewData.name}</CardTitle>
              <CardDescription className="text-gray-400">
                {previewData.type.charAt(0).toUpperCase() + previewData.type.slice(1)} • {previewData.total_tracks} tracks
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Track list */}
          <div className="max-h-80 overflow-y-auto rounded-lg border border-gray-700">
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-gray-800 text-gray-400">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">#</th>
                  <th className="px-3 py-2 text-left font-medium">Title</th>
                  <th className="px-3 py-2 text-left font-medium hidden sm:table-cell">Artist</th>
                  <th className="px-3 py-2 text-right font-medium">
                    <Clock className="h-4 w-4 inline" />
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-700">
                {previewData.tracks.map((track, index) => (
                  <tr key={track.id} className="hover:bg-gray-800/50">
                    <td className="px-3 py-2 text-gray-500">{index + 1}</td>
                    <td className="px-3 py-2 text-white truncate max-w-[200px]">
                      <div className="flex items-center gap-2">
                        <Music className="h-4 w-4 text-gray-500 flex-shrink-0" />
                        <span className="truncate">{track.name}</span>
                      </div>
                      <div className="text-xs text-gray-500 sm:hidden truncate">
                        {track.artist}
                      </div>
                    </td>
                    <td className="px-3 py-2 text-gray-400 truncate max-w-[150px] hidden sm:table-cell">
                      {track.artist}
                    </td>
                    <td className="px-3 py-2 text-gray-400 text-right whitespace-nowrap">
                      {formatDuration(track.duration_ms)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Actions */}
          <div className="flex gap-4 justify-between pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={handleBack}
              disabled={createBatch.isPending}
            >
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Button>
            <div className="flex gap-2">
              {onCancel && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={onCancel}
                  disabled={createBatch.isPending}
                >
                  Cancel
                </Button>
              )}
              <Button
                onClick={handleCreateBatch}
                disabled={createBatch.isPending}
              >
                {createBatch.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  `Sync ${previewData.total_tracks} Tracks`
                )}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  // Input step - enter URL
  const formContent = (
    <>
      <form onSubmit={(e) => { e.preventDefault(); handlePreview(); }} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="spotifyUrl" className="text-gray-300">
            Spotify URL
          </Label>
          <div className="relative">
            <LinkIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <Input
              id="spotifyUrl"
              placeholder="https://open.spotify.com/playlist/..."
              value={spotifyUrl}
              onChange={(e) => setSpotifyUrl(e.target.value)}
              className="border-gray-700 bg-gray-800 text-white pl-10"
              disabled={isPreviewLoading}
            />
          </div>
          <p className="text-xs text-gray-500">
            Supports Spotify track, album, playlist, or artist URLs
          </p>
        </div>

        <div className="flex gap-4 justify-end">
          {onCancel && (
            <Button
              type="button"
              variant="outline"
              onClick={onCancel}
              disabled={isPreviewLoading}
            >
              Cancel
            </Button>
          )}
          <Button
            type="submit"
            disabled={isPreviewLoading || !spotifyUrl.trim()}
          >
            {isPreviewLoading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Loading Preview...
              </>
            ) : (
              <>
                <Eye className="mr-2 h-4 w-4" />
                Preview
              </>
            )}
          </Button>
        </div>
      </form>
    </>
  )

  if (inModal) {
    return <div className="space-y-4">{formContent}</div>
  }

  return (
    <Card className="border-gray-700 bg-gray-900 w-full">
      <CardHeader>
        <CardTitle className="text-white">Create New Sync Batch</CardTitle>
        <CardDescription className="text-gray-400">
          Enter a Spotify URL to preview and sync music to your library
        </CardDescription>
      </CardHeader>
      <CardContent>{formContent}</CardContent>
    </Card>
  )
}
