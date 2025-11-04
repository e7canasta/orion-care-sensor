# Session Notes: Module Templates and Scaffold Infrastructure

**Fecha**: 2025-11-03
**Participantes**: Ernesto (Visiona) + Gaby (AI Companion)
**Sprint**: Pre-Sprint 1.1 (Foundation Phase)
**Duración**: ~2 horas (sesión completa)

---

## 🎯 Objetivo de la Sesión

Crear infraestructura reutilizable (templates + scripts) para generar módulos en el multi-module monorepo.

---

## 📋 Contexto

Después de decidir usar multi-module monorepo (ADR-001), necesitábamos:
1. Templates para documentación consistente por módulo
2. Scripts de automatización para generar módulos
3. Validar approach con módulo piloto (stream-capture)

---

## 🔨 Trabajo Realizado

### 1. Templates Creados

**Ubicación**: `BACKLOG/TEMPLATES/module/`

#### CLAUDE.md.template (5.3 KB)

**Secciones**:
- Module Overview (bounded context, version, sprint)
- Responsibility / Anti-Responsibility
- Public API (interfaces, types)
- Internal Structure
- Dependencies (internal packages, external modules, workspace modules)
- Configuration (config structure, env vars)
- Testing (unit tests, integration tests)
- Development Workflow (before/during/after coding)
- Backlog / Design Decisions references
- Philosophy (bounded context enforcement, complejidad por diseño)

**Placeholder variables**:
```
{{MODULE_NAME}}, {{MODULE_DIR}}, {{BOUNDED_CONTEXT}}, {{SPRINT_NUMBER}},
{{PACKAGE_NAME}}, {{RESPONSIBILITY_1-3}}, {{ANTI_RESPONSIBILITY_1-3}},
{{MAIN_INTERFACE}}, {{PUBLIC_API_FILE}}, {{IMPLEMENTATION_FILE}},
{{INTERNAL_PACKAGE_1-2}}, {{CONFIG_SECTION}}, {{DATE}}
```

#### README.md.template (3.7 KB)

**Secciones**:
- Overview (description, features)
- Responsibility / Anti-Responsibility
- Quick Start (installation, usage)
- Public API (interfaces, types)
- Configuration
- Testing
- Dependencies
- Architecture (component diagram, bounded context)
- Documentation links
- Changelog

#### BACKLOG.md.template (5.0 KB)

**Secciones**:
- Sprint Goal
- Tasks (table with status, estimación, owner)
- Acceptance Criteria (functional, non-functional, testing)
- Implementation Plan (Phase 1-3)
- Technical Details (public API, internal structure, dependencies)
- Blockers
- Decisiones Pendientes
- Session Checklist (antes/durante/después de codear)
- Lecciones Aprendidas (post-sprint)

#### docs/DESIGN.md.template (7.2 KB)

**Secciones**:
- Overview
- Design Goals
- Architecture (high-level design, component breakdown)
- Public API Design
- Data Flow (scenarios con diagramas)
- Design Patterns
- Performance Considerations
- Error Handling (strategy, error types)
- Testing Strategy (unit tests, integration tests)
- Dependencies
- Constraints (technical, business)
- Design Decisions (contexto, opciones, decisión, rationale, consecuencias)
- Future Enhancements
- Design Philosophy

#### go.mod.template

**Contenido**:
```go
module github.com/e7canasta/orion-care-sensor/modules/{{MODULE_DIR}}
go 1.21
// Conditional sections for workspace/external deps
```

---

### 2. Scripts de Automatización

**Ubicación**: `scripts/`

#### create-module.sh (4.2 KB)

**Funcionalidad**:
1. Valida argumentos (`<module-name> <bounded-context> [sprint-number]`)
2. Convierte module-name a kebab-case (e.g., "Stream Capture" → "stream-capture")
3. Crea estructura de directorios:
   ```
   modules/<module-dir>/
   ├── go.mod
   ├── CLAUDE.md
   ├── README.md
   ├── BACKLOG.md
   ├── types.go (placeholder)
   ├── <module-dir>_test.go (placeholder)
   ├── docs/
   │   ├── DESIGN.md
   │   └── proposals/
   └── internal/
   ```
4. Copia templates desde `BACKLOG/TEMPLATES/module/`
5. Genera `go.mod` con module path correcto
6. Actualiza `go.work` automáticamente (crea si no existe)
7. Output con colores (✅ info, ⚠️ warnings)

