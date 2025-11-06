# Visual Map (Mapa Mental de Meta-Patterns)

**Propósito**: Síntesis visual (hemisferio espacial) - acceso rápido a conceptos

**Uso**: Quick orientation (dónde estoy, qué pattern aplicar)

---

## Mapa Completo (Vista Aérea)

```
┌───────────────────────────────────────────────────────────────────────────┐
│                                                                           │
│                    META-PATTERNS DE ARQUITECTURA                          │
│                         (Visiona Vocabulary)                              │
│                                                                           │
└───────────────────────────────────────────────────────────────────────────┘


 ╔═══════════════════════════════════════════════════════════════════════╗
 ║                    MOVILIDAD FUTURA (Evolution)                       ║
 ╚═══════════════════════════════════════════════════════════════════════╝

        ♟️ Rey No Ahogado                    🎸 Blues con Guardrails
        (Chess King)                         (Jazz with Structure)
              │                                       │
              ├─ Test 1: ¿Multi-X?                   ├─ Test 1: ¿Guardrails?
              ├─ Test 2: ¿Movimientos?               ├─ Test 2: ¿3 Ojos?
              └─ Test 3: ¿Cambios <10%?              └─ Test 3: ¿PROPOSAL?
                                                            │
                    │                                      │
                    └──────────────┬───────────────────────┘
                                   │
                         📋 Forecast Arquitectónico
                           (PROPOSAL Lifecycle)
                                   │
                      Discovery → Forecast → ADR
                                   │
                         (Knowledge Preserved)


 ╔═══════════════════════════════════════════════════════════════════════╗
 ║                    BOUNDED CONTEXTS (Modules)                         ║
 ╚═══════════════════════════════════════════════════════════════════════╝

     📦 Bounded Context                 🔧 Unix Philosophy
     (SRP Arquitectónico)                (Pipe + Tee)
              │                                 │
              ├─ Test: ¿1 motivo?              ├─ Test: ¿Pipe 1→N?
              ├─ Test: ¿Independiente?         ├─ Test: ¿Tee N→1?
              └─ Test: ¿Testeable?             └─ Test: ¿Composable?
                                                      │
                                           Composition > Monolith


 ╔═══════════════════════════════════════════════════════════════════════╗
 ║                    OPTIMIZATION (Performance)                         ║
 ╚═══════════════════════════════════════════════════════════════════════╝

      ⚡ Physical Invariant             📊 Threshold from Business
      (Trust the Physics)               (POC-Expansion-Full)
              │                                 │
              └─ Ratio < 0.01                   └─ Threshold ≈ 0.6×BE
                 (Latency << Interval)             (Safety Margin)
                      │                                 │
                Fire-and-Forget                   Batching Activado
                 (No wg.Wait)                      (Cuando N > 8)


 ╔═══════════════════════════════════════════════════════════════════════╗
 ║                    BALANCE (Multi-Dimensional)                        ║
 ╚═══════════════════════════════════════════════════════════════════════╝

                          👁️👁️👁️ Los Tres Ojos
                      (Product-Architecture-Code)
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
              Ojo 1: Producto  Ojo 2: Arquitectura  Ojo 3: Código
              (Norte)          (Rieles)             (Transporte)
                    │              │              │
            Value Proposition  Movilidad      Implementación
            Business Need      Bounded Contexts  Performance
            Cómo Crece        Evolution Paths    YAGNI


 ╔═══════════════════════════════════════════════════════════════════════╗
 ║                    ATTENTION (Focus)                                  ║
 ╚═══════════════════════════════════════════════════════════════════════╝

                          👁️ Ojo de Sauron
                        (All-Seeing, Single Focus)
                                   │
                    Ve TODO pero enfoca en UNO
                                   │
                         (Fall detection: persona
                          en contexto de sala)


 ╔═══════════════════════════════════════════════════════════════════════╗
 ║                    ITERATION (MVP)                                    ║
 ╚═══════════════════════════════════════════════════════════════════════╝

                      🛹 Patineta → 🚲 Bici → 🚗 Auto → ✈️ Avión
                          (Funcional en Cada Fase)
                                   │
                         r1.0  →  r2.0  →  r3.0  →  r4.0
                       (Cada release FUNCIONA, valor inmediato)
```

---

## Relaciones Entre Patterns (Conexiones)

```
Rey No Ahogado ←──────────────┐
     │                         │
     │ (valida)                │ (alimenta)
     ▼                         │
Los Tres Ojos                  │
     │                         │
     │ (guía)                  │
     ▼                         │
Blues con Guardrails           │
     │                         │
     │ (genera)                │
     ▼                         │
Forecast Arquitectónico ───────┘
     │
     │ (documenta)
     ▼
PROPOSAL (r2.0, r3.0)
     │
     │ (cuando implementado)
     ▼
ADR (implementado)


Bounded Context ←───┐
     │              │ (refuerza)
     │ (habilita)   │
     ▼              │
Unix Philosophy ────┘
     │
     │ (ejemplos)
     ▼
Pipe + Tee (framesupplier, frame-buffer)


Physical Invariant ──┐
     │               │ (justifican)
     │ (optimiza)    │
     ▼               │
Fire-and-Forget      │
                     │
Threshold Business ──┘
     │
     │ (optimiza)
     ▼
Batching con Guardrails
```

