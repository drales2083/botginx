package actions

// DomainRepository defines the interface for domain data access.
// Implementations: modules/domains/services.DomainService
type DomainRepository interface {
	GetByID(id string) (interface{}, error)
	GetByName(name string) (interface{}, error)
	Create(input interface{}) (interface{}, error)
	Update(id string, input interface{}) error
	Delete(id string) error
}

// Note: DNSVerifier interface is defined in verify_dns.go
// Note: SSLProvisioner interface is defined in provision_ssl.go
