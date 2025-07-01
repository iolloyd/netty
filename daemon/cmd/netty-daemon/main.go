package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/iolloyd/netty/daemon/internal/capture"
	"github.com/iolloyd/netty/daemon/internal/websocket"
)

func main() {
	var (
		iface    = flag.String("i", "", "Network interface to monitor (required)")
		wsPort   = flag.String("port", "8080", "WebSocket server port")
		filter   = flag.String("f", "", "BPF filter expression")
		verbose  = flag.Bool("v", false, "Enable verbose logging")
	)
	flag.Parse()

	if *iface == "" {
		log.Fatal("Network interface is required. Use -i flag to specify interface.")
	}

	if *verbose {
		log.Println("Starting Netty daemon...")
		log.Printf("Interface: %s", *iface)
		log.Printf("WebSocket port: %s", *wsPort)
		if *filter != "" {
			log.Printf("Filter: %s", *filter)
		}
	}

	// Create packet capture instance
	capturer, err := capture.NewPacketCapture(*iface, *filter)
	if err != nil {
		log.Fatalf("Failed to create packet capture: %v", err)
	}
	defer capturer.Close()

	// Create WebSocket server
	wsServer := websocket.NewServer(*wsPort)
	
	// Start WebSocket server in background
	go func() {
		if err := wsServer.Start(); err != nil {
			log.Fatalf("WebSocket server failed: %v", err)
		}
	}()

	// Start packet capture
	packets := capturer.Start()
	
	// Process packets and send to WebSocket clients
	go func() {
		for packet := range packets {
			wsServer.Broadcast(packet)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Netty daemon...")
}