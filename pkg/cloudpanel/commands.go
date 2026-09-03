package cloudpanel

import (
	"fmt"
	"log"
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

// AddPanelUser creates a new CloudPanel panel user who can login to the web interface.
// username: panel username
// password: password for the account
// email: contact email address
// sites: comma-separated list of sites this user can manage
func (c *Client) AddPanelUser(username, password, email, sites string) error {
	cmd := fmt.Sprintf("clpctl user:add --userName=%s --email=%s --firstName='Site' --lastName='User' --password=%s --role='user' --sites=%s --timezone='UTC' --status='1'",
		shellEscape(username), shellEscape(email), shellEscape(password), shellEscape(sites))
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
// Uses "Botection" vhost template if available (routes through antibot on port 8080)
func (c *Client) AddSite(domain, siteUser, siteUserPassword string, phpVersion PHPVersion) error {
	log.Printf("[cloudpanel] AddSite called with domain=%s user=%s passwordLen=%d", domain, siteUser, len(siteUserPassword))

	// Determine which template to use - prefer Botection if botection is installed
	template := "Generic"
	if c.IsBotectionInstalled() && c.HasBotectionTemplate() {
		template = "Botection"
		log.Printf("[cloudpanel] using Botection vhost template (antibot routing enabled)")
	}

	cmd := fmt.Sprintf("clpctl site:add:php --domainName=%s --phpVersion=%s --vhostTemplate='%s' --siteUser=%s --siteUserPassword=%s",
		shellEscape(domain), string(phpVersion), template, shellEscape(siteUser), shellEscape(siteUserPassword))
	log.Printf("[cloudpanel] executing: %s", cmd)
	output, err := c.Execute(cmd)
	if err != nil {
		if output != "" {
			return fmt.Errorf("%s: %s", err.Error(), output)
		}
		return err
	}
	return nil
}

// HasBotectionTemplate checks if the Botection vhost template exists in CloudPanel
func (c *Client) HasBotectionTemplate() bool {
	output, err := c.Execute("clpctl vhost-templates:list 2>/dev/null | grep -q 'Botection' && echo yes || echo no")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "yes"
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

// ConfigureBotectionProxy modifies a site's nginx config to route through botection.
// This creates a two-server setup:
//   1. External server (80/443) -> proxy_pass to botection (8080)
//   2. Backend server (8081) -> serves actual site content
// Traffic flow: Internet -> nginx:443 -> botection:8080 -> nginx:8081 -> PHP/files
func (c *Client) ConfigureBotectionProxy(domain string) error {
	// This script modifies the CloudPanel-generated nginx config for botection integration
	script := fmt.Sprintf(`
#!/bin/bash
set -e
DOMAIN=%s
CONF="/etc/nginx/sites-enabled/${DOMAIN}.conf"
BACKEND_PORT=8081
BOTECTION_PORT=8080

if [ ! -f "$CONF" ]; then
    echo "Config not found: $CONF"
    exit 1
fi

# Skip if already configured for botection
if grep -q "proxy_pass http://127.0.0.1:${BOTECTION_PORT}" "$CONF"; then
    echo "Already configured for botection"
    exit 0
fi

# Backup original
cp "$CONF" "${CONF}.pre-botection"

# Get the site user from the config
SITE_USER=$(grep -oP "root /home/\K[^/]+" "$CONF" | head -1)
if [ -z "$SITE_USER" ]; then
    echo "Could not determine site user"
    exit 1
fi

# Get PHP-FPM port from config
PHP_PORT=$(grep -oP "fastcgi_pass 127.0.0.1:\K\d+" "$CONF" | head -1)
if [ -z "$PHP_PORT" ]; then
    PHP_PORT=9000
fi

# Determine root directory
ROOT_DIR="/home/${SITE_USER}/htdocs/${DOMAIN}"

# Create new config with botection integration
cat > "$CONF" << 'NGINXEOF'
# Botection-integrated config for DOMAIN_PLACEHOLDER
# Traffic: Internet -> nginx:443 -> botection:8080 -> nginx:8081 -> PHP

# Redirect www to non-www
server {
    listen 80;
    listen [::]:80;
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    ssl_certificate_key /etc/nginx/ssl-certificates/DOMAIN_PLACEHOLDER.key;
    ssl_certificate /etc/nginx/ssl-certificates/DOMAIN_PLACEHOLDER.crt;
    server_name www.DOMAIN_PLACEHOLDER;
    return 301 https://DOMAIN_PLACEHOLDER$request_uri;
}

# Main site - external traffic goes through botection
server {
    listen 80;
    listen [::]:80;
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    ssl_certificate_key /etc/nginx/ssl-certificates/DOMAIN_PLACEHOLDER.key;
    ssl_certificate /etc/nginx/ssl-certificates/DOMAIN_PLACEHOLDER.crt;
    server_name DOMAIN_PLACEHOLDER;
    root ROOT_DIR_PLACEHOLDER;

    access_log /home/SITE_USER_PLACEHOLDER/logs/nginx/access.log;
    error_log /home/SITE_USER_PLACEHOLDER/logs/nginx/error.log;

    # Force HTTPS
    if ($scheme != "https") {
        rewrite ^ https://$host$request_uri permanent;
    }

    # ACME challenge bypass (for SSL renewal)
    location ~ /.well-known {
        auth_basic off;
        allow all;
    }

    # All traffic through botection
    location / {
        proxy_pass http://127.0.0.1:BOTECTION_PORT_PLACEHOLDER;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_hide_header X-Varnish;
        proxy_redirect off;
        proxy_max_temp_file_size 0;
        proxy_connect_timeout 720;
        proxy_send_timeout 720;
        proxy_read_timeout 720;
        proxy_buffer_size 128k;
        proxy_buffers 4 256k;
        proxy_busy_buffers_size 256k;
        proxy_temp_file_write_size 256k;
    }

    # Static files bypass (optional - can go through botection too)
    location ~* ^.+\.(css|js|jpg|jpeg|gif|png|ico|svg|woff|woff2)$ {
        add_header Access-Control-Allow-Origin "*";
        expires max;
        access_log off;
    }

    # Deny hidden files
    location ~ /\.(ht|svn|git) {
        deny all;
    }
}

# Backend server - receives traffic from botection
server {
    listen 127.0.0.1:BACKEND_PORT_PLACEHOLDER;
    server_name DOMAIN_PLACEHOLDER;
    root ROOT_DIR_PLACEHOLDER;

    index index.php index.html;
    try_files $uri $uri/ /index.php?$args;

    # PHP handling
    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_intercept_errors on;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        try_files $uri =404;
        fastcgi_read_timeout 3600;
        fastcgi_send_timeout 3600;
        fastcgi_param HTTPS "on";
        fastcgi_param SERVER_PORT 443;
        fastcgi_pass 127.0.0.1:PHP_PORT_PLACEHOLDER;
        fastcgi_param PHP_VALUE "
error_log=/home/SITE_USER_PLACEHOLDER/logs/php/error.log;
memory_limit=512M;
max_execution_time=60;
max_input_time=60;
max_input_vars=10000;
post_max_size=64M;
upload_max_filesize=64M;
date.timezone=UTC;
display_errors=off;";
    }

    # Deny hidden files
    location ~ /\.(ht|svn|git) {
        deny all;
    }
}
NGINXEOF

# Replace placeholders
sed -i "s/DOMAIN_PLACEHOLDER/${DOMAIN}/g" "$CONF"
sed -i "s|ROOT_DIR_PLACEHOLDER|${ROOT_DIR}|g" "$CONF"
sed -i "s/SITE_USER_PLACEHOLDER/${SITE_USER}/g" "$CONF"
sed -i "s/BOTECTION_PORT_PLACEHOLDER/${BOTECTION_PORT}/g" "$CONF"
sed -i "s/BACKEND_PORT_PLACEHOLDER/${BACKEND_PORT}/g" "$CONF"
sed -i "s/PHP_PORT_PLACEHOLDER/${PHP_PORT}/g" "$CONF"

# Test and reload nginx
nginx -t && systemctl reload nginx
echo "Botection proxy configured for ${DOMAIN}"
`, shellEscape(domain))

	_, err := c.Execute(script)
	return err
}

// IsBotectionInstalled checks if botection is running on the server
func (c *Client) IsBotectionInstalled() bool {
	output, err := c.Execute("systemctl is-active botection 2>/dev/null || echo inactive")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "active"
}
