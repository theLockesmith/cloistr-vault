#!/bin/bash

# Coldforge Vault Deployment Script
set -e

echo "🚀 Deploying Coldforge Vault..."

# Configuration
NAMESPACE="coldforge-vault"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Functions
check_dependencies() {
    echo "📋 Checking dependencies..."
    
    if ! command -v kubectl &> /dev/null; then
        echo "❌ kubectl is required but not installed"
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        echo "❌ docker is required but not installed"
        exit 1
    fi
    
    echo "✅ Dependencies check passed"
}

build_image() {
    echo "🔨 Building Docker image..."
    
    cd "$PROJECT_ROOT"
    docker build -t coldforge/vault:latest .
    
    # Tag with version if provided
    if [ ! -z "$1" ]; then
        docker tag coldforge/vault:latest coldforge/vault:$1
        echo "✅ Built and tagged image: coldforge/vault:$1"
    else
        echo "✅ Built image: coldforge/vault:latest"
    fi
}

deploy_kubernetes() {
    echo "☸️ Deploying to Kubernetes..."
    
    # Create namespace
    kubectl apply -f "$PROJECT_ROOT/deployments/kubernetes/namespace.yaml"
    
    # Apply secrets (make sure to update them with real values)
    echo "⚠️ WARNING: Update secrets in deployments/kubernetes/secret.yaml with real values"
    kubectl apply -f "$PROJECT_ROOT/deployments/kubernetes/secret.yaml"
    
    # Apply config maps
    kubectl apply -f "$PROJECT_ROOT/deployments/kubernetes/configmap.yaml"
    
    # Deploy PostgreSQL
    kubectl apply -f "$PROJECT_ROOT/deployments/kubernetes/postgres.yaml"
    
    # Wait for PostgreSQL to be ready
    echo "⏳ Waiting for PostgreSQL to be ready..."
    kubectl wait --for=condition=ready pod -l app=postgres -n $NAMESPACE --timeout=300s
    
    # Deploy the application
    kubectl apply -f "$PROJECT_ROOT/deployments/kubernetes/app.yaml"
    
    # Wait for application to be ready
    echo "⏳ Waiting for application to be ready..."
    kubectl wait --for=condition=ready pod -l app=vault-api -n $NAMESPACE --timeout=300s
    
    echo "✅ Kubernetes deployment completed"
}

deploy_docker_compose() {
    echo "🐳 Deploying with Docker Compose..."
    
    cd "$PROJECT_ROOT"
    
    # Create .env file if it doesn't exist
    if [ ! -f .env ]; then
        echo "📝 Creating .env file..."
        cat > .env << EOF
JWT_SECRET=$(openssl rand -hex 32)
REDIS_PASSWORD=$(openssl rand -hex 16)
EOF
        echo "✅ Created .env file with random secrets"
    fi
    
    # Deploy
    docker-compose up -d --build
    
    echo "⏳ Waiting for services to be ready..."
    sleep 10
    
    # Check health
    if curl -f http://localhost:8080/api/v1/health > /dev/null 2>&1; then
        echo "✅ Docker Compose deployment completed successfully"
        echo "🌍 API available at: http://localhost:8080"
    else
        echo "❌ Health check failed"
        docker-compose logs vault-api
        exit 1
    fi
}

show_status() {
    echo "📊 Deployment Status:"
    echo "--------------------"
    
    if [ "$1" == "kubernetes" ]; then
        kubectl get pods -n $NAMESPACE
        kubectl get services -n $NAMESPACE
        kubectl get ingress -n $NAMESPACE
        
        # Get external access info
        echo ""
        echo "🌐 Access Information:"
        if kubectl get ingress vault-api-ingress -n $NAMESPACE &> /dev/null; then
            INGRESS_HOST=$(kubectl get ingress vault-api-ingress -n $NAMESPACE -o jsonpath='{.spec.rules[0].host}')
            echo "External URL: https://$INGRESS_HOST"
        else
            echo "Port-forward to access: kubectl port-forward svc/vault-api-service -n $NAMESPACE 8080:80"
        fi
    else
        docker-compose ps
        echo ""
        echo "🌐 Access Information:"
        echo "API URL: http://localhost:8080"
        echo "Health Check: http://localhost:8080/api/v1/health"
        echo "API Info: http://localhost:8080/api/v1/info"
    fi
}

# Main script
main() {
    case ${1:-help} in
        "docker")
            check_dependencies
            build_image $2
            deploy_docker_compose
            show_status docker
            ;;
        "kubernetes"|"k8s")
            check_dependencies
            build_image $2
            deploy_kubernetes
            show_status kubernetes
            ;;
        "build")
            check_dependencies
            build_image $2
            ;;
        "status")
            show_status ${2:-docker}
            ;;
        "help"|*)
            echo "Coldforge Vault Deployment Script"
            echo ""
            echo "Usage: $0 [command] [options]"
            echo ""
            echo "Commands:"
            echo "  docker           Deploy using Docker Compose"
            echo "  kubernetes|k8s   Deploy to Kubernetes cluster"
            echo "  build [version]  Build Docker image only"
            echo "  status [type]    Show deployment status (docker|kubernetes)"
            echo "  help             Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 docker                    # Deploy with Docker Compose"
            echo "  $0 kubernetes               # Deploy to Kubernetes"
            echo "  $0 build v1.0.0             # Build image with version tag"
            echo "  $0 status kubernetes        # Show Kubernetes status"
            ;;
    esac
}

main "$@"