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

Run the installer from a verified checkout and supply an independently obtained
SHA-256 checksum for the exact release. Do not use a checksum fetched from the
same unverified origin as the only authenticity check.

```bash
VERSION=vX.Y.Z
EXPECTED_SHA256=the-checksum-from-a-trusted-channel

sudo env \
  AGENT_VERSION="$VERSION" \
  AGENT_SHA256="$EXPECTED_SHA256" \
  ./scripts/install.sh \
  "your-agent-token" \
  "https://your-sitekit-instance.com" \
  "your-server-id"
```

The installer downloads into a root-owned temporary directory, verifies the
binary before installation, and keeps both `/opt/sitekit` and the systemd
service file root-owned.

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

`saas_url` must use HTTPS except for an explicit loopback development endpoint.
SSH deployments require a trusted `ssh_known_hosts` entry and a full commit
hash. File and log jobs must identify the managed `username`; a caller-supplied
`base_path` is not treated as an authorization boundary.

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
