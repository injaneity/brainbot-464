package video

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    
    ffmpeg "github.com/u2takey/ffmpeg-go"
)

func CreateVideo(input VideoInput, backgroundVideoPath string, outputPath string) error {
    audioPath := fmt.Sprintf("/tmp/%s_audio.mp3", input.UUID)
    if err := downloadFile(input.Voiceover, audioPath); err != nil {
        return fmt.Errorf("failed to download audio: %w", err)
    }
    defer os.Remove(audioPath)
    
    srtPath := fmt.Sprintf("/tmp/%s_subtitles.srt", input.UUID)
    if err := generateSRT(input.SubtitleTimestamps, srtPath); err != nil {
        return fmt.Errorf("failed to generate SRT: %w", err)
    }
    defer os.Remove(srtPath)
    
    duration := input.SubtitleTimestamps[len(input.SubtitleTimestamps)-1].End
    
    err := ffmpeg.Input(backgroundVideoPath).
        Output(outputPath, ffmpeg.KwArgs{
            "i":      audioPath,
            "t":      fmt.Sprintf("%.2f", duration),
            "vf":     generateSubtitleFilter(srtPath),
            "map":    "0:v:0",
            "map":    "1:a:0",
            "c:v":    "libx264",
            "c:a":    "aac",
            "b:a":    "192k",
            "preset": "fast",
            "s":      "720x1280",
            "aspect": "9:16",
        }).
        OverWriteOutput().
        Run()
    
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

func generateSRT(timestamps []SubtitleTimestamp, outputPath string) error {
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

func generateSubtitleFilter(srtPath string) string {
    style := "FontName=Impact," +
        "FontSize=32," +
        "PrimaryColour=&H00FFFF," +
        "OutlineColour=&H000000," +
        "BorderStyle=3," +
        "Outline=3," +
        "Shadow=0," +
        "Alignment=2," +
        "Bold=1"
    
    escapedPath := filepath.ToSlash(srtPath)
    return fmt.Sprintf("subtitles='%s':force_style='%s'", escapedPath, style)
}