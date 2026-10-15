# Intel (Blog) Module Design Spec

**Date:** 2026-10-15  
**Status:** Approved  
**Goal:** Add Intel content hub to drive SEO traffic, generate leads, and communicate product updates

---

## Overview

Intel is a blog/content section for GuardBot that serves three purposes:
1. **SEO traffic** — Target keywords like "bot protection", "link fraud prevention"
2. **Lead generation** — Educate prospects, drive signups
3. **Product updates** — Announce features, build trust

Content is created by admins via a full-featured markdown editor with image upload support.

---

## Database Schema

### intel_categories
| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| name | VARCHAR(100) | Display name (e.g., "Security") |
| slug | VARCHAR(100) | URL slug, unique |
| description | TEXT | Category description |
| sort_order | INT | Display order |
| created_at | TIMESTAMP | |

### intel_tags
| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| name | VARCHAR(50) | Display name (e.g., "Bot Detection") |
| slug | VARCHAR(50) | URL slug, unique |
| created_at | TIMESTAMP | |

### intel_articles
| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| category_id | UUID | FK to intel_categories |
| slug | VARCHAR(200) | URL slug, unique |
| title | VARCHAR(200) | Article title |
| excerpt | TEXT | Short preview for cards/SEO |
| content | TEXT | Markdown content |
| cover_image | VARCHAR(500) | Path to cover image |
| author_name | VARCHAR(100) | Display name |
| status | VARCHAR(20) | draft, published, archived |
| featured | BOOLEAN | Hand-picked for prominence |
| reading_time | INT | Minutes (auto-calculated) |
| meta_title | VARCHAR(60) | SEO title override |
| meta_description | VARCHAR(160) | SEO description override |
| published_at | TIMESTAMP | |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### intel_article_tags
| Column | Type | Description |
|--------|------|-------------|
| article_id | UUID | FK to intel_articles |
| tag_id | UUID | FK to intel_tags |
| PRIMARY KEY | (article_id, tag_id) | |

---

## URL Structure

### Public Routes
| URL | Purpose |
|-----|---------|
| `/intel` | Main listing page (paginated) |
| `/intel/{slug}` | Single article |
| `/intel/category/{slug}` | Category archive |
| `/intel/tag/{slug}` | Tag archive |
| `/intel/sitemap.xml` | Articles sitemap |
| `/intel/rss` | RSS feed |

### Admin Routes
| URL | Purpose |
|-----|---------|
| `/admin/intel` | Article list with filters |
| `/admin/intel/new` | Create article |
| `/admin/intel/{id}/edit` | Edit article |
| `/admin/intel/categories` | Manage categories |
| `/admin/intel/tags` | Manage tags |

### API Routes
| URL | Purpose |
|-----|---------|
| `POST /admin/intel/api/upload` | Image upload |
| `POST /admin/intel/api/preview` | Markdown preview |
| `GET /admin/intel/api/articles` | DataTable source |

---

## Homepage Integration

### Header
Add "Intel" link before "Get Started" button.

### Footer
Add "Intel" to links section: Privacy | Terms | Cookies | Intel

### Intel Preview Section
Located before footer, shows 3 most recent published articles:
- Card layout: cover image, category badge, title, excerpt, reading time
- Featured articles shown first if flagged
- "View all Intel →" link to `/intel`
- Section hidden if no published articles exist

---

## SEO Implementation

### Meta Tags (per article)
```html
<title>{meta_title || title} | Intel | {appName}</title>
<meta name="description" content="{meta_description || excerpt}">
<link rel="canonical" href="https://guardbot.sbs/intel/{slug}">
```

### Open Graph / Twitter Cards
```html
<meta property="og:type" content="article">
<meta property="og:title" content="{title}">
<meta property="og:description" content="{excerpt}">
<meta property="og:image" content="{cover_image}">
<meta property="og:url" content="https://guardbot.sbs/intel/{slug}">
<meta property="article:published_time" content="{published_at}">
<meta property="article:section" content="{category.name}">
<meta name="twitter:card" content="summary_large_image">
```

