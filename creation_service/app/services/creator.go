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

	assPath := filepath.Join(tmpDir, fmt.Sprintf("%s_subtitles.ass", input.UUID))
	if err := generateASS(input.SubtitleTimestamps, assPath); err != nil {
		return fmt.Errorf("failed to generate ASS: %w", err)
	}
	defer os.Remove(assPath)

	// Calculate duration: last subtitle end time + padding
	duration := input.SubtitleTimestamps[len(input.SubtitleTimestamps)-1].End + config.VideoEndPadding

	// Enforce maximum video duration (3 minutes)
	duration = math.Min(duration, config.MaxVideoDuration)

	// Build FFmpeg command: overlay subtitles on video, then merge with audio
	video := ffmpeg.Input(backgroundVideoPath, ffmpeg.KwArgs{"t": fmt.Sprintf("%.2f", duration)})
	audio := ffmpeg.Input(audioPath)

	// Crop and scale video to 9:16 vertical format (center crop for horizontal videos)
	// This ensures the output is always vertical, even if background is horizontal
	videoCropped := ffmpeg.Filter(
		[]*ffmpeg.Stream{video}, 
		"crop", 
		ffmpeg.Args{fmt.Sprintf("ih*9/16:ih")}, // Crop to 9:16 aspect ratio from center
	).Filter(
		"scale", 
		ffmpeg.Args{fmt.Sprintf("%d:%d", config.VideoWidth, config.VideoHeight)}, // Scale to target size
	)

	// Convert Windows path to format FFmpeg expects (forward slashes, escape colons)
	assPathForFFmpeg := filepath.ToSlash(assPath)
	assPathForFFmpeg = strings.ReplaceAll(assPathForFFmpeg, ":", "\\:")

	videoWithSubs := ffmpeg.Filter(
		[]*ffmpeg.Stream{videoCropped}, "ass", ffmpeg.Args{assPathForFFmpeg},
	)

	err := ffmpeg.Output([]*ffmpeg.Stream{videoWithSubs, audio}, outputPath, ffmpeg.KwArgs{
		"c:v":      config.VideoCodec,
		"c:a":      config.AudioCodec,
		"b:a":      config.AudioBitrate,
		"preset":   config.VideoPreset,
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

// Sentence represents a group of words that form a sentence
type Sentence struct {
	Words []app.SubtitleTimestamp
	Start float64
	End   float64
}

// groupIntoSentences batches words into sentences based on punctuation
func groupIntoSentences(timestamps []app.SubtitleTimestamp, maxWordsPerLine int) []Sentence {
	sentences := []Sentence{}
	currentSentence := Sentence{Words: []app.SubtitleTimestamp{}}
	
	for i, ts := range timestamps {
		currentSentence.Words = append(currentSentence.Words, ts)
		
		if currentSentence.Start == 0 {
			currentSentence.Start = ts.Start
		}
		currentSentence.End = ts.End
		
		// Check if this word ends with a period (sentence end)
		// Ignore periods that are part of numbers like "4.5"
		endsWithPeriod := false
		trimmed := strings.TrimSpace(ts.Text)
		if strings.HasSuffix(trimmed, ".") || strings.HasSuffix(trimmed, "!") || strings.HasSuffix(trimmed, "?") {
			// Check if it's not a number like "4.5" or "2.3.4"
			if strings.HasSuffix(trimmed, ".") && len(trimmed) > 2 {
				secondToLast := trimmed[len(trimmed)-2]
				thirdToLast := trimmed[len(trimmed)-3]
				// If both sides of the period are digits, it's part of a number
				if secondToLast >= '0' && secondToLast <= '9' && thirdToLast >= '0' && thirdToLast <= '9' {
					endsWithPeriod = false
				} else if secondToLast < '0' || secondToLast > '9' {
					endsWithPeriod = true
				} else {
					endsWithPeriod = true
				}
			} else if strings.HasSuffix(trimmed, ".") && len(trimmed) > 1 {
				secondToLast := trimmed[len(trimmed)-2]
				if secondToLast < '0' || secondToLast > '9' {
					endsWithPeriod = true
				}
			} else {
				endsWithPeriod = true
			}
		}
		
		// Split if sentence ends OR if max words reached OR if it's the last word
		shouldSplit := endsWithPeriod || len(currentSentence.Words) >= maxWordsPerLine || i == len(timestamps)-1
		
		if shouldSplit && len(currentSentence.Words) > 0 {
			sentences = append(sentences, currentSentence)
			currentSentence = Sentence{Words: []app.SubtitleTimestamp{}}
		}
	}
	
	return sentences
}

// generateASS creates an ASS subtitle file with word-by-word highlighting
func generateASS(timestamps []app.SubtitleTimestamp, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// ASS Header
	fmt.Fprintln(file, "[Script Info]")
	fmt.Fprintln(file, "Title: Brainbot Video")
	fmt.Fprintln(file, "ScriptType: v4.00+")
	fmt.Fprintln(file, "PlayResX: 1080")
	fmt.Fprintln(file, "PlayResY: 1920")
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "[V4+ Styles]")
	fmt.Fprintln(file, "Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding")
	
	// White text style (default/unhighlighted)
	// MarginV=768 positions subtitles at 40% from bottom (1920px * 0.4 = 768px)
	fmt.Fprintf(file, "Style: Default,Consolas,%d,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,-1,0,0,0,100,100,0,0,1,3,0,2,40,40,768,1\n", config.SubtitleFontSize)
	
	// Yellow highlighted text style
	fmt.Fprintf(file, "Style: Highlight,Consolas,%d,&H0000FFFF,&H0000FFFF,&H00000000,&H00000000,-1,0,0,0,100,100,0,0,1,3,0,2,40,40,768,1\n", config.SubtitleFontSize)
	
	fmt.Fprintln(file, "")
	fmt.Fprintln(file, "[Events]")
	fmt.Fprintln(file, "Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")
	
	// Group words into sentences with max words per line from config
	sentences := groupIntoSentences(timestamps, config.SubtitleMaxWordsLine)
	
	// Generate subtitle events for each sentence
	for _, sentence := range sentences {
		// For each word timing in the sentence, create a subtitle event
		for wordIdx := range sentence.Words {
			startTime := sentence.Start
			endTime := sentence.End
			
			// Build the subtitle text with word-by-word highlighting
			var textParts []string
			for i, word := range sentence.Words {
				if i == wordIdx {
					// Current word is highlighted (yellow)
					textParts = append(textParts, fmt.Sprintf("{\\c&H0000FFFF&}%s{\\c&H00FFFFFF&}", word.Text))
				} else {
					// Other words are white
					textParts = append(textParts, word.Text)
				}
			}
			
			text := strings.Join(textParts, " ")
			
			// Use the actual word's timing
				startTime = sentence.Words[wordIdx].Start
				if wordIdx < len(sentence.Words)-1 {
					endTime = sentence.Words[wordIdx+1].Start
				} else {
					endTime = sentence.Words[wordIdx].End
				}
			
			fmt.Fprintf(file, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
				formatASSTimestamp(startTime),
				formatASSTimestamp(endTime),
				text)
		}
	}
	
	return nil
}

// formatASSTimestamp converts seconds to ASS timestamp format (h:mm:ss.cc)
func formatASSTimestamp(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)
	secs := int(seconds) % 60
	centisecs := int((seconds - float64(int(seconds))) * 100)
	
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, secs, centisecs)
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
