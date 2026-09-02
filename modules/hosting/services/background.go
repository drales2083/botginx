package services

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
)

// BackgroundVerifier monitors hosting domains and auto-enables SSL when DNS is ready
type BackgroundVerifier struct {
	hostingService *HostingService
	stopCh         chan struct{}
}

// NewBackgroundVerifier creates a new background verifier for hosting domains
func NewBackgroundVerifier(hs *HostingService) *BackgroundVerifier {
	return &BackgroundVerifier{
		hostingService: hs,
		stopCh:         make(chan struct{}),
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
	ticker := time.NewTicker(60 * time.Second) // Check every minute
	defer ticker.Stop()

	// Run once immediately on startup
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
	// Get all domains that need DNS verification or SSL
	domains, err := b.hostingService.ListPendingDomains()
	if err != nil {
		log.Printf("[hosting-bg] error listing pending domains: %v", err)
		return
	}

	for _, domain := range domains {
		b.checkDomain(domain)
	}
}

func (b *BackgroundVerifier) checkDomain(domain models.HostingDomain) {
	// Get account and server info
	account, err := b.hostingService.GetAccount(domain.AccountID)
	if err != nil {
		return
	}

	if account.ServerID == nil {
		return // Account not linked to server yet
	}

	server, err := b.hostingService.GetServer(*account.ServerID)
	if err != nil {
		return
	}

	// Check DNS - domain should point to server IP
	serverIP := b.getServerIP(server.Hostname)
	if serverIP == "" {
		return
	}

	dnsVerified := b.checkDNS(domain.Domain, serverIP)

	if dnsVerified && !domain.DNSVerified {
		log.Printf("[hosting-bg] DNS verified for %s", domain.Domain)
		b.hostingService.UpdateDomainStatus(domain.ID, true, domain.SetupStatus, nil)

		// Try to enable SSL if DNS is verified but SSL not enabled
		if !domain.SSLEnabled {
			go b.tryEnableSSL(account, &domain)
		}
	}

	// If DNS verified and setup status is pending, try SSL
	if domain.DNSVerified && domain.SetupStatus == models.DomainStatusPendingDNS {
		go b.tryEnableSSL(account, &domain)
	}
}

func (b *BackgroundVerifier) getServerIP(hostname string) string {
	// Try to resolve hostname to IP
	ips, err := net.LookupHost(hostname)
	if err != nil || len(ips) == 0 {
		// Hostname might already be an IP
		if net.ParseIP(hostname) != nil {
			return hostname
		}
		return ""
	}
	return ips[0]
}

func (b *BackgroundVerifier) checkDNS(domain, expectedIP string) bool {
	// Check if domain resolves to expected IP
	ips, err := net.LookupHost(domain)
	if err != nil {
		return false
	}

	for _, ip := range ips {
		if ip == expectedIP {
			return true
		}
	}

	// Also check for Cloudflare IPs (domain might be proxied)
	for _, ip := range ips {
		if b.isCloudflareIP(ip) {
			return true // Assume DNS is set if behind Cloudflare
		}
	}

	return false
}

func (b *BackgroundVerifier) isCloudflareIP(ip string) bool {
	cfPrefixes := []string{"104.", "172.67.", "162.158.", "141.101.", "108.162.", "190.93.", "188.114.", "197.234.", "198.41.", "103.21.", "103.22.", "103.31."}
	for _, prefix := range cfPrefixes {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

func (b *BackgroundVerifier) tryEnableSSL(account *models.HostingAccount, domain *models.HostingDomain) {
	log.Printf("[hosting-bg] attempting SSL for %s", domain.Domain)

	// Update status to generating
	b.hostingService.UpdateDomainStatus(domain.ID, domain.DNSVerified, models.DomainStatusSSLGenerating, nil)

	// Try SSL generation
	err := b.hostingService.GenerateSSLForDomain(account, domain)
	if err != nil {
		errMsg := err.Error()
		log.Printf("[hosting-bg] SSL failed for %s: %v", domain.Domain, err)

		// Check if it's a retryable error
		if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "too many") {
			// Don't update status - will retry later
			return
		}

		// Update with error
		b.hostingService.UpdateDomainStatus(domain.ID, domain.DNSVerified, models.DomainStatusPendingDNS, &errMsg)
		return
	}

	// Success!
	log.Printf("[hosting-bg] SSL enabled for %s", domain.Domain)
	b.hostingService.UpdateDomainStatus(domain.ID, true, models.DomainStatusActive, nil)
	b.hostingService.SetDomainSSLEnabled(domain.ID, true)
}
