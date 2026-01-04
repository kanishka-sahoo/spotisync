'use client'

import { useState } from 'react'
import { Sidebar } from './sidebar'
import { Header } from './header'
import { Toaster } from '@/components/ui/toaster'
import { cn } from '@/lib/utils'

export function MainLayout({ children }: { children: React.ReactNode }) {
  const [isSidebarOpen, setIsSidebarOpen] = useState(false)

  const toggleSidebar = () => {
    setIsSidebarOpen(!isSidebarOpen)
  }

  const closeSidebar = () => {
    setIsSidebarOpen(false)
  }

  return (
    <div className="flex h-screen bg-gray-950">
      {/* Backdrop overlay for mobile */}
      <div
        className={cn(
          'fixed inset-0 bg-black/50 backdrop-blur-sm z-40 lg:hidden transition-opacity duration-300',
          isSidebarOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
        )}
        onClick={closeSidebar}
        aria-hidden="true"
      />

      <Sidebar isOpen={isSidebarOpen} onToggle={toggleSidebar} />
      
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header onMenuClick={toggleSidebar} />
        <main className="flex-1 overflow-y-auto bg-gray-950 p-4 sm:p-6">
          {children}
        </main>
      </div>
      <Toaster />
    </div>
  )
}
