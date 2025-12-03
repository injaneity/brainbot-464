package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"orchestrator/types"
	"time"
)

// handleStatus handles GET /api/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.stateManager.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleStart handles POST /api/start
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if already running
	currentState := s.stateManager.GetState()
	if currentState != types.StateIdle && currentState != types.StateComplete && currentState != types.StateError {
		http.Error(w, fmt.Sprintf("Workflow already running (state=%s)", currentState), http.StatusConflict)
		return
	}

	// Start workflow asynchronously
	go func() {
		ctx := context.Background()
		if err := s.workflowRunner.Run(ctx); err != nil {
			log.Printf("Workflow error: %v", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Workflow initiated",
	})
}

// handleShutdown handles POST /api/shutdown
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "shutting down",
	})

	// Shutdown in background
	go func() {
		time.Sleep(500 * time.Millisecond) // Give time to send response
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()
}

// handleWebhook handles POST /webhook
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload types.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.stateManager.SetWebhookPayload(&payload)
	log.Printf("Webhook received: UUID=%s Status=%s", payload.UUID, payload.Status)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "received",
	})
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
