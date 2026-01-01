'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import {
  LayoutDashboard,
  FolderSync,
  XCircle,
  Settings,
  LogOut,
  Music
} from 'lucide-react'
import { Button } from '@/components/ui/button'

const sidebarItems = [
  {
    title: 'Dashboard',
    href: '/dashboard',
    icon: LayoutDashboard,
  },
  {
    title: 'Batches',
    href: '/dashboard/batches',
    icon: FolderSync,
  },
  {
    title: 'Failed',
    href: '/dashboard/failed',
    icon: XCircle,
  },
  {
    title: 'Settings',
    href: '/dashboard/settings',
    icon: Settings,
  },
]

export function Sidebar() {
  const pathname = usePathname()

  return (
    <div className="flex h-full w-64 flex-col bg-black border-r border-gray-800">
      <div className="flex h-16 items-center gap-2 border-b border-gray-800 px-6">
        <Music className="h-8 w-8 text-spotify-green" />
        <span className="text-xl font-bold text-white">Spotisync</span>
      </div>
      <nav className="flex-1 space-y-1 px-3 py-4">
        {sidebarItems.map((item) => {
          const isActive = item.href === '/dashboard' 
            ? pathname === '/dashboard'
            : pathname.startsWith(item.href)
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-400 hover:bg-gray-800 hover:text-white'
              )}
            >
              <item.icon className="h-5 w-5" />
              {item.title}
            </Link>
          )
        })}
      </nav>
      <div className="border-t border-gray-800 p-4">
        <Button
          variant="ghost"
          className="w-full justify-start gap-3 text-gray-400 hover:text-white"
          onClick={() => {
            localStorage.removeItem('token')
            window.location.href = '/login'
          }}
        >
          <LogOut className="h-5 w-5" />
          Logout
        </Button>
      </div>
    </div>
  )
}
