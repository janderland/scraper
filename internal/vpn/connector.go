package vpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// Connector defines the interface for VPN connections
type Connector interface {
	Connect(ctx context.Context) error
	Disconnect() error
	IsConnected() bool
	GetHTTPClient() (*http.Client, error)
	HealthCheck(ctx context.Context) error
	GetName() string
}

// OpenVPNConnector handles OpenVPN connections
type OpenVPNConnector struct {
	config  *VPNConfig
	cmd     *exec.Cmd
	mu      sync.RWMutex
	connected bool
}

// NewOpenVPNConnector creates a new OpenVPN connector
func NewOpenVPNConnector(config *VPNConfig) *OpenVPNConnector {
	return &OpenVPNConnector{
		config: config,
	}
}

func (c *OpenVPNConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	args := []string{
		"--config", c.config.OpenVPN.ConfigFile,
		"--daemon",
	}

	// Add authentication if provided
	if c.config.OpenVPN.AuthFile != "" {
		args = append(args, "--auth-user-pass", c.config.OpenVPN.AuthFile)
	} else if c.config.OpenVPN.Username != "" && c.config.OpenVPN.Password != "" {
		// Create temporary auth file
		authFile, err := os.CreateTemp("", "openvpn-auth-*")
		if err != nil {
			return fmt.Errorf("failed to create auth file: %w", err)
		}
		defer os.Remove(authFile.Name())

		fmt.Fprintf(authFile, "%s\n%s\n", c.config.OpenVPN.Username, c.config.OpenVPN.Password)
		authFile.Close()

		args = append(args, "--auth-user-pass", authFile.Name())
	}

	c.cmd = exec.CommandContext(ctx, "openvpn", args...)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start OpenVPN: %w", err)
	}

	// Wait for connection to establish (max 30 seconds)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			c.Disconnect()
			return fmt.Errorf("OpenVPN connection timeout")
		case <-ticker.C:
			if c.checkConnection() {
				c.connected = true
				return nil
			}
		case <-ctx.Done():
			c.Disconnect()
			return ctx.Err()
		}
	}
}

func (c *OpenVPNConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill OpenVPN process: %w", err)
		}
		c.cmd.Wait()
	}

	c.connected = false
	return nil
}

func (c *OpenVPNConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *OpenVPNConnector) GetHTTPClient() (*http.Client, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("OpenVPN not connected")
	}

	// OpenVPN routes traffic through the VPN interface automatically
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}, nil
}

func (c *OpenVPNConnector) checkConnection() bool {
	// Check if tun/tap interface exists
	// This is a simplified check - in production you'd check the actual VPN interface
	_, err := net.InterfaceByName("tun0")
	if err != nil {
		_, err = net.InterfaceByName("tap0")
	}
	return err == nil
}

func (c *OpenVPNConnector) HealthCheck(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	client, err := c.GetHTTPClient()
	if err != nil {
		return err
	}

	healthURL := c.config.Options.HealthCheckURL
	if healthURL == "" {
		healthURL = "https://api.ipify.org" // Default health check
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *OpenVPNConnector) GetName() string {
	return c.config.Name
}

// WireGuardConnector handles WireGuard connections
type WireGuardConnector struct {
	config    *VPNConfig
	mu        sync.RWMutex
	connected bool
}

// NewWireGuardConnector creates a new WireGuard connector
func NewWireGuardConnector(config *VPNConfig) *WireGuardConnector {
	return &WireGuardConnector{
		config: config,
	}
}

func (c *WireGuardConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Use wg-quick to bring up the interface
	cmd := exec.CommandContext(ctx, "wg-quick", "up", c.config.WireGuard.Interface)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start WireGuard: %w", err)
	}

	// Wait for interface to be ready
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			c.Disconnect()
			return fmt.Errorf("WireGuard connection timeout")
		case <-ticker.C:
			if c.checkConnection() {
				c.connected = true
				return nil
			}
		case <-ctx.Done():
			c.Disconnect()
			return ctx.Err()
		}
	}
}

func (c *WireGuardConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	cmd := exec.Command("wg-quick", "down", c.config.WireGuard.Interface)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop WireGuard: %w", err)
	}

	c.connected = false
	return nil
}

func (c *WireGuardConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *WireGuardConnector) GetHTTPClient() (*http.Client, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("WireGuard not connected")
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}, nil
}

func (c *WireGuardConnector) checkConnection() bool {
	_, err := net.InterfaceByName(c.config.WireGuard.Interface)
	return err == nil
}

func (c *WireGuardConnector) HealthCheck(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	client, err := c.GetHTTPClient()
	if err != nil {
		return err
	}

	healthURL := c.config.Options.HealthCheckURL
	if healthURL == "" {
		healthURL = "https://api.ipify.org"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *WireGuardConnector) GetName() string {
	return c.config.Name
}

// SOCKS5Connector handles SOCKS5 proxy connections
type SOCKS5Connector struct {
	config *VPNConfig
	dialer proxy.Dialer
	mu     sync.RWMutex
}

// NewSOCKS5Connector creates a new SOCKS5 connector
func NewSOCKS5Connector(config *VPNConfig) *SOCKS5Connector {
	return &SOCKS5Connector{
		config: config,
	}
}

func (c *SOCKS5Connector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", c.config.SOCKS5.Host, c.config.SOCKS5.Port)

	var auth *proxy.Auth
	if c.config.SOCKS5.Username != "" {
		auth = &proxy.Auth{
			User:     c.config.SOCKS5.Username,
			Password: c.config.SOCKS5.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	c.dialer = dialer

	// Test connection
	conn, err := dialer.Dial("tcp", "api.ipify.org:80")
	if err != nil {
		return fmt.Errorf("SOCKS5 connection test failed: %w", err)
	}
	conn.Close()

	return nil
}

func (c *SOCKS5Connector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dialer = nil
	return nil
}

func (c *SOCKS5Connector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dialer != nil
}

func (c *SOCKS5Connector) GetHTTPClient() (*http.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.dialer == nil {
		return nil, fmt.Errorf("SOCKS5 not connected")
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Dial:                c.dialer.Dial,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}, nil
}

func (c *SOCKS5Connector) HealthCheck(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	client, err := c.GetHTTPClient()
	if err != nil {
		return err
	}

	healthURL := c.config.Options.HealthCheckURL
	if healthURL == "" {
		healthURL = "https://api.ipify.org"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	// Read IP to verify SOCKS5 is working
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read health check response: %w", err)
	}

	return nil
}

func (c *SOCKS5Connector) GetName() string {
	return c.config.Name
}

// CreateConnector creates the appropriate connector for a VPN configuration
func CreateConnector(config *VPNConfig) (Connector, error) {
	switch config.Type {
	case "openvpn":
		return NewOpenVPNConnector(config), nil
	case "wireguard":
		return NewWireGuardConnector(config), nil
	case "socks5":
		return NewSOCKS5Connector(config), nil
	default:
		return nil, fmt.Errorf("unknown VPN type: %s", config.Type)
	}
}
