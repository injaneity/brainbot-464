package services

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/config"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func CreateVideo(input app.VideoInput, backgroundVideoPath string, outputPath string) error {
	tmpDir := os.TempDir()
	audioPath := filepath.Join(tmpDir, fmt.Sprintf("%s_audio.mp3", input.UUID))
	if err := downloadFile(input.Voiceover, audioPath); err != nil {
		return fmt.Errorf("failed to download audio: %w", err)
	}
	defer os.Remove(audioPath)

	srtPath := filepath.Join(tmpDir, fmt.Sprintf("%s_subtitles.srt", input.UUID))
	if err := generateSRT(input.SubtitleTimestamps, srtPath); err != nil {
		return fmt.Errorf("failed to generate SRT: %w", err)
	}
	defer os.Remove(srtPath)

	// Calculate duration: last subtitle end time + padding
	duration := input.SubtitleTimestamps[len(input.SubtitleTimestamps)-1].End + config.VideoEndPadding

	// Enforce maximum video duration (3 minutes)
	duration = math.Min(duration, config.MaxVideoDuration)

	// Build FFmpeg command: overlay subtitles on video, then merge with audio
	video := ffmpeg.Input(backgroundVideoPath, ffmpeg.KwArgs{"t": fmt.Sprintf("%.2f", duration)})
	audio := ffmpeg.Input(audioPath)

	// Convert Windows path to format FFmpeg expects (forward slashes, escape colons)
	srtPathForFFmpeg := filepath.ToSlash(srtPath)
	srtPathForFFmpeg = strings.ReplaceAll(srtPathForFFmpeg, ":", "\\:")

	videoWithSubs := ffmpeg.Filter(
		[]*ffmpeg.Stream{video}, "subtitles", ffmpeg.Args{srtPathForFFmpeg},
		ffmpeg.KwArgs{"force_style": "FontName=Consolas,FontSize=32,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,BackColour=&H00000000,BorderStyle=1,Outline=2,Shadow=0,Alignment=2,Bold=1"},
	)

	err := ffmpeg.Output([]*ffmpeg.Stream{videoWithSubs, audio}, outputPath, ffmpeg.KwArgs{
		"c:v":      config.VideoCodec,
		"c:a":      config.AudioCodec,
		"b:a":      config.AudioBitrate,
		"preset":   config.VideoPreset,
		"s":        fmt.Sprintf("%dx%d", config.VideoWidth, config.VideoHeight),
		"shortest": "",
	}).OverWriteOutput().Run()

	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	return nil
}

func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: status %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func generateSRT(timestamps []app.SubtitleTimestamp, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for i, ts := range timestamps {
		fmt.Fprintf(file, "%d\n", i+1)
		fmt.Fprintf(file, "%s --> %s\n",
			formatTimestamp(ts.Start),
			formatTimestamp(ts.End))
		fmt.Fprintf(file, "%s\n\n", ts.Text)
	}

	return nil
}

func formatTimestamp(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}
