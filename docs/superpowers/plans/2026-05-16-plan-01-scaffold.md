# sigil-manager Plan 01: Scaffold + Tooling

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a working dev environment where a developer can clone the repo, install dependencies, run `make dev` (Go API + Vite frontend, hot reload), see a hello-world React page that talks to `/api/health`, and have `make test`, `make lint`, `make build` all work. CI green on push.

**Architecture:** Single Go module owning the binary, with a Vite + React + TypeScript frontend under `web/`. In dev, Vite serves the SPA on `:5173` and proxies `/api/*` to the Go server on `:8080`. In prod, `make build` builds the SPA to `web/dist/`, then Go embeds it via `embed.FS` and serves both API (`/api/*`) and SPA (`/*`) from a single binary.

**Tech Stack (locked):**
- Backend: Go 1.22+, chi v5 router, modernc.org/sqlite (pure-Go, no cgo), golang-jwt/jwt v5, golang.org/x/crypto/bcrypt
- Frontend: React 18 + Vite + TypeScript, Tailwind v4, shadcn/ui, TanStack Router, TanStack Query
- Tests: Go stdlib `testing` + testify; Vitest + @testing-library/react
- Lint: golangci-lint, Biome
- CI: GitHub Actions

Plan 1 wires the tooling and a smoke-test endpoint. Plans 2-7 build features on top.

---

## Prerequisites

The implementing engineer needs locally:
- **Go 1.22+** (`go version`)
- **Node 20 LTS+** and **npm 10+** (`node --version`, `npm --version`)
- **golangci-lint** (`golangci-lint --version`) — install via `brew install golangci-lint` or [official install script](https://golangci-lint.run/usage/install/)
- **air** for Go hot reload (`go install github.com/air-verse/air@latest`)
- **make**

If any of these is missing, stop and install before starting Task 1.

---

## File structure (target end state of Plan 1)

```
sigil-manager/
├── .github/
│   └── workflows/
│       └── ci.yml                       # CI: build + test + lint
├── .gitignore                           # already exists; will be extended
├── .golangci.yml                        # golangci-lint config
├── .editorconfig                        # consistent whitespace
├── .nvmrc                               # node version pin
├── .air.toml                            # Go hot reload config
├── CLAUDE.md                            # already exists
├── README.md                            # quickstart (NEW)
├── Makefile                             # dev/test/lint/build/clean targets
├── go.mod                               # NEW
├── go.sum                               # NEW (managed by go mod)
├── cmd/
│   └── sigil-manager/
│       └── main.go                      # binary entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go                  # HTTP handlers (health for now)
│   │   └── handlers_test.go
│   ├── httputil/
│   │   ├── response.go                  # JSON response helper (leaf pkg, no cycles)
│   │   └── response_test.go
│   └── server/
│       ├── server.go                    # http.Server setup + chi router
│       └── spa.go                       # embed.FS for production SPA
├── web/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── biome.json
│   ├── index.html
│   ├── public/                          # static assets (favicon etc.)
│   ├── src/
│   │   ├── main.tsx                     # React entry
│   │   ├── App.tsx                      # root component
│   │   ├── routes/
│   │   │   ├── __root.tsx               # TanStack Router root
│   │   │   └── index.tsx                # "/" route — hello world + /api/health probe
│   │   ├── lib/
│   │   │   └── api.ts                   # fetch helper
│   │   ├── styles/
│   │   │   └── tokens.css               # design tokens from spec §6
│   │   ├── components/
│   │   │   └── ui/                      # shadcn-generated
│   │   └── index.css                    # Tailwind + token import
│   └── tests/
│       └── App.test.tsx
└── docs/
    └── superpowers/
        ├── specs/
        │   └── 2026-05-16-ui-ux-design.md   # already exists
        └── plans/
            └── 2026-05-16-plan-01-scaffold.md   # this file
```

---

## Task 1: Initialize Go module + supporting config files

**Files:**
- Create: `go.mod`
- Create: `.editorconfig`
- Create: `.nvmrc`
- Modify: `.gitignore` (extend)

- [ ] **Step 1: Initialize the Go module**

Run from the repo root:

```bash
go mod init github.com/Ju571nK/sigil-manager
```

Expected output: `go: creating new go.mod: module github.com/Ju571nK/sigil-manager`. A `go.mod` file is created.

- [ ] **Step 2: Create `.editorconfig`**

Write `.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.go]
indent_style = tab
indent_size = 4

[*.{ts,tsx,js,jsx,json,css,html,md,yml,yaml}]
indent_style = space
indent_size = 2

[Makefile]
indent_style = tab
```

- [ ] **Step 3: Create `.nvmrc`**

Write `.nvmrc`:

```
20
```

- [ ] **Step 4: Extend `.gitignore`**

Open `.gitignore` and append these blocks (do not remove existing content):

```gitignore

# Go
/sigil-manager
/dist/
*.test
coverage.out
coverage.html

# Node / Vite
web/node_modules/
web/dist/
web/.vite/
web/coverage/

# Air (Go hot reload)
tmp/
build-errors.log
```

- [ ] **Step 5: Commit**

```bash
git add go.mod .editorconfig .nvmrc .gitignore
git commit -m "Initialize Go module and base editor config"
```

Expected: 1 commit, working tree clean (run `git status` to verify).

---

## Task 2: Minimal Go HTTP server with chi + /api/health (TDD)

**Files:**
- Create: `cmd/sigil-manager/main.go`
- Create: `internal/server/server.go`
- Create: `internal/api/handlers.go`
- Create: `internal/api/handlers_test.go`

- [ ] **Step 1: Add chi dependency**

```bash
go get github.com/go-chi/chi/v5
go get github.com/stretchr/testify/assert
```

Expected: `go.mod` and `go.sum` updated with chi and testify.

- [ ] **Step 2: Write the failing test for the health handler**

Write `internal/api/handlers_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "timestamp")
}
```

- [ ] **Step 3: Run the test, verify it fails**

```bash
go test ./internal/api/...
```

Expected: FAIL — `undefined: HealthHandler`.

- [ ] **Step 4: Implement the handler**

Write `internal/api/handlers.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Version is set at build time via -ldflags. Defaults to "dev" for local builds.
var Version = "dev"

type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:    "ok",
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
```

- [ ] **Step 5: Run the test, verify it passes**

```bash
go test ./internal/api/... -v
```

Expected: `PASS: TestHealthHandler_ReturnsOK`.

- [ ] **Step 6: Wire up the chi router**

Write `internal/server/server.go`:

```go
package server

import (
	"net/http"

	"github.com/Ju571nK/sigil-manager/internal/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", api.HealthHandler)
	})

	return r
}
```

- [ ] **Step 7: Wire up main.go**

Write `cmd/sigil-manager/main.go`:

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Ju571nK/sigil-manager/internal/server"
)

