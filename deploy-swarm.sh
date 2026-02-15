#!/bin/bash
set -e

# Make It Public - Docker Swarm Deployment Script
# This script deploys the make-it-public stack to a Docker Swarm cluster

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running in a Swarm
if ! docker info 2>/dev/null | grep -q "Swarm: active"; then
    print_error "Docker Swarm is not active. Please initialize or join a swarm first."
    exit 1
fi

# Check required environment variables
REQUIRED_VARS=("DOMAIN_NAME" "EMAIL" "CLOUDFLARE_API_TOKEN" "AUTH_SALT")
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING_VARS+=("$var")
    fi
done

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    print_error "Missing required environment variables:"
    for var in "${MISSING_VARS[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "Usage:"
    echo "  export DOMAIN_NAME=your-domain.com"
    echo "  export EMAIL=your-email@example.com"
    echo "  export CLOUDFLARE_API_TOKEN=your-cloudflare-token"
    echo "  export AUTH_SALT=your-random-salt"
    echo "  export MIT_VERSION=v1.0.0  # optional, defaults to 'latest'"
    echo ""
    echo "  ./deploy-swarm.sh"
    exit 1
fi

# Set default version if not provided
MIT_VERSION=${MIT_VERSION:-latest}

# Check if cloudlab-public network exists
if ! docker network inspect cloudlab-public >/dev/null 2>&1; then
    print_warn "Network 'cloudlab-public' does not exist. Creating it..."
    docker network create --driver overlay --attachable cloudlab-public
    print_info "Network 'cloudlab-public' created."
fi

# Display configuration
print_info "Deploying Make It Public to Docker Swarm"
echo "  Domain:  $DOMAIN_NAME"
echo "  Email:   $EMAIL"
echo "  Version: $MIT_VERSION"
echo ""

# Deploy the stack
print_info "Deploying stack 'makeitpublic'..."
docker stack deploy -c docker-stack.yml makeitpublic

# Wait a moment for services to initialize
sleep 3

# Show service status
print_info "Deployment completed. Current service status:"
docker stack services makeitpublic

echo ""
print_info "Deployment commands:"
echo "  Check services:  docker stack services makeitpublic"
echo "  View logs:       docker service logs makeitpublic_mitserver -f"
echo "  Remove stack:    docker stack rm makeitpublic"
