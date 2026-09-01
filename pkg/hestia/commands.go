package hestia

import (
	"encoding/json"
	"fmt"
)

// UserStats holds user statistics returned by v-list-user
type UserStats struct {
	Username  string `json:"USERNAME"`
	DiskUsed  string `json:"U_DISK"`
	DiskQuota string `json:"DISK_QUOTA"`
	BWUsed    string `json:"U_BANDWIDTH"`
	BWQuota   string `json:"BANDWIDTH"`
	Domains   string `json:"U_WEB_DOMAINS"`
	Mail      string `json:"U_MAIL_ACCOUNTS"`
	Databases string `json:"U_DATABASES"`
	Suspended string `json:"SUSPENDED"`
}

// AddUser creates a new HestiaCP user account.
// username: system username for the new account
// password: password for the account
// email: contact email address
// pkg: hosting package name (e.g., "default")
// name: display name for the user
func (c *Client) AddUser(username, password, email, pkg, name string) error {
	cmd := fmt.Sprintf("v-add-user %s %s %s %s %s",
		shellEscape(username), shellEscape(password), shellEscape(email),
		shellEscape(pkg), shellEscape(name))
	_, err := c.Execute(cmd)
	return err
}

// SuspendUser suspends a user account, disabling all services
func (c *Client) SuspendUser(username string) error {
	cmd := fmt.Sprintf("v-suspend-user %s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// UnsuspendUser unsuspends a user account, restoring all services
func (c *Client) UnsuspendUser(username string) error {
	cmd := fmt.Sprintf("v-unsuspend-user %s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// DeleteUser permanently deletes a user account and all associated data
func (c *Client) DeleteUser(username string) error {
	cmd := fmt.Sprintf("v-delete-user %s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// AddDomain adds a web domain to a user account
func (c *Client) AddDomain(username, domain string) error {
	cmd := fmt.Sprintf("v-add-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// DeleteDomain removes a web domain from a user account
func (c *Client) DeleteDomain(username, domain string) error {
	cmd := fmt.Sprintf("v-delete-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddLetsEncrypt enables Let's Encrypt SSL certificate for a domain
func (c *Client) AddLetsEncrypt(username, domain string) error {
	cmd := fmt.Sprintf("v-add-letsencrypt-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddMailDomain enables mail services for a domain
func (c *Client) AddMailDomain(username, domain string) error {
	cmd := fmt.Sprintf("v-add-mail-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddMailAccount creates an email account for a domain.
// account: the local part of the email (before @)
// password: password for the mail account
func (c *Client) AddMailAccount(username, domain, account, password string) error {
	cmd := fmt.Sprintf("v-add-mail-account %s %s %s %s",
		shellEscape(username), shellEscape(domain), shellEscape(account), shellEscape(password))
	_, err := c.Execute(cmd)
	return err
}

// DeleteMailAccount deletes an email account from a domain
func (c *Client) DeleteMailAccount(username, domain, account string) error {
	cmd := fmt.Sprintf("v-delete-mail-account %s %s %s",
		shellEscape(username), shellEscape(domain), shellEscape(account))
	_, err := c.Execute(cmd)
	return err
}

// AddDatabase creates a database for a user.
// dbName: name of the database
// dbUser: database username
// dbPassword: database user password
func (c *Client) AddDatabase(username, dbName, dbUser, dbPassword string) error {
	cmd := fmt.Sprintf("v-add-database %s %s %s %s",
		shellEscape(username), shellEscape(dbName), shellEscape(dbUser), shellEscape(dbPassword))
	_, err := c.Execute(cmd)
	return err
}

// DeleteDatabase deletes a database from a user account
func (c *Client) DeleteDatabase(username, dbName string) error {
	cmd := fmt.Sprintf("v-delete-database %s %s", shellEscape(username), shellEscape(dbName))
	_, err := c.Execute(cmd)
	return err
}

// AddFTP creates an FTP account for file access.
// ftpUser: FTP username
// ftpPassword: FTP password
// path: directory path relative to the user's home (e.g., "/web/domain.com/public_html")
func (c *Client) AddFTP(username, ftpUser, ftpPassword, path string) error {
	cmd := fmt.Sprintf("v-add-ftp %s %s %s %s",
		shellEscape(username), shellEscape(ftpUser), shellEscape(ftpPassword), shellEscape(path))
	_, err := c.Execute(cmd)
	return err
}

// DeleteFTP deletes an FTP account
func (c *Client) DeleteFTP(username, ftpUser string) error {
	cmd := fmt.Sprintf("v-delete-ftp %s %s", shellEscape(username), shellEscape(ftpUser))
	_, err := c.Execute(cmd)
	return err
}

// ListUser returns usage statistics for a user account
func (c *Client) ListUser(username string) (*UserStats, error) {
	cmd := fmt.Sprintf("v-list-user %s json", shellEscape(username))
	output, err := c.Execute(cmd)
	if err != nil {
		return nil, err
	}

	// HestiaCP returns JSON in format: {"username": {...stats...}}
	var result map[string]UserStats
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("parse user stats: %w", err)
	}

	for _, stats := range result {
		return &stats, nil
	}
	return nil, fmt.Errorf("user not found: %s", username)
}

// TestConnection verifies SSH connectivity to the HestiaCP server
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
