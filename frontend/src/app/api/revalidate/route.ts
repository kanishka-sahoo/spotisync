import { NextRequest, NextResponse } from 'next/server'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { path } = body

    if (!path) {
      return NextResponse.json(
        { error: 'Path is required' },
        { status: 400 }
      )
    }

    // Revalidate the cache for the given path
    await new Promise((resolve) => setTimeout(resolve, 1000)) // Simulate revalidation

    return NextResponse.json({ revalidated: true, now: Date.now() })
  } catch (_error) {
    return NextResponse.json(
      { error: 'Failed to revalidate' },
      { status: 500 }
    )
  }
}
