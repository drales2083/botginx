-- FAQ Categories
CREATE TABLE IF NOT EXISTS faq_categories (
    id VARCHAR(24) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    icon VARCHAR(50) DEFAULT 'bi-question-circle',
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- FAQ Items
CREATE TABLE IF NOT EXISTS faq_items (
    id VARCHAR(24) PRIMARY KEY,
    category_id VARCHAR(24) REFERENCES faq_categories(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    sort_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_faq_items_category ON faq_items(category_id);
CREATE INDEX IF NOT EXISTS idx_faq_items_active ON faq_items(is_active);

-- Seed Categories
INSERT INTO faq_categories (id, name, icon, sort_order) VALUES
    ('cat_domains', 'Adding Your Domain', 'bi-globe', 1),
    ('cat_links', 'Creating Redirect Links', 'bi-link-45deg', 2),
    ('cat_editing', 'Editing & Customizing', 'bi-pencil', 3),
    ('cat_protection', 'Bot Protection', 'bi-shield-check', 4),
    ('cat_analytics', 'Analytics', 'bi-graph-up', 5),
    ('cat_account', 'Account', 'bi-person-gear', 6)
ON CONFLICT (id) DO NOTHING;

-- Seed FAQ Items
INSERT INTO faq_items (id, category_id, question, answer, sort_order) VALUES

-- Adding Your Domain (detailed guide)
('faq_dom_1', 'cat_domains', 'How do I add my first domain?',
 'Go to <a href="/user/domains">Domains</a> and click <strong>Add Domain</strong>. Type your domain name (example.com) and hit Add. You''ll see DNS records to configure at your registrar. Once those are set, come back and verify.',
 1),

('faq_dom_2', 'cat_domains', 'Should I use a wildcard domain?',
 'If you''re on cPanel, yes. Add your domain as <code>*.yourdomain.com</code> instead of plain <code>yourdomain.com</code>. This lets you create unlimited subdomains (promo.yourdomain.com, offer.yourdomain.com, etc.) without touching cPanel again. Non-cPanel users can add either way.',
 2),

('faq_dom_3', 'cat_domains', 'What DNS records do I need to add?',
 'Two records. First, an <strong>A record</strong> pointing your domain to our server IP. Second, a <strong>TXT record</strong> for verification. Both values show up after you add the domain. Copy them exactly into your registrar''s DNS panel.',
 3),

('faq_dom_4', 'cat_domains', 'DNS is set but verification fails. What now?',
 'DNS can take anywhere from 5 minutes to 48 hours to propagate, though most changes show up within 30 minutes. We check authoritative nameservers first to speed things up. If it''s been over an hour, double-check your A record value and make sure there''s no typo in the TXT record.',
 4),

('faq_dom_5', 'cat_domains', 'Do I need to set up SSL manually?',
 'No. SSL certificates generate automatically once DNS verification passes. Wildcard domains get wildcard certificates. The whole process takes about a minute after verification.',
 5),

('faq_dom_6', 'cat_domains', 'The setup wizard is stuck. How do I restart it?',
 'Go to <a href="/user/domains">Domains</a>, find your domain, and click <strong>Setup</strong>. The wizard picks up where it left off. If something went wrong with the ACME challenge, you can delete the domain and add it again to start fresh.',
 6),

-- Creating Redirect Links (detailed guide)
('faq_link_1', 'cat_links', 'How do I create my first redirect link?',
 'Head to <a href="/user/redirectlinks">Redirect Links</a> and click <strong>Create Redirect Link</strong>. Pick a domain from the dropdown (you need at least one verified domain first). Enter your destination URL. Hit Create. Your link goes live in seconds.',
 1),

('faq_link_2', 'cat_links', 'What is the difference between Redirect and HTML type?',
 '<strong>Redirect</strong> shows a splash page with a loading animation before sending visitors to your destination. You can customize colors, text, and timing in the <a href="/user/redirectlinks">Customize</a> panel. <strong>HTML</strong> serves your own complete HTML page. Use this when you want full control over what visitors see.',
 2),

('faq_link_3', 'cat_links', 'Can I add multiple destination URLs?',
 'Yes. Click the + button to add more URLs. Visitors get randomly sent to one of them. Good for A/B testing landing pages or spreading traffic across multiple offers.',
 3),

('faq_link_4', 'cat_links', 'What''s the subdomain field for?',
 'Each link gets a unique subdomain like <code>promo.yourdomain.com</code> or <code>offer.yourdomain.com</code>. The system generates random ones (sunny-fox, quick-river), but you can type your own. Subdomains look more professional than long URL paths.',
 4),

('faq_link_5', 'cat_links', 'My link says "pending" and won''t deploy.',
 'Check that your domain is fully verified with SSL. Go to <a href="/user/domains">Domains</a> and confirm you see a green "SSL Ready" badge. If DNS or SSL setup isn''t complete, links can''t deploy.',
 5),

('faq_link_6', 'cat_links', 'How do I copy the link URL?',
 'On the <a href="/user/redirectlinks">Redirect Links</a> list, click the dropdown next to any link and select Copy URL. Or open the link details and use the copy button next to the Live URL field.',
 6),

-- Editing & Customizing
('faq_edit_1', 'cat_editing', 'How do I change my destination URLs?',
 'Open the link from <a href="/user/redirectlinks">Redirect Links</a>, scroll to Destinations, and click <strong>Edit URLs</strong>. Add, remove, or reorder URLs. Save when done. Changes deploy within a few seconds.',
 1),

('faq_edit_2', 'cat_editing', 'How do I customize the splash page?',
 'Click <strong>Customize</strong> on any Redirect-type link. You''ll see a live preview on the left and controls on the right. Change background colors, loader animation style, heading text, and timing. Click Save & Deploy to push changes live.',
 2),

('faq_edit_3', 'cat_editing', 'How do I edit my HTML page?',
 'For HTML-type links, click <strong>Edit HTML</strong> instead of Customize. You get a code editor with syntax highlighting. Edit your HTML, preview it, and click Save & Deploy.',
 3),

('faq_edit_4', 'cat_editing', 'Can I change a link from Redirect to HTML type?',
 'Not directly. Delete the link and create a new one with the other type. Your subdomain becomes available again after deletion.',
 4),

('faq_edit_5', 'cat_editing', 'Where are traffic settings?',
 'Open any link and click <strong>Traffic Settings</strong> in the top right. That''s where you manage country filtering, device filtering, and IP blocking. See the <a href="#protection">Bot Protection</a> section for details.',
 5),

-- Bot Protection
('faq_bot_1', 'cat_protection', 'How does bot protection work?',
 'Every visitor passes through our antibot system before reaching your destination. We check browser fingerprints, behavior patterns, IP reputation, and dozens of other signals. Bots get blocked. Real people pass through without noticing anything.',
 1),

('faq_bot_2', 'cat_protection', 'Will real visitors get blocked?',
 'Rarely. The system is tuned to minimize false positives. If you''re testing your own links and getting blocked, add your IP to the whitelist at <a href="/user/settings#whitelist">Settings → Whitelisted IPs</a>.',
 2),

('faq_bot_3', 'cat_protection', 'How do I whitelist my IP?',
 'Go to <a href="/user/settings">Settings</a>, scroll to the Whitelisted IPs tab, and add your IP address. You can also click "Add my current IP" to auto-detect it. Whitelisted IPs bypass all bot checks.',
 3),

('faq_bot_4', 'cat_protection', 'How do I block a specific IP or country?',
 'Open your link, click <strong>Traffic Settings</strong>, and configure blocking rules. You can block individual IPs, entire countries, or specific device types. Blocked visitors see nothing. They don''t reach your destination.',
 4),

('faq_bot_5', 'cat_protection', 'What do the blocked visits in analytics mean?',
 'Those are bot attempts the system stopped. Seeing blocked visits means protection is working. They don''t count as real visitors and never reached your destination.',
 5),

-- Analytics
('faq_ana_1', 'cat_analytics', 'Where do I see analytics?',
 'Click <strong>Analytics</strong> on any link from the <a href="/user/redirectlinks">Redirect Links</a> list. You''ll see visits, unique visitors, countries, devices, browsers, and blocked attempts. Data updates in real-time.',
 1),

('faq_ana_2', 'cat_analytics', 'Why are my analytics showing zero?',
 'A few possibilities. The link might not be deployed yet (check for a green "Deployed" badge). DNS might not have propagated. Or nobody has visited yet. Try clicking your link yourself to generate a test visit.',
 2),

('faq_ana_3', 'cat_analytics', 'Can I block an IP from the analytics page?',
 'Yes. In the Recent Visits table, click Block next to any IP. That IP gets added to your blocklist and won''t reach your destination again.',
 3),

-- Account
('faq_acc_1', 'cat_account', 'How do I get a subscription?',
 'Contact admin on Telegram to activate your account.',
 1),

('faq_acc_2', 'cat_account', 'What happens when my subscription expires?',
 'You enter view-only mode. Your links stay active and keep working, but you can''t create new ones or make changes until you renew.',
 2),

('faq_acc_3', 'cat_account', 'How do I change my password?',
 'Go to <a href="/user/settings">Settings</a> and open the Security tab. Enter your current password, then your new one. You''ll be signed out everywhere after changing it.',
 3)

ON CONFLICT (id) DO NOTHING;
