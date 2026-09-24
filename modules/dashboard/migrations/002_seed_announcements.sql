-- Seed initial announcements (news feed)

INSERT INTO announcements (id, title, body, badge, is_pinned, published_at) VALUES
('ann-001', 'Cloudflare Turnstile Integration',
'<p>Cloudflare Turnstile is now available on your domains. No puzzles, no annoying checkboxes. It runs in the background and filters bots without bothering real visitors. Turn it on per-domain from settings.</p>',
'NEW', true, '2026-09-08 11:00:00+00')

ON CONFLICT (id) DO NOTHING;

INSERT INTO announcements (id, title, body, badge, is_pinned, published_at) VALUES
('ann-002', 'New Protection Templates',
'<p>Three new protection page designs:</p>
<ul>
<li><strong>Latch</strong>: puzzle verification, works in dark or light mode</li>
<li><strong>Foyer</strong>: waiting room style with a countdown</li>
<li><strong>Interstitial</strong>: the classic loading animation</li>
</ul>
<p>All of them run our full antibot engine.</p>',
NULL, false, '2026-08-12 10:00:00+00')

ON CONFLICT (id) DO NOTHING;

INSERT INTO announcements (id, title, body, badge, is_pinned, published_at) VALUES
('ann-003', 'Automatic Wildcard SSL',
'<p>Point your DNS to us. SSL certificates get issued on their own, including wildcards for all subdomains. You don''t have to verify anything or renew anything.</p>',
NULL, false, '2026-07-15 16:00:00+00')

ON CONFLICT (id) DO NOTHING;

INSERT INTO announcements (id, title, body, badge, is_pinned, published_at) VALUES
('ann-004', 'Advanced Path URLs',
'<p>Redirect links now look like real paths: <code>/invoice/view/abc123</code> instead of <code>/r/xyz</code>. Looks more legit, lands better in inboxes. Same antibot protection behind it.</p>',
NULL, false, '2026-06-01 15:00:00+00')

ON CONFLICT (id) DO NOTHING;

INSERT INTO announcements (id, title, body, badge, is_pinned, published_at) VALUES
('ann-005', 'In-Panel Support Tickets',
'<p>Need help? Open a ticket right from the panel. Attach screenshots if you want. We get notified instantly and you''ll see when we reply.</p>',
NULL, false, '2026-05-10 11:00:00+00')

ON CONFLICT (id) DO NOTHING;

INSERT INTO announcements (id, title, body, badge, is_pinned, published_at) VALUES
('ann-006', 'Per-Domain Bot Settings',
'<p>You can now control antibot settings per domain. Block countries, change how aggressive the detection is, pick your challenge type. All in one place.</p>',
NULL, false, '2026-04-18 14:00:00+00')

ON CONFLICT (id) DO NOTHING;
