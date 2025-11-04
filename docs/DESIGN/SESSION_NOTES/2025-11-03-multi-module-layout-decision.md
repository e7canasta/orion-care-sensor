# Session Notes: Multi-Module Layout Decision

**Fecha**: 2025-11-03
**Participantes**: Ernesto (Visiona) + Gaby (AI Companion)
**Sprint**: Pre-Sprint 1.1 (Foundation Phase)
**Duración**: ~1 hora

---

## 🎯 Objetivo de la Sesión

Definir la estructura de paquetes y módulos para Orion 2.0, basándose en bounded contexts del C4 Model.

---

## 🗣️ Conversación Clave

### Pregunta Inicial (Ernesto)

> "comencemos companero. la propuesta es como se realiza en buenos projectos el estado de arte en la estructura de paquetes ahora si me preguntas el concepto de bounding context como modulos o librerias. me gusta digo tener cada uno un internal propio y exponga al resto sus interfaces o typos.. para poder evolucionarlos.. se te ocurre un layout en el estado del arte digo kubernetes por decirte uno pero vos sabes mejor que yo"

**Interpretación**:
- Busca estado del arte en Go project layouts
- Quiere bounded contexts como módulos independientes
- Cada módulo con su propio `internal/` y APIs públicas
- Referencias: Kubernetes, proyectos enterprise

### Evaluación de Opciones

Gaby presentó 3 opciones:
1. **Multi-Module Monorepo** (Kubernetes style)
2. **Single Module + Internal Packages** (HashiCorp style)
3. **Hybrid - Core + Plugin Modules** (NATS.io style)

### Decisión Final (Ernesto)

> "me gusta el uno te explico por que por que van a ir evolucionando vamos a configurar distintas recipes.. ademas cada modulo debe tener su proppio claude.md y documentacion que es importante tener su propiop plan y backlog, proposals y disenio"

**Rationale**:
- ✅ Evolución independiente de módulos
- ✅ Recipes configurables (edge vs datacenter)
- ✅ Documentación localizada por módulo (CLAUDE.md, BACKLOG, DESIGN)
- ✅ Versionado semántico real

---

## 📋 Decisiones Tomadas

### 1. Multi-Module Monorepo con Go Workspaces

**Estructura**:
```
OrionWork/
├── go.work
├── modules/
│   ├── stream-capture/
│   ├── worker-lifecycle/
│   ├── framebus/
│   ├── control-plane/
│   ├── event-emitter/
│   └── core/
└── cmd/oriond/
```

**Documentado en**: [ADR-001](../ADR/001-multi-module-monorepo-layout.md)

### 2. Documentación por Módulo

Cada módulo incluye:
- `CLAUDE.md` - Guía para AI companion
- `README.md` - Overview para humanos
- `BACKLOG.md` - Sprint-specific tasks
- `docs/DESIGN.md` - Decisiones arquitectónicas
- `docs/proposals/` - RFCs antes de implementar

### 3. ADR (Architecture Decision Records)

Creación de carpeta `docs/DESIGN/ADR/` para documentar decisiones técnicas con:
- Contexto y problema
- Opciones evaluadas
- Decisión y rationale
- Consecuencias (positivas, negativas, mitigations)

**Template creado**: [ADR/README.md](../ADR/README.md)

### 4. Dependency Graph

```
cmd/oriond → core → {stream-capture, worker-lifecycle, framebus, control-plane, event-emitter}
```

**Reglas**:
- ✅ Módulos leaf son independientes
- ❌ Dependencias circulares prohibidas
- ❌ `stream` no puede importar `worker` directamente

---

## 📝 Entregables de la Sesión

### Nuevos Archivos Creados

1. **docs/DESIGN/ADR/001-multi-module-monorepo-layout.md**
   - Documentación completa de la decisión
   - Evaluación de 3 opciones
   - Rationale, consecuencias, mitigations
   - Referencias a C4 Model y Plan Evolutivo

2. **docs/DESIGN/ADR/README.md**
   - Índice de ADRs
   - Template para futuros ADRs
   - Guía de cuándo crear ADRs

3. **docs/DESIGN/SESSION_NOTES/2025-11-03-multi-module-layout-decision.md**
   - Este archivo (notas de sesión)

### Archivos Modificados

