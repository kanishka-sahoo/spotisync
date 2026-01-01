package navidrome

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:4533", "admin", "password")

	assert.Equal(t, "http://localhost:4533", client.baseURL)
	assert.Equal(t, "admin", client.username)
	assert.Equal(t, "password", client.password)
	assert.NotNil(t, client.httpClient)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	client := NewClient("http://localhost:4533/", "admin", "password")
	assert.Equal(t, "http://localhost:4533", client.baseURL)
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := generateSalt()
	require.NoError(t, err)
	assert.Len(t, salt1, 32) // 16 bytes = 32 hex chars

	salt2, err := generateSalt()
	require.NoError(t, err)
	assert.NotEqual(t, salt1, salt2, "salts should be unique")
}

func TestGenerateToken(t *testing.T) {
	token := generateToken("password", "salt123")
	assert.Len(t, token, 32) // MD5 = 32 hex chars

	// Same inputs should produce same output
	token2 := generateToken("password", "salt123")
	assert.Equal(t, token, token2)

	// Different inputs should produce different output
	token3 := generateToken("password", "salt456")
	assert.NotEqual(t, token, token3)
}

func TestClient_BuildAuthParams(t *testing.T) {
	client := NewClient("http://localhost:4533", "testuser", "testpass")

	params, err := client.buildAuthParams()
	require.NoError(t, err)

	assert.Equal(t, "testuser", params.Get("u"))
	assert.NotEmpty(t, params.Get("t")) // token
	assert.NotEmpty(t, params.Get("s")) // salt
	assert.Equal(t, SubsonicAPIVersion, params.Get("v"))
	assert.Equal(t, SubsonicClientName, params.Get("c"))
	assert.Equal(t, "json", params.Get("f"))
}

func TestClient_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/ping.view")
		assert.Equal(t, "testuser", r.URL.Query().Get("u"))
		assert.NotEmpty(t, r.URL.Query().Get("t"))
		assert.NotEmpty(t, r.URL.Query().Get("s"))

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.Version = "1.16.1"

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	err := client.Ping(context.Background())
	require.NoError(t, err)
}

func TestClient_Ping_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "failed"
		resp.SubsonicResponse.Error = &SubsonicError{
			Code:    40,
			Message: "Wrong username or password",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "wrongpass")
	err := client.Ping(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Wrong username or password")
}

func TestClient_StartScan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/startScan.view")

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.ScanStatus = &ScanStatus{
			Scanning: true,
			Count:    0,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	err := client.StartScan(context.Background())
	require.NoError(t, err)
}

func TestClient_GetScanStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/getScanStatus.view")

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.ScanStatus = &ScanStatus{
			Scanning: true,
			Count:    1234,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	status, err := client.GetScanStatus(context.Background())
	require.NoError(t, err)
	assert.True(t, status.Scanning)
	assert.Equal(t, int64(1234), status.Count)
}

func TestClient_CreatePlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/createPlaylist.view")
		assert.Equal(t, "My Playlist", r.URL.Query().Get("name"))

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.Playlist = &PlaylistDetail{
			Playlist: Playlist{
				ID:        "pl-123",
				Name:      "My Playlist",
				SongCount: 0,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	playlistID, err := client.CreatePlaylist(context.Background(), "My Playlist")
	require.NoError(t, err)
	assert.Equal(t, "pl-123", playlistID)
}

func TestClient_UpdatePlaylistTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/createPlaylist.view")
		assert.Equal(t, "pl-123", r.URL.Query().Get("playlistId"))

		songIDs := r.URL.Query()["songId"]
		assert.Len(t, songIDs, 3)
		assert.Contains(t, songIDs, "song-1")
		assert.Contains(t, songIDs, "song-2")
		assert.Contains(t, songIDs, "song-3")

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	err := client.UpdatePlaylistTracks(context.Background(), "pl-123", []string{"song-1", "song-2", "song-3"})
	require.NoError(t, err)
}

func TestClient_UpdatePlaylistTracks_EmptySongs(t *testing.T) {
	client := NewClient("http://localhost:4533", "testuser", "testpass")
	err := client.UpdatePlaylistTracks(context.Background(), "pl-123", []string{})
	require.NoError(t, err) // Should return early without making request
}

