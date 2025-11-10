package vpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
)

// Manager handles VPN routing and failover
type Manager struct {
	db           *storage.Database
	vpns         []*models.VPNConfig
	clients      map[int64]*http.Client
	mu           sync.RWMutex
	currentIndex int
}

// NewManager creates a new VPN manager
func NewManager(db *storage.Database) (*Manager, error) {
	m := &Manager{
		db:      db,
		clients: make(map[int64]*http.Client),
	}

	// Load VPN configurations from database
	// For now, we'll use direct connections as VPN setup requires system-level config
	// In a real implementation, this would set up VPN tunnels

	return m, nil
}

// GetHTTPClient returns an HTTP client configured for an optimal VPN
func (m *Manager) GetHTTPClient(ctx context.Context) (*VPNHTTPClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If no VPNs configured, return default client
	if len(m.vpns) == 0 {
		return &VPNHTTPClient{
			client: &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
					IdleConnTimeout:     90 * time.Second,
				},
			},
			vpnID:   0,
			manager: m,
		}, nil
	}

	// Find best VPN based on failure rate and last usage
	bestVPN := m.selectBestVPN()
	if bestVPN == nil {
		return nil, fmt.Errorf("no available VPNs")
	}

	client, err := m.getOrCreateClient(bestVPN)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for VPN %s: %w", bestVPN.Name, err)
	}

	return &VPNHTTPClient{
		client:  client,
		vpnID:   bestVPN.ID,
		manager: m,
	}, nil
}

// selectBestVPN selects the best VPN based on performance metrics
func (m *Manager) selectBestVPN() *models.VPNConfig {
	var bestVPN *models.VPNConfig
	lowestScore := float64(1000000)

	for _, vpn := range m.vpns {
		if !vpn.Active {
			continue
		}

		// Calculate score based on failure count and time since last use
		score := float64(vpn.FailureCount * 10)
		if !vpn.LastUsed.IsZero() {
			timeSinceUse := time.Since(vpn.LastUsed).Minutes()
			score -= timeSinceUse // Prefer VPNs that haven't been used recently
		}

		if score < lowestScore {
			lowestScore = score
			bestVPN = vpn
		}
	}

	return bestVPN
}

// getOrCreateClient gets or creates an HTTP client for a VPN
func (m *Manager) getOrCreateClient(vpn *models.VPNConfig) (*http.Client, error) {
	if client, exists := m.clients[vpn.ID]; exists {
		return client, nil
	}

	// Create HTTP client with proxy if configured
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	// If VPN has a proxy configured, use it
	if vpn.Host != "" && vpn.Port > 0 {
		proxyURL := &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", vpn.Host, vpn.Port),
		}
		if vpn.Username != "" {
			proxyURL.User = url.UserPassword(vpn.Username, vpn.Password)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	m.clients[vpn.ID] = client
	return client, nil
}

// MarkVPNFailed marks a VPN as having failed
func (m *Manager) MarkVPNFailed(vpnID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, vpn := range m.vpns {
		if vpn.ID == vpnID {
			vpn.FailureCount++
			// Disable VPN if it has too many failures
			if vpn.FailureCount >= 5 {
				vpn.Active = false
			}
			break
		}
	}

	return nil
}

// MarkVPNSuccess marks a VPN as having succeeded
func (m *Manager) MarkVPNSuccess(vpnID int64, bytesTransferred int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, vpn := range m.vpns {
		if vpn.ID == vpnID {
			vpn.FailureCount = 0 // Reset failure count on success
			vpn.LastUsed = time.Now()
			break
		}
	}

	return nil
}

// AddVPN adds a new VPN configuration
func (m *Manager) AddVPN(vpn *models.VPNConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.vpns = append(m.vpns, vpn)
	return nil
}

// VPNHTTPClient wraps http.Client with VPN tracking
type VPNHTTPClient struct {
	client  *http.Client
	vpnID   int64
	manager *Manager
}

// Get performs a GET request and tracks metrics
func (c *VPNHTTPClient) Get(url string) ([]byte, error) {
	start := time.Now()

	resp, err := c.client.Get(url)
	if err != nil {
		c.manager.MarkVPNFailed(c.vpnID)
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.manager.MarkVPNFailed(c.vpnID)
		return nil, fmt.Errorf("GET request returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.manager.MarkVPNFailed(c.vpnID)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Mark success and track metrics
	c.manager.MarkVPNSuccess(c.vpnID, int64(len(data)))

	_ = time.Since(start) // Track latency for future optimization

	return data, nil
}

// GetStream performs a GET request optimized for large files
func (c *VPNHTTPClient) GetStream(url string) ([]byte, error) {
	return c.Get(url) // For now, use the same implementation
}

// GetClient returns the underlying HTTP client
func (c *VPNHTTPClient) GetClient() *http.Client {
	return c.client
}
