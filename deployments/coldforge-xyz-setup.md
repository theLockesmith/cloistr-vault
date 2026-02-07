# @coldforge.xyz Domain Infrastructure Setup

## 🎯 **Universal Bitcoin Identity Platform**

Transform **alice@coldforge.xyz** into a universal Bitcoin identity that handles:
- ⚡ Lightning payments (instant Bitcoin)
- 🔑 Authentication (passwordless login)
- 🆔 Nostr verification (social identity)
- 📧 Future: Encrypted messaging

## 🌐 **Domain Configuration**

### **DNS Records:**
```dns
# Main domain
coldforge.xyz.                 IN A     <SERVER_IP>
coldforge.xyz.                 IN AAAA  <SERVER_IPv6>

# Subdomains
www.coldforge.xyz.             IN CNAME coldforge.xyz.
app.coldforge.xyz.             IN CNAME coldforge.xyz.
api.coldforge.xyz.             IN CNAME coldforge.xyz.

# Lightning Address support
*.coldforge.xyz.               IN A     <SERVER_IP>  # Wildcard for user addresses

# Well-known services
# (Handled by reverse proxy, not DNS)
```

### **SSL Certificate:**
```bash
# Let's Encrypt wildcard certificate
certbot certonly --dns-cloudflare \
  --dns-cloudflare-credentials ~/.secrets/cloudflare.ini \
  --dns-cloudflare-propagation-seconds 60 \
  -d coldforge.xyz \
  -d "*.coldforge.xyz"
```

## 🔧 **Reverse Proxy Configuration (Nginx)**

```nginx
# /etc/nginx/sites-available/coldforge.xyz
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name coldforge.xyz www.coldforge.xyz;

    ssl_certificate /etc/letsencrypt/live/coldforge.xyz/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/coldforge.xyz/privkey.pem;

    # Main application
    location / {
        proxy_pass http://localhost:7710;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Lightning Address resolution
    location /.well-known/lnurlp/ {
        proxy_pass http://localhost:7710/api/v1/lightning/lnurlp/;
        add_header Access-Control-Allow-Origin *;
        add_header Content-Type application/json;
    }

    # NIP-05 Nostr verification
    location /.well-known/nostr.json {
        proxy_pass http://localhost:7710/api/v1/identity/nip05;
        add_header Access-Control-Allow-Origin *;
        add_header Content-Type application/json;
    }

    # LNURL-auth endpoints
    location /.well-known/lnurlauth/ {
        proxy_pass http://localhost:7710/api/v1/lightning/lnurlauth/;
        add_header Access-Control-Allow-Origin *;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name coldforge.xyz www.coldforge.xyz;
    return 301 https://$server_name$request_uri;
}
```

## ⚡ **Lightning Node Integration**

### **LND Configuration:**
```conf
# lnd.conf
[Application Options]
listen=localhost:9735
rpclisten=localhost:10009
restlisten=localhost:8080

[Bitcoin]
bitcoin.active=1
bitcoin.mainnet=1
bitcoin.node=bitcoind

[Lightning Address]
accept-amp=true
accept-keysend=true

# For Coldforge Vault integration
[RPC]
rpcuser=coldforge
rpcpass=<SECURE_PASSWORD>
```

### **Payment Processing:**
```go
// Lightning payment handler
func (s *LightningIdentityService) HandlePayment(username string, amount int64, comment string) (*PaymentResponse, error) {
    // 1. Validate Lightning Address exists
    identity, err := s.GetIdentityByUsername(username)
    if err != nil {
        return nil, err
    }

    // 2. Generate Lightning invoice
    invoice, err := s.lightningClient.CreateInvoice(amount, fmt.Sprintf("Payment to %s", identity.LightningAddr))
    if err != nil {
        return nil, err
    }

    // 3. Return LNURL-pay invoice response
    return &PaymentResponse{
        PR: invoice.PaymentRequest,
        Routes: []interface{}{}, // Empty for basic implementation
    }, nil
}
```

## 🔑 **LNURL-auth Integration**

### **Authentication Flow:**
```go
// LNURL-auth challenge handler
func (s *LightningIdentityService) HandleLNURLAuth(w http.ResponseWriter, r *http.Request) {
    k1 := r.URL.Query().Get("k1")
    if k1 == "" {
        http.Error(w, "Missing k1 parameter", http.StatusBadRequest)
        return
    }

    // Generate LNURL-auth response
    response := map[string]interface{}{
        "status": "OK",
        "tag":    "login",
        "k1":     k1,
        "action": map[string]string{
            "login": fmt.Sprintf("https://%s/api/v1/auth/lightning", s.domain),
        },
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

## 📋 **Deployment Checklist**

### **Phase 1: Domain & Infrastructure**
```bash
✅ Register coldforge.xyz domain
✅ Configure DNS with wildcard support
✅ Set up SSL certificates (Let's Encrypt)
✅ Configure reverse proxy (Nginx)
✅ Set up monitoring and logging
```

### **Phase 2: Lightning Integration**
```bash
🔄 Set up Lightning node (LND)
🔄 Configure payment processing
🔄 Test Lightning Address resolution
🔄 Implement LNURL-pay endpoints
🔄 Add payment webhook handling
```

### **Phase 3: Identity Service**
```bash
🔄 Deploy identity management backend
🔄 Configure NIP-05 verification
🔄 Add LNURL-auth endpoints
🔄 Test end-to-end identity flows
🔄 Add identity management UI
```

### **Phase 4: Production Polish**
```bash
🔄 Add rate limiting and DDoS protection
🔄 Implement monitoring and alerting
🔄 Add backup and disaster recovery
🔄 Security audit and penetration testing
🔄 Performance optimization
```

## 🚀 **Expected Timeline**

### **Week 1: Foundation**
- Domain registration and DNS setup
- SSL certificate configuration
- Basic reverse proxy setup
- Lightning node deployment

### **Week 2: Integration**
- Lightning Address resolution
- NIP-05 verification service
- LNURL-auth implementation
- Backend API integration

### **Week 3: Testing**
- End-to-end identity flows
- Payment processing tests
- Authentication integration
- Performance testing

### **Week 4: Launch**
- Production deployment
- Monitoring setup
- Documentation
- Community announcement

## 🎯 **Success Metrics**

### **Technical:**
- Lightning Address resolution: <200ms
- Payment success rate: >99.5%
- Authentication success rate: >99%
- Identity verification: <1 second

### **Adoption:**
- First 100 @coldforge.xyz addresses
- Bitcoin influencer adoption
- Developer ecosystem growth
- Enterprise customer interest

## 🌟 **Revolutionary Impact**

**@coldforge.xyz becomes the Gmail of Bitcoin:**
- **Universal identity** for the Bitcoin economy
- **Payment + authentication** in one address
- **Cross-platform verification** (Nostr social networks)
- **Enterprise Bitcoin identity** services

**This creates the foundational infrastructure for Bitcoin-native identity that the entire ecosystem has been waiting for.**

**Ready to build the future of Bitcoin identity? Let's make @coldforge.xyz the universal Bitcoin identity platform!** ⚡🆔🚀