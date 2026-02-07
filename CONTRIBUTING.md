# Contributing to Coldforge Vault

Thank you for your interest in contributing to Coldforge Vault! This document provides guidelines and information for contributors.

## Code of Conduct

We are committed to providing a welcoming and inspiring community for all. Please read and follow our Code of Conduct.

### Our Pledge
- Use welcoming and inclusive language
- Be respectful of differing viewpoints and experiences
- Gracefully accept constructive criticism
- Focus on what is best for the community
- Show empathy towards other community members

## Getting Started

### Development Environment Setup

1. **Prerequisites:**
   ```bash
   # Install Go 1.21+
   # Install Node.js 18+
   # Install PostgreSQL 12+
   # Install Docker (optional)
   ```

2. **Clone and Setup:**
   ```bash
   git clone https://github.com/coldforge/vault.git
   cd vault
   
   # Backend setup
   cd backend
   go mod download
   
   # Frontend setup  
   cd ../frontend/web
   npm install
   ```

3. **Run Tests:**
   ```bash
   # Backend tests
   make test
   
   # Frontend tests
   cd frontend/web && npm test
   ```

4. **Start Development:**
   ```bash
   # Start backend (requires PostgreSQL)
   make run
   
   # Start frontend (separate terminal)
   cd frontend/web && npm start
   ```

## Development Guidelines

### Code Style

**Go Backend:**
- Follow standard Go conventions (`gofmt`, `go vet`)
- Use meaningful variable and function names
- Add tests for all new functionality
- Include error handling for all operations
- Document exported functions and types

**TypeScript/React Frontend:**
- Use TypeScript for all new code
- Follow React best practices and hooks
- Use meaningful component and variable names
- Add tests for components and utilities
- Follow accessibility guidelines

**General:**
- Keep functions small and focused
- Use clear, descriptive commit messages
- Add documentation for complex logic
- Follow security best practices

### Testing Requirements

**All contributions must include:**
- Unit tests for new functionality
- Integration tests for API endpoints
- Security tests for cryptographic operations
- Performance tests for critical paths

**Test Coverage:**
- Aim for >90% code coverage
- Test error conditions and edge cases
- Include benchmark tests for performance-critical code
- Add security-focused tests

### Security Guidelines

**Critical Security Rules:**
- Never log sensitive data (passwords, keys, tokens)
- Always use constant-time comparisons for secrets
- Validate all inputs thoroughly
- Use prepared statements for database queries
- Follow zero-knowledge principles

**Cryptography:**
- Use only well-established cryptographic libraries
- Never implement custom cryptographic algorithms
- Include comprehensive tests for crypto operations
- Document cryptographic choices and parameters

## Contribution Process

### 1. Issue First
- Check existing issues before starting work
- Create an issue to discuss significant changes
- Get feedback on your approach before coding
- Reference the issue in your pull request

### 2. Development Workflow
```bash
# Create feature branch
git checkout -b feature/your-feature-name

# Make changes
# Add tests
# Update documentation

# Test your changes
make test
make lint

# Commit with clear message
git commit -m "feat: add password strength indicator

- Implements real-time password strength checking
- Adds visual feedback with color coding
- Includes unit tests for strength calculation
- Updates UI components with new indicator

Fixes #123"
```

### 3. Pull Request Guidelines

**Before Submitting:**
- [ ] All tests pass (`make test`)
- [ ] Code is linted (`make lint`)
- [ ] Documentation is updated
- [ ] Security implications considered
- [ ] Backward compatibility maintained

**PR Description Should Include:**
- Clear description of changes
- Motivation and context
- Testing instructions
- Screenshots for UI changes
- Security considerations
- Breaking changes (if any)

**PR Template:**
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix (non-breaking change)
- [ ] New feature (non-breaking change)  
- [ ] Breaking change (fix or feature causing existing functionality to change)
- [ ] Documentation update

## Testing
- [ ] Added unit tests
- [ ] Added integration tests
- [ ] Manual testing completed
- [ ] Security testing performed

## Security Review
- [ ] No sensitive data logged
- [ ] Input validation added
- [ ] Authentication/authorization checked
- [ ] Cryptographic operations reviewed

