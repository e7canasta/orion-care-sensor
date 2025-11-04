# Architecture Decision Records (ADR)

**Propósito**: Documentar decisiones arquitectónicas significativas con contexto, rationale, y consecuencias.

---

## 📚 Índice de ADRs

| ADR | Título | Fecha | Estado | Sprint |
|-----|--------|-------|--------|--------|
| [001](001-multi-module-monorepo-layout.md) | Multi-Module Monorepo Layout | 2025-11-03 | ✅ Aprobado | Sprint 1.1 |

---

## 🎯 ¿Qué es un ADR?

Un **Architecture Decision Record** documenta:
1. **Contexto**: ¿Qué problema estamos resolviendo?
2. **Opciones**: ¿Qué alternativas evaluamos?
3. **Decisión**: ¿Qué elegimos y por qué?
4. **Consecuencias**: ¿Qué trade-offs aceptamos?

---

## 📝 Template ADR

```markdown
# ADR-{numero}: {Título}

**Fecha**: YYYY-MM-DD
**Estado**: 🔄 Propuesta | ✅ Aprobado | ❌ Rechazado | 🗄️ Superseded
**Autores**: Ernesto + Gaby
**Contexto**: Sprint X

---

## 📋 Contexto y Problema

[Descripción del problema]

### Opciones Evaluadas

1. Opción A
2. Opción B
3. Opción C

---

## 🎯 Decisión

[Qué elegimos]

---

## 💡 Rationale

[Por qué elegimos esto]

---

## 🎸 Consecuencias

### Positivas ✅
[Beneficios]

### Negativas ⚠️
[Trade-offs]

### Mitigations 🛡️
[Cómo mitigamos las negativas]

---

## 🔗 Referencias

- [Links a docs relacionadas]

---

**Estado**: [Estado actual]
**Próximo paso**: [Qué sigue]
```

---

## 🚀 Cuándo Crear un ADR

**SÍ crear ADR cuando**:
- ✅ Cambio arquitectónico significativo
- ✅ Elección entre múltiples opciones con trade-offs
- ✅ Decisión que afecta múltiples bounded contexts
- ✅ Breaking change en public API
- ✅ Cambio de tecnología (ej: MQTT → gRPC)

**NO crear ADR cuando**:
- ❌ Bug fix sin cambio arquitectónico
- ❌ Refactor interno sin cambio de API
- ❌ Cambio trivial de configuración
- ❌ Implementación obvia sin alternativas

---

## 📁 Ubicación

```
docs/DESIGN/ADR/
├── README.md                          # Este archivo (índice)
├── 001-multi-module-monorepo-layout.md
├── 002-worker-ipc-protocol.md        # Futuro
└── 003-hot-reload-mechanism.md       # Futuro
```

**También puede haber ADRs por módulo**:
```
modules/stream-capture/docs/
└── ADR/
    └── 001-gstreamer-vs-ffmpeg.md
```

---

## 🎸 Filosofía

> **"Un ADR bien escrito vale más que mil líneas de código para entender por qué."**

Los ADRs son **memoria técnica viva**. Capturan el **contexto histórico** de decisiones para:
- Futuras sesiones de pair programming
- Onboarding de nuevos devs
- Evitar repetir debates ya resueltos
- Justificar cambios en code reviews

---

**Última actualización**: 2025-11-03
**Autor**: Ernesto + Gaby (AI Companion)
