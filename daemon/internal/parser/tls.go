package parser

import (
	"bytes"
	"encoding/binary"
)

const (
	tlsHandshake      = 0x16
	tlsClientHello    = 0x01
	extensionSNI      = 0x0000
	sniTypeHostname   = 0x00
)

// ExtractSNI attempts to extract the Server Name Indication from TLS ClientHello
func ExtractSNI(payload []byte) string {
	if len(payload) < 5 {
		return ""
	}

	// Check if this is a TLS handshake record
	if payload[0] != tlsHandshake {
		return ""
	}

	// Skip TLS record header (5 bytes)
	pos := 5

	if pos >= len(payload) {
		return ""
	}

	// Check if this is a ClientHello message
	if payload[pos] != tlsClientHello {
		return ""
	}
	pos++

	// Skip ClientHello length (3 bytes)
	if pos+3 > len(payload) {
		return ""
	}
	pos += 3

	// Skip protocol version (2 bytes)
	if pos+2 > len(payload) {
		return ""
	}
	pos += 2

	// Skip random (32 bytes)
	if pos+32 > len(payload) {
		return ""
	}
	pos += 32

	// Session ID length
	if pos >= len(payload) {
		return ""
	}
	sessionIDLen := int(payload[pos])
	pos++

	// Skip session ID
	if pos+sessionIDLen > len(payload) {
		return ""
	}
	pos += sessionIDLen

	// Cipher suites length
	if pos+2 > len(payload) {
		return ""
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(payload[pos:]))
	pos += 2

	// Skip cipher suites
	if pos+cipherSuitesLen > len(payload) {
		return ""
	}
	pos += cipherSuitesLen

	// Compression methods length
	if pos >= len(payload) {
		return ""
	}
	compressionLen := int(payload[pos])
	pos++

	// Skip compression methods
	if pos+compressionLen > len(payload) {
		return ""
	}
	pos += compressionLen

	// Extensions length
	if pos+2 > len(payload) {
		return ""
	}
	extensionsLen := int(binary.BigEndian.Uint16(payload[pos:]))
	pos += 2

	// Parse extensions
	extensionsEnd := pos + extensionsLen
	if extensionsEnd > len(payload) {
		return ""
	}

	for pos < extensionsEnd {
		if pos+4 > len(payload) {
			break
		}

		// Extension type
		extType := binary.BigEndian.Uint16(payload[pos:])
		pos += 2

		// Extension length
		extLen := int(binary.BigEndian.Uint16(payload[pos:]))
		pos += 2

		if extType == extensionSNI {
			// Found SNI extension
			return parseSNIExtension(payload[pos:pos+extLen])
		}

		// Skip this extension
		pos += extLen
	}

	return ""
}

func parseSNIExtension(data []byte) string {
	if len(data) < 2 {
		return ""
	}

	// SNI list length
	listLen := int(binary.BigEndian.Uint16(data))
	pos := 2

	if pos+listLen > len(data) {
		return ""
	}

	// Parse SNI entries
	listEnd := pos + listLen
	for pos < listEnd {
		if pos+3 > len(data) {
			break
		}

		// SNI type
		sniType := data[pos]
		pos++

		// SNI length
		sniLen := int(binary.BigEndian.Uint16(data[pos:]))
		pos += 2

		if sniType == sniTypeHostname {
			if pos+sniLen <= len(data) {
				hostname := string(data[pos : pos+sniLen])
				// Validate hostname contains only valid characters
				if isValidHostname(hostname) {
					return hostname
				}
			}
		}

		pos += sniLen
	}

	return ""
}

func isValidHostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 255 {
		return false
	}

	// Basic validation - should contain only valid hostname characters
	for _, ch := range hostname {
		if !((ch >= 'a' && ch <= 'z') || 
			(ch >= 'A' && ch <= 'Z') || 
			(ch >= '0' && ch <= '9') || 
			ch == '.' || ch == '-') {
			return false
		}
	}

	return !bytes.Contains([]byte(hostname), []byte(".."))
}