---

## Decision Tree (¿Qué Pattern Aplicar?)

```
┌─────────────────────────────────────────┐
│ ¿Estoy diseñando módulo nuevo?          │
└────────────┬────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
   ¿Core?       ¿Feature?
      │             │
      ▼             ▼
Rey No Ahogado   Bounded Context
      │             │
      ├─ Multi-X?   ├─ 1 motivo?
      ├─ Movimientos│ ├─ Independiente?
      └─ <10%?      └─ Testeable?


┌─────────────────────────────────────────┐
│ ¿Emergieron insights r2.0?              │
└────────────┬────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
 ¿Pedido?       ¿No pedido?
      │             │
      ▼             ▼
   ADR          PROPOSAL
(implementar)  (forecast)
      │             │
      │             ├─ Guardrails?
      │             ├─ 3 Ojos?
      │             └─ Tests pasan?


┌─────────────────────────────────────────┐
│ ¿Optimizando performance?               │
└────────────┬────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
 ¿Latencias?    ¿Threshold?
      │             │
      ▼             ▼
Physical         Threshold
Invariant        from Business
      │             │
Ratio <0.01    Threshold ≈ 0.6×BE
      │             │
Fire-forget    Batching activado


┌─────────────────────────────────────────┐
│ ¿Separar módulo o integrar?             │
└────────────┬────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
  ¿Pipe?        ¿Tee?
  (1→N)         (N→1)
      │             │
      ▼             ▼
framesupplier   frame-buffer
(distribution)  (multiplexing)
      │             │
      └─────┬───────┘
            │
       Composición
```

---

## Cheatsheet (1 Línea por Pattern)

```
♟️  Rey No Ahogado:       r1.0 NO cierra r2.0 (movilidad preservada)
🎸 Blues con Guardrails:  Forecast r2.0 sin violar YAGNI (seniority + guardrails)
👁️👁️👁️ Los Tres Ojos:       Producto (norte) + Arquitectura (rieles) + Código (transporte)
👁️  Ojo de Sauron:        Foco único en región crítica, contexto global
📦 Bounded Context:       Módulo = 1 responsabilidad (SRP arquitectónico)
🔧 Unix Philosophy:       Do One Thing Well, Compose for Complexity
⚡ Physical Invariant:    Latency << Interval → Orden por física (no sync)
📊 Threshold Business:    Threshold ≈ 0.6×BE (guardrails de fases)
📋 Forecast Arquitect:    Discovery → PROPOSAL → ADR (knowledge preserved)
🛹 Patineta → Avión:      MVP funcional en cada fase (no piezas sueltas)
```

---

## Anexos (Full Docs)

```
ANNEX-001: Thinking in Chains (Rey No Ahogado)
ANNEX-002: Bounded Contexts (SRP Arquitectónico)
ANNEX-003: Physical Invariants (Trust the Physics)
ANNEX-004: Batching with Guardrails (Threshold Business)
ANNEX-005: Forecast Arquitectónico (Blues + 3 Ojos + PROPOSAL)
ANNEX-006: Unix Philosophy (Pipe + Tee + Composability)
```

**Ver**: [PATTERN_CATALOG.md](./PATTERN_CATALOG.md) para definiciones completas con aka.

---

## Navegación Rápida

**Por Fase de Discovery**:
- Pre-Discovery: Los Tres Ojos (producto, arquitectura, código)
- Point Silla: Rey No Ahogado (tests mentales)
- Discovery: Blues con Guardrails (forecast con límites)
- Crystallization: Forecast Arquitectónico (PROPOSAL o ADR)

**Por Tipo de Decisión**:
- Módulo nuevo: Bounded Context (SRP) + Unix Philosophy (Pipe/Tee)
- Optimization: Physical Invariant (ratio) + Threshold Business (fases)
- Evolution: Rey No Ahogado (movilidad) + Forecast (PROPOSAL)

**Por Problema**:
- "¿r1.0 cierra r2.0?" → Rey No Ahogado
- "¿Es sobre-diseño?" → Blues con Guardrails (guardrails presentes?)
- "¿Perdemos vista producto?" → Los Tres Ojos (balancear 3)
- "¿Módulo separado?" → Unix Philosophy (Pipe/Tee test)
- "¿Necesito wg.Wait()?" → Physical Invariant (ratio test)
- "¿Cuándo activar optimización?" → Threshold Business (fases)

---

**Versión**: 1.0
**Autor**: Synthesis session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (mapa vivo, evoluciona con patterns)
