package main

import (
	"fmt"
	"log"

	"yt-music-sync/youtube_client"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/api/googleapi"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	fmt.Println("YOUTUBE MUSIC SYNC...")

	var allVideoIds []string

	client := youtubeclient.GetAuthClient()
	service, _ := youtube.New(client)
	if service == nil {
		log.Fatal("Failed to create YouTube service")
	}
	fmt.Println("Fetching liked music videos...")
	videos, err := youtubeclient.FetchLikedVideos(client)
	if err != nil {
		log.Fatalf("Error fetching videos: %v", err)
	}

	for _, v := range videos {
		allVideoIds = append(allVideoIds,v.Snippet.ResourceId.VideoId)
    }
		
	playlistID, err := youtubeclient.GetOrCreatePlaylist(service, "Sync-Music Playlist")
	if err != nil { log.Fatal(err) }
	fmt.Printf("Working with playlist ID: %s\n", playlistID)

	existingVideos, err := youtubeclient.GetAllPlaylistVideoIds(service, playlistID)
    if err != nil { log.Fatal(err) }

	for i := 0; i < len(allVideoIds); i += 50 {
		end := i + 50
		if end > len(allVideoIds) {
			end = len(allVideoIds)
		}
		batch := allVideoIds[i:end]

		call := service.Videos.List([]string{"snippet"}).Id(batch...)
		response, err := call.Do()
		if err != nil {
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
					if err.(*googleapi.Error).Code == 403 {
						fmt.Println("Limit reached for adding videos to the playlist. Stopping further additions.")
						return
					}
					fmt.Printf("Error while adding video: %v\n", err)
				}
			}
		}
	}

	if client != nil {
		fmt.Println("Success! We have successfully authenticated with Google.")
	}
}
