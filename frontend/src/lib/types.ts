export interface User {
  id: string;
  username: string;
  email: string;
  navidrome_url: string | null;
  navidrome_username: string | null;
  created_at: string;
}

export interface AuthResponse {
  user: User;
  token: string;
}

export interface LoginCredentials {
  username: string;
  password: string;
}

export interface RegisterData {
  email: string;
  username: string;
  password: string;
}

export interface Batch {
  id: string;
  name: string;
  status: BatchStatus;
  spotify_url: string;
  spotify_type: SpotifyType;
  total_jobs: number;
  completed_jobs: number;
  failed_jobs: number;
  created_at: string;
  updated_at: string;
  jobs?: Job[];
  playlist_status?: PlaylistStatus;
  playlist_id?: string;
  playlist_message?: string;
  tracks_found?: number;
  tracks_failed?: number;
}

export type BatchStatus = 'pending' | 'processing' | 'completed' | 'failed';
export type SpotifyType = 'album' | 'playlist' | 'track' | 'artist';
export type PlaylistStatus = 'pending' | 'creating' | 'completed' | 'failed';

export interface Track {
  id: string;
  title: string;
  artist: string;
  album: string;
  duration: number;
  status: TrackStatus;
  batch_id: string;
  source_track_id?: string;
  error?: string;
  created_at: string;
}

export type TrackStatus = 'pending' | 'processing' | 'downloaded' | 'failed';

export interface NavidromeConfig {
  server_url: string;
  username: string;
  password: string;
}

export interface Job {
  id: string;
  batch_id: string;
  spotify_track_id?: string;
  isrc?: string;
  track_name?: string;
  artist_name?: string;
  album_name?: string;
  album_artist?: string;
  track_number?: number;
  disc_number?: number;
  total_tracks?: number;
  duration_ms?: number;
  release_year?: number;
  status: JobStatus;
  source_service?: string;
  local_path?: string;
  cover_path?: string;
  lyrics_path?: string;
  file_size?: number;
  retry_count?: number;
  error_message?: string;
  progress: number;
  song_status?: 'pending' | 'downloading' | 'completed' | 'failed';
  lyrics_status?: 'pending' | 'fetching' | 'completed' | 'failed' | 'not_found';
  cover_status?: 'pending' | 'fetching' | 'completed' | 'failed' | 'not_found';
  download_speed?: number;
  bytes_downloaded?: number;
  bytes_total?: number;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  in_playlist?: boolean;
  // Legacy field mappings for backwards compatibility
  type?: 'sync' | 'download';
  title?: string;
  artist?: string;
  album?: string;
  source_track_id?: string;
  error?: string;
}

export type JobStatus = 'queued' | 'running' | 'completed' | 'failed';

export interface WebSocketPayload {
  job_id?: string;
  id?: string;
  batch_id?: string;
  progress?: number;
  status?: string;
  song_status?: 'pending' | 'downloading' | 'completed' | 'failed';
  lyrics_status?: 'pending' | 'fetching' | 'completed' | 'failed' | 'not_found';
  cover_status?: 'pending' | 'fetching' | 'completed' | 'failed' | 'not_found';
  error_message?: string;
  [key: string]: unknown;
}

export interface PlaylistUpdatePayload {
  batch_id: string;
  status: PlaylistStatus;
  playlist_id?: string;
  message?: string;
  tracks_found?: number;
  tracks_failed?: number;
  total_tracks?: number;
  timestamp?: string;
}

export interface PlaylistProgressPayload {
  batch_id: string;
  operation: 'check' | 'sync';
  progress: number; // 0-100
  message: string;
  current_track: number;
  total_tracks: number;
  timestamp: string;
}

export interface WebSocketMessage {
  type: 'job_update' | 'batch_update' | 'playlist_update' | 'playlist_progress' | 'error';
  payload: WebSocketPayload | PlaylistUpdatePayload | PlaylistProgressPayload;
}
