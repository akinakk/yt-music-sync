# yt-music-sync

`yt-music-sync` is a Go script for syncing music videos from your YouTube liked videos into a separate private playlist.

The script reads your liked videos playlist, keeps only videos from the YouTube music category `CategoryId == "10"` (`Music`), checks which videos already exist in the target playlist, and adds only missing tracks.

By default, the target playlist is:

```text
Sync-Music Playlist
```

If this playlist does not exist yet, the script creates it as a private playlist.

## What The Project Does

1. Authenticates with Google through OAuth2.
2. Finds the YouTube system playlist that contains your liked videos.
3. Reads all liked videos across all result pages.
4. Collects the video IDs.
5. Gets or creates the target playlist `Sync-Music Playlist`.
6. Loads all existing videos from the target playlist.
7. Fetches video details in batches of 50 IDs.
8. Adds only videos where `CategoryId == "10"`.
9. Skips duplicates and exits gracefully when the API quota is exhausted.

## Architecture And Optimizations

### Music-Only Filtering

The script does not add every liked video to the playlist. Before inserting a video, it requests the `snippet` data through `Videos.List` and checks:

```go
item.Snippet.CategoryId == "10"
```

`CategoryId == "10"` is the YouTube category for `Music`. This ensures that `Sync-Music Playlist` contains only music videos.

### Map Caching For Quota Optimization

Before adding new tracks, the script loads the full contents of the target playlist and stores all existing video IDs in:

```go
map[string]bool
```

Duplicate checks are then performed in memory in `O(1)` time:

```go
if existingVideos[item.Id] {
    // video is already in the playlist
}
```

This is important for the YouTube Data API because the script does not need to make a separate API request for every video just to check whether it already exists in the playlist.

### Pagination With NextPageToken

The YouTube API returns data page by page. This project implements full pagination with `NextPageToken`:

- when fetching liked videos;
- when loading all videos from the target playlist.

This means the script is not limited to the first page of results and works correctly with large playlists.

### Batch Processing In Groups Of 50

The `Videos.List` method accepts a limited number of IDs per request. The script processes videos in strict batches of 50 IDs:

```go
for i := 0; i < len(allVideoIds); i += 50 {
    end := i + 50
    if end > len(allVideoIds) {
        end = len(allVideoIds)
    }
    batch := allVideoIds[i:end]
}
```

This matches the API limit and reduces the number of requests compared with fetching video details one by one.

### Graceful Degradation When API Quota Is Exhausted

If the YouTube Data API returns `403` while inserting a video, for example because of `Quota Exceeded`, the script does not crash with an unhandled error. It prints a message and exits cleanly:

```text
Limit reached for adding videos to the playlist. Stopping further additions.
```

Videos that were already added stay in the playlist. On the next run, the script loads the saved progress from the playlist itself and skips existing tracks through `map[string]bool`.

## Requirements

The instructions below are for macOS only.

You need:

- macOS;
- Homebrew;
- a Go version compatible with the project;
- a Google Cloud project with YouTube Data API v3 enabled;
- OAuth2 Client ID and Client Secret.

The Go version is defined in [go.mod](go.mod):

```text
go 1.26.1
```

## Installation On macOS

### 1. Install Homebrew

If Homebrew is already installed, skip this step.

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

### 2. Install Go

```bash
brew install go
```

Check the installation:

```bash
go version
```

### 3. Open The Project Directory

```bash
cd yt-music-sync
```

### 4. Download Dependencies

```bash
go mod download
```

## Google Cloud Setup

### 1. Create A Project

1. Open Google Cloud Console.
2. Create a new project or select an existing one.
3. Open `APIs & Services`.

### 2. Enable YouTube Data API v3

1. Go to `APIs & Services` -> `Library`.
2. Search for `YouTube Data API v3`.
3. Click `Enable`.

### 3. Configure OAuth Consent Screen

1. Open `APIs & Services` -> `OAuth consent screen`.
2. Choose the application type.
3. Fill in the required fields.
4. If the app is in testing mode, add your Google account to the test users list.

### 4. Create OAuth2 Credentials

1. Open `APIs & Services` -> `Credentials`.
2. Click `Create Credentials` -> `OAuth client ID`.
3. Choose `Web application` as the application type.
4. Add this authorized redirect URI:

```text
http://localhost:8080
```

5. Copy the `Client ID` and `Client Secret`.

## .env Setup

Create `.env` from the example file:

```bash
cp .env.example .env
```

Open the file:

```bash
nano .env
```

Fill in the values:

```env
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
```

Save the file:

- `Control + O`, then `Enter`;
- `Control + X` to exit `nano`.

The `.env` file contains secrets and should not be committed to a public repository.

## First Run And OAuth2 Authorization

Run the script:

```bash
go run main.go
```

On the first run, `token.json` does not exist yet, so the script will:

1. Generate an OAuth2 authorization URL.
2. Print it in the terminal.
3. Ask you to open the URL in a browser.
4. Ask you to sign in to your Google account and approve access.
5. Redirect the browser to `http://localhost:8080`.
6. Ask you to copy the `code` parameter from the browser address bar.
7. Exchange that code for an OAuth2 token.
8. Save the token to `token.json`.

After that, future runs will use the saved `token.json`, so you usually will not need to authorize in the browser again.

`token.json` grants access to the YouTube API on behalf of your account. Do not publish this file.

## Running

Normal run:

```bash
go run main.go
```

During execution, the script prints status messages such as:

- sync start;
- liked playlist ID;
- target playlist ID;
- which videos are being added;
- which videos are already in the playlist and skipped;
- quota limit message if the daily API limit is reached.

## Behavior On Repeated Runs

You can run the script multiple times. Every run loads the current contents of `Sync-Music Playlist` into `map[string]bool`, so previously added videos are detected and skipped.

This makes repeated runs safe: the playlist should not be filled with duplicates.

## Project Structure

```text
.
|-- .env.example
|-- go.mod
|-- go.sum
|-- main.go
`-- youtube_client
    `-- youtube_client.go
```

Main files:

- `main.go` - entry point, sync orchestration, batching, music filtering, and playlist insertion;
- `youtube_client/youtube_client.go` - OAuth2 authorization, `token.json` handling, liked videos fetching, playlist creation, and existing playlist video loading;
- `.env.example` - example environment variables for Google OAuth2.

## Environment Variables

| Variable | Purpose |
| --- | --- |
| `GOOGLE_CLIENT_ID` | OAuth2 Client ID from Google Cloud |
| `GOOGLE_CLIENT_SECRET` | OAuth2 Client Secret from Google Cloud |

## Important Notes

- The script uses `youtube.YoutubeScope` because it needs to read YouTube data and add videos to a playlist.
- The target playlist is created as private.
- Only videos from the `Music` category are added.
- Duplicate checks are performed locally through `map[string]bool`.
- YouTube API has daily quotas. If the quota is exhausted, wait for the quota reset and run the script again.
