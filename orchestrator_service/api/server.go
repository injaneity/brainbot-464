package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"orchestrator/state"
	"orchestrator/types"
	"orchestrator/workflow"
	"sync"

	"github.com/robfig/cron/v3"
)

// Server is the orchestrator HTTP server
type Server struct {
	stateManager   *state.Manager
	workflowRunner *workflow.Runner
	httpServer     *http.Server
	cron           *cron.Cron
	cronID         cron.EntryID
	mu             sync.Mutex
	shutdown       chan struct{}
}

// NewServer creates a new orchestrator server
func NewServer(stateManager *state.Manager, workflowRunner *workflow.Runner, port string) *Server {
	s := &Server{
		stateManager:   stateManager,
		workflowRunner: workflowRunner,
		cron:           cron.New(),
		shutdown:       make(chan struct{}),
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/start", s.handleStart)

	// Webhook endpoint (called by generation service)
	mux.HandleFunc("/webhook", s.handleWebhook)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting orchestrator server on %s", s.httpServer.Addr)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// StartCron starts the cron job for automated workflow runs
func (s *Server) StartCron(schedule string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add cron job
	id, err := s.cron.AddFunc(schedule, func() {
		log.Println("Cron triggered: starting automated workflow")

		// Only run if we're in idle or complete state
		currentState := s.stateManager.GetState()
		if currentState == types.StateIdle || currentState == types.StateComplete {
			ctx := context.Background()
			if err := s.workflowRunner.Run(ctx); err != nil {
				log.Printf("Cron workflow error: %v", err)
			}
		} else {
			log.Printf("Cron skipped: orchestrator is busy (state=%s)", currentState)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.cronID = id
	s.cron.Start()
	log.Printf("Cron job started with schedule: %s", schedule)
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down orchestrator server...")

	// Stop cron
	if s.cron != nil {
		s.cron.Stop()
	}

	// Stop HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}

	close(s.shutdown)
	return nil
}
