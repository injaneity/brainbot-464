package services

import (
	"context"
	"fmt"
	"log"
	"os"

	"brainbot/creation_service/app"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Uploader struct {
	service *youtube.Service
}

func NewUploader(serviceAccountFile string) (*Uploader, error) {
	ctx := context.Background()

	data, err := os.ReadFile(serviceAccountFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read service account file: %w", err)
	}

	config, err := google.JWTConfigFromJSON(data, youtube.YoutubeUploadScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse service account: %w", err)
	}

	client := config.Client(ctx)

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
		"%s\n\n"+
			"🔗 Source: %s\n\n"+
			"📱 Follow for daily tech updates!\n"+
			"#tech #ai #technology #shorts",
		articleTitle,
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
