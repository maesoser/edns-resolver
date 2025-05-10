package main

import (
	"log"
	"os"
	"strconv"
)

const (
	// DNS server configuration
	defaultDNSPort       = 5053
	defaultDNSListenAddr = "0.0.0.0"
	defaultMetricsPort   = 9100
	defaultUpstream      = "1.0.0.1:53"
	defaultCacheSize     = 1024
	defaultRestUrl       = "http://localhost:8080/categories" // Default REST service URL
	defaultTTL           = 60                                 // Default TTL for cached entries (seconds)
)

func main() {
	// Parse configuration from environment variables
	config := Config{
		ListenAddr:  getEnv("DNS_LISTEN_ADDR", defaultDNSListenAddr),
		Port:        getEnvAsInt("DNS_PORT", defaultDNSPort),
		MetricsPort: getEnvAsInt("DNS_METRICS_PORT", defaultMetricsPort),
		Upstream:    getEnv("DNS_UPSTREAM", defaultUpstream),
		CacheSize:   getEnvAsInt("DNS_CACHE_SIZE", defaultCacheSize),
		RestURL:     getEnv("DNS_REST_URL", defaultRestUrl),
		DefaultTTL:  uint32(getEnvAsInt("DNS_DEFAULT_TTL", defaultTTL)),
	}

	resolver, err := NewDNSResolver(config)
	if err != nil {
		log.Fatalf("Failed to initialize DNS resolver: %v", err)
	}

	if err := resolver.Start(); err != nil {
		log.Fatalf("Failed to start DNS resolver: %v", err)
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt gets an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid value for %s, using default: %d", key, defaultValue)
		return defaultValue
	}
	return value
}
