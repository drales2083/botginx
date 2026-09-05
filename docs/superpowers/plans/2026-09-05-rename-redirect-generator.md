# Rename Redirect Links → Redirect Generator

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the "Redirect Links" module to "Redirect Generator" throughout the codebase for better branding.

**Architecture:** Simple rename - update menu titles, page headings, and documentation. No structural changes.

**Tech Stack:** Go templates, Go module code

**Spec:** Cosmetic rename only - all functionality remains identical.

## Global Constraints

- Do not change any URLs/routes (keep `/user/redirectlinks`)
- Do not change database table names
- Do not change Go package names
- Only update user-facing text

---

### Task 1: Update Module Menu Title

**Files:**
- Modify: `modules/redirectlinks/module.go:122-130`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated menu item with new title

- [ ] **Step 1: Update MenuItems function**

```go
func (m *Module) MenuItems() []module.MenuItem {
    return []module.MenuItem{
        {
            Title:   "Redirect Generator",  // Changed from "Redirect Links"
            Icon:    "bi-arrow-repeat",     // Changed from "bi-link-45deg"
            Path:    "/user/redirectlinks",
            Order:   20,
            Section: module.MenuSectionUser,
        },
    }
}
```

- [ ] **Step 2: Verify build compiles**

Run: `go build ./cmd/server`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/redirectlinks/module.go
git commit -m "refactor(redirectlinks): rename menu title to Redirect Generator"
```

---

### Task 2: Update List Page Title

**Files:**
- Modify: `modules/redirectlinks/templates/list.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated page title

- [ ] **Step 1: Find and update page title**

Change any instance of "Redirect Links" or "My Redirect Links" to "Redirect Generator" or "My Redirects"

```html
<h4 class="mb-0"><i class="bi bi-arrow-repeat me-2"></i>My Redirects</h4>
```

- [ ] **Step 2: Update any button text**

Change "Create Redirect Link" to "Create Redirect"

- [ ] **Step 3: Commit**

```bash
git add modules/redirectlinks/templates/list.html
git commit -m "refactor(redirectlinks): update list page titles"
```

---

### Task 3: Update New Page Title

**Files:**
- Modify: `modules/redirectlinks/templates/new.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated page title

- [ ] **Step 1: Update page heading**

Change "Create Redirect Link" to "Create Redirect"

- [ ] **Step 2: Update any helper text**

Update any references to "redirect link" → "redirect"

- [ ] **Step 3: Commit**

```bash
git add modules/redirectlinks/templates/new.html
git commit -m "refactor(redirectlinks): update new page titles"
```

---

### Task 4: Update Show Page Title

**Files:**
- Modify: `modules/redirectlinks/templates/show.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated page title

- [ ] **Step 1: Update page heading**

Change "Redirect Link Details" to "Redirect Details"

- [ ] **Step 2: Update breadcrumb if present**

Change "Redirect Links" → "Redirects" in breadcrumb

- [ ] **Step 3: Commit**

```bash
git add modules/redirectlinks/templates/show.html
git commit -m "refactor(redirectlinks): update show page titles"
```

---

### Task 5: Update Customize Page Title

**Files:**
- Modify: `modules/redirectlinks/templates/customize.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated page title

- [ ] **Step 1: Update any titles/headings**

Ensure consistency with "Redirect Generator" branding

- [ ] **Step 2: Commit**

```bash
git add modules/redirectlinks/templates/customize.html
git commit -m "refactor(redirectlinks): update customize page titles"
```

---

### Task 6: Update Handler Titles

**Files:**
- Modify: `modules/redirectlinks/handlers/handler.go`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated Title values in render calls

- [ ] **Step 1: Search for Title values**

Run: `grep -n '"Title"' modules/redirectlinks/handlers/handler.go`

- [ ] **Step 2: Update each Title**

Change:
- "My Redirect Links" → "My Redirects"
- "Create Redirect Link" → "Create Redirect"
- "Redirect Link Details" → "Redirect Details"
- "Customize Redirect" → keep as is (already good)

- [ ] **Step 3: Commit**

```bash
git add modules/redirectlinks/handlers/handler.go
git commit -m "refactor(redirectlinks): update handler titles"
```

---

### Task 7: Update Documentation

**Files:**
- Modify: `CLAUDE.md`

**Interfaces:**
- Consumes: Nothing
- Produces: Updated documentation

- [ ] **Step 1: Search for references**

Run: `grep -n -i "redirect link" CLAUDE.md`

- [ ] **Step 2: Update terminology**

Change "Redirect Links" → "Redirect Generator" where appropriate

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update terminology to Redirect Generator"
```

---

### Task 8: Final Verification

**Files:**
- None (verification only)

- [ ] **Step 1: Build and run**

```bash
go build -o botginx ./cmd/server
./botginx
```

- [ ] **Step 2: Visual verification**

Open browser, verify:
- Sidebar shows "Redirect Generator"
- List page shows "My Redirects"
- New page shows "Create Redirect"
- Show page shows "Redirect Details"

- [ ] **Step 3: Final commit if any fixes needed**

```bash
git add -A
git commit -m "refactor(redirectlinks): complete rename to Redirect Generator"
```