func main() {
	addr := os.Getenv("SIGIL_MANAGER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	router := server.NewRouter()
	log.Printf("sigil-manager listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

- [ ] **Step 8: Smoke-test the binary**

```bash
go run ./cmd/sigil-manager &
sleep 1
curl -s http://localhost:8080/api/health
kill %1
```

Expected JSON output: `{"status":"ok","version":"dev","timestamp":"2026-05-16T..."}`.

- [ ] **Step 9: Commit**

```bash
git add cmd/ internal/ go.mod go.sum
git commit -m "Add minimal Go HTTP server with chi and /api/health endpoint"
```

---

## Task 3: JSON response helper in a leaf `httputil` package

**Files:**
- Create: `internal/httputil/response.go`
- Create: `internal/httputil/response_test.go`
- Modify: `internal/api/handlers.go`

> **Why a separate `httputil` package, not `server`:** `internal/server`
> already imports `internal/api` (to wire up handlers in `NewRouter`). If
> `internal/api` then imported `internal/server` for `WriteJSON`, that's
> an import cycle. `httputil` is a leaf package with no internal deps —
> safe for any handler in `internal/api` (and any future package) to import.

- [ ] **Step 1: Write the failing test for `WriteJSON`**

Write `internal/httputil/response_test.go`:

```go
package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteJSON_EncodesPayloadAndSetsHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"hello": "world"}

	WriteJSON(rec, http.StatusCreated, payload)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &got)
	assert.NoError(t, err)
	assert.Equal(t, payload, got)
}
```

- [ ] **Step 2: Run the test, verify it fails**

```bash
go test ./internal/httputil/... -v
```

Expected: FAIL — `undefined: WriteJSON`.

- [ ] **Step 3: Implement `WriteJSON`**

Write `internal/httputil/response.go`:

```go
package httputil

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/httputil/... -v
```

Expected: `PASS: TestWriteJSON_EncodesPayloadAndSetsHeaders`.

- [ ] **Step 5: Refactor `HealthHandler` to use `httputil.WriteJSON`**

Replace the body of `internal/api/handlers.go` with:

```go
package api

import (
	"net/http"
	"time"

	"github.com/Ju571nK/sigil-manager/internal/httputil"
)

var Version = "dev"

type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
```

- [ ] **Step 6: Re-run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/httputil/response.go internal/httputil/response_test.go internal/api/handlers.go
git commit -m "Extract WriteJSON helper and use it from HealthHandler"
```

---

## Task 4: Scaffold Vite + React + TypeScript under `web/`

**Files:**
- Create: `web/` (entire Vite project tree)

- [ ] **Step 1: Scaffold with Vite's react-ts template**

From the repo root:

```bash
npm create vite@latest web -- --template react-ts
```

When prompted, accept all defaults. Expected: a new `web/` directory is created with package.json, tsconfig.json, vite.config.ts, src/, etc.

- [ ] **Step 2: Install dependencies**

```bash
cd web
npm install
cd ..
```

Expected: `web/node_modules/` is populated, `web/package-lock.json` is created.

- [ ] **Step 3: Smoke-test the default Vite app**

```bash
cd web
npm run dev &
sleep 3
curl -s http://localhost:5173/ | grep -q "Vite + React" && echo "OK" || echo "FAIL"
kill %1
cd ..
```

Expected output: `OK`.

- [ ] **Step 4: Commit the scaffolded frontend**

```bash
git add web/
git commit -m "Scaffold React + Vite + TypeScript frontend under web/"
```

`web/node_modules/` is gitignored; only source files commit.

---

## Task 5: Configure Vite dev proxy + minimal hello-world page calling /api/health

**Files:**
- Modify: `web/vite.config.ts`
- Modify: `web/src/App.tsx`
- Create: `web/src/lib/api.ts`
- Delete: `web/src/App.css`, `web/src/assets/react.svg` (default Vite cruft we will replace)

- [ ] **Step 1: Configure Vite dev proxy**

Replace `web/vite.config.ts` with:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
```

- [ ] **Step 2: Create the API fetch helper**

Write `web/src/lib/api.ts`:

```ts
// Vite 8's TS 6 template enables erasableSyntaxOnly, which forbids parameter
// property shorthand (e.g. `constructor(public status: number)`). Use
// explicit declaration + assignment.
export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = 'ApiError';
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  });
  if (!res.ok) {
    throw new ApiError(res.status, `${res.status} ${res.statusText}`);
  }
  return (await res.json()) as T;
}
```

- [ ] **Step 3: Replace `App.tsx` with a hello-world that calls `/api/health`**

Replace `web/src/App.tsx` with:

```tsx
import { useEffect, useState } from 'react';
import { apiFetch } from './lib/api';

interface Health {
  status: string;
  version: string;
  timestamp: string;
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<Health>('/api/health')
      .then(setHealth)
      .catch((e: Error) => setError(e.message));
  }, []);

  return (
    <main style={{ fontFamily: 'system-ui', padding: '2rem', color: '#fafafa', background: '#0a0a0c', minHeight: '100vh' }}>
      <h1>sigil-manager</h1>
      <p>Scaffolded. API probe:</p>
      {error && <pre style={{ color: '#ef4444' }}>{error}</pre>}
      {health && <pre>{JSON.stringify(health, null, 2)}</pre>}
      {!error && !health && <p>Loading…</p>}
    </main>
  );
}
```

- [ ] **Step 4: Remove default Vite cruft**

```bash
rm web/src/App.css web/src/assets/react.svg
```

Open `web/src/main.tsx` and remove the `import './index.css'` line ONLY IF the index.css contains the default Vite styles you want gone. (We will reset `index.css` in Task 6 anyway, so leave the import for now.)

Also remove the `import './App.css'` line from `web/src/App.tsx` — it was deleted above.

- [ ] **Step 5: Smoke-test the wired dev environment**

Open two terminals.

Terminal 1:

```bash
go run ./cmd/sigil-manager
```

Terminal 2:

```bash
cd web && npm run dev
```

Open `http://localhost:5173` in a browser. Expected: the page shows the health response JSON (status: ok, version: dev, ...). Kill both processes when done.

- [ ] **Step 6: Commit**

```bash
git add web/vite.config.ts web/src/App.tsx web/src/lib/api.ts
git add -u web/src/                      # picks up deletions
git commit -m "Wire Vite dev proxy and frontend hello-world calling /api/health"
```

---

## Task 6: Install Tailwind v4 + bake in design tokens from spec §6

**Files:**
- Modify: `web/package.json` (deps)
- Modify: `web/vite.config.ts` (plugin)
- Create: `web/src/styles/tokens.css`
- Replace: `web/src/index.css`

- [ ] **Step 1: Install Tailwind v4**

```bash
cd web
npm install -D tailwindcss @tailwindcss/vite
cd ..
```

- [ ] **Step 2: Add Tailwind to Vite config**

Replace `web/vite.config.ts` with:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
```

- [ ] **Step 3: Create the design tokens file**

Write `web/src/styles/tokens.css`:

```css
/*
 * Design tokens from docs/superpowers/specs/2026-05-16-ui-ux-design.md §6.
 * Severity hex values are binding semantics — do not repurpose them.
 */
@theme {
  /* Backgrounds */
  --color-bg-page: #0a0a0c;
  --color-bg-surface: #0d0d10;
  --color-bg-elevated: #18181b;
  --color-border-subtle: #1f1f23;
  --color-border: #27272a;

  /* Text */
  --color-text-primary: #fafafa;
  --color-text-body: #e4e4e7;
  --color-text-muted: #a1a1aa;
  --color-text-subtle: #71717a;

  /* Severity (binding semantic) */
  --color-sev-critical: #ef4444;
  --color-sev-high: #f97316;
  --color-sev-medium: #eab308;
  --color-sev-low: #3b82f6;
  --color-sev-info: #71717a;

  /* Status / accent */
  --color-status-healthy: #22c55e;
  --color-status-degraded: #f59e0b;
  --color-status-down: #ef4444;
  --color-accent: #a78bfa;

  /* Typography */
  --font-sans: 'Inter', -apple-system, system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'SF Mono', Menlo, monospace;
}
```

- [ ] **Step 4: Replace `web/src/index.css`**

Overwrite `web/src/index.css` with:

```css
@import 'tailwindcss';
@import './styles/tokens.css';

html, body, #root {
  background: var(--color-bg-page);
  color: var(--color-text-body);
  font-family: var(--font-sans);
  margin: 0;
  min-height: 100vh;
}
```

- [ ] **Step 5: Refactor `App.tsx` to use Tailwind + tokens**

Replace `web/src/App.tsx` with:

```tsx
import { useEffect, useState } from 'react';
import { apiFetch } from './lib/api';

interface Health {
  status: string;
  version: string;
  timestamp: string;
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<Health>('/api/health')
      .then(setHealth)
      .catch((e: Error) => setError(e.message));
  }, []);

  return (
    <main className="font-sans p-8 text-text-body bg-bg-page min-h-screen">
      <h1 className="text-2xl font-semibold text-text-primary">sigil-manager</h1>
      <p className="text-text-muted mt-1">Scaffolded. API probe:</p>
      {error && <pre className="mt-4 text-sev-critical">{error}</pre>}
      {health && (
        <pre className="mt-4 p-3 bg-bg-elevated border border-border rounded font-mono text-sm">
          {JSON.stringify(health, null, 2)}
        </pre>
      )}
      {!error && !health && <p className="mt-4 text-text-subtle">Loading…</p>}
    </main>
  );
}
```

- [ ] **Step 6: Smoke-test (visual)**

Run Go server in terminal 1, Vite in terminal 2. Open `http://localhost:5173`. Expected: dark background, white heading, JSON in a bordered monospace block. The hex values come from CSS variables — confirm by inspecting an element in DevTools and seeing `var(--color-bg-page)`.

