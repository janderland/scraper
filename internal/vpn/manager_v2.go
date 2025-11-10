package vpn

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ManagerV2 handles VPN routing and failover with real VPN support
type ManagerV2 struct {
	config     *Config
	connectors map[string]Connector
	stats      map[string]*Stats
	mu         sync.RWMutex
	healthCheckInterval time.Duration
	stopHealthCheck chan struct{}
}

// Stats tracks connection statistics for a VPN
type Stats struct {
	Name             string
	Connected        bool
	BytesTransferred int64
	RequestCount     int64
	FailureCount     int64
	LastUsed         time.Time
	LastHealthCheck  time.Time
	HealthCheckOK    bool
}

// NewManagerV2 creates a new VPN manager with real VPN support
func NewManagerV2(configPath string) (*ManagerV2, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	m := &ManagerV2{
		config:              config,
		connectors:          make(map[string]Connector),
		stats:               make(map[string]*Stats),
		healthCheckInterval: 60 * time.Second,
		stopHealthCheck:     make(chan struct{}),
	}

	// Create connectors for each VPN
	for _, vpnConfig := range config.VPNs {
		if !vpnConfig.Enabled {
			continue
		}

		connector, err := CreateConnector(&vpnConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create connector for %s: %w", vpnConfig.Name, err)
		}

		m.connectors[vpnConfig.Name] = connector
		m.stats[vpnConfig.Name] = &Stats{
			Name: vpnConfig.Name,
		}

		// Auto-connect if configured
		if vpnConfig.Options.AutoConnect {
			go func(name string, conn Connector) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				if err := conn.Connect(ctx); err != nil {
					fmt.Printf("Failed to auto-connect VPN %s: %v\n", name, err)
				} else {
					fmt.Printf("VPN %s connected successfully\n", name)
					m.mu.Lock()
					m.stats[name].Connected = true
					m.mu.Unlock()
				}
			}(vpnConfig.Name, connector)
		}
	}

	// Start health check routine
	go m.healthCheckRoutine()

	return m, nil
}

// Connect connects to a specific VPN
func (m *ManagerV2) Connect(ctx context.Context, vpnName string) error {
	m.mu.RLock()
	connector, exists := m.connectors[vpnName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("VPN %s not found", vpnName)
	}

	if err := connector.Connect(ctx); err != nil {
		m.mu.Lock()
		m.stats[vpnName].FailureCount++
		m.stats[vpnName].Connected = false
		m.mu.Unlock()
		return err
	}

	m.mu.Lock()
	m.stats[vpnName].Connected = true
	m.stats[vpnName].FailureCount = 0
	m.mu.Unlock()

	return nil
}

// Disconnect disconnects from a specific VPN
func (m *ManagerV2) Disconnect(vpnName string) error {
	m.mu.RLock()
	connector, exists := m.connectors[vpnName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("VPN %s not found", vpnName)
	}

	if err := connector.Disconnect(); err != nil {
		return err
	}

	m.mu.Lock()
	m.stats[vpnName].Connected = false
	m.mu.Unlock()

	return nil
}

// DisconnectAll disconnects all VPNs
func (m *ManagerV2) DisconnectAll() error {
	m.mu.RLock()
	connectors := make([]Connector, 0, len(m.connectors))
	for _, conn := range m.connectors {
		connectors = append(connectors, conn)
	}
	m.mu.RUnlock()

	var lastError error
	for _, conn := range connectors {
		if err := conn.Disconnect(); err != nil {
			lastError = err
		}
	}

	return lastError
}

// GetHTTPClient returns an HTTP client configured to use an optimal VPN
func (m *ManagerV2) GetHTTPClient(ctx context.Context) (*VPNHTTPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find best connected VPN based on priority and stats
	var bestConnector Connector
	var bestVPNName string
	bestScore := float64(-1)

	for name, connector := range m.connectors {
		if !connector.IsConnected() {
			continue
		}

		stats := m.stats[name]
		vpnConfig := m.getVPNConfig(name)

		// Calculate score: priority - (failure_count * 10) + time_since_use
		score := float64(vpnConfig.Priority)
		score -= float64(stats.FailureCount * 10)

		if !stats.LastUsed.IsZero() {
			timeSinceUse := time.Since(stats.LastUsed).Minutes()
			score += timeSinceUse / 60 // Bonus for idle time
		}

		// Penalize if health check failed
		if !stats.HealthCheckOK {
			score -= 100
		}

		if score > bestScore {
			bestScore = score
			bestConnector = connector
			bestVPNName = name
		}
	}

	if bestConnector == nil {
		return nil, fmt.Errorf("no connected VPNs available")
	}

	client, err := bestConnector.GetHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP client: %w", err)
	}

	return &VPNHTTPClient{
		client:     client,
		vpnName:    bestVPNName,
		manager:    m,
	}, nil
}

