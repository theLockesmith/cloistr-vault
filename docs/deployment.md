# Deployment Guide

## Overview

Coldforge Vault can be deployed in multiple ways to suit different needs:

- **Docker Compose** - Quick local/development deployment
- **Kubernetes** - Production-ready container orchestration  
- **Bare Metal** - Direct server installation
- **Cloud Providers** - AWS, GCP, Azure, DigitalOcean

## Prerequisites

### Required:
- **PostgreSQL 12+** - Primary database
- **Go 1.21+** - Backend runtime
- **Node.js 18+** - Frontend build tools

### Optional:
- **Redis** - Session storage and rate limiting
- **Nginx** - Reverse proxy and SSL termination
- **Docker** - Containerization
- **Kubernetes** - Container orchestration

## Quick Start (Docker Compose)

### 1. Clone and Configure
```bash
git clone https://github.com/coldforge/vault.git
cd vault
cp .env.example .env
```

### 2. Update Environment Variables
```bash
# Generate secure secrets
openssl rand -hex 32  # Use for JWT_SECRET
openssl rand -hex 16  # Use for REDIS_PASSWORD

# Edit .env file
nano .env
```

### 3. Deploy
```bash
# Start all services
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f vault-api
```

### 4. Verify Deployment
```bash
# Health check
curl http://localhost:8080/api/v1/health

# API info
curl http://localhost:8080/api/v1/info
```

## Production Kubernetes Deployment

### 1. Prepare Kubernetes Cluster
```bash
# Ensure kubectl is configured
kubectl cluster-info

# Create namespace
kubectl apply -f deployments/kubernetes/namespace.yaml
```

### 2. Configure Secrets
```bash
# Update secrets with real values
cp deployments/kubernetes/secret.yaml deployments/kubernetes/secret-prod.yaml

# Edit with base64-encoded production secrets
nano deployments/kubernetes/secret-prod.yaml

# Apply secrets
kubectl apply -f deployments/kubernetes/secret-prod.yaml
```

### 3. Deploy PostgreSQL
```bash
# Apply PostgreSQL resources
kubectl apply -f deployments/kubernetes/postgres.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n coldforge-vault --timeout=300s
```

### 4. Deploy Application
```bash
# Build and push Docker image
docker build -t your-registry/coldforge-vault:v1.0.0 .
docker push your-registry/coldforge-vault:v1.0.0

# Update image in deployment
sed -i 's|coldforge/vault:latest|your-registry/coldforge-vault:v1.0.0|' deployments/kubernetes/app.yaml

# Deploy application
kubectl apply -f deployments/kubernetes/app.yaml

# Wait for application
kubectl wait --for=condition=ready pod -l app=vault-api -n coldforge-vault --timeout=300s
```

### 5. Configure Ingress
```bash
# Update domain in ingress.yaml
sed -i 's|vault.yourdomain.com|vault.your-actual-domain.com|' deployments/kubernetes/app.yaml

# Apply ingress
kubectl apply -f deployments/kubernetes/app.yaml
```

### 6. Verify Deployment
```bash
# Check pod status
kubectl get pods -n coldforge-vault

# Check services
kubectl get services -n coldforge-vault

# Check logs
kubectl logs -f deployment/vault-api -n coldforge-vault
```

## Bare Metal Deployment

### 1. System Requirements

**Minimum:**
- 2 CPU cores
- 4GB RAM
- 20GB storage
- Ubuntu 20.04+ or CentOS 8+

**Recommended:**
- 4+ CPU cores
- 8GB+ RAM
- 100GB+ SSD storage
- Load balancer for high availability

### 2. Install Dependencies

**Ubuntu/Debian:**
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install PostgreSQL
sudo apt install postgresql postgresql-contrib

# Install Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Install Nginx (optional)
sudo apt install nginx

# Install Node.js (for frontend builds)
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt install nodejs
```

**CentOS/RHEL:**
```bash
# Update system
sudo yum update -y

# Install PostgreSQL
sudo yum install postgresql-server postgresql-contrib
sudo postgresql-setup initdb
sudo systemctl enable postgresql

# Install Go (same as Ubuntu)
# Install Nginx
sudo yum install nginx
```

### 3. Database Setup
```bash
# Switch to postgres user
sudo -u postgres psql

# Create database and user
CREATE DATABASE vault_db;
CREATE USER vault_user WITH PASSWORD 'secure_password_here';
GRANT ALL PRIVILEGES ON DATABASE vault_db TO vault_user;
\q

