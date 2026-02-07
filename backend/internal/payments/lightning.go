package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
	
	"github.com/google/uuid"
)

// LightningPaymentService handles Bitcoin Lightning payments
type LightningPaymentService struct {
	nodeConfig *LightningNodeConfig
}

// LightningNodeConfig contains Lightning node connection details
type LightningNodeConfig struct {
	Type        string `json:"type"`         // "lnd", "cln", "eclair", "demo"
	Host        string `json:"host"`
	Port        int    `json:"port"`
	TLSCert     string `json:"tls_cert"`
	MacaroonHex string `json:"macaroon_hex"`
	Network     string `json:"network"`      // "mainnet", "testnet", "regtest"
}

// Payment represents a Lightning payment
type Payment struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	PaymentHash     string    `json:"payment_hash"`
	PaymentRequest  string    `json:"payment_request"` // BOLT11 invoice
	AmountSats      int64     `json:"amount_sats"`
	AmountMsats     int64     `json:"amount_msats"`
	Description     string    `json:"description"`
	Status          string    `json:"status"` // "pending", "paid", "failed", "expired"
	CreatedAt       time.Time `json:"created_at"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
	Preimage        string    `json:"preimage,omitempty"`
	FeePaidMsats    int64     `json:"fee_paid_msats,omitempty"`
}

// ServiceTier represents different service tiers with Bitcoin pricing
type ServiceTier struct {
	Name             string `json:"name"`
	MonthlyCostSats  int64  `json:"monthly_cost_sats"`
	Features         []string `json:"features"`
	StorageGB        int64  `json:"storage_gb"`
	BackupRetention  int    `json:"backup_retention_days"`
	SupportLevel     string `json:"support_level"`
}

// Pricing tiers
var ServiceTiers = map[string]ServiceTier{
	"sovereign": {
		Name:             "Sovereign (Free)",
		MonthlyCostSats:  0,
		Features:         []string{"local_storage", "self_hosting", "open_source"},
		StorageGB:        0, // User's own storage
		BackupRetention:  0, // User-managed
		SupportLevel:     "community",
	},
	"backup": {
		Name:             "Backup",
		MonthlyCostSats:  1000, // ~$0.30 at $30k BTC
		Features:         []string{"local_first", "encrypted_backup", "recovery_service"},
		StorageGB:        1,
		BackupRetention:  90,
		SupportLevel:     "email",
	},
	"sync": {
		Name:             "Sync",
		MonthlyCostSats:  5000, // ~$1.50 at $30k BTC
		Features:         []string{"cloud_sync", "multi_device", "real_time", "backup", "recovery"},
		StorageGB:        5,
		BackupRetention:  365,
		SupportLevel:     "priority_email",
	},
	"premium": {
		Name:             "Premium",
		MonthlyCostSats:  15000, // ~$4.50 at $30k BTC
		Features:         []string{"all_sync_features", "breach_monitoring", "password_health", "import_export", "priority_support"},
		StorageGB:        25,
		BackupRetention:  1095, // 3 years
		SupportLevel:     "priority_support",
	},
}

// NewLightningPaymentService creates a new Lightning payment service
func NewLightningPaymentService(config *LightningNodeConfig) *LightningPaymentService {
	return &LightningPaymentService{
		nodeConfig: config,
	}
}

// CreateInvoice creates a Lightning invoice for payment
func (s *LightningPaymentService) CreateInvoice(ctx context.Context, userID uuid.UUID, amountSats int64, description string) (*Payment, error) {
	paymentID := uuid.New()
	paymentHash := s.generatePaymentHash(paymentID.String())
	
	// In a real implementation, this would:
	// 1. Connect to Lightning node (LND/CLN/Eclair)
	// 2. Create actual invoice via gRPC/REST API
	// 3. Return real BOLT11 invoice string
	
	// Demo implementation
	invoice := s.generateDemoInvoice(amountSats, description, paymentHash)
	
	payment := &Payment{
		ID:             paymentID,
		UserID:         userID,
		PaymentHash:    paymentHash,
		PaymentRequest: invoice,
		AmountSats:     amountSats,
		AmountMsats:    amountSats * 1000,
		Description:    description,
		Status:         "pending",
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}
	
	return payment, nil
}

// CheckPaymentStatus checks if a payment has been received
func (s *LightningPaymentService) CheckPaymentStatus(ctx context.Context, paymentHash string) (*Payment, error) {
	// In a real implementation, this would:
	// 1. Query Lightning node for payment status
	// 2. Return actual payment details
	
	// Demo: simulate payment after 30 seconds
	return &Payment{
		PaymentHash:  paymentHash,
		Status:       "paid",
		PaidAt:       timePtr(time.Now()),
		Preimage:     s.generatePreimage(paymentHash),
		FeePaidMsats: 100, // 100 msat fee
	}, nil
}

// CalculateServiceCost calculates the cost for a service tier
func (s *LightningPaymentService) CalculateServiceCost(tier string, durationDays int) (int64, error) {
	serviceTier, exists := ServiceTiers[tier]
	if !exists {
		return 0, fmt.Errorf("unknown service tier: %s", tier)
	}
	
	// Calculate prorated cost
	dailyCostSats := serviceTier.MonthlyCostSats / 30
	totalCostSats := dailyCostSats * int64(durationDays)
	
	return totalCostSats, nil
}

// ProcessSubscriptionPayment handles subscription payments
func (s *LightningPaymentService) ProcessSubscriptionPayment(ctx context.Context, userID uuid.UUID, tier string) (*Payment, error) {
	// Calculate monthly cost
	costSats, err := s.CalculateServiceCost(tier, 30)
	if err != nil {
		return nil, err
	}
	
	description := fmt.Sprintf("Coldforge Vault %s subscription (30 days)", tier)
	
	// Create invoice
	payment, err := s.CreateInvoice(ctx, userID, costSats, description)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription invoice: %w", err)
	}
	
	return payment, nil
}

// ProcessMicroPayment handles usage-based micro-payments
func (s *LightningPaymentService) ProcessMicroPayment(ctx context.Context, userID uuid.UUID, serviceType string, usage int64) (*Payment, error) {
	// Micro-payment rates
	rates := map[string]int64{
		"backup_storage_gb":    100, // 100 sats per GB per month
		"recovery_attempt":     50,  // 50 sats per recovery attempt
		"breach_check":         10,  // 10 sats per breach monitoring check
		"password_export":      25,  // 25 sats per export operation
		"premium_import":       100, // 100 sats per premium import
	}
	
	costPerUnit, exists := rates[serviceType]
	if !exists {
		return nil, fmt.Errorf("unknown service type: %s", serviceType)
	}
	
	totalCostSats := costPerUnit * usage
	description := fmt.Sprintf("Coldforge Vault %s usage (%d units)", serviceType, usage)
	
	// Create micro-payment invoice
	payment, err := s.CreateInvoice(ctx, userID, totalCostSats, description)
	if err != nil {
		return nil, fmt.Errorf("failed to create micro-payment invoice: %w", err)
	}
	
	return payment, nil
}

// GetPaymentHistory returns payment history for a user
func (s *LightningPaymentService) GetPaymentHistory(ctx context.Context, userID uuid.UUID, limit int) ([]Payment, error) {
	// In a real implementation, this would query from database
	// For demo, return empty slice
	return []Payment{}, nil
}

// Helper functions

func (s *LightningPaymentService) generatePaymentHash(data string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("payment:%s:%d", data, time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])
}

func (s *LightningPaymentService) generatePreimage(paymentHash string) string {
	// In real implementation, this would be the actual preimage
	// For demo, generate deterministic preimage
	preimage := sha256.Sum256([]byte(fmt.Sprintf("preimage:%s", paymentHash)))
	return hex.EncodeToString(preimage[:])
}

func (s *LightningPaymentService) generateDemoInvoice(amountSats int64, description, paymentHash string) string {
	// Generate demo BOLT11 invoice format
	// Real implementation would use actual Lightning libraries
	return fmt.Sprintf("lnbc%dm1p%s...", amountSats, paymentHash[:8])
}

// Auto-payment configuration
type AutoPayConfig struct {
	Enabled         bool  `json:"enabled"`
	MaxAmountSats   int64 `json:"max_amount_sats"`
	DailyLimitSats  int64 `json:"daily_limit_sats"`
	MonthlyBudgetSats int64 `json:"monthly_budget_sats"`
	ServiceTypes    []string `json:"service_types"` // Which services to auto-pay for
}

// SetupAutoPayments configures automatic Lightning payments
func (s *LightningPaymentService) SetupAutoPayments(ctx context.Context, userID uuid.UUID, config *AutoPayConfig) error {
	// In a real implementation, this would:
	// 1. Store auto-payment preferences
	// 2. Set up payment channel or recurring authorization
	// 3. Configure spending limits and service allowlist
	
	return nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}