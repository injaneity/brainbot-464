package main

import (
	"brainbot/demo/client"
	"brainbot/demo/tui"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	// Parse command-line flags
	clearCache := flag.Bool("clear", true, "Clear ChromaDB cache before starting (default: true)")
	flag.Parse()
	_ = clearCache // Store for later use if needed

	// Load environment
	_ = godotenv.Load()

	// Set up webhook server
	webhookPort := client.GetEnvOrDefault("WEBHOOK_PORT", "9999")

	// Create app client
	apiURL := client.GetEnvOrDefault("API_URL", "http://localhost:8080")
	appClient := client.NewClient(apiURL)

	// Create the model (without server reference - managed externally)
	m := tui.NewModel(webhookPort, appClient)

	// Create the tea program
	program := tea.NewProgram(m)

	// Start webhook server (managed OUTSIDE the model)
	// The server sends messages to the program, but isn't part of the model
	server, err := tui.StartWebhookServer(webhookPort, program)
	if err != nil {
		fmt.Printf("Failed to start webhook server: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	// Server lifecycle is managed here in main(), not in the TUI
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		// Shutdown server first
		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}
		// Then quit the program
		program.Quit()
	}()

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Clean up server after program exits
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}