# Configure PostgreSQL
sudo nano /etc/postgresql/13/main/postgresql.conf
# Uncomment and set: listen_addresses = 'localhost'

sudo nano /etc/postgresql/13/main/pg_hba.conf  
# Add: local vault_db vault_user md5

# Restart PostgreSQL
sudo systemctl restart postgresql
```

### 4. Application Setup
```bash
# Clone repository
git clone https://github.com/coldforge/vault.git
cd vault

# Build backend
cd backend
go mod download
go build -o ../bin/vault-api ./cmd/server

# Build frontend
cd ../frontend/web
npm install
npm run build

# Create systemd service
sudo cp scripts/vault-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vault-api
```

### 5. Configuration
```bash
# Create config directory
sudo mkdir -p /etc/coldforge-vault

# Create environment file
sudo tee /etc/coldforge-vault/config.env << EOF
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=vault_user
DB_PASSWORD=secure_password_here
DB_NAME=vault_db
DB_SSLMODE=prefer

# Server
PORT=8080
HOST=127.0.0.1
ENVIRONMENT=production

# Security (generate with openssl rand -hex 32)
JWT_SECRET=your_jwt_secret_here
SCRYPT_N=32768
SCRYPT_R=8
SCRYPT_P=1
SESSION_DURATION_HOURS=24
EOF

# Secure the config file
sudo chmod 600 /etc/coldforge-vault/config.env
sudo chown vault-api:vault-api /etc/coldforge-vault/config.env
```

### 6. Start Services
```bash
# Start application
sudo systemctl start vault-api

# Check status
sudo systemctl status vault-api

# View logs
sudo journalctl -u vault-api -f
```

### 7. Nginx Configuration (Optional)
```bash
# Create Nginx config
sudo tee /etc/nginx/sites-available/vault << EOF
server {
    listen 80;
    server_name vault.yourdomain.com;
    return 301 https://\$server_name\$request_uri;
}

server {
    listen 443 ssl http2;
    server_name vault.yourdomain.com;

    ssl_certificate /path/to/ssl/cert.pem;
    ssl_certificate_key /path/to/ssl/private.key;
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }
}
EOF

# Enable site
sudo ln -s /etc/nginx/sites-available/vault /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## Cloud Provider Deployment

### AWS (ECS Fargate)
```bash
# Build and push to ECR
aws ecr create-repository --repository-name coldforge-vault
docker build -t coldforge-vault .
docker tag coldforge-vault:latest 123456789012.dkr.ecr.us-west-2.amazonaws.com/coldforge-vault:latest
docker push 123456789012.dkr.ecr.us-west-2.amazonaws.com/coldforge-vault:latest

# Deploy with ECS
aws ecs create-cluster --cluster-name coldforge-vault
# ... Additional ECS configuration
```

### Google Cloud (Cloud Run)
```bash
# Build and deploy
gcloud builds submit --tag gcr.io/PROJECT-ID/coldforge-vault
gcloud run deploy coldforge-vault \
  --image gcr.io/PROJECT-ID/coldforge-vault \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated
```

### DigitalOcean (App Platform)
```yaml
# app.yaml
name: coldforge-vault
services:
- name: api
  source_dir: /
  github:
    repo: your-username/vault
    branch: main
  build_command: cd backend && go build -o bin/vault-api ./cmd/server
  run_command: ./backend/bin/vault-api
  environment_slug: go
  instance_count: 1
  instance_size_slug: basic-xxs
  http_port: 8080
  envs:
  - key: DB_HOST
    value: ${db.HOSTNAME}
  - key: DB_USER  
    value: ${db.USERNAME}
  - key: DB_PASSWORD
    value: ${db.PASSWORD}
  - key: DB_NAME
    value: ${db.DATABASE}

databases:
- name: db
  engine: PG
  version: "13"
  size: db-s-1vcpu-1gb
```

## Monitoring and Maintenance

### Health Checks
```bash
# API health
curl https://vault.yourdomain.com/api/v1/health

# Database health
psql -h localhost -U vault_user -d vault_db -c "SELECT 1;"

# Container health (Docker)
docker compose ps
```

### Log Monitoring
```bash
# Application logs
tail -f /var/log/vault-api/app.log

# Nginx logs  
tail -f /var/log/nginx/access.log
tail -f /var/log/nginx/error.log

# PostgreSQL logs
tail -f /var/log/postgresql/postgresql-13-main.log
```