**Output ejemplo**:
```
✅ Creating module: Stream Capture
✅ Bounded Context: Stream Acquisition
✅ Directory: modules/stream-capture
...
✅ Module created successfully! 🚀
```

#### template-replace.sh (764 bytes)

**Funcionalidad**:
- Reemplaza variables `{{KEY}}` por valor en archivos `.md` y `.go`
- Uso: `./scripts/template-replace.sh <module-dir> <key> <value>`
- Busca recursivamente en `modules/<module-dir>/`

**Uso batch**:
```bash
MODULE="stream-capture"
./scripts/template-replace.sh $MODULE MODULE_NAME "Stream Capture"
./scripts/template-replace.sh $MODULE BOUNDED_CONTEXT "Stream Acquisition"
# etc...
```

#### scripts/README.md

**Contenido**:
- Documentación de scripts disponibles
- Ejemplos de uso
- Template variables reference
- Future scripts (test-all.sh, sync-versions.sh, validate-dependencies.sh)

---

### 3. Módulo Piloto: stream-capture

**Creado con**: `./scripts/create-module.sh "Stream Capture" "Stream Acquisition" "Sprint 1.1"`

**Resultado**:
```
modules/stream-capture/
├── go.mod                      # github.com/e7canasta/orion-care-sensor/modules/stream-capture
├── CLAUDE.md                   # Template copiado (con placeholders)
├── README.md                   # Template copiado
├── BACKLOG.md                  # Template copiado
├── types.go                    # package streamcapture
├── stream-capture_test.go      # Placeholder test (skipped)
├── docs/
│   ├── DESIGN.md               # Template copiado
│   └── proposals/              # .gitkeep
└── internal/                   # .gitkeep
```

**go.work generado**:
```go
go 1.21

use (
    ./modules/stream-capture
)
```

**Validación**:
- ✅ `go build ./...` compila sin errores
- ✅ `go test ./...` pasa (1 test skipped)

---

## 🎸 Filosofía Aplicada

### "Complejidad por Diseño"

Templates enfuerzan:
- ✅ Bounded contexts claros (Responsibility + Anti-Responsibility)
- ✅ Public API design explícito
- ✅ Internal structure documentada
- ✅ Decision tracking (DESIGN.md con template de decisiones)

### "Knowledge Management Exitoso"

Cada módulo documenta:
- **CLAUDE.md** - Guía para AI companion (qué hace, qué NO hace, API)
- **README.md** - User-facing overview
- **BACKLOG.md** - Sprint-specific tasks + lecciones aprendidas
- **docs/DESIGN.md** - Design decisions con rationale

### "KISS Auto-Recovery"

Scripts simples:
- `create-module.sh` - Un solo comando crea estructura completa
- Output claro con ✅/⚠️
- Validaciones básicas (módulo ya existe, argumentos faltantes)
- No reinventa herramientas (usa `sed`, `mkdir`, `cp`)

---

## 📊 Métricas

### Templates

- **Archivos creados**: 5 (CLAUDE.md, README.md, BACKLOG.md, DESIGN.md, go.mod)
- **Total líneas**: ~600 líneas de templates
- **Variables placeholders**: ~50 variables diferentes

### Scripts

- **Archivos creados**: 3 (create-module.sh, template-replace.sh, README.md)
- **Total líneas**: ~250 líneas de bash + docs
- **Funcionalidades**: Crear módulo, reemplazar variables, auto-update go.work

### Módulo Piloto

- **Tiempo de generación**: <1 segundo
- **Archivos generados**: 7 archivos + 2 directorios
- **Compilación**: ✅ Sin errores
- **Tests**: ✅ Pasan (1 placeholder test)

---

## 💡 Lecciones Aprendidas

### Lo que Funcionó Bien ✅

1. **Templates Completos**
   - Cubren todos los aspectos necesarios (docs, backlog, diseño)
   - Variables placeholders facilitan customización
   - Estructura consistente entre módulos

2. **Script de Scaffold Robusto**
   - `create-module.sh` genera estructura completa automáticamente
   - Auto-actualiza `go.work` (crea si no existe)
   - Output claro con colores (✅/⚠️)

3. **Validación con Piloto**
   - `stream-capture` generado exitosamente
   - Compila y tests pasan
   - Valida que templates tienen estructura correcta

4. **Documentación de Scripts**
   - `scripts/README.md` documenta uso y ejemplos
   - Template variables reference clara
   - Future scripts planificados

### Mejoras para Próximas Sesiones 📈

