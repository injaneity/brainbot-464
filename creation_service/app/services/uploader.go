package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"brainbot/creation_service/app"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Uploader struct {
	service *youtube.Service
}

const (
	envYouTubeClientID     = "YOUTUBE_CLIENT_ID"
	envYouTubeClientSecret = "YOUTUBE_CLIENT_SECRET"
	envYouTubeRefreshToken = "YOUTUBE_REFRESH_TOKEN"
	envYouTubeAccountSlot  = "YOUTUBE_ACCOUNT_SLOT"
)

type oauthCredentials struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

func loadOAuthCredentials() (oauthCredentials, error) {
	slot := strings.TrimSpace(os.Getenv(envYouTubeAccountSlot))
	if slot != "" {
		return loadSlotCredentials(slot)
	}
	return loadLegacyCredentials()
}

func loadSlotCredentials(slot string) (oauthCredentials, error) {
	if _, err := strconv.Atoi(slot); err != nil || slot == "0" {
		return oauthCredentials{}, fmt.Errorf("invalid %s %q: must be a positive integer", envYouTubeAccountSlot, slot)
	}
	suffix := fmt.Sprintf("_%s", slot)
	creds := oauthCredentials{
		ClientID:     os.Getenv(envYouTubeClientID + suffix),
		ClientSecret: os.Getenv(envYouTubeClientSecret + suffix),
		RefreshToken: os.Getenv(envYouTubeRefreshToken + suffix),
	}

	missing := missingEnvKeys(map[string]string{
		envYouTubeClientID + suffix:     creds.ClientID,
		envYouTubeClientSecret + suffix: creds.ClientSecret,
		envYouTubeRefreshToken + suffix: creds.RefreshToken,
	})
	if len(missing) > 0 {
		return oauthCredentials{}, fmt.Errorf("missing required env vars for slot %s: %v", slot, missing)
	}
	return creds, nil
}

func loadLegacyCredentials() (oauthCredentials, error) {
	creds := oauthCredentials{
		ClientID:     os.Getenv(envYouTubeClientID),
		ClientSecret: os.Getenv(envYouTubeClientSecret),
		RefreshToken: os.Getenv(envYouTubeRefreshToken),
	}
	missing := missingEnvKeys(map[string]string{
		envYouTubeClientID:     creds.ClientID,
		envYouTubeClientSecret: creds.ClientSecret,
		envYouTubeRefreshToken: creds.RefreshToken,
	})
	if len(missing) > 0 {
		return oauthCredentials{}, fmt.Errorf("missing required env vars: %v", missing)
	}
	return creds, nil
}

func missingEnvKeys(values map[string]string) []string {
	missing := make([]string, 0, len(values))
	for key, val := range values {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func NewUploader() (*Uploader, error) {
	creds, err := loadOAuthCredentials()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	conf := &oauth2.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{youtube.YoutubeUploadScope},
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	}

	token := &oauth2.Token{RefreshToken: creds.RefreshToken}
	client := conf.Client(ctx, token)

	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create YouTube service: %w", err)
	}

	return &Uploader{service: service}, nil
}

func (u *Uploader) UploadVideo(videoPath string, metadata app.VideoMetadata) (string, error) {
	file, err := os.Open(videoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open video file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat video file: %w", err)
	}

	log.Printf("📤 Uploading: %s (%.2f MB)", videoPath, float64(fileInfo.Size())/(1024*1024))

	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       metadata.Title,
			Description: metadata.Description,
			Tags:        metadata.Tags,
			CategoryId:  metadata.CategoryID,
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus:           "public",
			SelfDeclaredMadeForKids: false,
		},
	}

	call := u.service.Videos.Insert([]string{"snippet", "status"}, video)
	call = call.Media(file)

	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("failed to upload video: %w", err)
	}

	videoID := response.Id
	log.Printf("✅ Uploaded! https://youtube.com/shorts/%s", videoID)

	return videoID, nil
}

func GenerateMetadata(input app.VideoInput, articleTitle string, sourceURL string) app.VideoMetadata {
	title := articleTitle
	if len(title) > 100 {
		title = title[:97] + "..."
	}

	description := fmt.Sprintf(
		"🔗 Source: %s\n\n"+
			"📱 Follow for daily tech updates!\n"+
			"#tech #ai #technology #shorts",
		sourceURL,
	)

	tags := []string{
		"tech news",
		"AI",
		"technology",
		"artificial intelligence",
		"tech shorts",
		"daily tech",
	}

	return app.VideoMetadata{
		Title:       title,
		Description: description,
		Tags:        tags,
		CategoryID:  "28",
	}
}