- [ ] **Step 7: Commit**

```bash
git add web/package.json web/package-lock.json web/vite.config.ts web/src/styles/tokens.css web/src/index.css web/src/App.tsx
git commit -m "Add Tailwind v4 and design tokens from UI/UX spec section 6"
```

---

## Task 7: Initialize shadcn/ui + add a Button to validate

**Files:**
- Create: `web/components.json`
- Create: `web/src/lib/utils.ts`
- Create: `web/src/components/ui/button.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Install shadcn dependencies**

```bash
cd web
npm install class-variance-authority clsx tailwind-merge lucide-react
npm install -D @types/node
cd ..
```

- [ ] **Step 2: Create the components.json**

shadcn v4 with Tailwind v4 needs a manual `components.json`. Write `web/components.json`:

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "default",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/index.css",
    "baseColor": "neutral",
    "cssVariables": true,
    "prefix": ""
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  }
}
```

- [ ] **Step 3: Configure path alias `@`**

Edit `web/tsconfig.json` and add the `paths` block under `compilerOptions`. The file likely looks like:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

Add the `baseUrl` and `paths` entries shown above if missing. Make sure the JSON stays valid.

Also edit `web/vite.config.ts` to resolve `@`:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
```

- [ ] **Step 4: Create the `cn` utility**

Write `web/src/lib/utils.ts`:

```ts
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 5: Add a hand-written Button (avoids running the shadcn CLI for now — copy of canonical shadcn button)**

