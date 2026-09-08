-- Add new Bot Protection FAQs for challenge templates and advanced settings

INSERT INTO faq_items (id, category_id, question, answer, sort_order) VALUES

-- Challenge Templates
('faq_bot_6', 'cat_protection', 'What are Challenge Templates?',
 'Challenge templates determine what visitors see when they need to verify they''re human. Each template has a different look and feel:<br><br>
 <ul>
   <li><strong>Cloudflare Style:</strong> Familiar verification page that looks like Cloudflare''s browser check</li>
   <li><strong>Human Check:</strong> Simple checkbox verification with a loading animation</li>
   <li><strong>Human Security:</strong> Press and hold button challenge - the most interactive option</li>
   <li><strong>Slide Puzzle:</strong> GeeTest-style slide puzzle with random shapes - strongest bot protection</li>
 </ul>',
 6),

('faq_bot_7', 'cat_protection', 'Which challenge template should I choose?',
 '<ul>
   <li><strong>Cloudflare Style:</strong> Best for general use - visitors are familiar with it and it feels legitimate</li>
   <li><strong>Human Check:</strong> Best for simple, quick verification with minimal friction</li>
   <li><strong>Human Security:</strong> Best when you need stronger bot protection - the press-and-hold action is harder for bots to automate</li>
   <li><strong>Slide Puzzle:</strong> Best for maximum security - requires solving a visual puzzle that''s very hard for bots</li>
   <li><strong>Use Global Default:</strong> Uses your server''s default template - good if you want a consistent experience across all links</li>
 </ul>',
 7),

-- ASN Blocking
('faq_bot_8', 'cat_protection', 'What is ASN blocking?',
 'ASN (Autonomous System Number) identifies a network or ISP. Blocking by ASN lets you block entire networks instead of individual IPs.<br><br>
 <strong>Common uses:</strong>
 <ul>
   <li>Block cloud providers (AWS, DigitalOcean, Google Cloud) where bots often run</li>
   <li>Whitelist only residential ISPs (Comcast, Verizon) to allow real users</li>
   <li>Block specific hosting companies known for abuse</li>
 </ul>
 <small class="text-muted">Find ASN numbers at <a href="https://bgp.he.net" target="_blank">bgp.he.net</a></small>',
 8),

-- Detection Settings
('faq_bot_9', 'cat_protection', 'What do the detection toggles do?',
 '<ul>
   <li><strong>Known Bots:</strong> Blocks crawlers, scrapers, and automated tools identified by user-agent and behavior</li>
   <li><strong>Headless Browsers:</strong> Blocks Puppeteer, Selenium, Playwright - often used for scraping</li>
   <li><strong>Tor Exit Nodes:</strong> Blocks visitors coming through the Tor network</li>
   <li><strong>Proxy/VPN:</strong> Blocks visitors using VPN or proxy services to hide their identity</li>
   <li><strong>Datacenter IPs:</strong> Blocks traffic from cloud servers (AWS, GCP, Azure, hosting providers)</li>
 </ul>',
 9),

-- Behavior Score
('faq_bot_10', 'cat_protection', 'What is the Behavior Score Threshold?',
 'Behavior score measures how "human" a visitor''s actions appear (0-100). Higher scores indicate more human-like behavior.<br><br>
 <ul>
   <li><strong>0 (Disabled):</strong> All visitors pass regardless of behavior</li>
   <li><strong>1-30 (Light):</strong> Only blocks obvious bots with very low scores</li>
   <li><strong>31-60 (Moderate):</strong> Balanced filtering - blocks suspicious activity</li>
   <li><strong>61-100 (Strict):</strong> May block some legitimate users with unusual behavior</li>
 </ul>
 <small class="text-muted">Start with 0 or low values and increase only if you see bot activity.</small>',
 10),

-- Redirect on Block
('faq_bot_11', 'cat_protection', 'What is "Blocked Visitor Redirect"?',
 'When a visitor is blocked, they can be redirected to a URL of your choice instead of seeing an error page.<br><br>
 <strong>Common uses:</strong>
 <ul>
   <li>Redirect to Google or a generic page to avoid revealing you have protection</li>
   <li>Redirect to a "blocked" landing page explaining why access was denied</li>
   <li>Leave empty to show the default error page</li>
 </ul>',
 11)

ON CONFLICT (id) DO NOTHING;
