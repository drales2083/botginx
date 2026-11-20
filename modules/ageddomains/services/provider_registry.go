package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/botginx/botginx/modules/ageddomains/models"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/botginx/botginx/pkg/providers/porkbun"
	"github.com/jmoiron/sqlx"
)

// DomainProvider is the interface for domain registrar APIs
type DomainProvider interface {
	// ListAgedDomains returns available aged domains from the provider
	ListAgedDomains(ctx context.Context) ([]ProviderDomain, error)
	// RegisterDomain purchases a domain and returns the order ID
	RegisterDomain(ctx context.Context, domain string, priceCents int) (string, error)
	// SetNameservers configures nameservers for a domain
	SetNameservers(ctx context.Context, domain string, nameservers []string) error
	// TestConnection verifies API credentials work
	TestConnection(ctx context.Context) error
}

// ProviderDomain is a domain returned from a provider
type ProviderDomain struct {
	Domain    string
	TLD       string
	PriceCents int
	AgeYears  int
	IsPremium bool
	ExpiresAt string
}

// ProviderRegistry manages domain provider configurations and clients
type ProviderRegistry struct {
	db      *sqlx.DB
	clients map[string]DomainProvider
	mu      sync.RWMutex
}

func NewProviderRegistry(db *sqlx.DB) *ProviderRegistry {
	return &ProviderRegistry{
		db:      db,
		clients: make(map[string]DomainProvider),
	}
}

// ListActive returns all active provider configurations
func (r *ProviderRegistry) ListActive(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	err := r.db.SelectContext(ctx, &providers,
		`SELECT * FROM domain_providers WHERE is_active = true ORDER BY name`)
	return providers, err
}

// List returns all provider configurations
func (r *ProviderRegistry) List(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	err := r.db.SelectContext(ctx, &providers,
		`SELECT * FROM domain_providers ORDER BY name`)
	return providers, err
}

// Get returns a provider configuration by ID
func (r *ProviderRegistry) Get(ctx context.Context, id string) (*models.Provider, error) {
	var provider models.Provider
	err := r.db.GetContext(ctx, &provider,
		`SELECT * FROM domain_providers WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// Create adds a new provider configuration
func (r *ProviderRegistry) Create(ctx context.Context, provider *models.Provider) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO domain_providers
		(id, name, display_name, provider_type, api_key, api_secret, is_active, markup_percent, min_age_years, max_price_cents, allowed_tlds, created_at, updated_at)
		VALUES (:id, :name, :display_name, :provider_type, :api_key, :api_secret, :is_active, :markup_percent, :min_age_years, :max_price_cents, :allowed_tlds, :created_at, :updated_at)
	`, provider)
	return err
}

// Update modifies a provider configuration
func (r *ProviderRegistry) Update(ctx context.Context, provider *models.Provider) error {
	_, err := r.db.NamedExecContext(ctx, `
		UPDATE domain_providers SET
			name = :name,
			display_name = :display_name,
			provider_type = :provider_type,
			api_key = :api_key,
			api_secret = :api_secret,
			is_active = :is_active,
			markup_percent = :markup_percent,
			min_age_years = :min_age_years,
			max_price_cents = :max_price_cents,
			allowed_tlds = :allowed_tlds,
			updated_at = :updated_at
		WHERE id = :id
	`, provider)
	return err
}

