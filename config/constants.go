package config

import "time"

// Video Processing Constants
const (
	// MaxConcurrentVideos limits the number of videos processed simultaneously
	MaxConcurrentVideos = 2
	
	// VideoBatchDelay is the wait time between processing video batches
	VideoBatchDelay = 2 * time.Minute
	
	// MaxVideoDuration is the maximum allowed video length in seconds (3 minutes)
	MaxVideoDuration = 180.0
	
	// VideoEndPadding adds a delay at the end of the video in seconds
	VideoEndPadding = 0.5
)

// Video Output Constants
const (
	// VideoWidth is the output video width (9:16 aspect ratio)
	VideoWidth = 720
	
	// VideoHeight is the output video height (9:16 aspect ratio)
	VideoHeight = 1280
	
	// VideoCodec is the video encoding codec
	VideoCodec = "libx264"
	
	// AudioCodec is the audio encoding codec
	AudioCodec = "aac"
	
	// AudioBitrate is the audio quality bitrate
	AudioBitrate = "192k"
	
	// VideoPreset is the ffmpeg encoding speed preset
	VideoPreset = "fast"
)

// Title and Metadata Constants
const (
	// MaxTitleWords is the maximum number of words to use from subtitles for title
	MaxTitleWords = 10
	
	// MaxTitleLength is the maximum character length for video titles
	MaxTitleLength = 100
)

// Directory Constants
const (
	// BackgroundsDir is the directory containing background videos
	BackgroundsDir = "backgroundvids"
	
	// InputDir is the directory containing input JSON files
	InputDir = "input"
	
	// OutputDir is the directory for generated videos
	OutputDir = "output"
	
	// TempDir is the directory for temporary files
	TempDir = "/tmp"
)

// YouTube Constants
const (
	// YouTubeCategoryID for Science & Technology
	YouTubeCategoryID = "28"
	
	// YouTubePrivacyStatus sets video visibility
	YouTubePrivacyStatus = "public"
)
