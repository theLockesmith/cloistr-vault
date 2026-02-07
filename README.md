# Coldforge Vault

A zero-knowledge password manager with multi-platform support and Nostr integration.

## 🔒 Security Features

- **Zero-Knowledge Architecture**: Server never has access to your decrypted data
- **Multiple Authentication Methods**: Email/password, Nostr keypairs, master password + keyfile
- **Client-Side Encryption**: All encryption/decryption happens on your device
- **Open Source**: Fully auditable codebase

## 🏗️ Architecture

- **Backend**: Go with PostgreSQL
- **Web**: React with TypeScript
- **Mobile**: React Native (iOS/Android)  
- **Desktop**: Electron (Windows/macOS/Linux)
- **Browser Extension**: Chrome/Firefox compatible
- **Deployment**: Docker, Kubernetes, or bare metal

## 🚀 Quick Start

### Development
```bash
# Start backend
cd backend && go run cmd/server/main.go

# Start web frontend
cd frontend/web && npm start
```

### Production
```bash
# Docker Compose
docker-compose up -d

# Kubernetes
kubectl apply -f deployments/kubernetes/
```

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage
```

## 📚 Documentation

- [API Documentation](docs/api.md)
- [Deployment Guide](docs/deployment.md)
- [Security Model](docs/security.md)
- [Development Setup](docs/development.md)

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.