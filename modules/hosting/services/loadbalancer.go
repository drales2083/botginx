// Package services provides business logic for the hosting module.
package services

import (
	"errors"
	"sort"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/jmoiron/sqlx"
)

// LoadBalancer handles server selection based on current load
type LoadBalancer struct {
	db *sqlx.DB
}

// NewLoadBalancer creates a new load balancer instance
func NewLoadBalancer(db *sqlx.DB) *LoadBalancer {
	return &LoadBalancer{db: db}
}

// HasAvailableServers checks if any active servers with capacity exist
func (lb *LoadBalancer) HasAvailableServers() bool {
	var count int
	err := lb.db.Get(&count, `
		SELECT COUNT(*) FROM hosting_servers
		WHERE is_active = TRUE
		  AND current_accounts < max_accounts
		  AND COALESCE(type, 'cloudpanel') = 'cloudpanel'
	`)
	return err == nil && count > 0
}

// PickServer selects the least loaded active CloudPanel server with available capacity.
// Returns an error if no servers are available.
func (lb *LoadBalancer) PickServer() (*models.HostingServer, error) {
	var servers []models.HostingServer
	err := lb.db.Select(&servers, `
		SELECT * FROM hosting_servers
		WHERE is_active = TRUE
		  AND current_accounts < max_accounts
		  AND COALESCE(type, 'cloudpanel') = 'cloudpanel'
		ORDER BY current_accounts ASC
	`)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, errors.New("hosting service temporarily unavailable")
	}

	// Sort by least loaded (already ordered by DB, but ensure consistency)
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].CurrentAccounts < servers[j].CurrentAccounts
	})

	return &servers[0], nil
}

// IncrementServerCount increases the current_accounts count for a server
func (lb *LoadBalancer) IncrementServerCount(serverID string) error {
	_, err := lb.db.Exec(`
		UPDATE hosting_servers
		SET current_accounts = current_accounts + 1
		WHERE id = $1
	`, serverID)
	return err
}

// DecrementServerCount decreases the current_accounts count for a server (minimum 0)
func (lb *LoadBalancer) DecrementServerCount(serverID string) error {
	_, err := lb.db.Exec(`
		UPDATE hosting_servers
		SET current_accounts = GREATEST(current_accounts - 1, 0)
		WHERE id = $1
	`, serverID)
	return err
}
