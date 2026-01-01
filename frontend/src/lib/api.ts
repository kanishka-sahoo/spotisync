import axios, { InternalAxiosRequestConfig } from 'axios';
import { getApiUrl, getConfig, isConfigLoaded } from './config';

// Preview types
export interface PreviewTrack {
  id: string;
  name: string;
  artist: string;
  artists: string[];
  album: string;
  duration_ms: number;
  isrc: string;
}

export interface PreviewResponse {
  name: string;
  type: string;
  cover_url: string;
  total_tracks: number;
  tracks: PreviewTrack[];
}

// Storage settings types
export interface StorageSettings {
  music_root: string;
}

// Create axios instance with a placeholder baseURL
// The actual URL will be set dynamically via interceptor
export const api = axios.create({
  baseURL: getApiUrl(), // Initial value from build-time env or default
  headers: {
    'Content-Type': 'application/json',
  },
});

// Track if we've initialized the runtime config
let configInitialized = false;
let configInitPromise: Promise<void> | null = null;

/**
 * Initialize the API client with runtime configuration
 * This fetches the actual API URL from /api/config for Docker deployments
 */
async function ensureConfigInitialized(): Promise<void> {
  if (configInitialized && isConfigLoaded()) {
    return;
  }

  if (configInitPromise) {
    return configInitPromise;
  }

  configInitPromise = getConfig().then((config) => {
    api.defaults.baseURL = config.apiUrl;
    configInitialized = true;
    configInitPromise = null;
  });

  return configInitPromise;
}

// Request interceptor: ensure config is loaded and set baseURL dynamically
api.interceptors.request.use(async (config: InternalAxiosRequestConfig) => {
  // Ensure runtime config is loaded
  await ensureConfigInitialized();
  
  // Update baseURL with the latest from runtime config
  // This handles cases where config was loaded after initial axios creation
  config.baseURL = getApiUrl();

  // Add auth token
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
  }
  
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      if (typeof window !== 'undefined') {
        localStorage.removeItem('token');
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);

export const authApi = {
  login: async (username: string, password: string) => {
    const response = await api.post('/api/v1/auth/login', { username, password });
    return response.data;
  },
  register: async (username: string, email: string, password: string) => {
    const response = await api.post('/api/v1/auth/register', { username, email, password });
    return response.data;
  },
  me: async () => {
    const response = await api.get('/api/v1/auth/me');
    return response.data;
  },
};

export const batchApi = {
  getAll: async () => {
    const response = await api.get('/api/v1/batches');
    return response.data;
  },
  getById: async (id: string) => {
    const response = await api.get(`/api/v1/batches/${id}`);
    return response.data;
  },
  create: async (data: { name: string; source: string }) => {
    const response = await api.post('/api/v1/batches', data);
    return response.data;
  },
  delete: async (id: string) => {
    const response = await api.delete(`/api/v1/batches/${id}`);
    return response.data;
  },
  retry: async (id: string) => {
    const response = await api.post(`/api/v1/batches/${id}/retry`);
    return response.data;
  },
};

export const navidromeApi = {
  getConfig: async () => {
    const response = await api.get('/api/v1/settings');
    return response.data;
  },
  saveConfig: async (config: { navidrome_url: string; navidrome_username: string; navidrome_password: string }) => {
    const response = await api.put('/api/v1/settings', config);
    return response.data;
  },
  testConnection: async (config?: { navidrome_url: string; navidrome_username: string; navidrome_password: string }) => {
    const response = await api.post('/api/v1/settings/test-navidrome', config || {});
    return response.data;
  },
};

// Scan types
export interface ScanResponse {
  message: string;
  scanning: boolean;
  count?: number;
}

// Scan API - trigger Navidrome library scan
export const scanApi = {
  triggerScan: async (): Promise<ScanResponse> => {
    const response = await api.post('/api/v1/scan');
    return response.data;
  },
};

// Playlist types
export interface PlaylistCreateRequest {
  name?: string;
}

export interface PlaylistCreateResponse {
  playlist_id: string;
  playlist_name: string;
  tracks_found: number;
  tracks_total: number;
  message: string;
}

// Playlist API - create Navidrome playlist from batch
export const playlistApi = {
  createFromBatch: async (batchId: string, name?: string): Promise<PlaylistCreateResponse> => {
    const response = await api.post(`/api/v1/batches/${batchId}/playlist`, name ? { name } : {});
    return response.data;
  },
};

export const jobsApi = {
  create: async (spotifyUrl: string) => {
    const response = await api.post('/api/v1/jobs', { spotify_url: spotifyUrl });
    return response.data;
  },
  list: async () => {
    const response = await api.get('/api/v1/jobs');
    return response.data;
  },
  getById: async (id: string) => {
    const response = await api.get(`/api/v1/jobs/${id}`);
    return response.data;
  },
  retry: async (id: string) => {
    const response = await api.post(`/api/v1/jobs/${id}/retry`);
    return response.data;
  },
  cancel: async (id: string) => {
    const response = await api.delete(`/api/v1/jobs/${id}`);
    return response.data;
  },
};

// Preview API - fetch track info before creating batch
export const previewApi = {
  preview: async (spotifyUrl: string) => {
    const response = await api.post('/api/v1/preview', { spotify_url: spotifyUrl });
    return response.data;
  },
};

// Storage settings API
export const storageApi = {
  getSettings: async () => {
    const response = await api.get('/api/v1/settings/storage');
    return response.data;
  },
  updateSettings: async (musicRoot: string) => {
    const response = await api.put('/api/v1/settings/storage', { music_root: musicRoot });
    return response.data;
  },
};

export const trackApi = {
  getByBatchId: async (batchId: string) => {
    const response = await api.get(`/api/v1/batches/${batchId}/tracks`);
    return response.data;
  },
};

// Source settings types
export interface TidalSourceSettings {
  configured: boolean;
  client_id: string;  // masked
  quality: string;
}

export interface QobuzSourceSettings {
  configured: boolean;
  app_id: string;  // masked
  quality: string;
}

export interface SourceSettingsResponse {
  tidal: TidalSourceSettings;
  qobuz: QobuzSourceSettings;
  preferred_source: string;
}

export interface SourceSettingsRequest {
  tidal?: {
    client_id?: string;
    client_secret?: string;
    quality?: string;
  };
  qobuz?: {
    app_id?: string;
    secret?: string;
    quality?: string;
  };
  preferred_source?: string;
}

// Source settings API
export const sourcesApi = {
  getSettings: async (): Promise<SourceSettingsResponse> => {
    const response = await api.get('/api/v1/settings/sources');
    return response.data;
  },
  updateSettings: async (data: SourceSettingsRequest): Promise<any> => {
    const response = await api.put('/api/v1/settings/sources', data);
    return response.data;
  },
  testTidal: async (): Promise<{ success: boolean; message: string }> => {
    const response = await api.post('/api/v1/settings/test-tidal');
    return response.data;
  },
  testQobuz: async (): Promise<{ success: boolean; message: string }> => {
    const response = await api.post('/api/v1/settings/test-qobuz');
    return response.data;
  },
};
