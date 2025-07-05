package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/netty/tui/internal/ui"
	"github.com/netty/tui/internal/websocket"
)

func main() {
	var (
		host = flag.String("host", "localhost", "Daemon host address")
		port = flag.Int("port", 8080, "Daemon WebSocket port")
	)
	flag.Parse()

	// Create WebSocket client
	wsClient := websocket.NewClient(*host, *port)

	// Create the UI model
	model := ui.NewModel(wsClient)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Clean up
	if err := wsClient.Close(); err != nil {
		log.Printf("Error closing WebSocket connection: %v", err)
	}
}