Write `web/src/components/ui/button.tsx`:

```tsx
import { cva, type VariantProps } from 'class-variance-authority';
import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

const buttonVariants = cva(
  'inline-flex items-center justify-center rounded text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50 disabled:pointer-events-none',
  {
    variants: {
      variant: {
        primary: 'bg-accent text-bg-page hover:opacity-90',
        secondary: 'bg-bg-elevated text-text-primary hover:bg-border',
        destructive: 'border border-sev-critical text-sev-critical hover:bg-sev-critical/10',
        ghost: 'text-text-body hover:bg-bg-elevated',
      },
      size: {
        sm: 'h-7 px-2.5 text-xs',
        md: 'h-8 px-3',
        lg: 'h-10 px-4',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  }
);

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
  )
);
Button.displayName = 'Button';
```

- [ ] **Step 6: Render the Button on the hello page to validate**

Replace `web/src/App.tsx` with:

```tsx
import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import { Button } from '@/components/ui/button';

interface Health {
  status: string;
  version: string;
  timestamp: string;
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  const probe = () => {
    setError(null);
    setHealth(null);
    apiFetch<Health>('/api/health')
      .then(setHealth)
      .catch((e: Error) => setError(e.message));
  };

  useEffect(probe, []);

  return (
    <main className="font-sans p-8 text-text-body bg-bg-page min-h-screen">
      <h1 className="text-2xl font-semibold text-text-primary">sigil-manager</h1>
      <p className="text-text-muted mt-1">Scaffolded. API probe:</p>
      <div className="mt-4 flex gap-2">
        <Button onClick={probe}>Probe again</Button>
        <Button variant="secondary" onClick={() => setHealth(null)}>
          Clear
        </Button>
      </div>
      {error && <pre className="mt-4 text-sev-critical">{error}</pre>}
      {health && (
        <pre className="mt-4 p-3 bg-bg-elevated border border-border rounded font-mono text-sm">
          {JSON.stringify(health, null, 2)}
        </pre>
      )}
      {!error && !health && <p className="mt-4 text-text-subtle">Loading…</p>}
    </main>
  );
}
```

