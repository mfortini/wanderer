package immich

import (
	"net"
	"testing"
)

func TestIsBlockedOutboundIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{name: "loopback", ip: "127.0.0.1", blocked: true},
		{name: "ipv6 loopback", ip: "::1", blocked: true},
		{name: "private ipv4", ip: "10.0.0.1", blocked: true},
		{name: "private ipv6", ip: "fd00::1", blocked: true},
		{name: "link local", ip: "169.254.169.254", blocked: true},
		{name: "unspecified", ip: "0.0.0.0", blocked: true},
		{name: "public", ip: "8.8.8.8", blocked: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlockedOutboundIP(net.ParseIP(tt.ip))
			if got != tt.blocked {
				t.Fatalf("isBlockedOutboundIP(%q) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestSafeBaseURLBlocksLocalAddresses(t *testing.T) {
	tests := []string{
		"http://127.0.0.1:2283",
		"http://[::1]:2283",
		"http://10.0.0.2:2283",
		"http://169.254.169.254",
		"http://localhost:2283",
	}

	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			cfg := &Integration{URL: rawURL}
			if _, err := cfg.safeBaseURL(); err == nil {
				t.Fatalf("safeBaseURL(%q) succeeded, want error", rawURL)
			}
		})
	}
}

func TestSafeBaseURLAllowsPublicLiteralIP(t *testing.T) {
	cfg := &Integration{URL: "https://8.8.8.8"}

	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		t.Fatalf("safeBaseURL returned error: %v", err)
	}
	if baseURL != "https://8.8.8.8/api" {
		t.Fatalf("safeBaseURL = %q, want %q", baseURL, "https://8.8.8.8/api")
	}
}

func TestIsAssetID(t *testing.T) {
	tests := []struct {
		name    string
		assetID string
		valid   bool
	}{
		{name: "v4 uuid", assetID: "550e8400-e29b-41d4-a716-446655440000", valid: true},
		{name: "uppercase uuid", assetID: "550E8400-E29B-41D4-A716-446655440000", valid: true},
		{name: "empty", assetID: "", valid: false},
		{name: "path traversal", assetID: "../550e8400-e29b-41d4-a716-446655440000", valid: false},
		{name: "query injection", assetID: "550e8400-e29b-41d4-a716-446655440000?size=original", valid: false},
		{name: "not uuid", assetID: "asset_123", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAssetID(tt.assetID); got != tt.valid {
				t.Fatalf("IsAssetID(%q) = %v, want %v", tt.assetID, got, tt.valid)
			}
		})
	}
}

func TestNormalizePhotoMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "copy", mode: PhotoModeCopy, want: PhotoModeCopy},
		{name: "private link", mode: PhotoModeLinkPrivate, want: PhotoModeLinkPrivate},
		{name: "public link", mode: PhotoModeLinkPublic, want: PhotoModeLinkPublic},
		{name: "unknown", mode: "wat", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePhotoMode(tt.mode); got != tt.want {
				t.Fatalf("NormalizePhotoMode(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestIntegrationNormalizeDefaultsUnknownPhotoModeToCopy(t *testing.T) {
	cfg := &Integration{PhotoMode: "wat"}
	cfg.normalize()
	if cfg.PhotoMode != PhotoModeCopy {
		t.Fatalf("cfg.PhotoMode = %q, want %q", cfg.PhotoMode, PhotoModeCopy)
	}
}

func TestIsRemotePhotoMode(t *testing.T) {
	if IsRemotePhotoMode(PhotoModeCopy) {
		t.Fatal("copy mode must not be remote")
	}
	if !IsRemotePhotoMode(PhotoModeLinkPrivate) {
		t.Fatal("private link mode must be remote")
	}
	if !IsRemotePhotoMode(PhotoModeLinkPublic) {
		t.Fatal("public link mode must be remote")
	}
	if IsRemotePhotoMode("wat") {
		t.Fatal("unknown mode must not be remote")
	}
}
