package services

import (
	"log"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
)

// BackgroundVerifier runs periodic checks on domains
type BackgroundVerifier struct {
	domainService *DomainService
	verifyService *VerificationService
	stopCh        chan struct{}
}

// NewBackgroundVerifier creates a new background verifier
func NewBackgroundVerifier(ds *DomainService, vs *VerificationService) *BackgroundVerifier {
	// Set server provider to get credentials from database
	vs.SetServerProvider(func() (*ServerInfo, error) {
		ip, port, user, pass, err := ds.GetDeployServer()
		if err != nil {
			return nil, err
		}
		return &ServerInfo{IP: ip, Port: port, User: user, Password: pass}, nil
	})

	return &BackgroundVerifier{
		domainService: ds,
		verifyService: vs,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background verification loop
func (b *BackgroundVerifier) Start() {
	go b.run()
}

// Stop halts the background verifier
func (b *BackgroundVerifier) Stop() {
	close(b.stopCh)
}

func (b *BackgroundVerifier) run() {
	// Wait for migrations to complete before first check
	time.Sleep(5 * time.Second)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Run first check after startup delay
	b.checkAllDomains()

	for {
		select {
		case <-ticker.C:
			b.checkAllDomains()
		case <-b.stopCh:
			return
		}
	}
}

func (b *BackgroundVerifier) checkAllDomains() {
	// Get all unverified domains
	domains, err := b.domainService.ListUnverified()
	if err != nil {
		log.Printf("[domains] background check error: %v", err)
		return
	}

	for _, domain := range domains {
		// Check DNS via TXT record
		verified, _ := b.verifyService.VerifyDNS(domain.Name, domain.VerifyToken)
		if verified {
			log.Printf("[domains] auto-verified: %s", domain.Name)
			t := true
			b.domainService.Update(domain.ID, models.UpdateDomainInput{DNSVerified: &t})

			// Auto-setup SSL after verification
			go b.setupSSL(domain.ID, domain.Name)
		}
	}

	// Check verified domains without SSL - try to generate SSL for them
	verifiedDomains, _ := b.domainService.ListVerifiedWithoutSSL()
	for _, domain := range verifiedDomains {
		go b.setupSSL(domain.ID, domain.Name)
	}
}

func (b *BackgroundVerifier) setupSSL(domainID, domainName string) {
	isCloudflare := b.verifyService.IsCloudflare(domainName)

	// Get domain to check for acme-dns credentials (cPanel domains have these)
	domain, err := b.domainService.Get(domainID)
	if err != nil {
		log.Printf("[domains] %s failed to get domain for SSL setup: %v", domainName, err)
		return
	}

	// Domain is "wildcard-capable" if: name starts with *. OR has acme-dns credentials OR is cPanel
	hasAcmeDns := domain.AcmeSubdomain != nil && domain.AcmeUsername != nil && domain.AcmePassword != nil
	isCpanel := domain.CpanelConnectionID != nil && *domain.CpanelConnectionID != ""
	isWildcard := strings.HasPrefix(domainName, "*.") || hasAcmeDns || isCpanel

	if isCloudflare {
		log.Printf("[domains] %s is behind Cloudflare - setting up origin SSL", domainName)

		// Generate self-signed cert for Cloudflare Full mode
		if err := b.verifyService.GenerateSSL(domainName); err != nil {
			log.Printf("[domains] self-signed cert failed for %s: %v", domainName, err)
		}

		// Setup nginx config
		if err := b.verifyService.SetupDomainNginx(domainName); err != nil {
			log.Printf("[domains] nginx setup failed for %s: %v", domainName, err)
			return
		}

		log.Printf("[domains] %s SSL ready (Cloudflare)", domainName)
		t := true
		b.domainService.Update(domainID, models.UpdateDomainInput{SSLEnabled: &t})
		return
	}

	// For non-Cloudflare: generate SSL FIRST, then nginx config
	if isWildcard {
		// If acme-dns is configured and CNAME is verified, generate SSL automatically
		if hasAcmeDns && domain.AcmeCnameVerified {
			log.Printf("[domains] %s generating wildcard SSL via acme-dns", domainName)
			if err := b.verifyService.GenerateWildcardSSLWithAcmeDNS(
				domainName,
				*domain.AcmeSubdomain,
				*domain.AcmeUsername,
				*domain.AcmePassword,
			); err != nil {
				log.Printf("[domains] wildcard SSL generation failed for %s: %v", domainName, err)
				errStr := err.Error()
				b.domainService.Update(domainID, models.UpdateDomainInput{SSLError: &errStr})
				return
			}
			// SSL generated, now setup nginx and enable
			if err := b.verifyService.SetupDomainNginx(domainName); err != nil {
				log.Printf("[domains] nginx setup failed for %s: %v", domainName, err)
				return
			}
			b.checkAndEnableSSL(domainID, domainName)
			return
		}

		// No acme-dns or CNAME not verified - skip auto SSL
		log.Printf("[domains] %s is wildcard - waiting for CNAME verification, skipping auto SSL", domainName)
		return
	}

	// Non-wildcard: use HTTP-01 challenge
	log.Printf("[domains] %s generating Let's Encrypt SSL", domainName)
	if err := b.verifyService.GenerateSSL(domainName); err != nil {
		log.Printf("[domains] SSL generation failed for %s: %v", domainName, err)
		return
	}

	// Now setup nginx (cert exists)
	if err := b.verifyService.SetupDomainNginx(domainName); err != nil {
		log.Printf("[domains] nginx setup failed for %s: %v", domainName, err)
		return
	}

	// Check if SSL exists and enable
	b.checkAndEnableSSL(domainID, domainName)
}

func (b *BackgroundVerifier) checkAndEnableSSL(domainID, domainName string) {
	status, err := b.verifyService.CheckSSL(domainName)
	if err != nil {
		log.Printf("[domains] SSL check failed for %s: %v", domainName, err)
		return
	}

	if status.Exists {
		log.Printf("[domains] SSL active for %s (wildcard: %v)", domainName, status.IsWildcard)
		t := true
		b.domainService.Update(domainID, models.UpdateDomainInput{SSLEnabled: &t})
	}
}
