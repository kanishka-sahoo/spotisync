import { LoginForm } from '@/components/auth/login-form'
import { Music } from 'lucide-react'

export default function LoginPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-black p-4">
      <div className="flex flex-col items-center gap-8">
        <Music className="h-16 w-16 text-spotify-green" />
        <LoginForm />
      </div>
    </div>
  )
}