// Delete removes a provider configuration
func (r *ProviderRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM domain_providers WHERE id = $1`, id)
	return err
}

// GetClient returns the API client for a provider
func (r *ProviderRegistry) GetClient(providerID string) DomainProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[providerID]
}

// RegisterClient registers an API client for a provider
func (r *ProviderRegistry) RegisterClient(providerID string, client DomainProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[providerID] = client
}

// InitializeClients loads all active providers and creates their clients
func (r *ProviderRegistry) InitializeClients(ctx context.Context) error {
	providers, err := r.ListActive(ctx)
	if err != nil {
		return err
	}

	for _, p := range providers {
		client, err := r.createClient(p)
		if err != nil {
			continue // Skip failed clients
		}
		r.RegisterClient(p.ID, client)
	}
	return nil
}

// ReinitializeClient reinitializes the client for a specific provider (after credentials update)
func (r *ProviderRegistry) ReinitializeClient(ctx context.Context, providerID string) error {
	provider, err := r.Get(ctx, providerID)
	if err != nil {
		return err
	}
	if !provider.IsActive {
		return nil
	}
	client, err := r.createClient(*provider)
	if err != nil {
		return err
	}
	if client != nil {
		r.RegisterClient(providerID, client)
		log.Printf("[ageddomains] Reinitialized client for provider %s", provider.DisplayName)
	}
	return nil
}

// createClient creates an API client based on provider type
func (r *ProviderRegistry) createClient(p models.Provider) (DomainProvider, error) {
	switch p.ProviderType {
	case "porkbun":
		return NewPorkbunAdapter(p), nil
	default:
		return nil, nil
	}
}

// PorkbunAdapter wraps the Porkbun client to implement DomainProvider interface
type PorkbunAdapter struct {
	client   *porkbun.Client
	provider models.Provider
}

func NewPorkbunAdapter(p models.Provider) *PorkbunAdapter {
	// Decrypt API credentials
	apiKey := p.APIKey
	apiSecret := p.APISecret

	// Try to decrypt - if it fails, assume the value is not encrypted (legacy)
	if decrypted, err := crypto.Decrypt(p.APIKey); err == nil && decrypted != "" {
		apiKey = decrypted
	} else if err != nil {
		log.Printf("[ageddomains] Warning: could not decrypt API key for provider %s: %v", p.DisplayName, err)
	}

	if decrypted, err := crypto.Decrypt(p.APISecret); err == nil && decrypted != "" {
		apiSecret = decrypted
	} else if err != nil {
		log.Printf("[ageddomains] Warning: could not decrypt API secret for provider %s: %v", p.DisplayName, err)
	}

	return &PorkbunAdapter{
		client:   porkbun.NewClient(apiKey, apiSecret),
		provider: p,
	}
}

func (a *PorkbunAdapter) ListAgedDomains(ctx context.Context) ([]ProviderDomain, error) {
	// Parse allowed TLDs
	tlds := []string{"com", "net", "org"} // defaults
	if a.provider.AllowedTLDs != "" {
		tlds = strings.Split(a.provider.AllowedTLDs, ",")
		for i := range tlds {
			tlds[i] = strings.TrimSpace(tlds[i])
		}
	}

	closeouts, err := a.client.ListAgedDomains(ctx, a.provider.MinAgeYears, a.provider.MaxPriceCents, tlds)
	if err != nil {
		return nil, err
	}

	// Convert to ProviderDomain
	domains := make([]ProviderDomain, 0, len(closeouts))
	for _, c := range closeouts {
		domains = append(domains, ProviderDomain{
			Domain:     c.Domain,
			TLD:        c.TLD,
			PriceCents: c.TotalPrice,
			AgeYears:   0, // Calculate from RegistrationDate if needed
			IsPremium:  false,
			ExpiresAt:  c.EndTime,
		})
	}

	return domains, nil
}

func (a *PorkbunAdapter) RegisterDomain(ctx context.Context, domain string, priceCents int) (string, error) {
	resp, err := a.client.BuyCloseout(ctx, domain, priceCents)
	if err != nil {
		return "", err
	}
	return resp.OrderID, nil
}

func (a *PorkbunAdapter) SetNameservers(ctx context.Context, domain string, nameservers []string) error {
	return a.client.SetNameservers(ctx, domain, nameservers)
}

func (a *PorkbunAdapter) TestConnection(ctx context.Context) error {
	return a.client.VerifyCredentials(ctx)
}

// TestProvider tests the connection for a provider
func (r *ProviderRegistry) TestProvider(ctx context.Context, id string) error {
	client := r.GetClient(id)
	if client == nil {
		return fmt.Errorf("no client registered for provider - restart server to initialize")
	}
	return client.TestConnection(ctx)
}
