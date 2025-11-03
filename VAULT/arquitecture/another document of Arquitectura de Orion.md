Perfecto 💪 Aquí tenés la **traducción completa al español** del documento `ARCHITECTURE.md` que subiste. La mantuve fiel al contenido técnico original, pero con lenguaje natural y términos técnicos en su forma más clara posible en español.

---

# Documentación de Arquitectura de Orion

**Vistas Arquitectónicas 4+1 orientadas a Agentes**

> **Propósito**: Este documento ofrece una descripción arquitectónica integral de **Orion**, diseñada para que agentes de IA y desarrolladores comprendan rápidamente la estructura del sistema, sus patrones y la ubicación de los componentes sin necesidad de recorrer el código en detalle.

---

## Descripción General del Sistema

**Orion** es un servicio de inferencia en tiempo real para monitoreo de pacientes mediante visión computarizada. Está construido como una **tubería de streaming** con **control basado en eventos** y **comunicación entre procesos** en Go y Python.

### Patrón Arquitectónico Central

```
Canal de Streaming con Distribución en Paralelo (Fan-Out)

RTSP/Mock Stream → FrameBus (Fan-Out) → Workers (Paralelos) → MQTT Emitter
                         ↓
                   Plano de Control (Comandos MQTT)
```

### Características Clave

- **Pipeline de streaming**: Procesamiento continuo de frames con tasa de inferencia configurable.
    
- **Distribución fan-out**: Un solo stream se distribuye a múltiples workers en paralelo.
    
- **Canales no bloqueantes**: Política de descarte (drop) evita bloqueos en la tubería.
    
- **Comunicación entre procesos (IPC)**: Integración Go–Python mediante JSON sobre stdin/stdout.
    
- **Control basado en eventos**: Comandos remotos por MQTT con capacidad de recarga en caliente.
    
- **Degradación gradual**: Auto-recuperación de workers y monitoreo de salud.
    

---

## 1. Vista Lógica: Componentes y Responsabilidades

Describe los módulos principales:

- **Orion Orchestrator**: ciclo de vida del servicio, arranque, apagado, recuperación.
    
- **StreamProvider**: fuente de video (RTSP real o mock).
    
- **FrameBus**: distribuye cada frame a todos los workers.
    
- **InferenceWorkers**: ejecutan inferencias (Python o broadcast).
    
- **Control Handler**: recibe comandos MQTT.
    
- **MQTT Emitter**: publica inferencias y estado.
    
- **Config Layer**: carga y validación de configuración.
    
- **Watchdog**: supervisa workers y reinicia si se cuelgan.
    

---

## 2. Vista de Procesos: Concurrencia y Comunicación

### Flujo de procesamiento de frames

1. **Inicialización del stream** (RTSP o Mock) y calentamiento (medición de FPS).
    
2. **Distribución de frames** a todos los workers por `FrameBus`.
    
3. **Procesamiento en Python** vía JSON (stdin/stdout).
    
4. **Publicación de inferencias** por MQTT.
    

### Patrones de concurrencia

- Envíos no bloqueantes (drop si el buffer está lleno).
    
- Canales con política de descarte.
    
- Secuencia estricta de apagado ordenado.
    
- Watchdog que detecta cuelgues y reinicia workers.
    

---

## 3. Vista de Desarrollo: Organización del Código

Estructura de módulos:

- `cmd/`: puntos de entrada (`oriond`, visualizador).
    
- `internal/core/`: orquestación y control del servicio.
    
- `internal/stream/`: decodificación RTSP, mock stream, medición de FPS.
    
- `internal/framebus/`: distribución fan-out.
    
- `internal/worker/`: workers de inferencia (Python).
    
- `internal/broadcast/`: RTMP + overlays.
    
- `internal/emitter/`: publicación MQTT.
    
- `internal/control/`: comandos remotos.
    
- `internal/config/`: carga y validación YAML.
    
- `models/`: scripts Python (ONNX + YOLO11).
    

