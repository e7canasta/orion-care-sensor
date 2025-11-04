# Scripts - Orion Workspace

**Purpose**: Automation scripts for workspace management.

---

## 📜 Available Scripts

### `create-module.sh`

**Purpose**: Generate new module structure from templates.

**Usage**:
```bash
./scripts/create-module.sh <module-name> <bounded-context> [sprint-number]
```

**Example**:
```bash
./scripts/create-module.sh "Stream Capture" "Stream Acquisition" "Sprint 1.1"
```

**What it does**:
1. Creates `modules/<module-dir>/` structure
2. Copies templates (CLAUDE.md, README.md, BACKLOG.md, DESIGN.md)
3. Generates `go.mod`
4. Creates placeholder files (`types.go`, `*_test.go`)
5. Updates `go.work`

**Output**:
```
modules/stream-capture/
├── go.mod
├── CLAUDE.md
├── README.md
├── BACKLOG.md
├── types.go
├── stream-capture_test.go
├── docs/
│   ├── DESIGN.md
│   └── proposals/
└── internal/
```

---

### `template-replace.sh`

**Purpose**: Replace template variables in generated module files.

**Usage**:
```bash
./scripts/template-replace.sh <module-dir> <key> <value>
```

**Example**:
```bash
./scripts/template-replace.sh stream-capture MODULE_NAME "Stream Capture"
./scripts/template-replace.sh stream-capture BOUNDED_CONTEXT "Stream Acquisition"
./scripts/template-replace.sh stream-capture SPRINT_NUMBER "Sprint 1.1"
```

**Batch replacement**:
```bash
# Replace all variables for a module
MODULE="stream-capture"
./scripts/template-replace.sh $MODULE MODULE_NAME "Stream Capture"
./scripts/template-replace.sh $MODULE MODULE_DIR "stream-capture"
./scripts/template-replace.sh $MODULE BOUNDED_CONTEXT "Stream Acquisition"
./scripts/template-replace.sh $MODULE SPRINT_NUMBER "Sprint 1.1"
./scripts/template-replace.sh $MODULE DATE "$(date +%Y-%m-%d)"
```

---

## 🚀 Quick Start: Create New Module

```bash
# 1. Create module structure
./scripts/create-module.sh "Worker Lifecycle" "Worker Lifecycle Management" "Sprint 1.2"

# 2. Replace template variables
cd modules/worker-lifecycle
# Manually edit CLAUDE.md, BACKLOG.md, DESIGN.md to replace {{...}} placeholders

# 3. Implement module
# Edit files, add internal packages, etc.

# 4. Test
go test ./...
```

---

## 📚 Template Variables

Common template variables used in module templates:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{MODULE_NAME}}` | Human-readable module name | "Stream Capture" |
| `{{MODULE_DIR}}` | Directory name (kebab-case) | "stream-capture" |
| `{{BOUNDED_CONTEXT}}` | Bounded context description | "Stream Acquisition" |
| `{{SPRINT_NUMBER}}` | Sprint identifier | "Sprint 1.1" |
| `{{DATE}}` | Current date | "2025-11-03" |
| `{{PACKAGE_NAME}}` | Go package name | "streamcapture" |
| `{{RESPONSIBILITY_1}}` | Main responsibility | "Capture RTSP frames" |
| `{{ANTI_RESPONSIBILITY_1}}` | Anti-responsibility | "Does NOT process frames" |

---

## 🔧 Future Scripts (To Be Implemented)

### `test-all.sh`

Run tests for all modules in workspace.

```bash
./scripts/test-all.sh
```

### `sync-versions.sh`

Detect version mismatches between modules.

```bash
./scripts/sync-versions.sh
```

### `validate-dependencies.sh`

Validate dependency graph (no circular dependencies).

```bash
./scripts/validate-dependencies.sh
```

---

## 📝 Adding New Scripts

1. Create script in `scripts/` directory
2. Make executable: `chmod +x scripts/your-script.sh`
3. Add documentation to this README
4. Test thoroughly before committing

---

**Last Updated**: 2025-11-03
**Authors**: Ernesto + Gaby (AI Companion)