## Checklist
- [ ] My code follows the style guidelines
- [ ] I have performed a self-review
- [ ] I have added tests for my changes
- [ ] All tests pass
- [ ] Documentation has been updated
```

### 4. Review Process
- Maintainers will review within 48 hours
- Address feedback promptly
- Keep PRs focused and small when possible
- Be open to suggestions and changes

## Types of Contributions

### 🐛 Bug Fixes
- Fix existing functionality
- Add regression tests
- Update documentation if needed

### ✨ New Features
- Discuss in issues first
- Follow zero-knowledge principles
- Include comprehensive tests
- Update API documentation

### 📚 Documentation
- Improve existing docs
- Add missing documentation
- Fix typos and formatting
- Translate documentation

### 🔒 Security
- Security vulnerability reports (private)
- Security feature improvements
- Cryptographic enhancements
- Audit and compliance features

### 🏗️ Infrastructure
- Build and deployment improvements
- CI/CD enhancements
- Performance optimizations
- Monitoring and logging

## Coding Standards

### Backend (Go)

```go
// Good: Clear function with proper error handling
func (s *Service) GetVault(userID uuid.UUID) (*models.VaultResponse, error) {
    if userID == uuid.Nil {
        return nil, fmt.Errorf("invalid user ID")
    }
    
    vault, err := s.repository.GetByUserID(userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get vault: %w", err)
    }
    
    return &models.VaultResponse{
        ID:           vault.ID,
        Version:      vault.Version,
        LastModified: vault.LastModified,
    }, nil
}

// Bad: No error handling, unclear naming
func (s *Service) GetV(id string) *VaultResponse {
    v := s.repo.Get(id)
    return &VaultResponse{ID: v.ID}
}
```

### Frontend (React/TypeScript)

```typescript
// Good: Typed component with proper error handling
interface VaultItemProps {
  item: VaultEntry;
  onSelect: (item: VaultEntry) => void;
  showSensitive?: boolean;
}

const VaultItem: React.FC<VaultItemProps> = ({ 
  item, 
  onSelect, 
  showSensitive = false 
}) => {
  const handleClick = useCallback(() => {
    onSelect(item);
  }, [item, onSelect]);

  return (
    <div 
      className="vault-item"
      onClick={handleClick}
      role="button"
      tabIndex={0}
    >
      {/* Component content */}
    </div>
  );
};

// Bad: Untyped, no accessibility
const VaultItem = ({ item, onSelect }) => {
  return <div onClick={() => onSelect(item)}>{item.name}</div>;
};
```

## Release Process

### Versioning
We follow [Semantic Versioning](https://semver.org/):
- `MAJOR.MINOR.PATCH`
- Major: Breaking changes
- Minor: New features (backward compatible)
- Patch: Bug fixes (backward compatible)

### Release Checklist
- [ ] All tests pass
- [ ] Documentation updated
- [ ] Security review completed
- [ ] Performance benchmarks run
- [ ] Database migrations tested
- [ ] Deployment guide updated
- [ ] Release notes prepared

## Community

### Getting Help
- **GitHub Discussions** - Questions and ideas
- **GitHub Issues** - Bug reports and feature requests
- **Documentation** - Comprehensive guides
- **Code Examples** - See `/examples` directory

### Communication Channels
- **GitHub Issues** - Primary communication
- **Email** - security@coldforge-vault.com (security only)
- **Matrix** - Coming soon

## Recognition

Contributors are recognized in:
- **CONTRIBUTORS.md** - All contributors listed
- **Release Notes** - Major contributors highlighted
- **GitHub Contributors** - Automatic recognition

## Legal

By contributing, you agree that:
- Your contributions will be licensed under the MIT License
- You have the right to submit the contribution
- You understand the project's security requirements
- You will follow responsible disclosure for security issues

## Quick Reference

### Useful Commands
```bash
# Development
make dev          # Start development environment
make test         # Run all tests
make lint         # Run linter
make build        # Build application

# Testing
make test-crypto  # Test cryptographic functions
make test-coverage # Generate coverage report
make benchmark    # Run performance benchmarks

# Deployment
make docker-build # Build Docker image
make deploy-docker # Deploy with Docker Compose
make deploy-k8s   # Deploy to Kubernetes
```

### File Structure
```
backend/
  cmd/server/         # Main application entry
  internal/
    api/             # REST API handlers
    auth/            # Authentication logic
    crypto/          # Cryptographic functions
    database/        # Database layer
    models/          # Data models
  migrations/        # Database migrations
  
frontend/
  web/              # React web application
  mobile/           # React Native mobile app
  desktop/          # Electron desktop app
  browser-extension/ # Browser extensions

docs/               # Documentation
deployments/        # Deployment configurations
scripts/           # Utility scripts
```

Thank you for contributing to Coldforge Vault! 🔒