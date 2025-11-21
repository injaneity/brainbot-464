package config

import "time"

const (
	// Video configuration
	VideoWidth     = 1080
	VideoHeight    = 1920
	VideoCodec     = "libx264"
	AudioCodec     = "aac"
	AudioBitrate   = "192k"
	VideoPreset    = "fast"
	MaxVideoDuration = 180.0  // 3 minutes max
	VideoEndPadding  = 0.5    // 0.5 seconds padding at end
	
	// Directory paths
	BackgroundsDir = "backgroundvids"
	OutputDir      = "output"
	InputDir       = "input"
	
	// Processing configuration
	MaxConcurrentVideos = 3
	VideoBatchDelay     = 2 * time.Second
	
	// Title generation
	MaxTitleWords  = 10
	MaxTitleLength = 100
)
