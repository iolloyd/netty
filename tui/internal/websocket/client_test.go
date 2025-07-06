package websocket

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"
)

func TestClientErrorHandling(t *testing.T) {
	// Capture log output to ensure no errors are printed
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Create a client that will fail to connect
	client := NewClient("localhost", 9999) // Using a port that's likely not in use

	// Try to connect (should fail)
	cmd := client.Connect()
	msg := cmd()

	// Check that connection failed
	statusMsg, ok := msg.(ConnectionStatusMsg)
	if !ok {
		t.Fatalf("Expected ConnectionStatusMsg, got %T", msg)
	}

	if statusMsg.Connected {
		t.Error("Expected connection to fail")
	}

	if statusMsg.Error == nil {
		t.Error("Expected error to be non-nil")
	}

	// Give some time for any goroutines to finish
	time.Sleep(100 * time.Millisecond)

	// Check that nothing was logged
	if buf.Len() > 0 {
		t.Errorf("Expected no log output, but got: %s", buf.String())
	}
}

func TestClientReconnection(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	client := NewClient("localhost", 9999)

	// First connection attempt
	cmd := client.Connect()
	msg := cmd()

	statusMsg, ok := msg.(ConnectionStatusMsg)
	if !ok || statusMsg.Connected {
		t.Skip("Test requires daemon not to be running on port 9999")
	}

	// Try reconnection
	reconnectCmd := client.Reconnect()
	reconnectMsg := reconnectCmd()

	// Should get another connection status message
	statusMsg2, ok := reconnectMsg.(ConnectionStatusMsg)
	if !ok {
		t.Fatalf("Expected ConnectionStatusMsg on reconnect, got %T", reconnectMsg)
	}

	if statusMsg2.Connected {
		t.Error("Expected reconnection to fail (no daemon running)")
	}

	// Ensure client can be closed without errors
	err := client.Close()
	if err != nil {
		t.Errorf("Expected Close() to succeed, got error: %v", err)
	}

	// Check that nothing was logged
	if buf.Len() > 0 {
		t.Errorf("Expected no log output, but got: %s", buf.String())
	}
}

func TestClientIsConnected(t *testing.T) {
	client := NewClient("localhost", 9999)

	// Initially should not be connected
	if client.IsConnected() {
		t.Error("Expected client to not be connected initially")
	}

	// After failed connection attempt, should still not be connected
	cmd := client.Connect()
	cmd()

	if client.IsConnected() {
		t.Error("Expected client to not be connected after failed connection")
	}
}