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

	// Create the tea program first (without model)
	var program *tea.Program

	// Start webhook server with the program (we'll pass it after creation)
	// For now, we create a placeholder and update it after program creation
	m := tui.NewModel(webhookPort, nil, appClient)
	program = tea.NewProgram(m)

	// Now start the webhook server with the program reference
	server, err := tui.StartWebhookServer(webhookPort, program)
	if err != nil {
		fmt.Printf("Failed to start webhook server: %v\n", err)
		os.Exit(1)
	}

	// Update model with server reference
	// Note: Since Model uses value semantics, we need to create a new model with the server
	// and send it to the program. However, since the program already started with the old model,
	// we store the server in the model that will be used during updates.
	// The server is a pointer, so all model copies share the same server instance.
	m.WebhookServer = server

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}
		program.Quit()
	}()

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Clean up
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}