1. **Auto-replacement de Variables**
   - Actualmente `template-replace.sh` requiere llamadas manuales
   - Mejorar: `create-module.sh` podría aceptar JSON con variables
   - Ejemplo: `./create-module.sh --vars vars.json`

2. **Interactive Mode**
   - Script interactivo que pregunta variables una por una
   - Mejor UX que argumentos posicionales

3. **Validation Script**
   - Script que valida templates tienen todas las variables necesarias
   - Detecta `{{MISSING_VAR}}` en archivos generados

4. **Template Inheritance**
   - Algunos módulos necesitarán templates especializados
   - Considerar templates base + templates específicos (e.g., template para Python workers)

### Decisiones Tomadas 🎯

1. **Template variables con `{{...}}`**
   - Fácil de identificar visualmente
   - Compatible con handlebars/mustache (futuro)
   - `sed` puede reemplazar fácilmente

2. **kebab-case para module-dir**
   - Consistente con convenciones Go
   - Evita espacios en paths
   - Fácil conversión desde human-readable name

3. **Placeholder tests con `t.Skip()`**
   - Tests pasan por defecto (no bloquean CI)
   - Recordatorio visual de implementar tests
   - Mejor que no tener tests

4. **go.work auto-update**
   - Reduce fricción al crear módulos
   - Evita error común (olvidar agregar a workspace)
   - Script crea `go.work` si no existe

---

## 🚧 Deuda Técnica Identificada

1. **Template Replacement Manual**
   - Actualmente requiere múltiples llamadas a `template-replace.sh`
   - **Prioridad**: Media
   - **Solución**: Script interactivo o batch replacement

2. **No Validation de Templates**
   - No hay check de que todas las variables fueron reemplazadas
   - **Prioridad**: Media
   - **Solución**: `validate-module.sh` script

3. **Hardcoded Module Path**
   - `github.com/e7canasta/orion-care-sensor` hardcoded en templates
   - **Prioridad**: Baja (solo cambia si fork)
   - **Solución**: Variable {{REPO_PATH}} en templates

---

## 🔗 Referencias

### Documentos Generados Esta Sesión

- [ADR-001: Multi-Module Monorepo Layout](../ADR/001-multi-module-monorepo-layout.md)
- [Session Notes: Multi-Module Layout Decision](2025-11-03-multi-module-layout-decision.md)

### Templates

- [BACKLOG/TEMPLATES/module/CLAUDE.md.template](../../../BACKLOG/TEMPLATES/module/CLAUDE.md.template)
- [BACKLOG/TEMPLATES/module/README.md.template](../../../BACKLOG/TEMPLATES/module/README.md.template)
- [BACKLOG/TEMPLATES/module/BACKLOG.md.template](../../../BACKLOG/TEMPLATES/module/BACKLOG.md.template)
- [BACKLOG/TEMPLATES/module/docs/DESIGN.md.template](../../../BACKLOG/TEMPLATES/module/docs/DESIGN.md.template)

### Scripts

- [scripts/create-module.sh](../../../scripts/create-module.sh)
- [scripts/template-replace.sh](../../../scripts/template-replace.sh)
- [scripts/README.md](../../../scripts/README.md)

### Módulo Piloto

- [modules/stream-capture/](../../../modules/stream-capture/)

---

## 📋 Próximos Pasos (Secuencia)

**Continuación de esta sesión**:

1. ✅ **Documentar sesión** (este archivo)
2. ⬜ **Reemplazar variables en stream-capture** - Customizar templates con valores reales
3. ⬜ **Implementar Sprint 1.1** - Codear Stream Capture module
4. ⬜ **Commit** - Preservar trabajo

**Próximas sesiones**:
- Sprint 1.1: Implementar Stream Capture (RTSP capture, reconnection, FPS adaptation)
- Sprint 1.2: Implementar Worker Lifecycle (spawning, health monitoring)
- Sprint 2: Implementar Control Plane (MQTT commands, hot-reload)

---

## 🎯 Criterios de Éxito

- ✅ Templates cubren documentación completa por módulo
- ✅ Scripts automatizan generación de estructura
- ✅ Módulo piloto valida approach
- ✅ Compila y tests pasan
- ✅ Documentación clara (scripts/README.md)

**Estado**: ✅ Todos los criterios cumplidos

---

**Cierre de Fase**: Infrastructure de templates lista para Sprint 1.1 🚀

**Próximo paso**: Customizar stream-capture y empezar implementación
