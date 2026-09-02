package cloudpanel

import (
	"fmt"
	"strings"
)

// SiteType represents CloudPanel site types
type SiteType string

const (
	SiteTypePHP       SiteType = "php"
	SiteTypeNodeJS    SiteType = "nodejs"
	SiteTypePython    SiteType = "python"
	SiteTypeStatic    SiteType = "static"
	SiteTypeReverse   SiteType = "reverse-proxy"
)

// PHPVersion represents available PHP versions
type PHPVersion string

const (
	PHP74 PHPVersion = "7.4"
	PHP80 PHPVersion = "8.0"
	PHP81 PHPVersion = "8.1"
	PHP82 PHPVersion = "8.2"
	PHP83 PHPVersion = "8.3"
)

// SiteInfo holds site information from CloudPanel
type SiteInfo struct {
	Domain       string `json:"domain"`
	Type         string `json:"type"`
	PHPVersion   string `json:"php_version,omitempty"`
	RootDir      string `json:"root_directory"`
	User         string `json:"user"`
	VhostEnabled bool   `json:"vhost_enabled"`
}

// UserStats holds user statistics from CloudPanel
type UserStats struct {
	Username  string `json:"userName"`
	Sites     int    `json:"sites"`
	Databases int    `json:"databases"`
	SSHAccess bool   `json:"sshAccess"`
	Suspended bool   `json:"suspended"`
}

// AddUser creates a new CloudPanel user account.
// username: system username for the new account
// password: password for the account
// email: contact email address (optional in CloudPanel)
func (c *Client) AddUser(username, password, email string) error {
	// CloudPanel creates users via site creation typically
	// For SSH/FTP user: clpctl user:add --userName=X --password=X --siteUser=X
	cmd := fmt.Sprintf("clpctl user:add --userName=%s --password=%s",
		shellEscape(username), shellEscape(password))
	_, err := c.Execute(cmd)
	return err
}

