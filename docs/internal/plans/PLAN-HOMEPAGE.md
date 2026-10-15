# Homepage Redesign Plan

Inspired by Cloudflare OS landing page (https://os.cloudflare.app)

## Design Style
- Dark theme (#09090b background)
- Kumo UI flat design
- Large bold typography
- Orange/coral accent labels
- Clean sections with generous spacing

## Sections

### 1. Header
- Logo (left)
- "Contact Us" / "Sign In" button (right)
- Minimal, transparent background

### 2. Hero Section
- **Badge**: Small pill label (e.g., "GuardBot")
- **Headline**: Large text - "Protect your links from bots and fraud"
- **Subtitle**: Brief description of the service
- **CTA Button**: "Get Started" or "View Dashboard"

### 3. Feature Tabs (horizontal)
| Tab | Description |
|-----|-------------|
| Protect | Bot detection & blocking |
| Create | Generate short links |
| Track | Analytics & insights |
| Manage | Domain & link management |
| Deploy | Hosting integration |

### 4. Product Screenshot
- Large interactive-looking screenshot of the dashboard
- Floating UI elements showing features

### 5. Integration Section
- Label: "INTEGRATIONS"
- Headline: "Works with your existing tools"
- Logo grid: cPanel, Cloudflare, Telegram, etc.
- Brief description

### 6. Security Section
- Label: "SECURITY"
- Headline: "Keep bots out. Let humans in."
- Feature cards:
  - **Bot Detection**: AI-powered detection
  - **IP Whitelisting**: Your IPs always pass
  - **Real-time Analytics**: See who visits

### 7. Pricing/CTA Section
- Label: "GET STARTED"
- Headline: "Start protecting your links today"
- CTA button: "Create Account" or "View Pricing"

### 8. Footer
- Copyright
- Links: Privacy Policy, Terms, Support

## Technical Notes
- New template: `web/templates/pages/home.html`
- Public route (no auth required)
- Mobile responsive
- Smooth scroll animations (optional)

## Assets Needed
- Product screenshots
- Integration logos (cPanel, Cloudflare, Telegram)
- Feature icons

## Implementation Order
1. Create basic HTML structure
2. Add Kumo-styled sections
3. Add responsive CSS
4. Add content/copy
5. Test on mobile
