package services

import (
	"log"
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
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run once on startup after a short delay
	time.Sleep(30 * time.Second)
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

	// Check verified domains without SSL
	verifiedDomains, _ := b.domainService.ListVerifiedWithoutSSL()
	for _, domain := range verifiedDomains {
		go b.checkAndEnableSSL(domain.ID, domain.Name)
	}
}

func (b *BackgroundVerifier) setupSSL(domainID, domainName string) {
	// First setup nginx config
	if err := b.verifyService.SetupDomainNginx(domainName); err != nil {
		log.Printf("[domains] nginx setup failed for %s: %v", domainName, err)
		return
	}

	// Generate SSL certificate
	if err := b.verifyService.GenerateSSL(domainName); err != nil {
		log.Printf("[domains] SSL generation failed for %s: %v", domainName, err)
		// Continue anyway - might be Cloudflare or already have cert
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

	if status.Exists && status.IsWildcard {
		log.Printf("[domains] SSL active for %s", domainName)
		t := true
		b.domainService.Update(domainID, models.UpdateDomainInput{SSLEnabled: &t})
	}
}
