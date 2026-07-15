package youtubeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

func GetAuthClient() *http.Client {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("API keys not found. Please check the .env file")
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{youtube.YoutubeScope},
		RedirectURL: "http://localhost:8080",
	}

	tokenFile := "token.json"
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}

	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\n=== FIRST RUN ===\n")
	fmt.Printf("1. Go to this link in your browser:\n\n%v\n\n", authURL)
	fmt.Println("2. Sign in and click 'Continue'.")
	fmt.Println("3. Your browser will take you to an empty page (localhost:8080).")
	fmt.Println("4. Copy the code from the browser's address bar (after 'code=') and paste it below.")
	fmt.Print("\nPaste the code here and press Enter: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Failed to read code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving authorization token to file: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed to save token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func FetchLikedVideos(client *http.Client) ([]*youtube.PlaylistItem, error) {
	service, err := youtube.New(client)
	if err != nil {
		return nil, err
	}

	call := service.Channels.List([]string{"contentDetails"}).Mine(true)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	likedPlaylistId := response.Items[0].ContentDetails.RelatedPlaylists.Likes
	fmt.Printf("ID of playlist with likes: %s\n", likedPlaylistId)

	var videos []*youtube.PlaylistItem
	nextPageToken := ""

	for {
		playlistCall := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(likedPlaylistId).
			MaxResults(50).
			PageToken(nextPageToken)

		resp, err := playlistCall.Do()
		if err != nil {
			return nil, err
		}

		videos = append(videos, resp.Items...)

		nextPageToken = resp.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return videos, nil
}

func GetOrCreatePlaylist(service *youtube.Service, title string) (string, error) {
	resp, err := service.Playlists.List([]string{"snippet"}).Mine(true).Do()
    if err != nil { return "", err }

	for _, playlist := range resp.Items {
		if playlist.Snippet.Title == title {
			return playlist.Id, nil
		}
	}

	newPlaylist := &youtube.Playlist{
        Snippet: &youtube.PlaylistSnippet{Title: title},
        Status:  &youtube.PlaylistStatus{PrivacyStatus: "private"},
    }
    created, err := service.Playlists.Insert([]string{"snippet", "status"}, newPlaylist).Do()
    return created.Id, err
}

func GetAllPlaylistVideoIds (service *youtube.Service, playlistID string) (map[string]bool, error) {
	videos := make(map[string]bool)
	nextPageToken := ""
	
	for {
		call := service.PlaylistItems.List([]string{"snippet"}).PlaylistId(playlistID).MaxResults(50).PageToken(nextPageToken)
		response, err := call.Do()
		if err != nil {
			return nil, err
		}

		for _, item := range response.Items {
			videos[item.Snippet.ResourceId.VideoId] = true
		}

		nextPageToken = response.NextPageToken
		if nextPageToken == "" {
			break
		}
	}
	return videos, nil
}