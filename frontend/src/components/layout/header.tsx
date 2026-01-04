'use client'

import { Bell, Search, User, Menu } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface HeaderProps {
  onMenuClick: () => void
}

export function Header({ onMenuClick }: HeaderProps) {
  return (
    <header className="flex h-16 items-center justify-between border-b border-gray-800 bg-black px-4 sm:px-6">
      <div className="flex items-center gap-2 sm:gap-4 flex-1">
        {/* Hamburger menu button - only visible on mobile/tablet */}
        <Button
          variant="ghost"
          size="icon"
          onClick={onMenuClick}
          className="text-gray-400 hover:text-white lg:hidden"
          aria-label="Open menu"
        >
          <Menu className="h-5 w-5" />
        </Button>

        {/* Search input - responsive */}
        <div className="relative hidden sm:block flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <Input
            placeholder="Search..."
            className="w-full pl-10 bg-gray-900 border-gray-700"
          />
        </div>
      </div>

      {/* Right side actions */}
      <div className="flex items-center gap-2 sm:gap-4">
        {/* Search button on mobile - optional: toggle search input */}
        <Button
          variant="ghost"
          size="icon"
          className="text-gray-400 hover:text-white sm:hidden"
          aria-label="Search"
        >
          <Search className="h-5 w-5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="text-gray-400 hover:text-white"
          aria-label="Notifications"
        >
          <Bell className="h-5 w-5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="text-gray-400 hover:text-white"
          aria-label="User profile"
        >
          <User className="h-5 w-5" />
        </Button>
      </div>
    </header>
  )
}
