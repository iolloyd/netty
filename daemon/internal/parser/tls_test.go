package parser

import (
	"encoding/hex"
	"testing"
)

func TestExtractSNI(t *testing.T) {
	// Sample TLS ClientHello with SNI for github.com
	// This is a real packet capture snippet
	clientHelloHex := "1603010200010001fc03037e184b2f1e8f7c7a0a7f6d4e8c9a2b5f3d7e9c0a1b2c3d4e5f6a7b8c9d0e1f20e0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfe003e130213031301c02cc030009fcca9cca8ccaac02bc02f009ec024c028006bc023c0270067c00ac0140039c009c0130033009d009c003d003c0035002f00ff01000193000b000403000102000a000a0008001d00170019001800230000001600000017000000000d002a0028040305030603080708080809080a080b080408050806040105010601030303010302040205020602002b00050403040303002d00020101003300260024001d00206d0e5f7a1b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0000001000110000000a6769746875622e636f6d"
	
	clientHello, err := hex.DecodeString(clientHelloHex)
	if err != nil {
		t.Fatalf("Failed to decode hex: %v", err)
	}

	// Test SNI extraction
	sni := ExtractSNI(clientHello)
	expected := "github.com"
	
	if sni != expected {
		t.Errorf("Expected SNI '%s', got '%s'", expected, sni)
	}
}

func TestExtractSNI_NoSNI(t *testing.T) {
	// TLS packet without SNI extension
	noSNI := []byte{0x16, 0x03, 0x01, 0x00, 0x05, 0x01, 0x00, 0x00, 0x01, 0x03}
	
	sni := ExtractSNI(noSNI)
	if sni != "" {
		t.Errorf("Expected empty SNI, got '%s'", sni)
	}
}

func TestExtractSNI_InvalidPacket(t *testing.T) {
	// Not a TLS handshake
	invalid := []byte{0x17, 0x03, 0x01, 0x00, 0x05}
	
	sni := ExtractSNI(invalid)
	if sni != "" {
		t.Errorf("Expected empty SNI for non-handshake packet, got '%s'", sni)
	}
}