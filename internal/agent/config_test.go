package agent

import "testing"

func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		secure   string
		fallback string
		wantErr  bool
	}{
		{"HTTPS", "https://agent.example.com/api", "https", "http", false},
		{"HTTP non-loopback", "http://agent.example.com", "https", "http", true},
		{"localhost development", "http://localhost:8080", "https", "http", false},
		{"loopback development", "http://127.0.0.1:8080", "https", "http", false},
		{"embedded credentials", "https://token@agent.example.com", "https", "http", true},
		{"query", "https://agent.example.com?token=x", "https", "http", true},
		{"relative", "/api", "https", "http", true},
		{"secure websocket", "wss://agent.example.com/socket", "wss", "ws", false},
		{"insecure websocket", "ws://agent.example.com/socket", "wss", "ws", true},
		{"loopback websocket", "ws://[::1]:8080/socket", "wss", "ws", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateEndpointURL(test.rawURL, test.secure, test.fallback)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateEndpointURL() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
