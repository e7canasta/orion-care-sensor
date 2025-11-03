# 📋 Backlog - Orion 2.0 Evolution

**GitHub Project**: https://github.com/users/e7canasta/projects/7  
**Repository**: https://github.com/e7canasta/orion-care-sensor

---

## 🎯 Filosofía del Backlog

> **"De menos a más. Llevar de a poco pieza a pieza. Diseño paso a paso."**  
> — Ernesto

### Principios
1. **Incremental**: Cada sprint deployable y testeable
2. **Evolutivo**: Diseño emerge de feedback
3. **Domain-Driven**: Bounded contexts claros
4. **Blues Style**: Conocer escalas, improvisar con contexto

---

## 🗺️ Roadmap Visual

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ORION 2.0 EVOLUTION                          │
└─────────────────────────────────────────────────────────────────────┘

 FASE 1: FOUNDATION           FASE 2: SCALE            FASE 3: INTELLIGENCE
 v1.0 → v1.5                  v1.5 → v2.0              v2.0 → v3.0
 ├─ Sprint 1.1 ✓              ├─ Sprint 4.1            ├─ Sprint 5.1
 ├─ Sprint 1.2 ⬜             ├─ Sprint 4.2            └─ Sprint 5.2
 ├─ Sprint 2   ⬜             └─────────────            
 └─ Sprint 3   ⬜                                      
    
 🎯 Due: 2025-01-31           🎯 Due: 2025-03-31       🎯 Due: 2025-06-30
```

---

## 📊 Milestones

| Milestone | Objetivo | Issues | Due Date | Status |
|---|---|---|---|---|
| **v1.5 - Foundation** | Bounded contexts, single-stream, hot-reload | 4 | 2025-01-31 | 🔄 In Progress |
| **v2.0 - Scale** | Multi-stream (4-8 rooms), resource mgmt | 1 | 2025-03-31 | 📅 Planned |
| **v3.0 - Intelligence** | Cell orchestration, motion pooling | 1 | 2025-06-30 | 📅 Planned |

---

## 🏗️ Estructura del Backlog

```
BACKLOG/
├── README.md ..................... Este archivo (overview)
├── FASE_1_FOUNDATION.md .......... Sprints 1.1, 1.2, 2, 3
├── FASE_2_SCALE.md ............... Sprints 4.1, 4.2
├── FASE_3_INTELLIGENCE.md ........ Sprints 5.1, 5.2
└── TEMPLATES/ .................... Templates para nuevos items
    ├── sprint_template.md
    └── issue_template.md
```

---

## 🔗 Integración con GitHub

Cada archivo de fase contiene:
- Link a issues de GitHub
- Bounded contexts claros
- Acceptance criteria
- Referencias a C4 Model y docs

**Workflow:**
1. Leer `BACKLOG/FASE_X.md` para entender sprint
2. Trabajar en código siguiendo bounded contexts
3. Actualizar issue en GitHub al completar
4. Actualizar `BACKLOG/FASE_X.md` con lecciones aprendidas

---

## 📚 Referencias

- [Plan Evolutivo](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md) - Documento maestro
- [C4 Model](../docs/DESIGN/C4_MODEL.md) - Arquitectura completa
- [CLAUDE.md](../CLAUDE.md) - Guía de desarrollo AI-assisted

---

**Última actualización**: 2025-11-03  
**Autor**: Ernesto + Gaby (AI Companion)