1. **CLAUDE.md** (workspace-level)
   - Agregada sección "Orion 2.0 Architecture"
   - Referencias a ADRs
   - Estructura de multi-module monorepo
   - Links a documentación estratégica

---

## 🎸 Filosofía Aplicada

### "Complejidad por Diseño"

- ✅ Atacamos complejidad mediante módulos independientes
- ✅ Boundary enforcement a nivel de Go toolchain
- ✅ Documentación localizada reduce cognitive load

### "Knowledge Management Exitoso"

Sesión siguió el flow:
1. **Café ☕** - Conversación exploratoria (opciones de layout)
2. **Whiteboard 📋** - Análisis de trade-offs (3 opciones)
3. **Decisión 🎯** - Elección basada en contexto (recipes, evolución)
4. **Blueprint 📚** - ADR-001 documenta decisión
5. **Reutilización 🚀** - Template ADR para futuras decisiones

### "Tocar Blues"

- **Conocer escalas** - 3 layouts evaluados (multi-module, single-module, hybrid)
- **Improvisar con contexto** - Elegimos multi-module por recipes y evolución
- **Pragmatismo** - No hybrid (over-engineering para v1.5)

---

## 🔄 Próximos Pasos

### Inmediato (Próxima Sesión)

1. **Crear scaffold de directorios**
   - Script `scripts/create-module.sh`
   - Generar estructura `modules/stream-capture/` (Sprint 1.1 piloto)

2. **Templates reutilizables**
   - Template `CLAUDE.md` por módulo
   - Template `BACKLOG.md` sprint-specific
   - Template `docs/DESIGN.md`

3. **Iniciar Sprint 1.1 (Stream Capture)**
   - Migrar código RTSP existente
   - Implementar bounded context
   - Testing granular

### Fase 1 (Foundation)

- Sprint 1.1: `stream-capture` (2 semanas)
- Sprint 1.2: `worker-lifecycle` (2 semanas)
- Sprint 2: `control-plane` (2 semanas)
- Sprint 3: Integración con Care Scene (2 semanas)

---

## 💡 Lecciones Aprendidas

### Lo que Funcionó Bien ✅

1. **Evaluación de Opciones**
   - Presentar 3 alternativas con pros/cons fue efectivo
   - Ernesto eligió basándose en contexto real (recipes, evolución)

2. **Captura de Rationale**
   - Quote de Ernesto capturado para documentar "por qué"
   - ADR documenta contexto histórico para futuras sesiones

3. **ADR como Memoria Técnica**
   - Crear ADR inmediatamente captura decisión fresca
   - Template permite replicar proceso en futuras decisiones

### Mejoras para Próximas Sesiones 📈

1. **Diagramas Visuales**
   - Incluir Mermaid diagrams en ADRs para dependency graphs
   - Visualizar evolution path (v1.0 → v1.5 → v2.0)

2. **Scripts de Automatización**
   - `create-module.sh` acelera creación de nuevos módulos
   - `test-all.sh` corre tests de todos los módulos

3. **Validation de Dependency Graph**
   - Script que valida no hay dependencias circulares
   - CI/CD check de dependency rules

---

## 🔗 Referencias

### Documentos Generados

- [ADR-001: Multi-Module Monorepo Layout](../ADR/001-multi-module-monorepo-layout.md)
- [ADR Index](../ADR/README.md)

### Documentos Relacionados

- [C4 Model](../C4_MODEL.md)
- [Plan Evolutivo](../ORION_2.0_PLAN_EVOLUTIVO.md)
- [BACKLOG - Fase 1](../../../BACKLOG/FASE_1_FOUNDATION.md)

### External References

- [Go Workspaces](https://go.dev/doc/tutorial/workspaces)
- [Kubernetes Project Layout](https://github.com/kubernetes/kubernetes)

---

## 📊 Estadísticas

- **Archivos creados**: 3
- **Archivos modificados**: 1
- **Decisiones documentadas**: 1 (ADR-001)
- **Bounded contexts definidos**: 6 (stream-capture, worker-lifecycle, framebus, control-plane, event-emitter, core)

---

**Cierre de Sesión**: ✅ ADRs creados, CLAUDE.md actualizado, próximos pasos claros

**Próxima sesión**: Crear scaffold de `modules/` y atacar Sprint 1.1 🚀