---

## 4. Vista Física: Despliegue y Ejecución

### Arquitectura de despliegue

- **Dispositivo Edge** (Raspberry Pi o x86):
    
    - Proceso `oriond` (Go)
        
    - Subproceso Python (`person_detector.py`)
        
    - GStreamer para decodificar RTSP.
        
- **Infraestructura de red**:
    
    - Cámara IP (RTSP)
        
    - Broker MQTT (Mosquitto)
        
    - Servidor RTMP (MediaMTX o YouTube Live)
        

### Entorno de ejecución

|Componente|Tecnología|Dependencias|
|---|---|---|
|Servicio principal|Go 1.21+|GStreamer, paho.mqtt|
|Worker Python|Python 3.9+|onnxruntime, numpy, opencv|
|Broker|Mosquitto|localhost:1883 o remoto|

---

## 5. Vista de Escenarios: Casos de Uso Clave

### Escenario 1: Operación normal

Cámara → decodificación → distribución → inferencia → publicación MQTT.

### Escenario 2: Recarga de modelo en caliente

Comando MQTT (`set_model_size`) → Go lo reenvía a Python → recarga del modelo YOLO → respuesta de confirmación.

### Escenario 3: Auto-recuperación del worker

Watchdog detecta inactividad >30s → reinicia proceso Python → reanuda operación.

### Escenario 4: Apagado ordenado

Secuencia: detener workers → detener stream → esperar goroutines → desconectar MQTT.

---

## Decisiones Arquitectónicas

1. **Canales no bloqueantes con política de drop**
    
    - Prioriza latencia sobre completitud.
        
    - Se monitorean los drops.
        
2. **IPC Go–Python por JSON (stdin/stdout)**
    
    - Simplicidad, bajo overhead, aislamiento de procesos.
        
3. **Watchdog simple (KISS)**
    
    - Un intento de reinicio; si falla, requiere intervención manual.
        
4. **Fase de calentamiento (Warm-up)**
    
    - Mide FPS real del stream para ajustar la tasa de inferencia.
        
5. **MQTT para plano de control**
    
    - Ideal para edge devices, asincrónico y NAT-friendly.
        

---

## Dimensiones de Crecimiento

1. **Escalado horizontal**: múltiples workers especializados (detección, caídas, posturas).
    
2. **Escalado vertical**: habilitar GPU (ONNX con CUDA).
    
3. **Procesamiento multi-stream**: múltiples cámaras simultáneas.
    
4. **Despliegue distribuido**: contenedores y Kubernetes.
    
5. **Inferencias avanzadas**: tracking, reconocimiento de actividad, fusión multi-cámara.
    

---

## Resumen Mental

**Orion = pipeline de streaming en 3 etapas:**

1. **Ingesta**: video → frames RGB.
    
2. **Distribución**: fan-out a workers paralelos.
    
3. **Inferencia**: subprocessos Python via JSON IPC.
    

**Invariantes clave:**

- Canales no bloqueantes → evitar bloqueos.
    
- Orden estricto de apagado.
    
- Watchdog con métricas de salud.
    
- Python IPC síncrono (frame → resultado).
    

**Archivos esenciales:**

- `internal/core/orion.go` → orquestador.
    
- `internal/framebus/bus.go` → distribución.
    
- `internal/worker/person_detector_python.go` → IPC Python.
    
- `models/person_detector.py` → inferencia ONNX.
    
- `internal/control/handler.go` → comandos MQTT.
    

---

¿Querés que te prepare una **versión en formato Markdown** ya traducida (para reemplazar el original en GitHub, por ejemplo)? Puedo mantener el mismo formato con los diagramas Mermaid intactos y los títulos en español.