- [ ] **Step 7: Smoke-test**

Run Go + Vite. Visit `http://localhost:5173`. Expected: two buttons appear, the primary one is violet (`--color-accent`). Clicking "Probe again" re-fetches health.

- [ ] **Step 8: Commit**

```bash
git add web/components.json web/tsconfig.json web/vite.config.ts web/src/lib/utils.ts web/src/components/ui/button.tsx web/src/App.tsx web/package.json web/package-lock.json
git commit -m "Add shadcn/ui foundation (cn util, Button) and path alias @"
```

---

## Task 8: Install TanStack Router + Query with a minimal root route

**Files:**
- Modify: `web/package.json` (deps)
- Modify: `web/src/main.tsx`
- Create: `web/src/routes/__root.tsx`
- Create: `web/src/routes/index.tsx`
- Delete: `web/src/App.tsx` (logic moves into the index route)

- [ ] **Step 1: Install TanStack libraries**

```bash
cd web
npm install @tanstack/react-router @tanstack/react-query
npm install -D @tanstack/router-plugin @tanstack/router-devtools @tanstack/react-query-devtools
cd ..
```

- [ ] **Step 2: Add the TanStack Router Vite plugin**

Edit `web/vite.config.ts`:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { TanStackRouterVite } from '@tanstack/router-plugin/vite';
import path from 'node:path';

