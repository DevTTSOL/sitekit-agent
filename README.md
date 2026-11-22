# SiteKit Agent

A lightweight server management agent for the SiteKit platform. This agent runs on managed servers and handles provisioning, deployments, service management, and health monitoring.

## Features

- Server provisioning and configuration
- Web application deployment (PHP, Laravel, WordPress, Node.js)
- Database management (MariaDB, PostgreSQL)
- SSL certificate management (Let's Encrypt)
- Service monitoring and auto-restart
- Cron job management
- Firewall rule management
- Real-time server metrics and health checks

## Installation

The agent is automatically installed when you provision a server through the SiteKit platform. The provisioning script handles:

1. Installing required dependencies
2. Setting up the web stack (Nginx, PHP, MariaDB, etc.)
3. Configuring the agent service
4. Registering the server with your SiteKit instance

## Manual Installation

```bash
# Download the agent binary
VERSION=vX.Y.Z
RELEASE=https://github.com/DevTTSOL/sitekit-agent/releases/download
curl -fsSL "$RELEASE/$VERSION/sentinel-linux-amd64" -o /tmp/sentinel-linux-amd64
curl -fsSL "$RELEASE/$VERSION/SHA256SUMS" -o /tmp/SHA256SUMS
(cd /tmp && grep ' sentinel-linux-amd64$' SHA256SUMS | sha256sum -c -) || { echo "Checksum verification failed" >&2; exit 1; }
install -m 755 /tmp/sentinel-linux-amd64 /opt/sitekit/bin/sentinel

# Configure the agent
cat > /opt/sitekit/agent.yaml << EOF
server_id: "your-server-id"
saas_url: "https://your-sitekit-instance.com"
agent_token: "your-agent-token"
poll_interval: "5s"
stats_interval: "60s"
log_level: "info"
EOF

# Start the agent
systemctl start sitekit-agent
```

## Building from Source

```bash
# Clone the repository
git clone https://github.com/DevTTSOL/sitekit-agent.git
cd sitekit-agent

# Build stripped Linux binaries without build-machine paths
make build-linux

# Build and verify release artifacts and checksums
make release
```

Tagged `v*` pushes run [the release workflow](.github/workflows/release.yml) and
publish amd64 and arm64 Linux artifacts with a `SHA256SUMS` manifest. Builds use
`-trimpath`, strip symbols, and reject known build-machine paths.

## Architecture

```
sitekit-agent/
├── cmd/sentinel/       # Main entry point
├── internal/
│   ├── agent/          # Core agent logic
│   ├── comm/           # API communication
│   ├── executor/       # Job execution handlers
│   ├── crypto/         # Encryption utilities
│   └── health/         # Health monitoring
└── pkg/                # Shared packages
```

## Configuration

The agent reads configuration from `/opt/sitekit/agent.yaml`:

| Option | Description | Default |
|--------|-------------|---------|
| `server_id` | Unique server identifier | Required |
| `saas_url` | SiteKit platform URL | Required |
| `agent_token` | Authentication token | Required |
| `poll_interval` | Job polling interval | `5s` |
| `stats_interval` | Stats reporting interval | `60s` |
| `log_level` | Logging level (debug, info, warn, error) | `info` |

## Developed By

[AvanSaber.com](https://avansaber.com)

## License

Copyright (c) 2024-2025 AvanSaber

This software is licensed under the **SiteKit Source Available License**.

### Grant of Rights

You are permitted to use, copy, modify, and distribute this software for any purpose, subject to the following limitations:

### Limitations

1. **Server Limit**: You may not use this software to manage more than **20 servers** without obtaining a commercial license.

2. **Revenue Limit**: If you offer this software as a hosted service (SaaS), your annual revenue from such service must not exceed **$100,000 USD** without obtaining a commercial license.

4. **Attribution**: You must retain this license notice and attribution in all copies or substantial portions of the software.

### Commercial License

For usage beyond these limits, please contact [licensing@avansaber.com](mailto:licensing@avansaber.com) for a commercial license.

### Warranty Disclaimer

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
