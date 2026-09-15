# Botginx - Complete Sections Reference

This document describes ALL sections of the Botginx application for theme recreation.
The application uses AdminLTE v4.9.1 with a dark theme and red primary color.

---

## Application Overview

- **Framework:** Go backend with HTML templates
- **Template Engine:** Go html/template
- **CSS Framework:** AdminLTE 4.9.1 (Bootstrap 5.3)
- **Theme:** Dark mode (`data-bs-theme="dark"`)
- **Primary Color:** Red/Danger (`data-lte-primary="danger"`)
- **Icons:** Bootstrap Icons

---

## USER SECTION (`/user/*`)

### 1. Dashboard
- **Route:** `/user/dashboard`
- **Template:** `modules/dashboard/templates/index.html`
- **Icon:** `bi-speedometer2`
- **Description:** Overview of user's account with stats widgets
- **Data:**
  - Total domains count
  - Total redirect links count
  - Total visits (today/all time)
  - Recent activity list
  - Quick stats cards

### 2. Domains
- **Route:** `/user/domains`
- **Template:** `modules/domains/templates/list.html`
- **Icon:** `bi-globe`
- **Description:** List of user's domains with status indicators
- **Sub-pages:**
  - `/user/domains/new` - Add new domain form
  - `/user/domains/{id}` - Domain details page
  - `/user/domains/{id}/setup` - Domain setup wizard (DNS, SSL)
- **Data:**
  ```
  Domain {
    ID, Name, ServerIP, ServerName
    DNSVerified, SSLEnabled, IsWildcard
    SetupStep, CreatedAt
  }
  ```

### 3. Buy Domain (Marketplace)
- **Route:** `/user/marketplace`
- **Template:** `modules/marketplace/templates/user_browse.html`
- **Icon:** `bi-bag`
- **Description:** Browse and purchase domains from admin inventory
- **Data:**
  ```
  MarketplaceDomain {
    ID, Name, Price, TLD
    IsAvailable, CreatedAt
  }
  ```

### 4. Redirect Links
- **Route:** `/user/links`
- **Template:** `modules/redirectlinks/templates/list.html`
- **Icon:** `bi-link-45deg`
- **Description:** List of redirect links with deploy status
- **Sub-pages:**
  - `/user/links/new` - Create new redirect link
  - `/user/links/{id}` - Link details and analytics
  - `/user/links/{id}/customize` - Full customizer with live preview
  - `/user/links/{id}/edit-html` - Direct HTML editor
- **Data:**
  ```
  RedirectLink {
    ID, Name, Slug, DomainID
    DestinationURLs[], DeployStatus
    AnimationDuration, Customization{}
    CreatedAt, UpdatedAt
  }
  ```

### 5. Analytics
- **Route:** `/user/analytics`
- **Template:** `modules/analytics/templates/overview.html`
- **Icon:** `bi-graph-up`
- **Description:** Traffic analytics with charts and tables
- **Sub-pages:**
  - `/user/analytics/settings` - Bot protection settings
  - `/user/analytics/link/{id}` - Per-link analytics
- **Data:**
  ```
  Visit {
    ID, LinkID, IP, Country
    UserAgent, Referer, Blocked
    BehaviorScore, CreatedAt
  }
  Stats: timeline, countries, devices, browsers
  ```

### 6. Short Links
- **Route:** `/user/shortener`
- **Template:** `modules/shortener/templates/list.html`
- **Icon:** `bi-link`
- **Description:** URL shortener with custom slugs
- **Sub-pages:**
  - `/user/shortener/new` - Create short link
  - `/user/shortener/{id}` - Short link details
- **Data:**
  ```
  ShortLink {
    ID, Slug, DestinationURL
    DomainID, Clicks, CreatedAt
  }
  ```

### 7. IP Lists
- **Route:** `/user/iplists/whitelist`
- **Template:** `modules/iplists/templates/whitelist.html`
- **Icon:** `bi-shield-lock`
- **Description:** Manage IP whitelist and blocklist
- **Sub-pages:**
  - `/user/iplists/blocklist` - IP blocklist
- **Data:**
  ```
  IPWhitelist { ID, IP, Note, CreatedAt }
  IPBlocklist { ID, IP, Reason, CreatedAt }
  ```

### 8. Hosting (CloudPanel)
- **Route:** `/user/hosting`
- **Template:** `modules/hosting/templates/index.html`
- **Icon:** `bi-shield-check`
- **Description:** Managed hosting accounts
- **Sub-pages:**
  - `/user/hosting/buy` - Purchase hosting package
  - `/user/hosting/{accountID}` - Account overview
  - `/user/hosting/{accountID}/domains` - Hosting domains
  - `/user/hosting/{accountID}/domains/{id}/setup` - Domain setup
  - `/user/hosting/{accountID}/domains/{id}/analytics` - Domain analytics
  - `/user/hosting/{accountID}/domains/{id}/settings` - Domain settings
- **Data:**
  ```
  HostingAccount {
    ID, UserID, PackageID, ServerID
    Status, PanelUsername, CreatedAt
  }
  HostingDomain {
    ID, AccountID, Name, SSLEnabled
  }
  ```

