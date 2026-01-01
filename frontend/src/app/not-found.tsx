import { Music } from 'lucide-react'

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-black">
      <Music className="h-16 w-16 text-spotify-green" />
      <h1 className="mt-4 text-2xl font-bold text-white">Page Not Found</h1>
      <p className="mt-2 text-gray-400">
        The page you&apos;re looking for doesn&apos;t exist.
      </p>
    </div>
  )
}
