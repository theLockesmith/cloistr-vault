# HashiCorp Vault Configuration for Coldforge Vault KMS

# Development/Testing Configuration
# In production, use proper backend storage (Consul, etcd, etc.)

ui = true

# Storage backend - file for development
storage "file" {
  path = "/vault/data"
}

# API listener
listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = 1
}

# Disable mlock in development (Docker constraint)
disable_mlock = true

# API address
api_addr = "http://0.0.0.0:8200"

# Cluster address
cluster_addr = "http://0.0.0.0:8201"

# Default lease TTL
default_lease_ttl = "24h"

# Maximum lease TTL
max_lease_ttl = "168h"

# Log level
log_level = "Info"

# Enable raw endpoint (for health checks)
raw_storage_endpoint = true

# Telemetry (optional)
telemetry {
  disable_hostname = true
  prometheus_retention_time = "30s"
}