// SuspendUser suspends a user account by disabling SSH access and site vhosts
func (c *Client) SuspendUser(username string) error {
	// CloudPanel doesn't have direct user suspension; suspend via disabling sites/SSH
	cmd := fmt.Sprintf("clpctl user:disable:ssh --userName=%s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// UnsuspendUser unsuspends a user account
func (c *Client) UnsuspendUser(username string) error {
	cmd := fmt.Sprintf("clpctl user:enable:ssh --userName=%s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// DeleteUser permanently deletes a user account
func (c *Client) DeleteUser(username string) error {
	cmd := fmt.Sprintf("clpctl user:delete --userName=%s --force", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// AddSite creates a new website with PHP support
// domain: primary domain name
// siteUser: the system user for this site
// siteUserPassword: password for the site user
// phpVersion: PHP version (e.g., "8.2")
func (c *Client) AddSite(domain, siteUser, siteUserPassword string, phpVersion PHPVersion) error {
	cmd := fmt.Sprintf("clpctl site:add:php --domainName=%s --phpVersion=%s --vhostTemplate='Generic' --siteUser=%s --siteUserPassword=%s",
		shellEscape(domain), string(phpVersion), shellEscape(siteUser), shellEscape(siteUserPassword))
	output, err := c.Execute(cmd)
	if err != nil {
		if output != "" {
			return fmt.Errorf("%s: %s", err.Error(), output)
		}
		return err
	}
	return nil
}

// AddStaticSite creates a static HTML site
func (c *Client) AddStaticSite(domain, siteUser string) error {
	cmd := fmt.Sprintf("clpctl site:add:static --domainName=%s --siteUser=%s",
		shellEscape(domain), shellEscape(siteUser))
	_, err := c.Execute(cmd)
	return err
}

// AddNodeJSSite creates a Node.js application site
func (c *Client) AddNodeJSSite(domain, siteUser string, nodeVersion string, port int) error {
	cmd := fmt.Sprintf("clpctl site:add:nodejs --domainName=%s --nodeJsVersion=%s --appPort=%d --siteUser=%s",
		shellEscape(domain), shellEscape(nodeVersion), port, shellEscape(siteUser))
	_, err := c.Execute(cmd)
	return err
}

// AddReverseProxySite creates a reverse proxy site
func (c *Client) AddReverseProxySite(domain, siteUser, reverseProxyURL string) error {
	cmd := fmt.Sprintf("clpctl site:add:reverse-proxy --domainName=%s --reverseProxyUrl=%s --siteUser=%s",
		shellEscape(domain), shellEscape(reverseProxyURL), shellEscape(siteUser))
	_, err := c.Execute(cmd)
	return err
}

// DeleteSite removes a website and all associated data
func (c *Client) DeleteSite(domain string) error {
	cmd := fmt.Sprintf("clpctl site:delete --domainName=%s --force", shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddDomain is an alias for AddSite with default PHP version
func (c *Client) AddDomain(username, password, domain string) error {
	return c.AddSite(domain, username, password, PHP82)
}

// DeleteDomain removes a web domain
func (c *Client) DeleteDomain(username, domain string) error {
	return c.DeleteSite(domain)
}

// AddLetsEncrypt enables Let's Encrypt SSL certificate for a domain
func (c *Client) AddLetsEncrypt(domain string) error {
	cmd := fmt.Sprintf("clpctl lets-encrypt:install:certificate --domainName=%s", shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddDatabase creates a database for a site.
// siteDomain: the domain the database belongs to
// dbName: name of the database
// dbUser: database username
// dbPassword: database user password
func (c *Client) AddDatabase(siteDomain, dbName, dbUser, dbPassword string) error {
	cmd := fmt.Sprintf("clpctl db:add --domainName=%s --databaseName=%s --databaseUserName=%s --databaseUserPassword=%s",
		shellEscape(siteDomain), shellEscape(dbName), shellEscape(dbUser), shellEscape(dbPassword))
	_, err := c.Execute(cmd)
	return err
}

// DeleteDatabase deletes a database
func (c *Client) DeleteDatabase(dbName string) error {
	cmd := fmt.Sprintf("clpctl db:delete --databaseName=%s --force", shellEscape(dbName))
	_, err := c.Execute(cmd)
	return err
}

// ListSites returns a list of all sites by parsing nginx sites-enabled
func (c *Client) ListSites() ([]SiteInfo, error) {
	// CloudPanel doesn't have site:list, parse nginx configs instead
	cmd := "ls /etc/nginx/sites-enabled/*.conf 2>/dev/null | xargs -n1 basename | sed 's/\\.conf$//' | grep -v default"
	output, err := c.Execute(cmd)
	if err != nil {
		return nil, err
	}

	var sites []SiteInfo
	for _, line := range strings.Split(output, "\n") {
		domain := strings.TrimSpace(line)
		if domain != "" {
			sites = append(sites, SiteInfo{Domain: domain})
		}
	}
	return sites, nil
}

// GetSiteInfo returns information about a specific site
func (c *Client) GetSiteInfo(domain string) (*SiteInfo, error) {
	sites, err := c.ListSites()
	if err != nil {
		return nil, err
	}

	for _, site := range sites {
		if site.Domain == domain {
			return &site, nil
		}
	}
	return nil, fmt.Errorf("site not found: %s", domain)
}

// EnableVhost enables the nginx vhost for a site
func (c *Client) EnableVhost(domain string) error {
	cmd := fmt.Sprintf("clpctl site:enable --domainName=%s", shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// DisableVhost disables the nginx vhost for a site
func (c *Client) DisableVhost(domain string) error {
	cmd := fmt.Sprintf("clpctl site:disable --domainName=%s", shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// TestConnection verifies SSH connectivity to the CloudPanel server
func (c *Client) TestConnection() error {
	output, err := c.Execute("echo ok")
	if err != nil {
		return err
	}
	if output != "ok" {
		return fmt.Errorf("unexpected response: %s", output)
	}
	return nil
}

// GetCloudPanelVersion returns the installed CloudPanel version
func (c *Client) GetCloudPanelVersion() (string, error) {
	output, err := c.Execute("clpctl system:info | grep 'CloudPanel' | head -1")
	if err != nil {
		// Try alternative method
		output, err = c.Execute("cat /home/clp/htdocs/app/files/VERSION 2>/dev/null || echo 'unknown'")
		if err != nil {
			return "unknown", nil
		}
	}
	return strings.TrimSpace(output), nil
}

// GetServerLoad returns current server load
func (c *Client) GetServerLoad() (string, error) {
	output, err := c.Execute("uptime | awk -F'load average:' '{print $2}'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// GetDiskUsage returns disk usage information
func (c *Client) GetDiskUsage() (string, error) {
	output, err := c.Execute("df -h / | tail -1 | awk '{print $3 \"/\" $2 \" (\" $5 \")\"}'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// ReloadNginx reloads nginx configuration
func (c *Client) ReloadNginx() error {
	_, err := c.Execute("nginx -t && systemctl reload nginx")
	return err
}

// RestartPHP restarts PHP-FPM service for a specific version
func (c *Client) RestartPHP(version PHPVersion) error {
	cmd := fmt.Sprintf("systemctl restart php%s-fpm", string(version))
	_, err := c.Execute(cmd)
	return err
}
