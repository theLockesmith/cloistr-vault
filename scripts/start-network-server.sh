#!/bin/bash

# Coldforge Vault Network Test Server
set -e

NETWORK_IP="10.0.0.169"
API_PORT="7700"
WEB_PORT="7701"
EXPO_PORT="7702"
DOWNLOAD_PORT="7703"

echo "🚀 Starting Coldforge Vault Network Test Servers"
echo "=================================================="
echo "Network IP: $NETWORK_IP"
echo ""

# Kill any existing processes on these ports
echo "🧹 Cleaning up existing processes..."
pkill -f "react-scripts start" || true
pkill -f "expo start" || true
pkill -f "go run" || true
pkill -f "python -m http.server" || true
sleep 2

# Function to check if port is free
check_port() {
    if lsof -Pi :$1 -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "⚠️  Port $1 is in use, trying to free it..."
        lsof -ti:$1 | xargs kill -9 2>/dev/null || true
        sleep 1
    fi
}

# Check and free ports
check_port $API_PORT
check_port $WEB_PORT
check_port $EXPO_PORT
check_port $DOWNLOAD_PORT

# Start API Server (Go backend)
echo "🔧 Starting API server on port $API_PORT..."
cd /home/forgemaster/Development/Coldforge-Vault/backend
export PATH=$PATH:/usr/local/go/bin
export HOST=0.0.0.0
export PORT=$API_PORT
nohup go run cmd/server/main.go > ../logs/api.log 2>&1 &
API_PID=$!
echo "✅ API Server started (PID: $API_PID)"

# Wait for API to start
sleep 3

# Start Web Frontend
echo "🌐 Starting web frontend on port $WEB_PORT..."
cd /home/forgemaster/Development/Coldforge-Vault/frontend/web
export HOST=0.0.0.0
export PORT=$WEB_PORT
nohup npm start > ../../logs/web.log 2>&1 &
WEB_PID=$!
echo "✅ Web Frontend started (PID: $WEB_PID)"

# Start Expo Mobile App
echo "📱 Starting Expo mobile app on port $EXPO_PORT..."
cd /home/forgemaster/Development/Coldforge-Vault/frontend/mobile-expo
export EXPO_DEVTOOLS_LISTEN_ADDRESS=0.0.0.0
nohup npx expo start --tunnel --port $EXPO_PORT > ../../logs/expo.log 2>&1 &
EXPO_PID=$!
echo "✅ Expo Mobile App started (PID: $EXPO_PID)"

# Start Download Server
echo "📦 Starting download server on port $DOWNLOAD_PORT..."
cd /home/forgemaster/Development/Coldforge-Vault
mkdir -p downloads logs
cd downloads
nohup python3 -m http.server $DOWNLOAD_PORT > ../logs/download.log 2>&1 &
DOWNLOAD_PID=$!
echo "✅ Download Server started (PID: $DOWNLOAD_PID)"

# Wait for services to start
echo "⏳ Waiting for services to initialize..."
sleep 5

# Test services
echo ""
echo "🧪 Testing services..."

# Test API
if curl -s http://$NETWORK_IP:$API_PORT/api/v1/health > /dev/null; then
    echo "✅ API Server: http://$NETWORK_IP:$API_PORT"
else
    echo "❌ API Server failed to start"
fi

# Test Web (just check if port is open)
if nc -z $NETWORK_IP $WEB_PORT 2>/dev/null; then
    echo "✅ Web Frontend: http://$NETWORK_IP:$WEB_PORT"
else
    echo "⏳ Web Frontend: http://$NETWORK_IP:$WEB_PORT (still starting...)"
fi

# Test Download Server
if curl -s http://$NETWORK_IP:$DOWNLOAD_PORT > /dev/null; then
    echo "✅ Download Server: http://$NETWORK_IP:$DOWNLOAD_PORT"
else
    echo "❌ Download Server failed to start"
fi

echo ""
echo "🎉 Coldforge Vault Test Environment Ready!"
echo "=========================================="
echo ""
echo "📱 MOBILE TESTING:"
echo "   1. Install 'Expo Go' app on your phone"
echo "   2. Scan QR code from: http://$NETWORK_IP:$EXPO_PORT"
echo "   3. Or check logs: tail -f logs/expo.log"
echo ""
echo "🌐 WEB TESTING:"
echo "   Frontend: http://$NETWORK_IP:$WEB_PORT"
echo "   API: http://$NETWORK_IP:$API_PORT/api/v1/info"
echo ""
echo "📦 DOWNLOADS:"
echo "   Download site: http://$NETWORK_IP:$DOWNLOAD_PORT"
echo ""
echo "📊 MONITORING:"
echo "   API logs: tail -f logs/api.log"
echo "   Web logs: tail -f logs/web.log"
echo "   Expo logs: tail -f logs/expo.log"
echo ""
echo "🛑 TO STOP ALL SERVICES:"
echo "   kill $API_PID $WEB_PID $EXPO_PID $DOWNLOAD_PID"
echo "   Or run: ./scripts/stop-network-server.sh"
echo ""

# Save PIDs for cleanup script
echo "$API_PID $WEB_PID $EXPO_PID $DOWNLOAD_PID" > /tmp/coldforge_pids.txt

echo "🎯 Ready to test! Use password 'demo123' in mobile app"