func TestClient_SearchTrackByTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/search3.view")
		query := r.URL.Query().Get("query")
		assert.Contains(t, query, "Metallica")
		assert.Contains(t, query, "Enter Sandman")

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.SearchResult3 = &SearchResult3{
			Song: []Track{
				{
					ID:     "track-1",
					Title:  "Enter Sandman",
					Artist: "Metallica",
					Album:  "Metallica",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	track, err := client.SearchTrackByTitle(context.Background(), "Enter Sandman", "Metallica")
	require.NoError(t, err)
	assert.Equal(t, "track-1", track.ID)
	assert.Equal(t, "Enter Sandman", track.Title)
	assert.Equal(t, "Metallica", track.Artist)
}

func TestClient_SearchTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/search3.view")
		assert.Equal(t, "20", r.URL.Query().Get("songCount"))

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.SearchResult3 = &SearchResult3{
			Song: []Track{
				{ID: "track-1", Title: "Song 1"},
				{ID: "track-2", Title: "Song 2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	tracks, err := client.SearchTracks(context.Background(), "test", 20)
	require.NoError(t, err)
	assert.Len(t, tracks, 2)
}

func TestClient_GetPlaylists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/getPlaylists.view")

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.Playlists = &PlaylistsData{
			Playlist: []Playlist{
				{ID: "pl-1", Name: "Playlist 1", SongCount: 10},
				{ID: "pl-2", Name: "Playlist 2", SongCount: 20},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	playlists, err := client.GetPlaylists(context.Background())
	require.NoError(t, err)
	assert.Len(t, playlists, 2)
	assert.Equal(t, "pl-1", playlists[0].ID)
	assert.Equal(t, "Playlist 1", playlists[0].Name)
}

func TestClient_GetPlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/getPlaylist.view")
		assert.Equal(t, "pl-123", r.URL.Query().Get("id"))

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.Playlist = &PlaylistDetail{
			Playlist: Playlist{
				ID:        "pl-123",
				Name:      "My Playlist",
				SongCount: 2,
			},
			Entry: []Track{
				{ID: "track-1", Title: "Song 1"},
				{ID: "track-2", Title: "Song 2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	playlist, err := client.GetPlaylist(context.Background(), "pl-123")
	require.NoError(t, err)
	assert.Equal(t, "pl-123", playlist.ID)
	assert.Equal(t, "My Playlist", playlist.Name)
	assert.Len(t, playlist.Entry, 2)
}

func TestClient_DeletePlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rest/deletePlaylist.view")
		assert.Equal(t, "pl-123", r.URL.Query().Get("id"))

		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	err := client.DeletePlaylist(context.Background(), "pl-123")
	require.NoError(t, err)
}

func TestClient_FindPlaylistByName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"
		resp.SubsonicResponse.Playlists = &PlaylistsData{
			Playlist: []Playlist{
				{ID: "pl-1", Name: "Rock Favorites", SongCount: 10},
				{ID: "pl-2", Name: "Jazz Classics", SongCount: 20},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")

	// Test finding existing playlist (case insensitive)
	playlist, err := client.FindPlaylistByName(context.Background(), "rock favorites")
	require.NoError(t, err)
	require.NotNil(t, playlist)
	assert.Equal(t, "pl-1", playlist.ID)

	// Test playlist not found
	playlist, err = client.FindPlaylistByName(context.Background(), "Pop Hits")
	require.NoError(t, err)
	assert.Nil(t, playlist)
}

func TestClient_CreateOrUpdatePlaylist_New(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := SubsonicResponse{}
		resp.SubsonicResponse.Status = "ok"

		if r.URL.Path == "/rest/getPlaylists.view" {
			resp.SubsonicResponse.Playlists = &PlaylistsData{
				Playlist: []Playlist{}, // Empty, no existing playlists
			}
		} else if r.URL.Path == "/rest/createPlaylist.view" {
			resp.SubsonicResponse.Playlist = &PlaylistDetail{
				Playlist: Playlist{
					ID:   "new-pl-id",
					Name: "New Playlist",
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	playlistID, err := client.CreateOrUpdatePlaylist(context.Background(), "New Playlist", []string{"song-1", "song-2"})
	require.NoError(t, err)
	assert.Equal(t, "new-pl-id", playlistID)
}

func TestSubsonicError_Error(t *testing.T) {
	err := &SubsonicError{
		Code:    40,
		Message: "Wrong username or password",
	}
	assert.Equal(t, "subsonic error 40: Wrong username or password", err.Error())
}

func TestTrack_Struct(t *testing.T) {
	track := Track{
		ID:          "track-123",
		Title:       "Enter Sandman",
		Album:       "Metallica",
		Artist:      "Metallica",
		AlbumArtist: "Metallica",
		TrackNumber: 1,
		DiscNumber:  1,
		Year:        1991,
		Duration:    331,
		BitRate:     1411,
		Size:        58000000,
		Path:        "/music/Metallica/Metallica/01 - Enter Sandman.flac",
		ISRC:        "USEE1000233",
		CoverArtID:  "al-123",
	}

	assert.Equal(t, "track-123", track.ID)
	assert.Equal(t, "Enter Sandman", track.Title)
	assert.Equal(t, "Metallica", track.Artist)
	assert.Equal(t, 1991, track.Year)
}

func TestPlaylist_Struct(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	playlist := Playlist{
		ID:        "pl-123",
		Name:      "My Favorites",
		SongCount: 50,
		Duration:  12000,
		Created:   created,
		Changed:   created,
		Owner:     "admin",
		Public:    false,
	}

	assert.Equal(t, "pl-123", playlist.ID)
	assert.Equal(t, "My Favorites", playlist.Name)
	assert.Equal(t, 50, playlist.SongCount)
	assert.Equal(t, "admin", playlist.Owner)
}

func TestFlexibleISRC_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected FlexibleISRC
	}{
		{
			name:     "string value",
			json:     `"USEE1000233"`,
			expected: FlexibleISRC("USEE1000233"),
		},
		{
			name:     "array with single value",
			json:     `["USEE1000233"]`,
			expected: FlexibleISRC("USEE1000233"),
		},
		{
			name:     "array with multiple values uses first",
			json:     `["USEE1000233", "USEE1000234", "USEE1000235"]`,
			expected: FlexibleISRC("USEE1000233"),
		},
		{
			name:     "empty array",
			json:     `[]`,
			expected: FlexibleISRC(""),
		},
		{
			name:     "empty string",
			json:     `""`,
			expected: FlexibleISRC(""),
		},
		{
			name:     "null value",
			json:     `null`,
			expected: FlexibleISRC(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var isrc FlexibleISRC
			err := json.Unmarshal([]byte(tt.json), &isrc)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, isrc)
		})
	}
}

func TestTrack_UnmarshalJSON_WithISRCArray(t *testing.T) {
	// Test unmarshaling a track with ISRC as an array (as Navidrome returns)
	jsonData := `{
		"id": "track-123",
		"title": "Enter Sandman",
		"album": "Metallica",
		"artist": "Metallica",
		"isrc": ["USEE1000233", "USEE1000234"]
	}`

	var track Track
	err := json.Unmarshal([]byte(jsonData), &track)
	require.NoError(t, err)
	assert.Equal(t, "track-123", track.ID)
	assert.Equal(t, "Enter Sandman", track.Title)
	assert.Equal(t, FlexibleISRC("USEE1000233"), track.ISRC)
}

func TestTrack_UnmarshalJSON_WithISRCString(t *testing.T) {
	// Test unmarshaling a track with ISRC as a string
	jsonData := `{
		"id": "track-123",
		"title": "Enter Sandman",
		"album": "Metallica",
		"artist": "Metallica",
		"isrc": "USEE1000233"
	}`

	var track Track
	err := json.Unmarshal([]byte(jsonData), &track)
	require.NoError(t, err)
	assert.Equal(t, "track-123", track.ID)
	assert.Equal(t, "Enter Sandman", track.Title)
	assert.Equal(t, FlexibleISRC("USEE1000233"), track.ISRC)
}
