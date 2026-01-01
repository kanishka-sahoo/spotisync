'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Music } from 'lucide-react'
import { Button } from '@/components/ui/button'

export default function Home() {
  const router = useRouter()

  useEffect(() => {
    // Check if user has a token and redirect to dashboard
    const token = localStorage.getItem('token')
    if (token) {
      router.push('/dashboard')
    }
  }, [router])

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-black">
      <div className="text-center">
        <Music className="mx-auto h-24 w-24 text-spotify-green" />
        <h1 className="mt-8 text-4xl font-bold text-white">Spotisync</h1>
        <p className="mt-4 text-xl text-gray-400">
          Sync your Spotify library to Navidrome with high-quality audio
        </p>
        <div className="mt-8 flex gap-4 justify-center">
          <Link href="/login">
            <Button size="lg">Sign In</Button>
          </Link>
          <Link href="/register">
            <Button size="lg" variant="outline">Create Account</Button>
          </Link>
        </div>
      </div>
    </div>
  )
}
