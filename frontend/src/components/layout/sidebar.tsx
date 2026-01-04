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
  Music,
  X
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

interface SidebarProps {
  isOpen: boolean
  onToggle: () => void
}

export function Sidebar({ isOpen, onToggle }: SidebarProps) {
  const pathname = usePathname()

  const handleLinkClick = () => {
    // Close sidebar on mobile when clicking a link
    if (window.innerWidth < 1024) {
      onToggle()
    }
  }

  return (
    <>
      {/* Desktop sidebar - always visible on lg+ */}
      <div className="hidden lg:flex h-full w-64 flex-col bg-black border-r border-gray-800">
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

      {/* Mobile/Tablet sidebar - overlay */}
      <div
        className={cn(
          'fixed inset-y-0 left-0 z-50 flex h-full w-64 flex-col bg-black border-r border-gray-800 transform transition-transform duration-300 ease-in-out lg:hidden',
          isOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-16 items-center justify-between border-b border-gray-800 px-6">
          <div className="flex items-center gap-2">
            <Music className="h-8 w-8 text-spotify-green" />
            <span className="text-xl font-bold text-white">Spotisync</span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={onToggle}
            className="text-gray-400 hover:text-white lg:hidden"
            aria-label="Close sidebar"
          >
            <X className="h-5 w-5" />
          </Button>
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
                onClick={handleLinkClick}
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
    </>
  )
}
