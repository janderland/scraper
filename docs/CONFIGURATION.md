# Configuration Guide

## Overview

The scraper uses a single YAML configuration file for all settings including database, storage, and VPN configurations.

## Configuration File Location

Default locations (in order of precedence):
1. Path specified with `--config` flag
2. `./config/vpn.yaml`
3. `./vpn.yaml`

## Configuration Structure

```yaml
# Database settings
database:
  path: "./scraper.db"

# Storage settings
storage:
  media_dir: "./downloads"

# VPN configurations (optional)
vpns:
  - name: "VPN1"
    type: "openvpn"
    # ... VPN-specific config
```

## Database Configuration

```yaml
database:
  path: "./scraper.db"  # Path to SQLite database file
```

### Options

- **path** (required): Path to SQLite database file
  - Relative paths are relative to working directory
  - Absolute paths supported: `/var/lib/scraper/db.sqlite`
  - Parent directories are created automatically

### Examples

```yaml
# Local database
database:
  path: "./scraper.db"

# System-wide database
database:
  path: "/var/lib/scraper/scraper.db"

# User-specific database
database:
  path: "~/.local/share/scraper/scraper.db"
```

## Storage Configuration

```yaml
storage:
  media_dir: "./downloads"  # Root directory for media files
```

### Directory Structure

Media files are organized as:
```
media_dir/
├── reddit/
│   └── 2024/
│       ├── 01/
│       │   ├── abc123.jpg
│       │   └── def456.mp4
│       └── 02/
├── instagram/
└── facebook/
```

### Options

- **media_dir** (required): Root directory for downloaded media
  - Subdirectories created automatically
  - Format: `{media_dir}/{platform}/{year}/{month}/{hash}.{ext}`

### Examples

```yaml
# Local storage
storage:
  media_dir: "./downloads"

# External drive
storage:
  media_dir: "/mnt/media/scraper"

# Network storage
storage:
  media_dir: "/nas/media/scraper"
```

## VPN Configuration

See [VPN_SETUP.md](VPN_SETUP.md) for complete VPN documentation.

### Basic Structure

```yaml
vpns:
  - name: "VPN Name"
    type: "openvpn|wireguard|socks5"
    enabled: true
    priority: 10
    # Type-specific configuration
```

### VPN Types

1. **OpenVPN**: Full VPN tunnel
2. **WireGuard**: Modern VPN protocol
3. **SOCKS5**: Proxy-based connection

See [VPN_SETUP.md](VPN_SETUP.md) for detailed configuration of each type.

## Environment Variables

Configuration values can use environment variables:

```yaml
database:
  path: "${DB_PATH:-./scraper.db}"  # Use $DB_PATH or default

storage:
  media_dir: "${MEDIA_DIR:-./downloads}"
```

Usage:
```bash
export DB_PATH="/var/lib/scraper/scraper.db"
export MEDIA_DIR="/mnt/storage/media"
./scraper -platform reddit -source pics
```

## Multiple Configurations

### Development vs Production

**config/dev.yaml**:
```yaml
database:
  path: "./dev.db"

storage:
  media_dir: "./dev-downloads"

vpns: []  # No VPNs in development
```

**config/prod.yaml**:
```yaml
database:
  path: "/var/lib/scraper/prod.db"

storage:
  media_dir: "/mnt/media/scraper"

vpns:
  - name: "ProductionVPN"
    type: "wireguard"
    enabled: true
    # ... production VPN config
```

Usage:
```bash
# Development
./scraper --config config/dev.yaml -platform reddit -source pics

# Production
./scraper --config config/prod.yaml -platform reddit -source pics
```

### Per-Platform Configurations

**config/reddit.yaml**:
```yaml
database:
  path: "./reddit.db"

storage:
  media_dir: "./reddit-media"

vpns:
  - name: "RedditVPN"
    # ... Reddit-specific VPN
```

**config/instagram.yaml**:
```yaml
database:
  path: "./instagram.db"

storage:
  media_dir: "./instagram-media"

vpns:
  - name: "InstagramVPN"
    # ... Instagram-specific VPN
```

## Configuration Validation

The scraper validates configuration on startup:

```bash
./scraper --config config/vpn.yaml --validate
```

Common validation errors:
- Missing required fields
- Invalid file paths
- Malformed VPN configuration
- YAML syntax errors

## Security Best Practices

1. **File Permissions**:
   ```bash
   chmod 600 config/vpn.yaml
   ```

2. **Separate Credentials**:
   ```yaml
   vpns:
     - name: "VPN"
       openvpn:
         auth_file: "/etc/openvpn/auth.txt"  # Not in repo
   ```

3. **Gitignore**:
   ```gitignore
   config/vpn.yaml
   config/*auth*
   config/*credentials*
   ```

4. **Environment Variables** for sensitive data:
   ```yaml
   vpns:
     - name: "VPN"
       socks5:
         username: "${SOCKS_USER}"
         password: "${SOCKS_PASS}"
   ```

## Configuration Examples

### Minimal (No VPNs)

```yaml
database:
  path: "./scraper.db"

storage:
  media_dir: "./downloads"

vpns: []
```

### Single VPN

```yaml
database:
  path: "./scraper.db"

storage:
  media_dir: "./downloads"

vpns:
  - name: "MyVPN"
    type: "wireguard"
    enabled: true
    priority: 10

    wireguard:
      config_file: "/etc/wireguard/wg0.conf"
      interface: "wg0"

    options:
      auto_connect: true
      health_check_interval: 60
```

### Multiple VPNs with Failover

```yaml
database:
  path: "./scraper.db"

storage:
  media_dir: "./downloads"

vpns:
  - name: "Primary"
    type: "wireguard"
    enabled: true
    priority: 100

    wireguard:
      config_file: "/etc/wireguard/wg0.conf"
      interface: "wg0"

    options:
      auto_connect: true
      health_check_interval: 60

  - name: "Backup"
    type: "openvpn"
    enabled: true
    priority: 50

    openvpn:
      config_file: "/etc/openvpn/backup.ovpn"
      auth_file: "/etc/openvpn/auth.txt"

    options:
      auto_connect: false
      health_check_interval: 120

  - name: "Fallback"
    type: "socks5"
    enabled: true
    priority: 10

    socks5:
      host: "localhost"
      port: 1080

    options:
      health_check_interval: 60
```

## Troubleshooting

### Configuration Not Found

```
Error: failed to load config: failed to read config file: no such file or directory
```

**Solution**: Specify config path explicitly:
```bash
./scraper --config /path/to/config.yaml
```

### Invalid YAML Syntax

```
Error: failed to parse YAML: yaml: line 10: did not find expected key
```

**Solution**: Validate YAML syntax:
```bash
# Install yamllint
pip install yamllint

# Validate config
yamllint config/vpn.yaml
```

### Permission Denied

```
Error: failed to create database: permission denied
```

**Solution**: Check file/directory permissions:
```bash
# Make directories writable
mkdir -p $(dirname /path/to/scraper.db)
chmod 755 $(dirname /path/to/scraper.db)

# Or use different path
```

## See Also

- [VPN_SETUP.md](VPN_SETUP.md) - Complete VPN configuration guide
- [EXAMPLES.md](../EXAMPLES.md) - Usage examples
- [README.md](../README.md) - Main documentation