### 9. Payments
- **Route:** `/user/payments`
- **Template:** `modules/payments/templates/transactions.html`
- **Icon:** `bi-wallet2`
- **Description:** Transaction history and deposits
- **Sub-pages:**
  - `/user/payments/deposit` - Add funds (crypto)
- **Data:**
  ```
  Transaction {
    ID, UserID, Type, Amount
    Currency, Status, CreatedAt
  }
  User.Balance (shown in navbar)
  ```

### 10. Subscription
- **Route:** `/user/subscription`
- **Template:** `modules/auth/templates/subscription.html`
- **Icon:** `bi-credit-card`
- **Description:** Subscription status and renewal
- **Data:**
  ```
  Subscription {
    Status: active|expiring|expired|none
    ExpiresAt, DaysLeft, Plan
  }
  ```

### 11. Referrals
- **Route:** `/user/referrals`
- **Template:** `modules/referrals/templates/index.html`
- **Icon:** `bi-people`
- **Description:** Referral program with earnings
- **Data:**
  ```
  User.ReferralCode
  Referrals[] { Email, SignedUp, Earned }
  TotalEarnings, PendingEarnings
  ```

### 12. Settings
- **Route:** `/user/settings`
- **Template:** `modules/auth/templates/settings.html`
- **Icon:** `bi-gear`
- **Description:** Account settings, password change
- **Data:**
  ```
  User { Email, Name }
  GlobalWhitelist[] (for bots)
  ```

### 13. Help / FAQs
- **Route:** `/user/help`
- **Template:** `modules/help/templates/index.html`
- **Icon:** `bi-question-circle`
- **Description:** FAQ accordion with categories
- **Data:**
  ```
  FAQCategory { ID, Name, Order }
  FAQItem { ID, CategoryID, Question, Answer, Order }
  ```

---

## ADMIN SECTION (`/admin/*`)

### 1. Admin Dashboard
- **Route:** `/admin/admindash`
- **Template:** `modules/admindash/templates/index.html`
- **Icon:** `bi-speedometer2`
- **Description:** System-wide statistics
- **Sub-pages:**
  - `/admin/admindash/domains/all` - All domains list
- **Data:**
  ```
  Stats {
    totalUsers, activeUsers (subscriptions)
    totalDomains, sharedDomains
    totalLinks, deployedLinks
    totalVisits, todayVisits, blockedVisits
  }
  Timeline chart data
  Recent activity
  ```

### 2. Users Management
- **Route:** `/admin/users`
- **Template:** `modules/users/templates/list.html`
- **Icon:** `bi-people`
- **Description:** Manage all users
- **Sub-pages:**
  - `/admin/users/new` - Create user
  - `/admin/users/{id}` - User details, grant subscription, impersonate
- **Data:**
  ```
  User {
    ID, Email, Name, Role
    Balance, IsActive
    SubscriptionStatus, CreatedAt
  }
  ```
- **Actions:** Grant subscription, Add balance, Impersonate, Deactivate

### 3. Shared Domains
- **Route:** `/admin/domains`
- **Template:** `modules/domains/templates/shared_list.html`
- **Icon:** `bi-globe2`
- **Description:** Manage shared domains for all users
- **Sub-pages:**
  - `/admin/domains/new` - Add shared domain
  - `/admin/domains/{id}/setup` - Setup shared domain
- **Data:**
  ```
  SharedDomain {
    ID, Name, ServerID, IsShared: true
    DNSVerified, SSLEnabled
  }
  ```

### 4. Servers
- **Route:** `/admin/servers`
- **Template:** `modules/servers/templates/list.html`
- **Icon:** `bi-server`
- **Description:** Manage deployment servers
- **Sub-pages:**
  - `/admin/servers/new` - Add server
  - `/admin/servers/{id}` - Server details
  - `/admin/servers/{id}/logs` - Server logs
  - `/admin/servers/{id}/terminal` - SSH terminal
- **Data:**
  ```
  Server {
    ID, Name, IP, SSHUser, SSHPort
    Type, IsActive, LastSeen
  }
  ```

### 5. Marketplace (Sell Domains)
- **Route:** `/admin/marketplace`
- **Template:** `modules/marketplace/templates/admin_list.html`
- **Icon:** `bi-tag`
- **Description:** Add domains to marketplace for sale
- **Data:**
  ```
  MarketplaceDomain {
    ID, Name, Price, TLD
    IsAvailable, SoldTo, SoldAt
  }
  ```

### 6. Hosting Management
- **Route:** `/admin/hosting`
- **Template:** `modules/hosting/templates/admin_servers.html`
- **Icon:** `bi-shield-check`
- **Description:** Manage hosting infrastructure
- **Sub-pages:**
  - `/admin/hosting/packages` - Hosting packages/plans
  - `/admin/hosting/accounts` - All user accounts
- **Data:**
  ```
  HostingServer { ID, Name, IP, PanelURL, MaxAccounts }
  HostingPackage { ID, Name, PriceMonthly, Specs }
  HostingAccount { ID, UserID, Status, ServerID }
  ```

