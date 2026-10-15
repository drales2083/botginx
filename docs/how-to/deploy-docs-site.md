# Deploy Documentation Site

Deploy the Starlight documentation site to docs.guardbot.sbs.

## Prerequisites

- SSH access to the Guard VPS (configured in `deploy.env`)
- `DOCS_DOMAIN` set in `deploy.env` (e.g., `docs.guardbot.sbs`)
- Node.js installed locally for building

## Quick Deploy

From the repository root:

```bash
# Build the docs site
cd docs-site && npm run build && cd ..

# Deploy to server
source deploy.env
rsync -avz --delete docs-site/dist/ "${SSH_USER}@${SSH_HOST}:/var/www/docs/"
```

## Step-by-Step Process

### 1. Build the Documentation

```bash
cd docs-site
npm ci          # Install dependencies (first time or after package changes)
npm run build   # Build static site to dist/
```

Build output goes to `docs-site/dist/`. The build includes:
- Static HTML pages
- Pagefind search index
- Sitemap for SEO

### 2. Deploy to Server

```bash
cd /home/this/dev/code/botginx
source deploy.env
rsync -avz --delete docs-site/dist/ "${SSH_USER}@${SSH_HOST}:/var/www/docs/"
```

The `--delete` flag removes files on the server that no longer exist locally, keeping the deployment clean.

### 3. Verify Deployment

Visit https://docs.guardbot.sbs to confirm:
- Pages load correctly
- Search works (Pagefind)
- Navigation functions properly

## Server Configuration

The docs site is served by nginx on the Guard VPS. Configuration location:

```
/etc/nginx/sites-available/docs.guardbot.sbs
```

Standard static file serving with SSL via Certbot.

## Troubleshooting

### Build Fails

```bash
# Clear node_modules and rebuild
cd docs-site
rm -rf node_modules
npm ci
npm run build
```

### Permission Denied on Deploy

Ensure SSH key is loaded and `deploy.env` has correct credentials:

```bash
source deploy.env
ssh ${SSH_USER}@${SSH_HOST} "ls /var/www/docs"
```

### Changes Not Appearing

1. Clear browser cache
2. Check rsync output for errors
3. Verify files exist on server:
   ```bash
   source deploy.env
   ssh ${SSH_USER}@${SSH_HOST} "ls -la /var/www/docs"
   ```

## Documentation Structure

The docs site uses Starlight (Astro) with content in:

```
docs-site/
├── src/content/docs/    # Markdown documentation files
├── astro.config.mjs     # Sidebar and site configuration
└── dist/                # Built output (gitignored)
```

### Adding New Pages

1. Create `.mdx` file in `src/content/docs/`
2. Add to sidebar in `astro.config.mjs` if needed
3. Build and deploy

### Editing Existing Pages

1. Edit the `.mdx` file in `src/content/docs/`
2. Build and deploy

## For AI Agents

When deploying docs changes:

```bash
# Full deployment sequence
cd /home/this/dev/code/botginx
cd docs-site && npm run build && cd ..
source deploy.env
rsync -avz --delete docs-site/dist/ "${SSH_USER}@${SSH_HOST}:/var/www/docs/"
```

No tagging or git push required for docs-only changes. The docs site is deployed independently of the main application.
