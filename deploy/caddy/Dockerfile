# Dockerfile for Caddy deployment with embedded configuration
# This image includes the Caddyfile and static assets baked in
# Eliminates the need for volume mounts from the host filesystem

FROM ghcr.io/caddybuilds/caddy-cloudflare:2.10-alpine

# Copy Caddyfile configuration
COPY runtime/Caddyfile /etc/caddy/Caddyfile

# Copy static website files
COPY static /srv

# Caddy will read config from /etc/caddy/Caddyfile by default
# Static files are served from /srv as configured in Caddyfile
