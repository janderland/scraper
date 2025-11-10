# VPN Setup Guide

This guide explains how to configure and use VPN support in the web scraper.

## Overview

The scraper supports three types of VPN connections:
- **OpenVPN** - Industry-standard VPN protocol
- **WireGuard** - Modern, fast VPN protocol
- **SOCKS5** - Proxy-based connections

VPNs are configured via a YAML configuration file that also includes database and storage settings.

## Configuration File

### Location

Create a configuration file at one of these locations:
- `./config/vpn.yaml` (default)
- Custom path via `--config` flag

### Basic Structure

```yaml
# Required: Database configuration
database:
  path: "./scraper.db"

# Required: Storage configuration
storage:
  media_dir: "./downloads"

# Optional: VPN configurations
vpns:
  - name: "MyVPN"
    type: "openvpn"  # or "wireguard", "socks5"
    enabled: true
    priority: 10
    # ... type-specific config
```

## OpenVPN Setup

### Prerequisites

1. Install OpenVPN:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install openvpn

   # macOS
   brew install openvpn

   # Fedora/RHEL
   sudo dnf install openvpn
   ```

2. Obtain `.ovpn` configuration file from your VPN provider

### Configuration

```yaml
vpns:
  - name: "MyOpenVPN"
    type: "openvpn"
    enabled: true
    priority: 10

    openvpn:
      config_file: "/etc/openvpn/client/myconfig.ovpn"

      # Option 1: Inline credentials (less secure)
      username: "myusername"
      password: "mypassword"

      # Option 2: Auth file (recommended)
      auth_file: "/etc/openvpn/auth.txt"

    options:
      auto_connect: true
      retry_attempts: 3
      retry_delay: 5
      health_check_url: "https://api.ipify.org"
      health_check_interval: 60
```

### Auth File Format

Create `/etc/openvpn/auth.txt`:
```
your_username
your_password
```

Set permissions:
```bash
chmod 600 /etc/openvpn/auth.txt
```

### Permissions

OpenVPN requires root/sudo privileges:

```bash
# Run scraper with sudo
sudo ./scraper -platform reddit -source pics -max 100

# Or configure sudoers to allow OpenVPN without password
echo "$USER ALL=(ALL) NOPASSWD: /usr/sbin/openvpn" | sudo tee /etc/sudoers.d/openvpn
```

## WireGuard Setup

### Prerequisites

1. Install WireGuard:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install wireguard

   # macOS
   brew install wireguard-tools

   # Fedora/RHEL
   sudo dnf install wireguard-tools
   ```

2. Obtain `.conf` configuration file from your VPN provider

### Configuration

```yaml
vpns:
  - name: "MyWireGuard"
    type: "wireguard"
    enabled: true
    priority: 20

    wireguard:
      config_file: "/etc/wireguard/wg0.conf"
      interface: "wg0"

    options:
      auto_connect: true
      retry_attempts: 3
      retry_delay: 5
      health_check_url: "https://api.ipify.org"
      health_check_interval: 60
```

### WireGuard Config Example

`/etc/wireguard/wg0.conf`:
```ini
[Interface]
PrivateKey = YOUR_PRIVATE_KEY
Address = 10.0.0.2/24
DNS = 1.1.1.1

[Peer]
PublicKey = SERVER_PUBLIC_KEY
Endpoint = vpn.example.com:51820
AllowedIPs = 0.0.0.0/0
```

### Permissions

WireGuard also requires root/sudo:

```bash
# Allow wg-quick without password
echo "$USER ALL=(ALL) NOPASSWD: /usr/bin/wg-quick" | sudo tee /etc/sudoers.d/wireguard
```

## SOCKS5 Proxy Setup

### Prerequisites

SOCKS5 is the simplest option - just need proxy server details.

No system-level installation required.

### Configuration

```yaml
vpns:
  - name: "MySocks5Proxy"
    type: "socks5"
    enabled: true
    priority: 5

    socks5:
      host: "127.0.0.1"  # or proxy.example.com
      port: 1080

      # Optional authentication
      username: "proxyuser"
      password: "proxypass"

    options:
      auto_connect: false  # SOCKS5 connects automatically
      health_check_url: "https://api.ipify.org"
      health_check_interval: 60
```

### Testing SOCKS5 Connection

```bash
# Test with curl
curl --socks5 127.0.0.1:1080 https://api.ipify.org

# With authentication
curl --socks5 username:password@127.0.0.1:1080 https://api.ipify.org
```

## Multiple VPNs and Failover

### Load Balancing

Configure multiple VPNs with different priorities:

```yaml
vpns:
  - name: "Primary"
    type: "wireguard"
    enabled: true
    priority: 100  # Highest priority - used first

  - name: "Secondary"
    type: "openvpn"
    enabled: true
    priority: 50

  - name: "Fallback"
    type: "socks5"
    enabled: true
    priority: 10  # Lowest priority - used last
```

### How Selection Works

The scraper selects VPNs based on:
1. **Priority** (higher is better)
2. **Failure count** (fewer failures preferred)
3. **Idle time** (recently unused VPNs get bonus)
4. **Health check status** (failing health checks penalized)