### Backup Strategy
```bash
# Database backup
pg_dump -h localhost -U vault_user vault_db > backup_$(date +%Y%m%d_%H%M%S).sql

# Automated backup script
#!/bin/bash
BACKUP_DIR="/backups/vault"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR
pg_dump -h localhost -U vault_user vault_db | gzip > $BACKUP_DIR/vault_backup_$DATE.sql.gz

# Keep only last 30 days
find $BACKUP_DIR -name "vault_backup_*.sql.gz" -mtime +30 -delete
```

### Updates
```bash
# Update application
git pull origin main
cd backend && go build -o ../bin/vault-api ./cmd/server
sudo systemctl restart vault-api

# Update dependencies
cd backend && go mod tidy
cd ../frontend/web && npm update

# Database migrations
cd backend && go run cmd/migrate/main.go up
```

## Security Hardening

### System Hardening
```bash
# Firewall configuration
sudo ufw enable
sudo ufw allow ssh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 8080/tcp  # Block direct API access

# Fail2ban for SSH protection
sudo apt install fail2ban
```

### Application Security
```bash
# Run as non-root user
sudo useradd -r -s /bin/false vault-api
sudo chown -R vault-api:vault-api /opt/coldforge-vault

# Secure file permissions
chmod 600 /etc/coldforge-vault/config.env
chmod 755 /opt/coldforge-vault/bin/vault-api
```

### Database Security
```bash
# PostgreSQL security
sudo -u postgres psql
ALTER USER vault_user WITH PASSWORD 'new_secure_password';
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO vault_user;
```

## Troubleshooting

### Common Issues

**Connection Refused:**
```bash
# Check if service is running
sudo systemctl status vault-api

# Check ports
sudo netstat -tlnp | grep 8080

# Check logs
sudo journalctl -u vault-api --no-pager
```

**Database Connection Errors:**
```bash
# Test database connection
psql -h localhost -U vault_user -d vault_db

# Check PostgreSQL status
sudo systemctl status postgresql

# Verify credentials
sudo -u postgres psql -c "\du"
```

**Permission Errors:**
```bash
# Check file ownership
ls -la /opt/coldforge-vault/
sudo chown -R vault-api:vault-api /opt/coldforge-vault/

# Check service permissions
sudo systemctl cat vault-api
```

### Performance Tuning

**PostgreSQL:**
```sql
-- Optimize for vault workload
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET random_page_cost = 1.1;
SELECT pg_reload_conf();
```

**Go Application:**
```bash
# Enable performance profiling
export GOMAXPROCS=4
export GOGC=100

# Memory optimization
export GODEBUG=gctrace=1
```

## Scaling

### Horizontal Scaling
- Deploy multiple API instances behind load balancer
- Use Redis for shared session storage
- Configure database connection pooling
- Implement proper health checks

### Vertical Scaling
- Increase CPU/memory allocation
- Optimize database queries
- Enable database connection pooling
- Configure Go garbage collection

### Database Scaling
- Read replicas for improved performance
- Connection pooling with PgBouncer
- Database partitioning for large datasets
- Regular VACUUM and ANALYZE operations

## Backup and Recovery

### Database Backup
```bash
# Full backup
pg_dump -h localhost -U vault_user -Fc vault_db > vault_backup.dump

# Restore
pg_restore -h localhost -U vault_user -d vault_db vault_backup.dump
```

### Application Backup
```bash
# Configuration backup
tar -czf config_backup.tar.gz /etc/coldforge-vault/

# Binary backup
cp /opt/coldforge-vault/bin/vault-api vault-api-backup
```

### Disaster Recovery
1. **Prepare recovery environment**
2. **Restore database from backup**
3. **Deploy application**
4. **Verify data integrity**
5. **Update DNS records**

## Security Checklist

- [ ] HTTPS enabled with valid certificates
- [ ] Database connections encrypted
- [ ] Firewall configured properly
- [ ] Regular security updates applied
- [ ] Audit logging enabled
- [ ] Backup strategy implemented
- [ ] Monitoring and alerting configured
- [ ] Incident response plan documented
- [ ] Recovery procedures tested
- [ ] Access controls implemented

## Support

For deployment issues:
- **Documentation**: https://github.com/coldforge/vault/docs
- **Issues**: https://github.com/coldforge/vault/issues
- **Community**: https://github.com/coldforge/vault/discussions

For security issues:
- **Email**: security@coldforge-vault.com
- **Responsible disclosure**: 90 days
- **Bug bounty**: Coming soon