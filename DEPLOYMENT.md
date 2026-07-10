# cookinc — VPS Deployment

Full setup guide for the **Linux VPS side** of cookinc (the sink / `cookinc-mcp`).

## Prerequisites

- Linux VPS (tested on Ubuntu 24.04, DigitalOcean 1vCPU/2GB)
- Go 1.22+ (if building from source)
- Chrome installed (for cookie injection to real profile)

## 1. Build (or download)

```bash
# From source
git clone https://github.com/XDayonline/cookinc
cd cookinc
go build -o cookinc-mcp ./cmd/mcp

# Or copy from a cross-compiled Windows build
cp /path/to/cookinc-mcp-linux .
```

## 2. Init

Generate the config and encryption key:

```bash
./cookinc-mcp init --secret "$(openssl rand -hex 32)" --listen "0.0.0.0:9876"
```

This creates `~/.config/cookinc/sink.yaml`. The secret must match the one on your Windows client.

## 3. Systemd (auto-start on boot)

Create `~/.config/systemd/user/cookinc-mcp.service`:

```ini
[Unit]
Description=cookinc-mcp — session sync sink for AI agents
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%h/Dev/cookinc/cookinc-mcp start
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
```

Enable and start:

```bash
systemctl --user daemon-reload
systemctl --user enable cookinc-mcp.service
systemctl --user start cookinc-mcp.service
```

Check status:

```bash
systemctl --user status cookinc-mcp.service
```

> Ensure `loginctl enable-linger $USER` is set so user services survive logout/reboot.

## 4. Cloudflare Tunnel (optional)

If you use [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) to expose services without opening ports:

Add an ingress rule to `~/.cloudflared/config.yml`:

```yaml
  - hostname: sync.example.com
    service: http://127.0.0.1:9876
```

Route DNS and restart:

```bash
cloudflared tunnel route dns <tunnel-name> sync.example.com
kill -HUP <cloudflared-pid>    # or restart the tunnel
```

The sink is then reachable at `https://sync.example.com` — no SSH tunnel needed.

## 5. Connect Hermes MCP

```bash
hermes config set mcp.cookinc "http://127.0.0.1:9898"
/restart
```

Now agents can query cookies:

```
get_cookies("github.com")
```

## 6. Health check

```bash
# Sink responds on / (POST for cookies, GET returns 404 — normal)
curl -w "%{http_code}" http://127.0.0.1:9876/

# MCP endpoint
curl -w "%{http_code}" http://127.0.0.1:9898/

# Logs
journalctl --user -u cookinc-mcp -f
```
