/**
 * Runtime configuration utility
 * 
 * This module provides runtime configuration that works in both development and Docker.
 * In Docker, NEXT_PUBLIC_* env vars are baked in at build time, so we fetch the actual
 * runtime config from the /api/config endpoint which reads env vars at request time.
 */

interface RuntimeConfig {
  apiUrl: string;
}

// Default fallback configuration (used during SSR and initial client render)
const DEFAULT_CONFIG: RuntimeConfig = {
  apiUrl: process.env.NEXT_PUBLIC_API_URL || '',
};

// Cached runtime config for client-side use
let cachedConfig: RuntimeConfig | null = null;
let configPromise: Promise<RuntimeConfig> | null = null;

/**
 * Check if we're running on the server
 */
function isServer(): boolean {
  return typeof window === 'undefined';
}

/**
 * Fetch runtime config from the /api/config endpoint
 * This reads env vars at runtime, which is necessary for Docker deployments
 */
async function fetchRuntimeConfig(): Promise<RuntimeConfig> {
  try {
    const response = await fetch('/api/config', {
      cache: 'no-store', // Always fetch fresh config
    });
    
    if (!response.ok) {
      console.warn('[Config] Failed to fetch runtime config, using defaults');
      return DEFAULT_CONFIG;
    }
    
    const data = await response.json();
    return {
      apiUrl: data.apiUrl || DEFAULT_CONFIG.apiUrl,
    };
  } catch (error) {
    console.warn('[Config] Error fetching runtime config, using defaults:', error);
    return DEFAULT_CONFIG;
  }
}

/**
 * Get the runtime configuration asynchronously
 * 
 * On the server: Returns config from process.env directly
 * On the client: Fetches from /api/config and caches the result
 */
export async function getConfig(): Promise<RuntimeConfig> {
  // Server-side: read directly from process.env
  if (isServer()) {
    return {
      apiUrl: process.env.NEXT_PUBLIC_API_URL || '',
    };
  }

  // Client-side: return cached config if available
  if (cachedConfig) {
    return cachedConfig;
  }

  // Client-side: if a fetch is already in progress, wait for it
  if (configPromise) {
    return configPromise;
  }

  // Client-side: fetch and cache the config
  configPromise = fetchRuntimeConfig().then((config) => {
    cachedConfig = config;
    configPromise = null;
    return config;
  });

  return configPromise;
}

/**
 * Get config synchronously (returns cached or default)
 * 
 * Use this when you need config immediately and can't await.
 * It will return the cached config if available, otherwise the build-time default.
 * Call initConfig() early in your app to ensure the cache is populated.
 */
export function getConfigSync(): RuntimeConfig {
  if (isServer()) {
    return {
      apiUrl: process.env.NEXT_PUBLIC_API_URL || '',
    };
  }
  
  return cachedConfig || DEFAULT_CONFIG;
}

/**
 * Initialize the config cache
 * Call this early in your app (e.g., in a provider or layout)
 */
export async function initConfig(): Promise<RuntimeConfig> {
  return getConfig();
}

/**
 * Clear the cached config (useful for testing)
 */
export function clearConfigCache(): void {
  cachedConfig = null;
  configPromise = null;
}

/**
 * Check if runtime config has been loaded
 */
export function isConfigLoaded(): boolean {
  return cachedConfig !== null;
}

/**
 * Get the API URL, preferring runtime config over build-time env var
 */
export function getApiUrl(): string {
  return getConfigSync().apiUrl;
}

/**
 * Get the WebSocket URL derived from the API URL
 */
export function getWsUrl(): string {
  const apiUrl = getApiUrl();
  return apiUrl.replace(/^http/, 'ws');
}
