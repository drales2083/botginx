# Kumo UI Theme Plan

Transform the panel's look to match Kumo UI aesthetics while keeping AdminLTE structure intact.

## Goal

Create a polished, modern UI with glassmorphism, soft shadows, and smooth animations — toggleable via a single CSS include.

## Design Principles

- **Glassmorphism**: Frosted glass cards with backdrop blur
- **Soft Shadows**: Large blur radius, subtle opacity
- **Rounded Corners**: 12-24px radius on all elements
- **Gradient Accents**: Subtle gradients on sidebar and buttons
- **Micro-animations**: Smooth 200-300ms transitions
- **Clean Typography**: Inter or Plus Jakarta Sans font

## Architecture

```
web/static/css/
├── adminlte.css          # Keep unchanged (AdminLTE base)
├── adminlte-colors.css   # Keep unchanged (color tokens)
└── kumo-theme.css        # NEW: Override layer (~600 lines)
```

### Toggle Mechanism

In `base.html` and `auth.html`:
```html
<!-- Kumo Theme (comment out to disable) -->
<link rel="stylesheet" href="/static/css/kumo-theme.css" />
```

To disable: comment out the line. Everything reverts to stock AdminLTE.

## Implementation Phases

### Phase 1: Foundation (Day 1 Morning)

**1.1 CSS Variables**
```css
:root {
  /* Glass */
  --kumo-glass-bg: rgba(255, 255, 255, 0.03);
  --kumo-glass-border: rgba(255, 255, 255, 0.08);
  --kumo-glass-blur: 12px;
  
  /* Shadows */
  --kumo-shadow-sm: 0 2px 8px rgba(0, 0, 0, 0.15);
  --kumo-shadow-md: 0 8px 24px rgba(0, 0, 0, 0.2);
  --kumo-shadow-lg: 0 16px 48px rgba(0, 0, 0, 0.25);
  
  /* Radius */
  --kumo-radius-sm: 8px;
  --kumo-radius-md: 12px;
  --kumo-radius-lg: 16px;
  --kumo-radius-xl: 24px;
  
  /* Transitions */
  --kumo-transition-fast: 150ms ease;
  --kumo-transition-normal: 250ms ease;
  --kumo-transition-slow: 400ms ease;
  
  /* Colors (softer red) */
  --kumo-primary: #e63946;
  --kumo-primary-soft: rgba(230, 57, 70, 0.15);
  --kumo-accent-gradient: linear-gradient(135deg, #e63946 0%, #f4a261 100%);
}
```

**1.2 Typography**
```css
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
  -webkit-font-smoothing: antialiased;
}
```

### Phase 2: Cards & Containers (Day 1 Afternoon)

**2.1 Cards**
```css
.card {
  background: var(--kumo-glass-bg);
  backdrop-filter: blur(var(--kumo-glass-blur));
  -webkit-backdrop-filter: blur(var(--kumo-glass-blur));
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-lg);
  box-shadow: var(--kumo-shadow-md);
  transition: transform var(--kumo-transition-normal),
              box-shadow var(--kumo-transition-normal);
}

.card:hover {
  transform: translateY(-2px);
  box-shadow: var(--kumo-shadow-lg);
}

.card-header {
  background: transparent;
  border-bottom: 1px solid var(--kumo-glass-border);
  padding: 1.25rem 1.5rem;
}

.card-body {
  padding: 1.5rem;
}
```

**2.2 Small Cards / Info Boxes**
```css
.small-box, .info-box {
  background: var(--kumo-glass-bg);
  backdrop-filter: blur(var(--kumo-glass-blur));
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-lg);
  overflow: hidden;
}

.small-box .icon {
  opacity: 0.15;
}
```

### Phase 3: Sidebar (Day 2 Morning)

**3.1 Sidebar Container**
```css
.app-sidebar {
  background: linear-gradient(
    180deg,
    rgba(25, 25, 35, 0.95) 0%,
    rgba(18, 18, 28, 0.98) 100%
  );
  backdrop-filter: blur(20px);
  border-right: 1px solid var(--kumo-glass-border);
}

.sidebar-brand {
  padding: 1.25rem 1rem;
  border-bottom: 1px solid var(--kumo-glass-border);
}
```

**3.2 Navigation Items**
```css
.nav-sidebar .nav-link {
  border-radius: var(--kumo-radius-md);
  margin: 2px 0.75rem;
  padding: 0.75rem 1rem;
  transition: all var(--kumo-transition-fast);
}

.nav-sidebar .nav-link:hover {
  background: var(--kumo-primary-soft);
}

.nav-sidebar .nav-link.active {
  background: var(--kumo-accent-gradient);
  box-shadow: 0 4px 12px rgba(230, 57, 70, 0.3);
}

.nav-icon {
  opacity: 0.7;
  transition: opacity var(--kumo-transition-fast);
}

.nav-link:hover .nav-icon,
.nav-link.active .nav-icon {
  opacity: 1;
}
```

### Phase 4: Buttons & Forms (Day 2 Afternoon)

**4.1 Buttons**
```css
.btn {
  border-radius: var(--kumo-radius-md);
  padding: 0.6rem 1.25rem;
  font-weight: 500;
  transition: all var(--kumo-transition-fast);
}

.btn-primary {
  background: var(--kumo-accent-gradient);
  border: none;
  box-shadow: 0 4px 12px rgba(230, 57, 70, 0.25);
}

.btn-primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(230, 57, 70, 0.35);
}

.btn-outline-secondary {
  border: 1px solid var(--kumo-glass-border);
  background: var(--kumo-glass-bg);
}

.btn-outline-secondary:hover {
  background: rgba(255, 255, 255, 0.08);
}
```

