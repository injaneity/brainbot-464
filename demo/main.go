package main

import (
	"brainbot/demo/tui"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment
	_ = godotenv.Load()

	// Parse command-line flags
	orchestratorURL := flag.String("url", "http://localhost:8081", "Orchestrator URL")
	flag.Parse()

	// Create TUI model
	m := tui.NewModel(*orchestratorURL)

	// Create the tea program
	program := tea.NewProgram(m)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		program.Quit()
	}()

	// Run the program
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
