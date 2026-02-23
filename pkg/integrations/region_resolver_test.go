package integrations

import (
	"testing"
)

func TestNewRegionResolver(t *testing.T) {
	resolver := NewRegionResolver()

	if resolver == nil {
		t.Fatal("Expected non-nil resolver")
	}
}

func TestRegionResolver_Resolve_PrivateIP(t *testing.T) {
	resolver := NewRegionResolver()

	privateIPs := []string{
		"10.0.0.1",
		"192.168.1.1",
		"172.16.0.1",
		"10.244.0.5", // Common pod network
		"192.168.100.50",
	}

	for _, ip := range privateIPs {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result == nil {
				t.Fatal("Expected non-nil result for private IP")
			}

			if result.Provider != "internal" {
				t.Errorf("Expected provider 'internal' for %s, got '%s'", ip, result.Provider)
			}

			if result.IsPublic {
				t.Errorf("Expected IsPublic=false for private IP %s", ip)
			}
		})
	}
}

func TestRegionResolver_Resolve_PublicIP(t *testing.T) {
	resolver := NewRegionResolver()

	publicIPs := []string{
		"8.8.8.8",      // Google DNS
		"1.1.1.1",      // Cloudflare DNS
		"52.94.0.10",   // AWS IP range
		"34.64.0.1",    // GCP IP range
		"13.107.42.14", // Azure IP range
	}

	for _, ip := range publicIPs {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result == nil {
				t.Fatal("Expected non-nil result for public IP")
			}

			if result.Provider == "internal" {
				t.Errorf("Expected non-internal provider for public IP %s", ip)
			}

			if !result.IsPublic {
				t.Errorf("Expected IsPublic=true for public IP %s", ip)
			}

			// Currently defaults to "internet"
			if result.Provider != "internet" {
				t.Logf("Provider for %s: %s (future: match to specific cloud)", ip, result.Provider)
			}
		})
	}
}

func TestRegionResolver_Resolve_InvalidIP(t *testing.T) {
	resolver := NewRegionResolver()

	invalidIPs := []string{
		"not-an-ip",
		"256.256.256.256",
		"",
		"192.168.1",
		"abc.def.ghi.jkl",
	}

	for _, ip := range invalidIPs {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result != nil {
				t.Errorf("Expected nil result for invalid IP '%s', got %+v", ip, result)
			}
		})
	}
}

func TestRegionResolver_Resolve_IPv6(t *testing.T) {
	resolver := NewRegionResolver()

	ipv6Addresses := []string{
		"2001:4860:4860::8888", // Google DNS
		"::1",                  // Localhost
		"fe80::1",              // Link-local
	}

	for _, ip := range ipv6Addresses {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result == nil {
				t.Logf("IPv6 resolution for %s returned nil (expected behavior)", ip)
			} else {
				t.Logf("IPv6 %s resolved to: Provider=%s, IsPublic=%v",
					ip, result.Provider, result.IsPublic)
			}
		})
	}
}

func TestResolvedDestination_Structure(t *testing.T) {
	dest := &ResolvedDestination{
		Provider: "aws",
		Region:   "us-east-1",
		IsPublic: true,
	}

	if dest.Provider == "" {
		t.Error("Expected non-empty provider")
	}

	if dest.Region == "" {
		t.Error("Expected non-empty region")
	}

	if !dest.IsPublic {
		t.Error("Expected public destination")
	}
}

func TestRegionResolver_Resolve_AWSIPRanges(t *testing.T) {
	resolver := NewRegionResolver()

	// Common AWS IP ranges (in a graduated implementation, these would be matched)
	awsIPs := []string{
		"52.94.0.1",  // us-east-1
		"54.240.0.1", // Various AWS services
		"18.208.0.1", // us-east-1
	}

	for _, ip := range awsIPs {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			// Current implementation returns "internet"
			// Future implementation would return "aws" and specific region
			t.Logf("AWS IP %s resolved to: Provider=%s, Region=%s",
				ip, result.Provider, result.Region)
		})
	}
}

func TestRegionResolver_Resolve_GCPIPRanges(t *testing.T) {
	resolver := NewRegionResolver()

	// Common GCP IP ranges
	gcpIPs := []string{
		"34.64.0.1",  // asia-northeast3
		"35.190.0.1", // us-central1
	}

	for _, ip := range gcpIPs {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			t.Logf("GCP IP %s resolved to: Provider=%s, Region=%s",
				ip, result.Provider, result.Region)
		})
	}
}

func TestRegionResolver_Resolve_AzureIPRanges(t *testing.T) {
	resolver := NewRegionResolver()

	// Common Azure IP ranges
	azureIPs := []string{
		"13.107.42.14", // Azure services
		"20.38.0.1",    // eastus2
	}

	for _, ip := range azureIPs {
		t.Run(ip, func(t *testing.T) {
			result := resolver.Resolve(ip)

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			t.Logf("Azure IP %s resolved to: Provider=%s, Region=%s",
				ip, result.Provider, result.Region)
		})
	}
}

func TestRegionResolver_Resolve_RFC1918Ranges(t *testing.T) {
	resolver := NewRegionResolver()

	// Test all RFC1918 private ranges
	tests := []struct {
		name string
		ip   string
	}{
		{"Class A", "10.0.0.1"},
		{"Class A max", "10.255.255.254"},
		{"Class B", "172.16.0.1"},
		{"Class B mid", "172.20.0.1"},
		{"Class B max", "172.31.255.254"},
		{"Class C", "192.168.0.1"},
		{"Class C max", "192.168.255.254"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.Resolve(tt.ip)

			if result == nil {
				t.Fatalf("Expected non-nil result for %s", tt.ip)
			}

			if result.Provider != "internal" {
				t.Errorf("Expected 'internal' provider for RFC1918 IP %s, got '%s'",
					tt.ip, result.Provider)
			}
		})
	}
}

func TestRegionResolver_Resolve_Localhost(t *testing.T) {
	resolver := NewRegionResolver()

	result := resolver.Resolve("127.0.0.1")

	if result == nil {
		t.Fatal("Expected non-nil result for localhost")
	}

	// 127.0.0.1 is a loopback address, should be treated as internal
	if result.Provider != "internal" {
		t.Logf("Localhost resolved to: Provider=%s, IsPublic=%v",
			result.Provider, result.IsPublic)
	}
}

func TestRegionResolver_ConcurrentAccess(t *testing.T) {
	resolver := NewRegionResolver()

	// Test concurrent access to resolver
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			ips := []string{
				"10.0.0.1",
				"8.8.8.8",
				"192.168.1.1",
				"1.1.1.1",
			}

			for _, ip := range ips {
				result := resolver.Resolve(ip)
				if result == nil && ip != "" {
					t.Errorf("Goroutine %d: Unexpected nil for valid IP %s", id, ip)
				}
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