**4.2 Form Inputs**
```css
.form-control, .form-select {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-md);
  padding: 0.75rem 1rem;
  transition: all var(--kumo-transition-fast);
}

.form-control:focus, .form-select:focus {
  background: rgba(0, 0, 0, 0.25);
  border-color: var(--kumo-primary);
  box-shadow: 0 0 0 3px var(--kumo-primary-soft);
}

.form-label {
  font-weight: 500;
  margin-bottom: 0.5rem;
  opacity: 0.85;
}
```

### Phase 5: Tables (Day 3 Morning)

**5.1 Table Styling**
```css
.table {
  --bs-table-bg: transparent;
}

.table thead th {
  background: rgba(0, 0, 0, 0.15);
  border-bottom: 1px solid var(--kumo-glass-border);
  font-weight: 600;
  text-transform: uppercase;
  font-size: 0.75rem;
  letter-spacing: 0.05em;
  opacity: 0.7;
}

.table tbody tr {
  transition: background var(--kumo-transition-fast);
}

.table tbody tr:hover {
  background: var(--kumo-primary-soft);
}

.table td {
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  padding: 1rem 0.75rem;
  vertical-align: middle;
}
```

**5.2 DataTables Override**
```css
.dataTables_wrapper .dataTables_filter input {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-md);
}

.dataTables_wrapper .dataTables_paginate .paginate_button {
  border-radius: var(--kumo-radius-sm);
}
```

### Phase 6: Modals & Dropdowns (Day 3 Morning)

**6.1 Modals**
```css
.modal-content {
  background: rgba(30, 30, 40, 0.95);
  backdrop-filter: blur(20px);
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-xl);
  box-shadow: var(--kumo-shadow-lg);
}

.modal-header {
  border-bottom: 1px solid var(--kumo-glass-border);
  padding: 1.5rem;
}

.modal-footer {
  border-top: 1px solid var(--kumo-glass-border);
  padding: 1.25rem 1.5rem;
}
```

**6.2 Dropdowns**
```css
.dropdown-menu {
  background: rgba(30, 30, 40, 0.95);
  backdrop-filter: blur(16px);
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-md);
  box-shadow: var(--kumo-shadow-md);
  padding: 0.5rem;
}

.dropdown-item {
  border-radius: var(--kumo-radius-sm);
  padding: 0.6rem 1rem;
  transition: background var(--kumo-transition-fast);
}

.dropdown-item:hover {
  background: var(--kumo-primary-soft);
}
```

### Phase 7: Alerts & Badges (Day 3 Afternoon)

**7.1 Alerts / Toasts**
```css
.alert {
  backdrop-filter: blur(8px);
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-md);
}

.toast {
  background: rgba(30, 30, 40, 0.95);
  backdrop-filter: blur(16px);
  border: 1px solid var(--kumo-glass-border);
  border-radius: var(--kumo-radius-md);
}
```

**7.2 Badges**
```css
.badge {
  border-radius: var(--kumo-radius-sm);
  padding: 0.4em 0.7em;
  font-weight: 500;
}
```

### Phase 8: Polish & Animations (Day 3 Afternoon)

**8.1 Page Transitions**
```css
.content-wrapper {
  animation: fadeIn var(--kumo-transition-slow);
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}
```

**8.2 Loading States**
```css
.skeleton {
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0.03) 25%,
    rgba(255, 255, 255, 0.08) 50%,
    rgba(255, 255, 255, 0.03) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: var(--kumo-radius-sm);
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
```

**8.3 Scrollbar**
```css
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.25);
}
```

## File Structure

```
web/static/css/kumo-theme.css    # All styles in one file (~600 lines)
```

## Integration

### base.html
```html
<link rel="stylesheet" href="/static/css/adminlte.css" />
<link rel="stylesheet" href="/static/css/adminlte-colors.css" />
<!-- Kumo Theme - comment out to disable -->
<link rel="stylesheet" href="/static/css/kumo-theme.css" />
```

### auth.html
Same as above.

## Testing Checklist

- [ ] Cards render with glass effect
- [ ] Sidebar gradient and blur working
- [ ] Buttons have gradient and hover lift
- [ ] Forms have soft focus states
- [ ] Tables have clean hover rows
- [ ] Modals have backdrop blur
- [ ] Dropdowns have glass effect
- [ ] Scrollbar is styled
- [ ] Transitions are smooth (no jank)
- [ ] Works in Chrome, Firefox, Safari
- [ ] Toggle off reverts cleanly to AdminLTE

## Browser Support

| Feature | Chrome | Firefox | Safari | Edge |
|---------|--------|---------|--------|------|
| backdrop-filter | ✅ | ✅ | ✅ | ✅ |
| CSS Variables | ✅ | ✅ | ✅ | ✅ |
| Gradients | ✅ | ✅ | ✅ | ✅ |

Note: Firefox needs `layout.css.backdrop-filter.enabled` (default on since v103).

## Timeline

| Day | Phase | Hours |
|-----|-------|-------|
| 1 AM | Foundation + Typography | 2h |
| 1 PM | Cards + Containers | 3h |
| 2 AM | Sidebar | 2h |
| 2 PM | Buttons + Forms | 3h |
| 3 AM | Tables + Modals | 3h |
| 3 PM | Polish + Testing | 3h |

**Total: ~16 hours / 2-3 days**

## Rollback

To disable Kumo theme:
1. Comment out `<link rel="stylesheet" href="/static/css/kumo-theme.css" />`
2. Commit and deploy

No other changes needed — AdminLTE renders as before.
