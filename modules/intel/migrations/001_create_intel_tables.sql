CREATE TABLE IF NOT EXISTS intel_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intel_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    slug VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intel_articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID REFERENCES intel_categories(id) ON DELETE SET NULL,
    slug VARCHAR(200) NOT NULL UNIQUE,
    title VARCHAR(200) NOT NULL,
    excerpt TEXT DEFAULT '',
    content TEXT DEFAULT '',
    cover_image VARCHAR(500) DEFAULT '',
    author_name VARCHAR(100) DEFAULT '',
    status VARCHAR(20) DEFAULT 'draft',
    featured BOOLEAN DEFAULT false,
    reading_time INT DEFAULT 1,
    meta_title VARCHAR(60) DEFAULT '',
    meta_description VARCHAR(160) DEFAULT '',
    published_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intel_article_tags (
    article_id UUID REFERENCES intel_articles(id) ON DELETE CASCADE,
    tag_id UUID REFERENCES intel_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_intel_articles_status ON intel_articles(status);
CREATE INDEX IF NOT EXISTS idx_intel_articles_slug ON intel_articles(slug);
CREATE INDEX IF NOT EXISTS idx_intel_articles_published ON intel_articles(published_at DESC) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_intel_articles_category ON intel_articles(category_id);
