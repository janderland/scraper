# Configuration Guide

This directory contains configuration files for the web scraper.

## Authentication Files

### instagram_session.txt

This file should contain your Instagram session ID.

**How to obtain:**
1. Open Instagram in your web browser
2. Log in to your account
3. Open Developer Tools (F12)
4. Navigate to Application → Cookies → https://www.instagram.com
5. Find the cookie named `sessionid`
6. Copy its value and paste it into this file

Example:
```
1234567890%3A1234567890%3A1234567890
```

### facebook_token.txt

This file should contain your Facebook Graph API access token.

**How to obtain:**
1. Go to https://developers.facebook.com/tools/explorer/
2. Click "Get Token" → "Get User Access Token"
3. Select permissions: `user_photos`, `user_posts`
4. Copy the generated token and paste it into this file

Example:
```
EAABsbCS1iHgBAOZBqc9jN1L5ZC...
```

**Note:** Access tokens expire. If scraping fails with authentication errors, regenerate the token.

## VPN Configuration

VPN configurations are stored in the database. To add a VPN:

```sql
INSERT INTO vpn_configs (name, host, port, protocol, username, password, config_path, max_bandwidth, active)
VALUES (
    'MyVPN',
    'vpn.example.com',
    1194,
    'openvpn',
    'myusername',
    'mypassword',
    '/path/to/vpn/config.ovpn',
    0,  -- 0 = unlimited bandwidth
    1   -- 1 = active
);
```

### VPN Protocol Support

- **openvpn**: OpenVPN connections
- **wireguard**: WireGuard connections
- **socks5**: SOCKS5 proxy
- **http**: HTTP/HTTPS proxy

### VPN Failover

The scraper automatically manages VPN failover:

1. Selects VPN with lowest failure count
2. Marks VPN as failed after 3 consecutive errors
3. Automatically switches to next available VPN
4. Reactivates VPNs after cooldown period

## Environment Variables

The scraper supports the following environment variables:

```bash
# Show filter suggestions for Facebook
export SHOW_SUGGESTIONS=true

# Enable debug logging
export DEBUG=1

# Set custom database path
export DB_PATH=/path/to/scraper.db

# Set custom download directory
export DOWNLOAD_DIR=/path/to/downloads

# Set custom config directory
export CONFIG_DIR=/path/to/config
```

## Credentials Security

⚠️ **Important Security Notes:**

1. Never commit credential files to version control
2. Set appropriate file permissions:
   ```bash
   chmod 600 config/instagram_session.txt
   chmod 600 config/facebook_token.txt
   ```
3. Rotate credentials regularly
4. Use read-only tokens when possible
5. Store credentials in a secure password manager

## Sample Configuration Files

### .env (optional)

Create a `.env` file in the project root:

```bash
DB_PATH=./scraper.db
DOWNLOAD_DIR=./downloads
CONFIG_DIR=./config
DEBUG=0
```

### vpn_credentials.json (optional)

Alternative VPN configuration format:

```json
{
  "vpns": [
    {
      "name": "VPN1",
      "host": "vpn1.example.com",
      "port": 1194,
      "protocol": "openvpn",
      "username": "user1",
      "password": "pass1",
      "config_path": "/etc/openvpn/vpn1.conf"
    },
    {
      "name": "VPN2",
      "host": "vpn2.example.com",
      "port": 51820,
      "protocol": "wireguard",
      "config_path": "/etc/wireguard/wg0.conf"
    }
  ]
}
```

## Testing Configuration

For testing without actual credentials:

1. Use mock mode (if implemented)
2. Test with sample data in `testdata/` directory
3. Use temporary credentials for testing purposes