### 7. Payments Dashboard
- **Route:** `/admin/payments`
- **Template:** `modules/payments/templates/admin_dashboard.html`
- **Icon:** `bi-currency-dollar`
- **Description:** Financial overview
- **Sub-pages:**
  - `/admin/payments/transactions` - All transactions
- **Data:**
  ```
  Stats { totalDeposits, pendingPayments, revenue }
  RecentTransactions[]
  ```

### 8. Referrals Settings
- **Route:** `/admin/referrals/settings`
- **Template:** `modules/referrals/templates/settings.html`
- **Icon:** `bi-diagram-3`
- **Description:** Configure referral program
- **Data:**
  ```
  Settings {
    CommissionPercent, MinPayout
    IsEnabled
  }
  ```

### 9. FAQs Management
- **Route:** `/admin/help`
- **Template:** `modules/help/templates/admin_list.html`
- **Icon:** `bi-question-circle`
- **Description:** Manage FAQ categories and items
- **Data:**
  ```
  FAQCategory { ID, Name, Order }
  FAQItem { ID, CategoryID, Question, Answer }
  ```

### 10. Modules
- **Route:** `/admin/modules`
- **Template:** `modules/modules/templates/list.html`
- **Icon:** `bi-puzzle`
- **Description:** Enable/disable system modules
- **Sub-pages:**
  - `/admin/modules/{id}` - Module details
- **Data:**
  ```
  Module { ID, Name, Description, IsEnabled, Version }
  ```

---

## AUTH PAGES (No sidebar)

### Login
- **Route:** `/auth/login`
- **Template:** `modules/auth/templates/login.html`
- **Layout:** Centered card, no sidebar
- **Fields:** Email, Password, Remember me

### Signup
- **Route:** `/auth/signup`
- **Template:** `modules/auth/templates/signup.html`
- **Layout:** Centered card, no sidebar
- **Fields:** Name, Email, Password, Referral code (optional)

---

## COMMON UI COMPONENTS

### Base Layout
- **Template:** `web/templates/layouts/base.html`
- **Sections:**
  - Navbar (top): Language switcher, Balance, User dropdown
  - Sidebar (left): User menu + Admin menu (if admin)
  - Content area: Breadcrumbs, Alerts, Page content
  - Footer: Build version

### Alerts/Banners
- Subscription warning (expired/expiring)
- Domain promo banner (no domains)
- Impersonation banner (admin viewing as user)

### Data Tables
- Uses simple-datatables library
- Search, sort, pagination
- Attributes: `data-datatable`, `data-per-page`, `data-no-sort`

### Toast Notifications
- Position: Bottom right
- Variants: success, danger, warning, primary
- Functions: `showToast()`, `flashToast()`, `reloadWithToast()`

### Cards
- `.card` with `.card-header`, `.card-body`
- Info boxes for stats (`.info-box`)
- Outline variants (`.card-primary`, `.card-outline`)

### Badges
- Status indicators: success (verified), warning (pending), danger (error)
- Icons inline with text

### Forms
- Bootstrap 5 form controls
- `.form-control-sm` for compact forms
- Input groups with copy buttons
- Form switches (square style, not round)

### Modals
- Confirmation dialogs
- Form modals
- Bootstrap 5 modal pattern

---

## COLOR TOKENS

```css
--bs-primary: #dc3545 (red/danger)
--bs-success: #198754
--bs-warning: #ffc107
--bs-danger: #dc3545
--bs-info: #0dcaf0
--bs-secondary: #6c757d
--bs-body-bg: dark theme bg
--bs-body-color: light text
```

---

## SAMPLE DATA FOR TESTING

### User
```json
{
  "id": "abc123",
  "email": "user@example.com",
  "name": "John Doe",
  "role": "user",
  "balance": 125.50,
  "isActive": true
}
```

### Domain
```json
{
  "id": "dom456",
  "name": "example.com",
  "serverIP": "123.45.67.89",
  "serverName": "US-East-1",
  "dnsVerified": true,
  "sslEnabled": true,
  "isWildcard": false,
  "setupStep": "complete"
}
```

### Redirect Link
```json
{
  "id": "link789",
  "name": "Summer Campaign",
  "slug": "summer",
  "domainId": "dom456",
  "destinationUrls": ["https://offer1.com", "https://offer2.com"],
  "deployStatus": "deployed",
  "animationDuration": 2
}
```

### Visit/Analytics
```json
{
  "id": "visit001",
  "linkId": "link789",
  "ip": "192.168.1.1",
  "country": "US",
  "userAgent": "Mozilla/5.0...",
  "referer": "https://facebook.com",
  "blocked": false,
  "behaviorScore": 85
}
```

---

## NOTES FOR THEME AGENT

1. **Dark theme is default** - All components assume dark backgrounds
2. **Red primary** - Primary buttons, active states, accents are red
3. **Icons** - Bootstrap Icons (`bi-*`) used throughout
4. **Responsive** - Mobile-friendly sidebar that collapses
5. **DataTables** - Custom dark theme overrides in base.html
6. **Square switches** - Form switches are square, not round (CSS override)
7. **Font scale** - Root font-size is 120% of default
8. **Sidebar width** - 330px (custom variable)
