# Debugging Guide for Missing Network Events in TUI

## Issue
The TUI shows that packets are being captured (stats show packet counts and bytes), but the actual network events are not visible in the event list.

## Debug Logging Added

### TUI Side
1. **WebSocket Client** (`tui/internal/websocket/client.go`):
   - Logs when `network_event` messages are received
   - Logs when events are queued for processing
   - Logs if the messages channel is full

2. **UI Model** (`tui/internal/ui/model.go`):
   - Logs when EventMsg is received in the Update function
   - Logs total events and filtered events count

### Daemon Side  
1. **WebSocket Server** (`daemon/internal/websocket/server.go`):
   - Logs when broadcasting events
   - Logs number of clients receiving broadcasts
   - Logs when messages are sent to clients
   - Logs when messages are written to WebSocket

2. **Main** (`daemon/cmd/netty-daemon/main.go`):
   - Logs each packet received from the capturer

## Testing Steps

### Terminal 1 - Run the daemon:
```bash
sudo ./daemon/netty-daemon -i en0 -v
```

Look for:
- "DEBUG: Received packet #X from capturer" messages
- "DEBUG: Broadcasting event" messages
- "DEBUG: Event queued for broadcast to X clients"
- "DEBUG: Sent message to client"
- "DEBUG: Successfully wrote message to WebSocket client"

### Terminal 2 - Run the TUI:
```bash
./tui/netty-tui
```

Look for:
- "DEBUG: Received network_event" messages
- "DEBUG: Event queued for processing"
- "DEBUG: WaitForEvent returning EventMsg"
- "DEBUG Model: Received EventMsg"
- "DEBUG Model: Total events: X, Filtered events: Y"

### Terminal 3 - Test with browser:
```bash
open test_websocket.html
```

This will show if the daemon is actually sending WebSocket messages.

## Common Issues to Check

1. **No packets from capturer**: If you don't see "Received packet from capturer", the issue is in packet capture
2. **No broadcast messages**: If packets are received but not broadcast, check the WebSocket server
3. **No client messages**: If broadcast but clients don't receive, check WebSocket connection
4. **Messages received but not displayed**: If TUI receives messages but doesn't show them, check the UI rendering

## Next Steps Based on Debug Output

1. If no packets are captured, check:
   - Network interface is correct
   - BPF filter is not too restrictive
   - Permissions are correct

2. If packets are captured but not sent to TUI:
   - Check WebSocket connection status
   - Verify client is registered

3. If TUI receives events but doesn't display:
   - Check filter settings
   - Verify view mode is set to packets
   - Check if events are being added to the model

4. Run network traffic generator:
   ```bash
   # Generate some HTTP traffic
   curl http://example.com
   
   # Generate HTTPS traffic  
   curl https://google.com
   ```