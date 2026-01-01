import { NextResponse } from 'next/server'

/**
 * Runtime configuration endpoint
 * Returns configuration values that can be set via environment variables at runtime
 * This allows Docker deployments to configure the frontend without rebuilding
 */
export async function GET() {
  return NextResponse.json({
    apiUrl: process.env.NEXT_PUBLIC_API_URL || '',
  })
}

// Force dynamic rendering to read environment variables at runtime
export const dynamic = 'force-dynamic'