export default defineConfig({
  plugins: [TanStackRouterVite({ target: 'react', autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
```

- [ ] **Step 3: Create the root route**

Write `web/src/routes/__root.tsx`:

```tsx
import { createRootRoute, Outlet } from '@tanstack/react-router';

export const Route = createRootRoute({
  component: () => (
    <div className="min-h-screen bg-bg-page text-text-body">
      <Outlet />
    </div>
  ),
});
```

- [ ] **Step 4: Create the index route**

Write `web/src/routes/index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api';
import { Button } from '@/components/ui/button';

interface Health {
  status: string;
  version: string;
  timestamp: string;
}

export const Route = createFileRoute('/')({
  component: IndexPage,
});

function IndexPage() {
  const { data, error, isFetching, refetch } = useQuery({
    queryKey: ['health'],
    queryFn: () => apiFetch<Health>('/api/health'),
  });

  return (
    <main className="font-sans p-8">
      <h1 className="text-2xl font-semibold text-text-primary">sigil-manager</h1>
      <p className="text-text-muted mt-1">Scaffolded. API probe:</p>
      <div className="mt-4 flex gap-2">
        <Button onClick={() => refetch()} disabled={isFetching}>
          {isFetching ? 'Probing…' : 'Probe again'}
        </Button>
      </div>
      {error && <pre className="mt-4 text-sev-critical">{(error as Error).message}</pre>}
      {data && (
        <pre className="mt-4 p-3 bg-bg-elevated border border-border rounded font-mono text-sm">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </main>
  );
}
```

- [ ] **Step 5: Rewrite `main.tsx` to bootstrap Router + Query**

Replace `web/src/main.tsx` with:

```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createRouter } from '@tanstack/react-router';
import './index.css';

// Generated by @tanstack/router-plugin on dev/build.
import { routeTree } from './routeTree.gen';

const router = createRouter({ routeTree });
const queryClient = new QueryClient();

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>
);
```

The `routeTree.gen.ts` file is generated automatically by the Vite plugin on the first run.

- [ ] **Step 6: Delete the old `App.tsx`**

```bash
rm web/src/App.tsx
```

- [ ] **Step 7: Add `routeTree.gen.ts` to gitignore**

Append to `.gitignore`:

```gitignore

# TanStack Router generated
web/src/routeTree.gen.ts
```

- [ ] **Step 8: Smoke-test**

Start Go (`go run ./cmd/sigil-manager`) and Vite (`cd web && npm run dev`). Visit `http://localhost:5173/`. Expected: same hello-world page, but now powered by TanStack Router + Query (refetch button uses `useQuery`'s `refetch`).

- [ ] **Step 9: Commit**

```bash
git add web/package.json web/package-lock.json web/vite.config.ts web/src/main.tsx web/src/routes/__root.tsx web/src/routes/index.tsx .gitignore
git add -u web/src/                      # picks up App.tsx deletion
git commit -m "Wire TanStack Router and TanStack Query with a single index route"
```

---

## Task 9: Embed the built SPA in the Go binary for production

**Files:**
- Create: `internal/server/spa.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Build the frontend once so `web/dist/` exists**

```bash
cd web
npm run build
cd ..
ls web/dist/
```

Expected: `index.html`, `assets/`, etc. exist under `web/dist/`.

- [ ] **Step 2: Implement the SPA handler with embed.FS**

Write `internal/server/spa.go`:

```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// spaHandler serves the SPA from the embedded dist directory.
// Unknown paths fall back to index.html so client-side routing works.
func spaHandler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(sub, clean); err != nil {
			// Not a real asset → SPA fallback to index.html.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
```

The `//go:embed` directive will fail to compile if `internal/server/dist/` doesn't exist. We'll set up a symlink + build flag in the Makefile (Task 10), but for now create a placeholder directory:

```bash
mkdir -p internal/server/dist
echo '<!doctype html><html><body><p>Placeholder. Build the frontend with `make build`.</p></body></html>' > internal/server/dist/index.html
```

- [ ] **Step 3: Wire the SPA handler into the router**

Replace `internal/server/server.go` with:

```go
package server

import (
	"log"
	"net/http"

	"github.com/Ju571nK/sigil-manager/internal/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", api.HealthHandler)
	})

	spa, err := spaHandler()
	if err != nil {
		log.Fatalf("failed to mount SPA: %v", err)
	}
	r.Handle("/*", spa)

	return r
}
```

- [ ] **Step 4: Add `internal/server/dist/` to gitignore but keep the placeholder index.html committed**

Append to `.gitignore`:

```gitignore

# SPA dist is copied here by `make build`. Only the placeholder index.html is tracked.
internal/server/dist/*
!internal/server/dist/index.html
```

- [ ] **Step 5: Smoke-test the full prod path**

```bash
go build -o sigil-manager ./cmd/sigil-manager
./sigil-manager &
sleep 1
curl -s http://localhost:8080/api/health   # expect JSON
curl -s http://localhost:8080/ | head -3   # expect the placeholder HTML
kill %1
rm sigil-manager
```

Expected: API responds with JSON, root responds with the placeholder HTML.

- [ ] **Step 6: Commit**

```bash
git add internal/server/spa.go internal/server/server.go internal/server/dist/index.html .gitignore
git commit -m "Embed built SPA via embed.FS and serve it on / with SPA fallback"
```

---

## Task 10: Makefile + air config for dev/test/lint/build/clean

**Files:**
- Create: `Makefile`
- Create: `.air.toml`

- [ ] **Step 1: Create `.air.toml` for Go hot reload**

Write `.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/sigil-manager ./cmd/sigil-manager"
bin = "./tmp/sigil-manager"
include_ext = ["go"]
exclude_dir = ["tmp", "web", "internal/server/dist"]
delay = 200

[log]
time = true
```

- [ ] **Step 2: Create the Makefile**

Write `Makefile`:

```makefile
.DEFAULT_GOAL := help

.PHONY: help dev dev-go dev-web build build-web build-go test test-go test-web lint lint-go lint-web clean

help:
	@echo "Available targets:"
	@echo "  make dev-go     - Run Go server with hot reload (terminal 1)"
	@echo "  make dev-web    - Run Vite frontend (terminal 2)"
	@echo "  make build      - Build SPA + Go binary (production artifact: ./sigil-manager)"
	@echo "  make test       - Run all tests (Go + frontend)"
	@echo "  make lint       - Run all linters"
	@echo "  make clean      - Remove build artifacts"

dev-go:
	air

dev-web:
	cd web && npm run dev

build-web:
	cd web && npm run build
	rm -rf internal/server/dist
	cp -r web/dist internal/server/dist

build-go:
	go build -ldflags "-X github.com/Ju571nK/sigil-manager/internal/api.Version=$$(git describe --always --dirty 2>/dev/null || echo dev)" -o sigil-manager ./cmd/sigil-manager

build: build-web build-go

test-go:
	go test ./...

test-web:
	cd web && npm test -- --run

test: test-go test-web

lint-go:
	golangci-lint run ./...

lint-web:
	cd web && npm run lint

lint: lint-go lint-web

clean:
	rm -rf tmp/ sigil-manager web/dist/ internal/server/dist/*
	@mkdir -p internal/server/dist
	@echo '<!doctype html><html><body><p>Placeholder. Build the frontend with `make build`.</p></body></html>' > internal/server/dist/index.html
```

- [ ] **Step 3: Smoke-test each target**

```bash
make help              # expect target list
make build             # expect ./sigil-manager binary and populated internal/server/dist/
./sigil-manager &
sleep 1
curl -s http://localhost:8080/api/health
curl -s http://localhost:8080/ | head -3      # the real built SPA index.html now
kill %1
make clean             # expect binary gone, dist reset to placeholder
make test-go           # expect existing Go tests pass
```

The `make test-web` target will fail until Vitest is configured (Task 13 of this plan handles linters; we add Vitest below). Skip `make test` for now.

- [ ] **Step 4: Add Vitest so `make test-web` passes**

```bash
cd web
npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom @vitest/coverage-v8
cd ..
```

Edit `web/package.json` and add this `scripts` block (replace the existing `scripts` block, keeping all existing entries and adding `test`):

```json
"scripts": {
  "dev": "vite",
  "build": "tsc -b && vite build",
  "lint": "biome check src/",
  "preview": "vite preview",
  "test": "vitest"
}
```

Add the `test` config to `web/vite.config.ts` (inside the `defineConfig` object, after the `build` block):

```ts
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./tests/setup.ts'],
  },
```

(TypeScript may complain about `test` not existing on the Vite config type; add this triple-slash directive at the top of `vite.config.ts`: `/// <reference types="vitest" />`.)

Create `web/tests/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
```

Create one smoke test at `web/tests/App.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';

describe('Button', () => {
  it('renders children', () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <Button>Click</Button>
      </QueryClientProvider>
    );
    expect(screen.getByRole('button', { name: 'Click' })).toBeInTheDocument();
  });
});
```

Run `make test-web` — expect 1 test passing.

- [ ] **Step 5: Commit**

```bash
git add Makefile .air.toml web/package.json web/package-lock.json web/vite.config.ts web/tests/setup.ts web/tests/App.test.tsx
git commit -m "Add Makefile, air config, and Vitest smoke test"
```

---

## Task 11: Configure golangci-lint and Biome

**Files:**
- Create: `.golangci.yml`
- Create: `web/biome.json`
- Modify: `web/package.json` (lint script — already added in Task 10)

- [ ] **Step 1: Create `.golangci.yml`**

Write `.golangci.yml`:

```yaml
run:
  timeout: 3m

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - revive
    - misspell

linters-settings:
  goimports:
    local-prefixes: github.com/Ju571nK/sigil-manager
  revive:
    rules:
      - name: exported
      - name: var-naming
      - name: indent-error-flow

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

- [ ] **Step 2: Run `make lint-go` and verify no findings**

```bash
make lint-go
```

Expected: no output and exit code 0.

If golangci-lint reports issues, fix them inline (most likely missing exported-symbol comments — add brief one-line doc comments).

- [ ] **Step 3: Install Biome**

```bash
cd web
npm install -D --save-exact @biomejs/biome
cd ..
```

- [ ] **Step 4: Create `web/biome.json`**

Write `web/biome.json`:

```json
{
  "$schema": "https://biomejs.dev/schemas/1.9.4/schema.json",
  "organizeImports": { "enabled": true },
  "formatter": {
    "enabled": true,
    "indentStyle": "space",
    "indentWidth": 2,
    "lineWidth": 100
  },
  "linter": {
    "enabled": true,
    "rules": {
      "recommended": true,
      "suspicious": {
        "noExplicitAny": "error"
      },
      "style": {
        "useImportType": "error"
      }
    }
  },
  "javascript": {
    "formatter": { "quoteStyle": "single", "semicolons": "always" }
  },
  "files": {
    "ignore": ["dist", "node_modules", "src/routeTree.gen.ts"]
  }
}
```

- [ ] **Step 5: Run `make lint-web` and fix any findings**

```bash
make lint-web
```

Biome will likely flag a few style issues — apply its auto-fixes:

```bash
cd web && npx biome check --apply src/ && cd ..
```

Re-run `make lint-web`. Expected: exit code 0.

- [ ] **Step 6: Run `make lint` to confirm both pass**

```bash
make lint
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add .golangci.yml web/biome.json web/package.json web/package-lock.json
git add -u web/src/                       # picks up any biome auto-fixes
git commit -m "Add golangci-lint and Biome configs"
```

---

## Task 12: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

Write `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    name: Go (test + lint + build)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Go test
        run: go test ./...

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.62

      - name: Go build
        run: go build -o sigil-manager ./cmd/sigil-manager

  web:
    name: Web (test + lint + build)
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - run: npm ci

      - run: npm run lint

      - run: npm test -- --run

      - run: npm run build
```

- [ ] **Step 2: Smoke-test locally (act / nektos optional)**

CI can only be fully smoke-tested when pushed. Locally, verify the commands work:

```bash
make test
make lint
make build
```

All should succeed.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "Add GitHub Actions CI: test, lint, build for Go and web"
```

---

## Task 13: README quickstart

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

Write `README.md`:

````markdown
# sigil-manager

Self-hostable web console for the [Sigil](https://github.com/Ju571nK/sigil)
AI-SPM project. Reads fleet data from `sigil-server`; owns local triage
state (acknowledge / assign / resolve / notes) for SOC analyst workflow.

> **Status:** Pre-alpha. Scaffolded — no features yet. See
> `docs/superpowers/specs/2026-05-16-ui-ux-design.md` for the v1 plan.

## Stack

- **Backend:** Go 1.22+, chi router, modernc.org/sqlite (pure-Go), JWT auth
- **Frontend:** React + Vite + TypeScript, TanStack Router/Query, Tailwind v4, shadcn/ui
- **Tests:** `go test` + Vitest
- **Lint:** golangci-lint + Biome
- **Distribution:** single Go binary with the SPA embedded via `embed.FS`

## Prerequisites

- Go 1.22+
- Node 20 LTS+
- `golangci-lint`, `air` (`go install github.com/air-verse/air@latest`)
- `make`

## Quickstart

```bash
# Install frontend deps
cd web && npm ci && cd ..

# Terminal 1: Go server with hot reload
make dev-go

# Terminal 2: Vite frontend (proxies /api to Go)
make dev-web

# Open http://localhost:5173 — should show the hello-world health probe
```

## Commands

| Command          | What it does                                            |
|------------------|---------------------------------------------------------|
| `make dev-go`    | Go server with hot reload via air (port 8080)           |
| `make dev-web`   | Vite frontend dev server (port 5173, proxies `/api`)    |
| `make build`     | Build SPA, embed it, build Go binary → `./sigil-manager` |
| `make test`      | All tests (Go + frontend)                               |
| `make lint`      | golangci-lint + Biome                                   |
| `make clean`     | Remove build artifacts                                  |

## Running the production binary

```bash
make build
./sigil-manager
# Default port :8080, override with SIGIL_MANAGER_ADDR=:9000
```

## Architecture

```
sigil-manager (single Go binary)
├── chi router
├── /api/*  → Go HTTP handlers
└── /*      → embedded SPA (web/dist/ → internal/server/dist/)
```

In dev, Vite serves the SPA directly and proxies `/api/*` to the Go
process. In prod, Go serves both from one process on one port.

## What this repo is NOT

This is the OSS, self-hostable console only. Multi-tenancy, billing,
SSO/SAML, and compliance pipelines live in the (private) `sigil-cloud`
repo. See `CLAUDE.md` and `sigil-strategy.md` for the scope boundaries.

## License

TBD between Apache-2.0 and BSL/source-available. Decision pending; see
`sigil-strategy.md` for context.
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Add README with quickstart and stack overview"
```

---

## Final acceptance test

Run all of these from a clean clone (or after `make clean`). Each must succeed.

- [ ] `go version` prints 1.22+
- [ ] `node --version` prints 20+
- [ ] `cd web && npm ci && cd ..` succeeds
- [ ] `make test` exits 0
- [ ] `make lint` exits 0
- [ ] `make build` produces `./sigil-manager`
- [ ] `./sigil-manager &` followed by `curl http://localhost:8080/api/health` returns JSON with `"status":"ok"`
- [ ] `curl http://localhost:8080/` returns the real built SPA `index.html` (not the placeholder)
- [ ] In dev (`make dev-go` + `make dev-web`), browsing to `http://localhost:5173` shows the hello-world page with a populated JSON probe result
- [ ] CI workflow file passes `actionlint` if installed (optional)

When all of these pass, Plan 1 is complete. Next: Plan 2 (Triage state DB + API).
