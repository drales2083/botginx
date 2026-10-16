# Research: Short Links and Redirect Links for Marketing

Prepared for: GuardBot Intel blog post (audience: digital marketers, affiliate marketers, business owners)
Date: 2026-10-15

**How to use this file:** Statistics below are labelled by source quality. Almost every click-fraud and branded-link figure comes from a vendor that sells a related product, so none is independently audited. Quote them with attribution ("according to Lunio…"), not as settled fact. Items marked **[unverified]** could not be traced to a primary source in this research pass; verify before publishing or leave them out.

---

## 1. What short links and redirect links are, and how they work

**Definition.** A short link is a compact URL (`go.brand.com/spring`) that, when requested, sends the visitor to a longer destination URL. A redirect link is the same mechanism, not necessarily short. Many marketers use "redirect link" for links that route traffic through an intermediate server for tracking, rotation or filtering.

**Mechanics (HTTP level)**
1. Visitor clicks `https://go.brand.com/spring`.
2. The link server looks up the slug and finds the destination.
3. The server logs the click: timestamp, IP, user agent, referrer, country, device.
4. The server answers with a 3xx status and a `Location:` header.
5. The browser follows it to the destination, usually in milliseconds.

**Redirect types**
| Code | Meaning | Typical use |
|------|---------|-------------|
| 301 | Moved permanently | Permanent moves; browsers and CDNs may cache it, so later clicks can skip your server and go uncounted |
| 302 / 307 | Temporary | Campaign links, A/B tests, rotators. Safer for analytics because the browser re-asks each time |
| Meta refresh / JS redirect | Page-level, not HTTP | Interstitial or bot-check pages. Slower, and weaker for SEO |