● ¡Listo! He creado ARCHITECTURE.md - un documento arquitectónico completo basado en el modelo 4+1 views con diagramas Mermaid y pensado específicamente para que agentes de IA (como yo) puedan navegar  
 eficientemente sin escanear todo el código.  
  
 Lo que contiene:  
  
 1. Logical View - Componentes y Responsabilidades  
  
 - Diagrama Mermaid de todos los componentes con colores por tipo  
 - Tabla de responsabilidades de cada componente con ubicación exacta en código  
 - Interfaces clave: InferenceWorker, StreamProvider, Publisher  
  
 2. Process View - Concurrencia y Comunicación  
  
 - Sequence diagram del flujo completo: Stream → FrameBus → Workers → MQTT  
 - 4 patrones de concurrencia documentados con diagramas:  
   - Non-blocking frame distribution  
   - Worker input channel with drop policy  
   - Graceful shutdown sequence (orden crítico!)  
   - Worker health watchdog (auto-recovery)  
 - Mapa de goroutines con lifecycle y propósito de cada una  
  
 3. Development View - Organización del Código  
  
 - Estructura de módulos con dependencias  
 - "File Path Quick Reference": tabla de "necesito hacer X → ir a archivo Y:línea Z"  
 - Package dependency graph  
  
 4. Physical View - Deployment y Runtime  
  
 - Diagrama de deployment en edge device (Raspberry Pi / x86)  
 - Diagrama de comunicación entre procesos (Go ↔ Python ↔ MQTT)  
 - Tabla de recursos típicos (CPU, memoria, red)  
  
 5. Scenarios View - Casos de Uso Clave  
  
 Con sequence diagrams para:  
 - Normal operation (steady state)  
 - Hot-reload model size (sin reiniciar servicio)  
 - Worker auto-recovery (watchdog detecta y recupera)  
 - Graceful shutdown (orden de cierre correcto)  
  
 6. Architectural Decisions (ADRs)  
  
 5 decisiones clave documentadas con formato:  
 - AD-1: Non-blocking channels with drop policy  
 - AD-2: Go-Python IPC via JSON (no gRPC)  
 - AD-3: Watchdog KISS strategy (1 retry, then manual)  
 - AD-4: Warm-up phase for FPS measurement  
 - AD-5: MQTT for control plane (not REST)  
  
 Cada ADR incluye: decisión, rationale, trade-offs, alternativas consideradas, dónde se aplica.  
  
 7. Growth Dimensions  
  
 5 ejes de crecimiento con "how-to":  
 - Horizontal scaling (múltiples workers)  
 - Vertical scaling (GPU acceleration)  
 - Multi-stream processing (múltiples cámaras)  
 - Distributed deployment (Kubernetes)  
 - Advanced inference patterns (tracking, activity recognition)  
  
 8. Quick Navigation Cheat Sheet  
  
 Tabla final: "I need to... → Go to file → Line"  
  
 ---  
 ★ Insight ─────────────────────────────────────  
 Este documento sigue el principio de "arquitectura como mapa mental": cada sección responde una pregunta diferente que un desarrollador o agente de IA podría tener. La Logical View responde "¿qué hace  
 cada cosa?", la Process View responde "¿cómo fluyen los datos y la concurrencia?", la Development View responde "¿dónde está el código?", la Physical View responde "¿cómo se despliega?", y los Scenarios  
 muestran "¿cómo se comporta en casos reales?". Los ADRs capturan el "por qué" de decisiones no obvias, evitando que futuros desarrolladores deshagan decisiones deliberadas pensando que son bugs.  
 ─────────────────────────────────────────────────  
  
 Lo más útil para un agente de IA:  
 - Los diagramas Mermaid son parseables y visuales  
 - Las tablas de "File Path Quick Reference" evitan escaneos repetitivos  
 - Los patrones de concurrencia están documentados con ejemplos de código inline  
 - La sección "Mental Model for AI Agents" al final resume todo en 3 párrafos  
  
 Ahora podés referenciar este documento cuando trabajes en nuevas features o cuando otros agentes necesiten contexto del sistema. ¿Querés que agregue algo específico o que profundice en alguna sección?