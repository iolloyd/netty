package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/gopacket/pcap"
	"github.com/iolloyd/netty/daemon/internal/capture"
	"github.com/iolloyd/netty/daemon/internal/websocket"
)

func main() {
	var (
		iface       = flag.String("i", "", "Network interface to monitor (required)")
		wsPort      = flag.String("port", "8080", "WebSocket server port")
		filter      = flag.String("f", "", "BPF filter expression")
		verbose     = flag.Bool("v", false, "Enable verbose logging")
		listIfaces  = flag.Bool("list", false, "List available network interfaces")
	)
	flag.Parse()

	// Handle interface listing
	if *listIfaces {
		listInterfaces()
		return
	}

	if *iface == "" {
		log.Println("ERROR: Network interface is required. Use -i flag to specify interface.")
		log.Println("\nAvailable interfaces:")
		listInterfaces()
		os.Exit(1)
	}

	// Always show startup information
	log.Println("Starting Netty daemon...")
	log.Printf("Interface: %s", *iface)
	log.Printf("WebSocket port: %s", *wsPort)
	if *filter != "" {
		log.Printf("Filter: %s", *filter)
	}
	log.Println("")

	// List available interfaces for debugging
	if *verbose {
		log.Println("Available interfaces:")
		listInterfaces()
		log.Println("")
	}

	// Get local IP address for the specified interface
	localIP, err := getLocalIP(*iface)
	if err != nil {
		log.Fatalf("Failed to get local IP for interface %s: %v", *iface, err)
	}
	if *verbose {
		log.Printf("Local IP: %s", localIP)
	}

	// Create packet capture instance
	capturer, err := capture.NewPacketCapture(*iface, *filter, localIP)
	if err != nil {
		log.Fatalf("Failed to create packet capture: %v", err)
	}
	defer capturer.Close()

	// Create WebSocket server
	wsServer := websocket.NewServer(*wsPort)
	
	// Connect conversation manager to WebSocket server
	wsServer.SetConversationManager(capturer.GetConversationManager())
	
	// Connect capture statistics to WebSocket server
	wsServer.SetStatsFunction(capturer.GetStats)
	
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
			// Also broadcast conversation update if packet has conversation ID
			if packet.ConversationID != "" {
				wsServer.BroadcastConversationUpdate(packet.ConversationID)
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Netty daemon...")
}

// getLocalIP returns the local IP address for the specified interface
func getLocalIP(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found for interface %s", ifaceName)
}

// listInterfaces lists all available network interfaces
func listInterfaces() {
	// Try using pcap.FindAllDevs first for more accurate results
	devices, err := pcap.FindAllDevs()
	if err == nil && len(devices) > 0 {
		for _, device := range devices {
			fmt.Printf("  %s", device.Name)
			if device.Description != "" {
				fmt.Printf(" - %s", device.Description)
			}
			
			// Show IP addresses
			var ips []string
			for _, addr := range device.Addresses {
				if addr.IP.To4() != nil {
					ips = append(ips, addr.IP.String())
				}
			}
			if len(ips) > 0 {
				fmt.Printf(" [%v]", ips)
			}
			fmt.Println()
		}
	} else {
		// Fallback to net.Interfaces if pcap fails
		interfaces, err := net.Interfaces()
		if err != nil {
			log.Printf("Failed to list interfaces: %v", err)
			return
		}
		
		for _, iface := range interfaces {
			addrs, _ := iface.Addrs()
			fmt.Printf("  %s", iface.Name)
			
			// Show status
			if iface.Flags&net.FlagUp != 0 {
				fmt.Print(" (UP)")
			} else {
				fmt.Print(" (DOWN)")
			}
			
			// Show IP addresses
			var ips []string
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
			if len(ips) > 0 {
				fmt.Printf(" [%v]", ips)
			}
			fmt.Println()
		}
	}
	
	fmt.Println("\nCommon interface names:")
	fmt.Println("  en0: Wi-Fi (macOS)")
	fmt.Println("  en1: Ethernet (macOS)")
	fmt.Println("  lo0: Loopback")
}