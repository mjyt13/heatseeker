package http

import "testing"

func TestAllowOrigin(t *testing.T) {
	configured := []string{"https://heatseeker.example", "http://localhost:4173/"}
	tests := []struct {
		origin   string
		dev      bool
		expected bool
	}{
		{"https://heatseeker.example", false, true},
		{"HTTPS://Heatseeker.Example", false, true},
		{"http://localhost:4173", false, true},
		{"http://localhost:8083", false, false},
		{"http://192.168.0.104:4173", false, false},

		{"http://localhost:8083", true, true},
		{"http://127.0.0.1:19006", true, true},
		{"http://192.168.0.104:4173", true, true},
		{"http://10.0.0.5", true, true},
		{"http://172.20.1.2:3000", true, true},
		{"http://[::1]:4173", true, true},
		{"http://[fd00::1]:4173", true, true},
		{"http://172.32.0.1:4173", true, false},
		{"http://8.8.8.8", true, false},
		{"http://evil.example", true, false},
		{"http://localhost.evil.example", true, false},
		{"http://192.168.0.104.nip.io", true, false},
		{"file://192.168.0.104", true, false},
		{"null", true, false},
		{"", true, false},
	}
	for _, tt := range tests {
		got := allowOrigin(configured, tt.dev)(nil, tt.origin)
		if got != tt.expected {
			t.Errorf("allowOrigin(%q, dev=%v) = %v, want %v", tt.origin, tt.dev, got, tt.expected)
		}
	}
}