// GetConnectedVPNs returns a list of currently connected VPNs
func (m *ManagerV2) GetConnectedVPNs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connected []string
	for name, connector := range m.connectors {
		if connector.IsConnected() {
			connected = append(connected, name)
		}
	}

	return connected
}

// GetStats returns statistics for all VPNs
func (m *ManagerV2) GetStats() map[string]*Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to avoid race conditions
	statsCopy := make(map[string]*Stats)
	for name, stats := range m.stats {
		statsCopy[name] = &Stats{
			Name:             stats.Name,
			Connected:        stats.Connected,
			BytesTransferred: stats.BytesTransferred,
			RequestCount:     stats.RequestCount,
			FailureCount:     stats.FailureCount,
			LastUsed:         stats.LastUsed,
			LastHealthCheck:  stats.LastHealthCheck,
			HealthCheckOK:    stats.HealthCheckOK,
		}
	}

	return statsCopy
}

// MarkSuccess marks a successful request for a VPN
func (m *ManagerV2) MarkSuccess(vpnName string, bytesTransferred int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stats, exists := m.stats[vpnName]; exists {
		stats.BytesTransferred += bytesTransferred
		stats.RequestCount++
		stats.LastUsed = time.Now()
		stats.FailureCount = 0 // Reset on success
	}
}

// MarkFailure marks a failed request for a VPN
func (m *ManagerV2) MarkFailure(vpnName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stats, exists := m.stats[vpnName]; exists {
		stats.FailureCount++
		stats.LastUsed = time.Now()

		// Auto-disconnect if too many failures
		if stats.FailureCount >= 5 {
			go func() {
				m.Disconnect(vpnName)
			}()
		}
	}
}

// healthCheckRoutine periodically checks VPN connections
func (m *ManagerV2) healthCheckRoutine() {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performHealthChecks()
		case <-m.stopHealthCheck:
			return
		}
	}
}

// performHealthChecks runs health checks on all connected VPNs
func (m *ManagerV2) performHealthChecks() {
	m.mu.RLock()
	connectors := make(map[string]Connector)
	for name, conn := range m.connectors {
		if conn.IsConnected() {
			connectors[name] = conn
		}
	}
	m.mu.RUnlock()

	for name, conn := range connectors {
		go func(vpnName string, connector Connector) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := connector.HealthCheck(ctx)

			m.mu.Lock()
			if stats, exists := m.stats[vpnName]; exists {
				stats.LastHealthCheck = time.Now()
				stats.HealthCheckOK = (err == nil)

				if err != nil {
					stats.FailureCount++
					fmt.Printf("Health check failed for VPN %s: %v\n", vpnName, err)

					// Disconnect if too many health check failures
					if stats.FailureCount >= 3 {
						go m.Disconnect(vpnName)
					}
				} else {
					stats.FailureCount = 0
				}
			}
			m.mu.Unlock()
		}(name, conn)
	}
}

// getVPNConfig retrieves the configuration for a VPN
func (m *ManagerV2) getVPNConfig(name string) *VPNConfig {
	for _, vpn := range m.config.VPNs {
		if vpn.Name == name {
			return &vpn
		}
	}
	return nil
}

// Close stops the manager and disconnects all VPNs
func (m *ManagerV2) Close() error {
	close(m.stopHealthCheck)
	return m.DisconnectAll()
}

// VPNHTTPClient wraps http.Client with VPN tracking
type VPNHTTPClient struct {
	client  *http.Client
	vpnName string
	manager *ManagerV2
}

// Get performs a GET request and tracks metrics
func (c *VPNHTTPClient) Get(url string) ([]byte, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		c.manager.MarkFailure(c.vpnName)
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.manager.MarkFailure(c.vpnName)
		return nil, fmt.Errorf("GET request returned status %d", resp.StatusCode)
	}

	data, err := http.Client{}.Get(url) // Read body
	if err != nil {
		c.manager.MarkFailure(c.vpnName)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Mark success and track metrics
	c.manager.MarkSuccess(c.vpnName, int64(len(data)))

	return data, nil
}

// GetStream performs a GET request optimized for large files
func (c *VPNHTTPClient) GetStream(url string) ([]byte, error) {
	return c.Get(url)
}

// GetClient returns the underlying HTTP client
func (c *VPNHTTPClient) GetClient() *http.Client {
	return c.client
}
