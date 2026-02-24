#!/bin/bash

set -e

if [ -z "$SERVER" ] || [ -z "$USER" ] || [ -z "$DEPLOY_PATH" ]; then
    echo "Error: SERVER, USER, and DEPLOY_PATH environment variables must be set"
    exit 1
fi

echo "Deploying to $USER@$SERVER:$DEPLOY_PATH..."

echo "1. Syncing files to server..."
rsync -avz --delete \
    --exclude='.git' \
    --exclude='.devenv' \
    --exclude='.env' \
    --exclude='*.log' \
    --exclude='.github/' \
    ./ $USER@$SERVER:$DEPLOY_PATH/

echo "2. Building and deploying on server..."
ssh $USER@$SERVER << EOF
set -e
export PATH=\$PATH:/usr/local/go/bin

cd $DEPLOY_PATH

echo "Removing local replace directives..."
go mod edit -dropreplace github.com/vultisig/recipes
go mod tidy

echo "Building mcp-server binary..."
go build -o mcp-server ./cmd/mcp-server/

echo "Stopping service before binary replacement..."
sudo systemctl stop mcp || true

echo "Installing binary to /usr/local/bin/..."
sudo cp mcp-server /usr/local/bin/
sudo chmod +x /usr/local/bin/mcp-server

if [ ! -f "/usr/local/bin/mcp-server" ]; then
    echo "ERROR: mcp-server binary not found in /usr/local/bin/"
    exit 1
fi

echo "Binary installation successful:"
ls -la /usr/local/bin/mcp-server

echo "Restarting mcp service..."
sudo systemctl restart mcp

echo "Checking service status..."
sleep 2
sudo systemctl status mcp --no-pager -l
EOF

echo "Deployment finished successfully!"