**Note on 301 vs 302 and SEO.** SEO blogs claim 301s pass 90–99% of link equity and that 302s pass less. The sources found did not trace these figures to Google. One secondary report quotes Google's John Mueller as saying the choice doesn't matter for ranking and should simply match whether the move is permanent or temporary **[unverified, check Mueller's original statement or Google Search Central docs]**. For campaign links, the more practical reason to prefer 302 is that it keeps every click hitting your tracker.

**Variants**
- *Generic shared-domain links* (bit.ly, tinyurl): quick, but the domain's reputation isn't yours.
- *Branded / custom-domain links*: your own domain or subdomain.
- *Redirect-link generators with per-link rules* (GuardBot's model): each link has settings such as geo filters, device filters, bot blocking and rotation.
- *Deep links*: send mobile users to an app screen, or fall back to a web URL.
- *Link rotators / split links*: weight traffic across several destinations.

---

## 2. Why marketers use them

1. **Tracking and attribution.** One click is one log row, so you get clicks by source, channel, country and device without depending on the destination site's analytics. This matters most when you don't control the landing page (affiliate offers, partner pages).
2. **Offline and closed-channel attribution.** Print, QR codes, podcasts, SMS and DMs can't carry long UTM strings. A short link is the only practical way to measure them.
3. **Branding and trust.** A recognisable domain tells people where the link goes. Vendor claims for the lift:
   - Bitly and Rebrandly both cite about **34%** higher click-through for branded vs generic short links, and Rebrandly claims up to **39%** from an experiment plus user data.
   - **Caveat:** these are vendor marketing figures with no published baseline, sample size or method, and the same number is credited to different companies. A 2026 u2l.ai guide argues the lift comes from a full branded domain, not just a custom slug on bit.ly. Frame as "vendors report" or run your own A/B test.
4. **Flexibility after publishing.** You can change the destination without reprinting, reposting or re-sending. Fix a broken offer, swap a seasonal page, or retire a campaign to a fallback.
5. **Platform constraints.** Character limits, clean-looking bios, and avoiding ugly query strings.
6. **Control over the traffic.** Redirect layers can filter by country or device, rotate offers, cap clicks, and block bad traffic before it reaches an expensive landing page or inflates a ad-network bill.
7. **Insulation from tracker/URL changes.** Affiliates can swap networks or offer URLs behind one stable link.

---

## 3. Common problems

### 3a. Bot traffic and fake clicks
Short-link click counts are a plain log of requests, so anything that requests the URL gets counted. Sources of non-human clicks:

- **Email and chat security scanners.** Microsoft 365 Safe Links, Proofpoint URL Defense, Mimecast and similar tools fetch links in delivered messages, often seconds after send and sometimes before a human opens the email. They inflate click counts and can even trigger unsubscribe links. Practitioner write-ups say to spot them by rewritten hostnames (`safelinks…`, `urldefense.com`, `protect.mimecast.com`), burst timing right after a send, and Microsoft/Azure cloud IP ranges. *(Sources are ESP/analytics vendors and practitioner Q&A, not Microsoft or Proofpoint documentation.)*
- **Link preview crawlers.** Slack, WhatsApp, Telegram, Facebook, X and iMessage fetch a URL to build the preview card. One share can produce a "click" with no person involved.
- **Search and SEO crawlers, uptime monitors, scrapers.**
- **Datacenter, VPN, proxy and Tor traffic**, including headless browsers.
- **Malicious click bots**: competitor click-spamming, fake-lead generation, cookie stuffing in affiliate programs, and bots that run up pay-per-click bills.

Consequences: wrong conversion rates, bad A/B test winners, inflated pay-per-click charges, polluted retargeting audiences, and fake "engaged" segments in email lists. A newsletter piece found in the search notes that targeting "all clickers" is a trap for this reason.

### 3b. Click fraud
Covered with numbers in section 5. In short: paid clicks fire your redirect, you pay for the click, and a share of those clicks are not people.

### 3c. Link rot
- Pew Research (May 2024, "When Online Content Disappears"): **38%** of webpages that existed in 2013 were no longer reachable in 2023. 23% of news pages and 21% of government pages contain at least one broken link. 54% of Wikipedia pages have at least one dead link in their references. Nearly one in five tweets are no longer visible within months.
- An Old Dominion University study of 27.3M Wayback Machine URLs found about **65%** dead when checked in 2023.
- **Gap:** none of the sources measured short links specifically. Don't claim a shortener-specific rot rate. What can be said safely: a short link is only as alive as (a) the shortener service and (b) the destination. If a third-party shortener shuts down, changes pricing, or deactivates your account, every printed or published link dies. A link you print on 10,000 flyers on someone else's domain is a liability.

### 3d. Reputation and deliverability problems
- Free shared shorteners are heavily used by spammers and phishers, so some mail filters and platforms treat them with suspicion. *(General industry knowledge; no source found in this pass. **[unverified]** for specific stats.)*
- A shared domain's blocklisting affects you. A custom domain's reputation is yours to protect, but also yours to lose.
- Cloaked or misleading redirects can break ad-platform and Safe Browsing policies. Keep destinations honest.

### 3e. Other
- Redirect chains (shortener → tracker → network → landing page) add latency and each hop is a point of failure. Keep chains to one or two hops.
- Cached 301s skip your tracker, so you under-count.
- Privacy: log IPs carefully. GuardBot's own tracking code hashes IPs with an HMAC salt (see `tracking_pixel_ip_salt` in CLAUDE.md), which is a good pattern to mention.

---

## 4. How bot protection integrates with redirect links

### Generic model
Bot protection sits in front of or inside the redirect step. For each request it collects signals, scores them, and picks an action:

| Signal | What it tells you |
|--------|-------------------|
| IP reputation / ASN | Datacenter, hosting, known proxy, Tor exit node |
| User agent | Known crawlers, empty or malformed UAs |
| Headless-browser detection | Puppeteer/Playwright/Selenium fingerprints |
| Geo | Traffic from countries the campaign doesn't target |
| Velocity | Hundreds of clicks in seconds from one IP block |
| Timing | Bursts immediately after an email send |
| Referrer / cookie / JS behaviour | Missing JS execution, no cookies |
| Composite bot score | Weighted combination, 0–100 |

Actions: allow, block, serve a fallback or decoy destination, show a challenge, or **allow but flag** so analytics keep humans and bots separate.

**Why the per-link part matters.** Different links need different strictness. A paid-ads landing link should block datacenter traffic aggressively. A public social-bio link needs to let link-preview crawlers through so the preview card renders. An email link needs to tolerate scanners without counting them. Per-link settings let one account do all three.

### How GuardBot / Botginx does it (from `docs/BOTECTION-CALLBACK-API.md`)
- Botginx is the panel where users create redirect links and configure per-link protection settings.
- **Botection** is the antibot reverse proxy in front of it. Traffic path: Internet → nginx :443 → botection :8080 → botginx :3001 (when on the same VPS).
- For each request, botection extracts the host, collects signals (IP, country, `is_tor`, `is_proxy`, `is_datacenter`, `is_headless`, user agent, bot `score` 0–100), then calls `POST /api/botection/should-block` on botginx.
- Botginx resolves host → link, loads that link's settings from the database, applies the blocking rules and returns a decision (`{"block": false}` or a block response). Botection applies it and fires an async webhook for logging.
- Settings changes take effect within about 30 seconds (cache TTL).
- Analytics then record visits with country, device and bot detection, so the dashboard can separate humans from bots.

Suggested blog framing: "Your tracker should know the difference between a customer and a crawler *before* it counts the click, not after you've paid for it."

*Check with the team before publishing implementation details (internal ports, endpoint path). The architecture can be described at a high level without naming them.*

---

## 5. Statistics on click fraud and bot traffic

Treat all as vendor-sourced. Definitions of "invalid" differ, so numbers are not comparable.

### Bot share of overall web traffic
- **Imperva 2025 Bad Bot Report** (2024 data): automated traffic was **51%** of web traffic, the first time bots exceeded humans in the report's history. **Bad bots 37%** (up from 32% in 2023), benign bots 14%. Based on 13 trillion blocked bad-bot requests. Imperva attributes the growth to AI/LLMs making bots cheap to build.
  - Caveat: one analysis notes the methodology counts API calls, uptime monitors and search crawlers, which may overstate the threat. Single vendor with commercial interest.
  - Source note: one outlet describes the rise as 5 points, another as from 32% to 37%; both are consistent (5 points). Cite 32% → 37%.

### Invalid traffic in paid advertising
- **Lunio, 2026 Global Invalid Traffic Report:** analysed **2.7 billion+** paid clicks (Google, Meta, TikTok, LinkedIn, Bing; Aug 2024–Aug 2025). **8.51%** invalid. Estimated cost **$63 billion** a year in global ad spend. Reported by Advanced Television (Jan 2026) and Mediacat. The most transparent methodology of those found.
- **Pixalate Q2 2025** (via ProtectAds): global invalid-click rate on web traffic **19%**.
- **Anura:** **25–28%** invalid rates across major channels in 2025. Highest of the set, treat with caution.
- **TrafficGuard** (via Webtonic): roughly **$250B** losses, **22%** of global digital ad spend, **20%** of digital traffic non-human. Aggressive figures.
- **Fraud Blocker:** $84B lost in 2023, projected **$172B by 2028**.
- **Webtonic roundup:** five independent 2026 measurements range **5.6% to 20.6%** of paid clicks or impressions. A defensible way to say it: "estimates range from roughly 6% to over 20%."
- Paid search: **14–22%** of clicks (Webtonic). Some verticals see bot share above 50%.

### Affiliate marketing
- Bot share of affiliate traffic about **24%** (one fraud-detection vendor); invalid traffic in large ad-impression datasets **18–21%**.
- **17%** of 2022 affiliate traffic was fraudulent clicks, costing about **$3.4B** (vendor estimate).
- About **25%** of affiliate leads may be fake or poor quality (vendor estimate).
- **Cookie stuffing** touches an estimated **5–10%** of affiliate transactions (vendor estimate).
- Patterns: high-velocity clicks from one IP block, unexpected geographies, post-conversion abuse (refunds, churn, chargebacks). Judge payouts on net quality.

### Link rot (context)
- Pew 2024: 38% of 2013 webpages gone by 2023. ODU: ~65% of 27.3M sampled historical URLs dead.

### Recommended way to cite
1. Lead with one well-sourced number (Imperva 51%/37%, Lunio 8.51%/$63B).
2. Give the range and say it varies by definition and vendor.
3. Name the source in the sentence. Link to the primary report where one exists.
4. Leave out the 2028 projections and the $250B figure, or label them clearly as vendor projections.

---

## 6. Best practices for redirect links in campaigns

**Setup**
1. **Use your own domain or subdomain** for anything public or printed. You own the reputation and can't be cut off by a third party. Keep a separate domain for risky or high-volume traffic so one burned domain doesn't take everything down.
2. **Use 302/307 for tracked campaign links** so each click reaches your logger.
3. **Keep chains short**: one hop to the destination where possible.
4. **Use readable slugs** (`/spring-sale`, not `/x7Qp2`) for anything humans see. Use random slugs for anything you don't want guessed.
5. **Add UTM parameters** (or your own) on the destination so analytics on the landing page agree with the link logs.
6. **Make links HTTPS only** and configure HSTS on the redirect domain.

**Tracking hygiene**
7. **Separate human and bot clicks in reporting.** Report "unique human clicks" as the headline number, with raw clicks as secondary.
8. **Expect scanner clicks on email and chat links.** Flag bursts right after send, and known security-vendor hostnames and IP ranges. Never let a single GET on an unsubscribe or confirm link perform the action; require a POST or a confirm page.
9. **Let preview crawlers through** (or serve them a lightweight page) on social links so cards render, but don't count them as clicks.
10. **Compare sources.** If link clicks are much higher than landing-page sessions, that gap is your bot/bounce rate. Investigate the discrepancy rather than average it away.

**Bot protection**
11. **Set per-link rules by channel:**
    - Paid ads: block datacenter, proxy, headless, Tor; consider geo-limiting to the targeted countries.
    - Email: tolerate scanners but don't count them.
    - Social: allow preview bots, block scrapers.
    - Affiliate: block non-target geos, cap velocity per IP.
12. **Use a fallback destination** for blocked traffic (a neutral page) instead of a hard error, so legitimate users with odd networks aren't lost entirely.
13. **Whitelist your own team and partners** so internal testing doesn't pollute or get blocked.
14. **Review blocked traffic weekly** to catch false positives, especially corporate VPN users.
15. **Claim refunds**: with ad-network invalid-click evidence (IP, timestamp, UA logs from your redirect layer), you have material to dispute charges.

**Lifecycle**
16. **Audit links quarterly** for dead destinations (HTTP checks), and redirect retired campaigns to a live page instead of a 404.
17. **Export your link data** regularly and know how you would migrate if your provider changed terms.
18. **Don't cloak deceptively.** Showing one page to ad reviewers and another to users violates ad-platform policy and risks account bans and Safe Browsing flags. Bot filtering is legitimate; deception is not. Draw this line clearly in the post.
19. **Privacy**: log what you need, hash or truncate IPs, set a retention period, and disclose tracking where required (GDPR/ePrivacy and similar).

**A/B testing your links**
20. Test branded vs generic on the same destination with the same audience, rather than relying on vendor claims of 34%+.

---

## 7. Suggested blog angles and hooks

- "Half the internet isn't human: what that does to your link analytics" (Imperva 51%/37%).
- "The click that wasn't: why your email click rate is lying to you" (security scanners).
- "Short links you don't own are a liability" (link rot, vendor lock-in).
- "How much of your ad budget is bots? Estimates range from 6% to 25%" (honest-range framing, Lunio 8.51%).
- Checklist post: "Redirect link setup for paid campaigns in 10 steps" (from section 6).
- Product tie-in: per-link bot protection enforced at the edge, settings live in about 30 seconds, humans vs bots separated in analytics.

**Possible structure:** hook stat → what redirect links are → why marketers use them → where it goes wrong (bots, fraud, rot) → how bot protection works with redirects → checklist → CTA.

---

## 8. Sources

- Lunio 2026 Global Invalid Traffic Report, via [Advanced Television](https://www.advanced-television.com/2026/01/21/report-63bn-lost-to-ivt-across-digital-advertising-in-2025/) and [Mediacat](https://mediacat.uk/?p=43936)
- [Webtonic: Click Fraud Benchmarks 2026](https://www.webtonic.io/blog/click-fraud-statistics) and [Ad Fraud in 2026 by the Numbers](https://www.webtonic.io/blog/ad-fraud-statistics)
- [Fraud Blocker click fraud statistics](https://fraudblocker.com/data/click-fraud-statistics)
- [Anura](https://www.anura.io/fraud-tidbits/annual-click-fraud-cost)
- [ProtectAds](https://protectads.net/click-fraud-calculator) (citing Pixalate Q2 2025)
- [TrafficGuard IVT stats](https://www.trafficguard.ai/ivt)
- [AffNinja affiliate fraud statistics 2026](https://affninja.com/affiliate-fraud-statistics/)
- [Influencer Marketing Hub: fraud prevention in affiliate programs](https://influencermarketinghub.com/fraud-prevention-in-affiliate-programs/)
- Imperva 2025 Bad Bot Report, via [Intelligent CIO](https://www.intelligentcio.com/me/2025/04/20/ai-fuels-hard-to-detect-bots-that-generate-half-of-internet-traffic-says-imperva/) and [Lenz.io](https://lenz.io/q/imperva-bad-bot-report-bot-traffic-findings)
- [Pew Research: When Online Content Disappears (2024)](https://www.pewresearch.org/data-labs/2024/05/17/when-online-content-disappears/), plus [Internet Archive blog analysis](https://blog.archive.org/tag/pew/)
- Email scanner clicks: [Suped on Office 365 auto clicks](https://www.suped.com/email-deliverability/answers/why-are-we-seeing-automatic-opens-and-clicks-on-office-365-hosted-recipient-domains), [Suped on Proofpoint](https://www.suped.com/knowledge/email-deliverability/technical/understanding-and-reducing-high-proofpoint-click-events-for-email-marketers), [Maestra](https://help.maestra.io/campaigns/filtering-out-fake-clicks-keeping-your-click-rate-clean)
- Branded link CTR: [60secondmarketer (2017)](https://60secondmarketer.com/2017/05/04/branded-short-links-and-new-domains-gaining-traction/amp/), [u2l.ai](https://u2l.ai/blog/what-is-branded-link), [Bitly](https://bitly.com/blog/branded-vs-generic-link/)
- 301 vs 302: [Dub.co](https://dub.co/blog/301-vs-302-redirect) (secondary; Google Search Central not consulted)
- Internal: `docs/BOTECTION-CALLBACK-API.md`

## 9. Open items to verify before publishing

- Primary Imperva report (confirm 51% / 37% / 32%).
- Primary Lunio report (confirm 8.51% and $63B).
- Google's own guidance on 301 vs 302 and PageRank.
- Any shortener-specific link-rot or shutdown data (none found). Possible example: goo.gl shutdown, but confirm facts and dates before using it.
- Any reputable source on spam/phishing abuse of free shorteners and its effect on deliverability.
- Whether marketing is comfortable naming internal architecture (botection/botginx ports and endpoint).
