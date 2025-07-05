package resolver

import (
	"context"
	"net"
	"sync"
	"time"
)

// DNSResolver provides DNS resolution with caching
type DNSResolver struct {
	cache     map[string]*cacheEntry
	cacheMu   sync.RWMutex
	resolver  *net.Resolver
	ttl       time.Duration
}

type cacheEntry struct {
	hostname  string
	timestamp time.Time
}

// NewDNSResolver creates a new DNS resolver with caching
func NewDNSResolver(ttl time.Duration) *DNSResolver {
	return &DNSResolver{
		cache: make(map[string]*cacheEntry),
		resolver: &net.Resolver{
			PreferGo: true,
		},
		ttl: ttl,
	}
}

// ResolveIP performs reverse DNS lookup with caching
func (r *DNSResolver) ResolveIP(ip string) string {
	// Check cache first
	r.cacheMu.RLock()
	if entry, exists := r.cache[ip]; exists {
		if time.Since(entry.timestamp) < r.ttl {
			r.cacheMu.RUnlock()
			return entry.hostname
		}
	}
	r.cacheMu.RUnlock()

	// Perform reverse DNS lookup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	names, err := r.resolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		// Cache negative result too
		r.cacheMu.Lock()
		r.cache[ip] = &cacheEntry{
			hostname:  ip,
			timestamp: time.Now(),
		}
		r.cacheMu.Unlock()
		return ip
	}

	// Use the first hostname, remove trailing dot
	hostname := names[0]
	if len(hostname) > 0 && hostname[len(hostname)-1] == '.' {
		hostname = hostname[:len(hostname)-1]
	}

	// Cache the result
	r.cacheMu.Lock()
	r.cache[ip] = &cacheEntry{
		hostname:  hostname,
		timestamp: time.Now(),
	}
	r.cacheMu.Unlock()

	return hostname
}

// StartCleanup starts a goroutine to periodically clean expired cache entries
func (r *DNSResolver) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			r.cleanupCache()
		}
	}()
}

func (r *DNSResolver) cleanupCache() {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	now := time.Now()
	for ip, entry := range r.cache {
		if now.Sub(entry.timestamp) > r.ttl {
			delete(r.cache, ip)
		}
	}
}