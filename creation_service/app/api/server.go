package api

import (
	"encoding/json"
	"log"
	"net/http"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/services"
)

// Server handles HTTP API requests for video processing
type Server struct {
	processor *services.VideoProcessor
}

// NewServer creates a new API server instance
func NewServer(proc *services.VideoProcessor) *Server {
	return &Server{
		processor: proc,
	}
}

// HandleProcessVideo processes a single video from JSON payload
// POST /api/process-video
// Expects: VideoInput JSON in request body
// Returns: ProcessVideoResponse JSON
func (s *Server) HandleProcessVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req app.ProcessVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	// Validate input
	if req.Status != "success" {
		respondWithError(w, http.StatusBadRequest, "Input status must be 'success'", nil)
		return
	}

	if req.UUID == "" {
		respondWithError(w, http.StatusBadRequest, "UUID is required", nil)
		return
	}

	if req.Voiceover == "" {
		respondWithError(w, http.StatusBadRequest, "Voiceover URL is required", nil)
		return
	}

	if len(req.SubtitleTimestamps) == 0 {
		respondWithError(w, http.StatusBadRequest, "Subtitle timestamps are required", nil)
		return
	}

	log.Printf("Received video processing request: UUID=%s", req.UUID)

	// Process video asynchronously (non-blocking for API response)
	go func() {
		if err := s.processor.ProcessVideoInput(req.VideoInput, false); err != nil {
			log.Printf("Video processing failed for UUID %s: %v", req.UUID, err)
		}
	}()

	// Return immediate success response
	respondWithSuccess(w, "Video processing started", "")
}

// HandleHealth provides a health check endpoint
// GET /health
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// respondWithError sends an error response
func respondWithError(w http.ResponseWriter, statusCode int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := app.ProcessVideoResponse{
		Success: false,
		Message: message,
	}

	if err != nil {
		response.Error = err.Error()
		log.Printf("API Error: %s - %v", message, err)
	}

	json.NewEncoder(w).Encode(response)
}

// respondWithSuccess sends a success response
func respondWithSuccess(w http.ResponseWriter, message string, videoID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := app.ProcessVideoResponse{
		Success: true,
		Message: message,
		VideoID: videoID,
	}

	json.NewEncoder(w).Encode(response)
}

// SetupRoutes configures all API routes
func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.HandleHealth)
	mux.HandleFunc("/api/process-video", s.HandleProcessVideo)

	return mux
}