### JSON-LD Article Schema
```json
{
  "@context": "https://schema.org",
  "@type": "Article",
  "headline": "{title}",
  "description": "{excerpt}",
  "image": "{cover_image}",
  "author": {"@type": "Person", "name": "{author_name}"},
  "publisher": {"@type": "Organization", "name": "{appName}"},
  "datePublished": "{published_at}",
  "dateModified": "{updated_at}",
  "mainEntityOfPage": "{canonical_url}"
}
```

### Additional SEO Features
- **Sitemap**: Auto-generated XML sitemap of all published articles
- **Table of Contents**: Auto-generated from H2/H3 headings
- **Related Articles**: 3 articles from same category at article bottom
- **Social Share**: Twitter, LinkedIn, Copy Link buttons
- **Reading Time**: Calculated at ~200 words/minute

---

## Markdown Editor

### Library
EasyMDE 2.18.0 (CDN)

### Features
- Side-by-side live preview
- Toolbar: Bold, Italic, Heading, Quote, Code, Link, Image, Lists
- Image upload via drag-drop or button
- Code syntax highlighting (highlight.js)
- Autosave to localStorage
- Keyboard shortcuts

### Image Upload Flow
1. User drags image or clicks image button
2. JS uploads to `POST /admin/intel/api/upload`
3. Server saves to `/static/uploads/intel/YYYY/MM/{uuid}.{ext}`
4. Server returns `{"url": "/static/uploads/intel/..."}`
5. Editor inserts `![](url)` into markdown

### Embed Support
- YouTube URLs auto-converted to embedded players
- Twitter/X URLs auto-converted to embedded tweets

---

## Admin UI

### Article List
- DataTable with search, filters (status, category)
- Columns: Title (with featured star), Category, Status, Date, Actions
- Tabs: All, Published, Drafts, Archived

### Article Editor
- Title and slug fields (slug auto-generated, editable)
- EasyMDE editor with side-by-side preview
- Sidebar: Category dropdown, Tags input, Cover image upload
- Author name, Status dropdown, Featured checkbox
- Collapsible SEO settings: Meta title (60 char), Meta description (160 char)
- Save Draft / Publish buttons

### Categories Manager
- CRUD table: Name, Slug, Description, Sort Order
- Drag to reorder

### Tags Manager
- List with usage count per tag
- Delete unused tags

---

## Public Pages

### Article Listing (`/intel`)
- Page header: "Intel" title, subtitle, category tabs, search
- 3×3 grid of article cards (9 per page)
- Pagination: Prev/Next with page numbers
- Cards: Cover image, category badge, title, excerpt, reading time

### Single Article (`/intel/{slug}`)
- Category badge, reading time
- Title, author, date
- Cover image
- Table of contents (auto-generated from headings)
- Article content (rendered markdown)
- Tags list
- Social share buttons
- Related articles (3 from same category)

### Category/Tag Archives
- Same layout as main listing, filtered by category or tag
- Breadcrumb: Intel > Category Name

---

## Module Structure

```
modules/intel/
├── module.go
├── handlers/
│   ├── public.go
│   └── admin.go
├── services/
│   ├── article_service.go
│   ├── category_service.go
│   ├── tag_service.go
│   └── markdown_service.go
├── models/
│   └── models.go
├── migrations/
│   └── 001_create_intel_tables.sql
└── templates/
    ├── public/
    │   ├── list.html
    │   ├── show.html
    │   ├── category.html
    │   └── tag.html
    └── admin/
        ├── list.html
        ├── edit.html
        ├── categories.html
        └── tags.html
```

### Additional Files
- `web/templates/pages/home.html` — Add Intel preview section
- `web/templates/partials/intel_card.html` — Reusable card component
- `static/css/intel.css` — Intel-specific styles
- `static/uploads/intel/` — Image uploads (gitignored)

### Dependencies (CDN)
- EasyMDE 2.18.0
- Highlight.js 11.9.0
- DOMPurify 3.0.6

---

## Access Control

- **Public**: Anyone can read published articles
- **Admin**: Only admin users can create/edit/delete articles and manage categories/tags

---

## Calculations

- **Reading Time**: `ceil(word_count / 200)` minutes
- **Slug**: Auto-generated from title using kebab-case, editable
- **Pagination**: 9 articles per page

---

## Out of Scope

- Comments/discussions on articles
- Newsletter subscription (can add later)
- Multi-author with author profiles
- Article scheduling (publish immediately or save as draft)
- Analytics per article (can use existing analytics)
