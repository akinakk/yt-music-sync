package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
	"yt-music-sync/youtube_client"

	"github.com/joho/godotenv"
)

func isQuotaExceeded(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}

	if apiErr.Code != 403 {
		return false
	}

	for _, item := range apiErr.Errors {
		if item.Reason == "quotaExceeded" || item.Reason == "dailyLimitExceeded" {
			return true
		}
	}

	message := strings.ToLower(apiErr.Message)
	body := strings.ToLower(apiErr.Body)
	return strings.Contains(message, "quota") || strings.Contains(body, "quota")
}

func printQuotaExceeded(stage string) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "YouTube API quota is exhausted.")
	fmt.Fprintf(os.Stderr, "Stopped while %s.\n", stage)
	fmt.Fprintln(os.Stderr, "Run the script again after the daily YouTube API quota resets.")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	fmt.Println("YOUTUBE MUSIC SYNC...")

	var allVideoIds []string

	client := youtubeclient.GetAuthClient()
	if client != nil {
		fmt.Println("Success! We have successfully authenticated with Google.")
	}
	service, err := youtube.New(client)
	if err != nil {
		log.Fatalf("Failed to create YouTube service: %v", err)
	}
	fmt.Println("Fetching liked music videos...")
	videos, err := youtubeclient.FetchLikedVideos(client)
	if err != nil {
		if isQuotaExceeded(err) {
			printQuotaExceeded("fetching liked videos")
			return
		}
		log.Fatalf("Error fetching liked videos: %v", err)
	}

	for _, v := range videos {
		allVideoIds = append(allVideoIds, v.Snippet.ResourceId.VideoId)
	}

	playlistID, err := youtubeclient.GetOrCreatePlaylist(service, "Sync-Music Playlist")
	if err != nil {
		if isQuotaExceeded(err) {
			printQuotaExceeded("getting or creating the target playlist")
			return
		}
		log.Fatalf("Error getting or creating playlist: %v", err)
	}
	fmt.Printf("Working with playlist ID: %s\n", playlistID)

	existingVideos, err := youtubeclient.GetAllPlaylistVideoIds(service, playlistID)
	if err != nil {
		if isQuotaExceeded(err) {
			printQuotaExceeded("loading existing playlist videos")
			return
		}
		log.Fatalf("Error loading existing playlist videos: %v", err)
	}

	for i := 0; i < len(allVideoIds); i += 50 {
		end := i + 50
		if end > len(allVideoIds) {
			end = len(allVideoIds)
		}
		batch := allVideoIds[i:end]

		call := service.Videos.List([]string{"snippet"}).Id(batch...)
		response, err := call.Do()
		if err != nil {
			if isQuotaExceeded(err) {
				printQuotaExceeded("fetching video details")
				return
			}
			log.Fatalf("Error fetching video details: %v", err)
		}

		for _, item := range response.Items {
			if item.Snippet.CategoryId == "10" {
				if item.Snippet.Title == "Music" {
					fmt.Printf("Skip: %s\n", item.Snippet.Title)
					continue
				}

				playlistItem := &youtube.PlaylistItem{
					Snippet: &youtube.PlaylistItemSnippet{
						PlaylistId: playlistID,
						ResourceId: &youtube.ResourceId{
							Kind:    "youtube#video",
							VideoId: item.Id,
						},
					},
				}

				if existingVideos[item.Id] {
					fmt.Printf("Already in playlist: %s. Skipping.\n", item.Snippet.Title)
					continue
				}

				fmt.Printf("Adding: %s\n", item.Snippet.Title)

				_, err = service.PlaylistItems.Insert([]string{"snippet"}, playlistItem).Do()

				if err != nil {
					if isQuotaExceeded(err) {
						printQuotaExceeded("adding videos to the playlist")
						return
					}
					fmt.Printf("Error while adding video: %v\n", err)
				}
			}
		}
	}
}