### Automatic Failover

If a VPN fails:
1. Failure count increases
2. After 5 failures, VPN is disconnected
3. Next request uses different VPN
4. Health checks attempt to recover failed VPN

## Health Checks

Health checks verify VPN connectivity:

```yaml
options:
  health_check_url: "https://api.ipify.org"  # URL to test
  health_check_interval: 60  # Check every 60 seconds
```

### Default Behavior

- Checks run every 60 seconds
- Failed checks increase failure count
- 3 consecutive failures disconnect VPN
- Successful check resets failure count

### Custom Health Check URLs

```yaml
# Check IP address
health_check_url: "https://api.ipify.org"

# Check specific service
health_check_url: "https://www.reddit.com"

# Check internal service
health_check_url: "http://192.168.1.1/health"
```

## Running Without VPNs

To run without VPNs, use minimal config:

```yaml
database:
  path: "./scraper.db"

storage:
  media_dir: "./downloads"

vpns: []  # Empty array = no VPNs
```

Or use the minimal example:
```bash
cp config/vpn.minimal.yaml config/vpn.yaml
```

## Docker Setup

For Docker deployments, use SOCKS5 proxy:

### docker-compose.yml

```yaml
version: '3.8'

services:
  scraper:
    image: web-scraper:latest
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./media:/app/media
    depends_on:
      - proxy

  proxy:
    image: serjs/go-socks5-proxy
    ports:
      - "1080:1080"
```

### Configuration

```yaml
database:
  path: "/app/data/scraper.db"

storage:
  media_dir: "/app/media"

vpns:
  - name: "docker-proxy"
    type: "socks5"
    enabled: true
    priority: 10

    socks5:
      host: "proxy"  # Docker service name
      port: 1080

    options:
      health_check_url: "https://api.ipify.org"
```

## Troubleshooting

### OpenVPN Issues

**Problem:** "Failed to start OpenVPN"

**Solutions:**
1. Check OpenVPN is installed: `which openvpn`
2. Verify config file exists and is readable
3. Check permissions (needs root/sudo)
4. Test manually: `sudo openvpn --config /path/to/config.ovpn`

### WireGuard Issues

**Problem:** "WireGuard connection timeout"

**Solutions:**
1. Check WireGuard is installed: `which wg-quick`
2. Verify interface name matches config
3. Check permissions (needs root/sudo)
4. Test manually: `sudo wg-quick up wg0`

### SOCKS5 Issues

**Problem:** "SOCKS5 connection test failed"

**Solutions:**
1. Verify proxy is running: `telnet host port`
2. Check firewall rules
3. Verify credentials if using authentication
4. Test with curl: `curl --socks5 host:port https://api.ipify.org`

### Health Check Failures

**Problem:** VPNs keep disconnecting

**Solutions:**
1. Increase health check interval
2. Use different health check URL
3. Check VPN server stability
4. Review scraper logs for errors

## Security Best Practices

1. **Use auth files** instead of inline credentials
2. **Set restrictive permissions** on config files: `chmod 600`
3. **Never commit** config files with credentials to git
4. **Use environment variables** for sensitive data
5. **Rotate credentials** regularly
6. **Monitor health checks** for unusual patterns

## Performance Tuning

### Optimal Settings

```yaml
vpns:
  - name: "FastVPN"
    type: "wireguard"  # WireGuard is fastest
    enabled: true
    priority: 100
    max_bandwidth: 0  # Unlimited

    options:
      health_check_interval: 120  # Less frequent checks
```

### Bandwidth Limiting

```yaml
vpns:
  - name: "LimitedVPN"
    max_bandwidth: 10485760  # 10 MB/s in bytes
```

## Example Configurations

See the `config/` directory for examples:
- `vpn.example.yaml` - Full featured configuration
- `vpn.minimal.yaml` - No VPNs
- `vpn.docker.yaml` - Docker deployment

## CLI Usage

```bash
# Use default config
./scraper --config config/vpn.yaml -platform reddit -source pics

# Use custom config
./scraper --config /path/to/custom.yaml -platform reddit -source pics

# Check VPN status
./scraper --config config/vpn.yaml --vpn-status
```

## Monitoring

### View VPN Statistics

The scraper tracks:
- Connection status
- Bytes transferred
- Request count
- Failure count
- Last used time
- Health check status

Access via GUI or API (if implemented).

## Advanced Topics

### Custom Connector

To implement a custom VPN type, create a new connector in `internal/vpn/connector.go`:

```go
type MyCustomConnector struct {
    config *VPNConfig
}

func (c *MyCustomConnector) Connect(ctx context.Context) error {
    // Implementation
}

// Implement other Connector interface methods
```

### Integration with Other Tools

The scraper's VPN manager can be used with:
- Network namespaces (Linux)
- VPN clients (OpenConnect, Cisco AnyConnect)
- Tor networks
- Custom proxy chains

## Support

For issues or questions:
1. Check troubleshooting section
2. Review example configurations
3. Open GitHub issue with logs and config (redact credentials!)
