# FrameBus - Architectural Summary

## Executive Summary

FrameBus: Non-blocking frame distribution con **separación estricta API pública/implementación interna** (Amazon-style).

## Principio Core

> **"Public API = Permanent Contract. Internal = Free Evolution."**

**Arquitectura:**
```
framebus/
├── api.go          → Public (type aliases)
├── framebus.go     → Factory
└── internal/bus/   → Implementation (can swap freely)
```

## Beneficios

✅ **v1.1**: Swap Mutex → RingBuffer sin romper consumers  
✅ **Contract tests**: 100% coverage en API pública  
✅ **Semver**: v1.x = internal, v2.0 = breaking API  

## Referencias

- [ADR-001: Internal/Public API Boundary](docs/ADR-001-internal-public-api.md)
- [Implementation](IMPLEMENTATION.md)
- [README](README.md)

Co-authored-by: Gaby de Visiona <noreply@visiona.app>
