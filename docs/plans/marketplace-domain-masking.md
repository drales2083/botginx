# Plan: Marketplace Domain Name Masking

## Overview

Hide part of domain names in the marketplace listing until after purchase. This prevents users from copying domain names and registering them elsewhere.

## Current State

Domains are shown in full:
```
benjaminrock.me.uk
kevindevale.org.uk
fantasytone.com
```

## Target State

Domains are partially masked with stars:
```
be⭑⭑nrock.me.uk
ke⭑⭑evale.org.uk
fa⭑⭑etone.com
```

## Masking Rules

| Domain Length | Mask Position | Example |
|---------------|---------------|---------|
| Short (≤6 chars) | Middle 2 chars | `do⭑⭑in.com` |
| Medium (7-10 chars) | Chars 3-6 | `be⭑⭑⭑⭑ock.me.uk` |
| Long (>10 chars) | Chars 3-6 | `fa⭑⭑⭑⭑tone.com` |

**Rules:**
- Always show first 2 characters
- Always show last part before TLD
- Mask 2-4 characters in the middle
- Never mask the TLD (.com, .me.uk, .org.uk)
- Use `⭑` character for masking (or `*` as fallback)

## Implementation Tasks

### Phase 1: Backend Masking Function

- [ ] Create `MaskDomain(domain string) string` function in `pkg/domainmask/` or `modules/marketplace/`
- [ ] Parse domain to separate name from TLD
- [ ] Apply masking rules based on length
- [ ] Return masked string

### Phase 2: API Changes

- [ ] Modify marketplace listing API to return masked domains for unpurchased items
- [ ] Store full domain in database, mask only on display
- [ ] After purchase, reveal full domain to buyer

### Phase 3: Frontend Updates

- [ ] Update `user_browse.html` to display masked domains
- [ ] Update purchase confirmation modal to show masked domain
- [ ] After purchase success, show full domain

### Phase 4: Admin View

- [ ] Admin should always see full domains (no masking)
- [ ] Admin list page unchanged

## Files to Modify

1. `pkg/domainmask/mask.go` (new) - Masking utility function
2. `modules/marketplace/handlers/handler.go` - Apply masking in user-facing endpoints
3. `modules/marketplace/templates/user_browse.html` - Display masked domains
4. `modules/marketplace/services/marketplace_service.go` - Add masking to listing queries

## Masking Algorithm

```go
func MaskDomain(fullDomain string) string {
    // Split domain and TLD
    // e.g., "benjaminrock.me.uk" -> name="benjaminrock", tld=".me.uk"
    
    // Apply mask to name portion
    // Keep first 2 chars, mask middle, keep last chars before TLD
    
    // Return: "be⭑⭑⭑⭑rock.me.uk"
}
```

## Example Transformations

| Full Domain | Masked |
|-------------|--------|
| `benjaminrock.me.uk` | `be⭑⭑⭑⭑rock.me.uk` |
| `kevindevale.org.uk` | `ke⭑⭑⭑⭑vale.org.uk` |
| `fantasytone.com` | `fa⭑⭑⭑⭑tone.com` |
| `short.io` | `sh⭑⭑t.io` |
| `a.com` | `⭑.com` (edge case - full mask) |

## Security Considerations

- Masking is display-only, full domain stays in database
- Admin APIs return full domain
- Purchase API returns full domain after successful payment
- Never expose full domain in HTML source or network requests before purchase

## Testing

- [ ] Test various domain lengths
- [ ] Test multi-part TLDs (.co.uk, .me.uk, .org.uk)
- [ ] Test single-part TLDs (.com, .io, .xyz)
- [ ] Verify admin sees full domains
- [ ] Verify buyer sees full domain after purchase
