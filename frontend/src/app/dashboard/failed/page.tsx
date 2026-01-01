'use client'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useBatches, useRetryBatch } from '@/hooks/useJobs'
import { Batch } from '@/lib/types'
import { formatDistanceToNow } from 'date-fns'
import {
  AlertCircle,
  RefreshCw,
  ArrowRight,
  Search,
  CheckCircle2,
  XCircle,
  Clock,
  AlertTriangle,
  PartyPopper,
} from 'lucide-react'
import Link from 'next/link'
import { useState, useMemo, useCallback, useEffect } from 'react'
import { useToast } from '@/hooks/useToast'

type SortOption = 'recent' | 'oldest' | 'most-failed'

export default function FailedPage() {
  const { toast } = useToast()
  const { data: batches, isLoading } = useBatches()
  const retryBatch = useRetryBatch()

  // State
  const [retryingIds, setRetryingIds] = useState<Set<string>>(new Set())
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [sortBy, setSortBy] = useState<SortOption>('recent')
  const [isBulkRetrying, setIsBulkRetrying] = useState(false)

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchQuery])

  // Get all failed batches
  const allFailedBatches = useMemo(() => {
    return batches?.filter((b: Batch) => b.status === 'failed') || []
  }, [batches])

  // Filter and sort batches
  const filteredBatches = useMemo(() => {
    let result = [...allFailedBatches]

    // Apply search filter
    if (debouncedSearch) {
      const query = debouncedSearch.toLowerCase()
      result = result.filter((batch: Batch) =>
        batch.name.toLowerCase().includes(query)
      )
    }

    // Apply sorting
    switch (sortBy) {
      case 'recent':
        result.sort(
          (a, b) =>
            new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
        )
        break
      case 'oldest':
        result.sort(
          (a, b) =>
            new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime()
        )
        break
      case 'most-failed':
        result.sort((a, b) => b.failed_jobs - a.failed_jobs)
        break
    }

    return result
  }, [allFailedBatches, debouncedSearch, sortBy])

  // Statistics
  const stats = useMemo(() => {
    const totalBatches = allFailedBatches.length
    const totalFailedJobs = allFailedBatches.reduce(
      (sum: number, b: Batch) => sum + b.failed_jobs,
      0
    )
    const totalJobs = allFailedBatches.reduce(
      (sum: number, b: Batch) => sum + b.total_jobs,
      0
    )
    return { totalBatches, totalFailedJobs, totalJobs }
  }, [allFailedBatches])

  // Selection handlers
  const handleSelectAll = useCallback(() => {
    if (selectedIds.size === filteredBatches.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(filteredBatches.map((b: Batch) => b.id)))
    }
  }, [filteredBatches, selectedIds.size])

  const handleSelectBatch = useCallback((id: string, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (checked) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return next
    })
  }, [])

  // Individual retry handler
  const handleRetry = async (id: string) => {
    setRetryingIds((prev) => new Set(prev).add(id))
    try {
      await retryBatch.mutateAsync(id)
      toast({
        title: 'Retry initiated',
        description: 'The batch has been queued for retry',
        variant: 'success',
      })
      // Remove from selection after successful retry
      setSelectedIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    } catch (_error) {
      toast({
        title: 'Retry failed',
        description: 'Failed to initiate retry for this batch',
        variant: 'destructive',
      })
    } finally {
      setRetryingIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }
  }

  // Bulk retry handler
  const handleBulkRetry = async () => {
    if (selectedIds.size === 0) return

    setIsBulkRetrying(true)
    const idsToRetry = Array.from(selectedIds)
    let successCount = 0
    let failCount = 0

    for (const id of idsToRetry) {
      try {
        await retryBatch.mutateAsync(id)
        successCount++
        // Remove from selection after successful retry
        setSelectedIds((prev) => {
          const next = new Set(prev)
          next.delete(id)
          return next
        })
      } catch (_error) {
        failCount++
      }
    }

    setIsBulkRetrying(false)

    if (successCount > 0 && failCount === 0) {
      toast({
        title: 'Bulk retry initiated',
        description: `${successCount} batch${successCount > 1 ? 'es' : ''} queued for retry`,
        variant: 'success',
      })
    } else if (successCount > 0 && failCount > 0) {
      toast({
        title: 'Partial success',
        description: `${successCount} succeeded, ${failCount} failed`,
        variant: 'default',
      })
    } else {
      toast({
        title: 'Bulk retry failed',
        description: 'Failed to retry selected batches',
        variant: 'destructive',
      })
    }
  }

  // Calculate progress for a batch
  const getProgressData = (batch: Batch) => {
    const total = batch.total_jobs
    const completed = batch.completed_jobs
    const failed = batch.failed_jobs
    const pending = total - completed - failed

    const completedPercent = total > 0 ? (completed / total) * 100 : 0
    const failedPercent = total > 0 ? (failed / total) * 100 : 0
    const pendingPercent = total > 0 ? (pending / total) * 100 : 0

    return { completed, failed, pending, total, completedPercent, failedPercent, pendingPercent }
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <RefreshCw className="h-8 w-8 animate-spin text-gray-400" />
      </div>
    )
  }

  const isAllSelected =
    filteredBatches.length > 0 && selectedIds.size === filteredBatches.length
  const isIndeterminate =
    selectedIds.size > 0 && selectedIds.size < filteredBatches.length

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold text-white">Failed Batches</h1>

      {/* Statistics Banner */}
      {allFailedBatches.length > 0 && (
        <div className="grid gap-4 md:grid-cols-3">
          <Card className="border-gray-700 bg-gray-900">
            <CardContent className="flex items-center gap-4 p-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-red-500/10">
                <AlertCircle className="h-6 w-6 text-red-500" />
              </div>
              <div>
                <p className="text-sm text-gray-400">Failed Batches</p>
                <p className="text-2xl font-bold text-white">
                  {stats.totalBatches}
                </p>
              </div>
            </CardContent>
          </Card>
          <Card className="border-gray-700 bg-gray-900">
            <CardContent className="flex items-center gap-4 p-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-orange-500/10">
                <XCircle className="h-6 w-6 text-orange-500" />
              </div>
              <div>
                <p className="text-sm text-gray-400">Failed Jobs</p>
                <p className="text-2xl font-bold text-white">
                  {stats.totalFailedJobs}
                </p>
              </div>
            </CardContent>
          </Card>
          <Card className="border-gray-700 bg-gray-900">
            <CardContent className="flex items-center gap-4 p-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-500/10">
                <Clock className="h-6 w-6 text-gray-400" />
              </div>
              <div>
                <p className="text-sm text-gray-400">Total Jobs in Failed Batches</p>
                <p className="text-2xl font-bold text-white">
                  {stats.totalJobs}
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <Card className="border-gray-700 bg-gray-900">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <AlertCircle className="h-5 w-5 text-red-500" />
            Failed ({allFailedBatches.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {allFailedBatches.length === 0 ? (
            // Empty State
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <div className="mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-green-500/10">
                <PartyPopper className="h-10 w-10 text-green-500" />
              </div>
              <h3 className="mb-2 text-xl font-semibold text-white">
                All Clear!
              </h3>
              <p className="max-w-sm text-gray-400">
                No failed batches found. All your syncs are running smoothly.
                Keep up the great work!
              </p>
              <Link href="/dashboard" className="mt-6">
                <Button variant="outline">
                  <CheckCircle2 className="mr-2 h-4 w-4" />
                  Go to Dashboard
                </Button>
              </Link>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Search and Sort Controls */}
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="relative flex-1 max-w-md">
                  <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
                  <Input
                    placeholder="Search batches by name..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="pl-9"
                  />
                </div>
                <Select
                  value={sortBy}
                  onValueChange={(value: SortOption) => setSortBy(value)}
                >
                  <SelectTrigger className="w-[180px]">
                    <SelectValue placeholder="Sort by" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="recent">Most Recent</SelectItem>
                    <SelectItem value="oldest">Oldest First</SelectItem>
                    <SelectItem value="most-failed">Most Failed Jobs</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {/* Bulk Actions Bar */}
              <div className="flex items-center gap-4 rounded-lg border border-gray-700 bg-gray-800/50 p-3">
                <div className="flex items-center gap-2">
                  <Checkbox
                    checked={isAllSelected}
                    onCheckedChange={handleSelectAll}
                    aria-label="Select all batches"
                    data-state={isIndeterminate ? 'indeterminate' : undefined}
                  />
                  <span className="text-sm text-gray-300">
                    {selectedIds.size > 0
                      ? `${selectedIds.size} selected`
                      : 'Select All'}
                  </span>
                </div>
                <div className="h-4 w-px bg-gray-600" />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleBulkRetry}
                  disabled={selectedIds.size === 0 || isBulkRetrying}
                >
                  <RefreshCw
                    className={`mr-2 h-4 w-4 ${isBulkRetrying ? 'animate-spin' : ''}`}
                  />
                  {isBulkRetrying
                    ? 'Retrying...'
                    : `Retry Selected${selectedIds.size > 0 ? ` (${selectedIds.size})` : ''}`}
                </Button>
              </div>

              {/* No results from search */}
              {filteredBatches.length === 0 && debouncedSearch && (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                  <AlertTriangle className="mb-4 h-12 w-12 text-gray-500" />
                  <p className="text-gray-400">
                    No batches match &quot;{debouncedSearch}&quot;
                  </p>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSearchQuery('')}
                    className="mt-2"
                  >
                    Clear search
                  </Button>
                </div>
              )}

              {/* Batch List */}
              <div className="space-y-3">
                {filteredBatches.map((batch: Batch) => {
                  const progress = getProgressData(batch)
                  const isRetrying = retryingIds.has(batch.id)
                  const isSelected = selectedIds.has(batch.id)

                  return (
                    <div
                      key={batch.id}
                      className={`rounded-lg border p-4 transition-colors ${
                        isSelected
                          ? 'border-spotify-green/50 bg-spotify-green/5'
                          : 'border-red-900/50 bg-red-900/10 hover:bg-red-900/20'
                      }`}
                    >
                      <div className="flex items-start gap-4">
                        {/* Checkbox */}
                        <div className="pt-1">
                          <Checkbox
                            checked={isSelected}
                            onCheckedChange={(checked: boolean) =>
                              handleSelectBatch(batch.id, checked)
                            }
                            aria-label={`Select ${batch.name}`}
                          />
                        </div>

                        {/* Batch Info */}
                        <div className="flex-1 min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-medium text-white truncate">
                              {batch.name}
                            </span>
                            <Badge variant="destructive">Failed</Badge>
                            <Badge variant="secondary" className="uppercase text-xs">
                              {batch.spotify_type}
                            </Badge>
                          </div>

                          {/* Timestamp */}
                          <p className="mt-1 text-sm text-gray-400">
                            Updated{' '}
                            {formatDistanceToNow(new Date(batch.updated_at), {
                              addSuffix: true,
                            })}
                          </p>

                          {/* Error reason - show if jobs have errors */}
                          {batch.jobs && batch.jobs.length > 0 && (
                            <div className="mt-2">
                              {batch.jobs
                                .filter((job) => job.error)
                                .slice(0, 1)
                                .map((job) => (
                                  <p
                                    key={job.id}
                                    className="text-sm text-red-400 flex items-start gap-1"
                                  >
                                    <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
                                    <span className="truncate">
                                      {job.error}
                                      {batch.jobs!.filter((j) => j.error).length > 1 &&
                                        ` (+${batch.jobs!.filter((j) => j.error).length - 1} more)`}
                                    </span>
                                  </p>
                                ))}
                            </div>
                          )}

                          {/* Progress bar */}
                          <div className="mt-3">
                            <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
                              <span>Progress</span>
                              <span>
                                {progress.completed} completed / {progress.failed} failed / {progress.total} total
                              </span>
                            </div>
                            <div className="h-2 w-full rounded-full bg-gray-800 overflow-hidden flex">
                              {/* Completed segment */}
                              <div
                                className="h-full bg-green-500 transition-all"
                                style={{ width: `${progress.completedPercent}%` }}
                              />
                              {/* Failed segment */}
                              <div
                                className="h-full bg-red-500 transition-all"
                                style={{ width: `${progress.failedPercent}%` }}
                              />
                              {/* Pending segment (gray, already the background) */}
                            </div>
                            <div className="flex gap-4 mt-1 text-xs">
                              <span className="flex items-center gap-1 text-green-400">
                                <span className="h-2 w-2 rounded-full bg-green-500" />
                                {progress.completed} completed
                              </span>
                              <span className="flex items-center gap-1 text-red-400">
                                <span className="h-2 w-2 rounded-full bg-red-500" />
                                {progress.failed} failed
                              </span>
                              {progress.pending > 0 && (
                                <span className="flex items-center gap-1 text-gray-400">
                                  <span className="h-2 w-2 rounded-full bg-gray-600" />
                                  {progress.pending} pending
                                </span>
                              )}
                            </div>
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-2 flex-shrink-0">
                          <Link href={`/dashboard/batches/${batch.id}`}>
                            <Button variant="ghost" size="sm">
                              View Details
                              <ArrowRight className="ml-2 h-4 w-4" />
                            </Button>
                          </Link>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleRetry(batch.id)}
                            disabled={isRetrying || isBulkRetrying}
                          >
                            <RefreshCw
                              className={`mr-2 h-4 w-4 ${isRetrying ? 'animate-spin' : ''}`}
                            />
                            {isRetrying ? 'Retrying...' : 'Retry'}
                          </Button>
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
