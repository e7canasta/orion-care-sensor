
# Arquitectura del Sistema Orión: Una Visión 4+1



## 1. Introducción y Filosofía de Diseño

Este documento describe la arquitectura del **Sistema Orión** utilizando el modelo de vistas 4+1. Sirve como la referencia canónica para el desarrollo, mantenimiento y evolución del sistema.

### Filosofía Central: "Orión Ve, No Interpreta"

Orión es un **servicio de inferencia visual, headless y en tiempo real**. Su única responsabilidad es procesar streams de video, ejecutar modelos de IA configurados dinámicamente y emitir datos de inferencia estructurados a través de un contrato bien definido (MQTT).

**Analogía:** Orión es un **radiólogo**. Ve y reporta hechos observables en una imagen (huesos, sombras, anomalías) con alta precisión. NO es un **médico de diagnóstico**; no interpreta esos hechos para determinar una enfermedad o un tratamiento. Orión es un sensor inteligente configurable, no un motor de decisiones de negocio.

---

## 2. Vista Lógica (Logical View)

Esta vista se centra en la funcionalidad que el sistema proporciona a los usuarios (en este caso, el sistema de monitoreo geriátrico que lo consume).

### Diagrama de Componentes Lógicos

```
┌──────────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│   Configuración  ├─────►   Plano de Control ├─────►   Gestor de Ciclo   │
│      (YAML)      │     │      (MQTT)      │     │      de Vida        │
└──────────────────┘     └──────────────────┘     └──────────┬──────────┘
                                                               │
                                     ┌───────────────────────┼───────────────────────┐
                                     │                       │                       │
                                     ▼                       ▼                       ▼
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ Gestor de Stream ├─────►   Pipeline de      ├─────►   ├─────►   Worker de        │
│      (RTSP)      │     │    Frames (Go)     │     │    │     │   Inferencia (Py)  │
└──────────────────┘     └──────────┬─────────┘     └──────────────────┘     └──────────┬─────────┘
                                     │                                                  │
                                     └───────────────────────┬──────────────────────────┘
                                                             │
                                                             ▼
                                                 ┌──────────────────┐
                                                 │  Emisor de Datos │
                                                 │      (MQTT)      │
                                                 └──────────────────┘
```

### Descripción de Componentes:

*   **Plano de Control (Control Plane):** Recibe comandos (pausar, cambiar modelo, etc.) vía MQTT. Es la API de control remoto del sistema.
*   **Gestor de Ciclo de Vida (Lifecycle Manager):** Orquesta el arranque, la supervisión y el apagado de todos los demás componentes.
*   **Gestor de Stream (Stream Manager):** Se conecta a la fuente RTSP, decodifica el video y produce un flujo de fotogramas crudos.
*   **Pipeline de Frames (Frame Pipeline):** Componente central en Go que distribuye los fotogramas a los workers. Implementa la política de descarte para garantizar la baja latencia.
*   **Worker de Inferencia (Inference Worker):** Proceso Python que recibe fotogramas, ejecuta el modelo de IA (ONNX), y devuelve los resultados. Está completamente gestionado por el orquestador de Go.
*   **Emisor de Datos (Data Emitter):** Publica los resultados de la inferencia y las métricas de salud en el bus de datos MQTT, cumpliendo con el contrato de datos de Orión.

---

## 3. Vista de Procesos (Process View)

Esta vista describe la concurrencia y las interacciones entre los procesos en tiempo de ejecución.

### Diagrama de Secuencia de Procesamiento de un Frame

```
           Orion (Go)                                     Worker (Python)
               |                                                  |
[RTSP Stream]--|--> [1. consumeFrames]                             |
               |     Lee frame del stream                          |
               |-------------------------------------------------->|
               |     [2. Distribute] Envía frame vía FrameBus       |
               |         (Canal Go con descarte)                    |
               |                                                  |
               |     [3. sendFrame]                               |
               |     Serializa con MsgPack                          |
               |     Escribe en stdin del subproceso Python --------|--> [4. Lee stdin]
               |                                                  |     Deserializa MsgPack
               |                                                  |     Ejecuta Inferencia ONNX
               |                                                  |     Serializa resultado (MsgPack)
               |<-------------------------------------------------|---- Escribe resultado en stdout
               |     [5. readResults]                               |
               |     Lee y deserializa resultado de stdout          |
               |                                                  |
               |     [6. consumeInferences]                         |
               |     Procesa resultado (feedback a ROI Processor)   |
               |                                                  |
               |--> [7. emitter.Publish]                           |
               |     Publica inferencia en MQTT                     |
               |                                                  |
[MQTT Broker]<--|--------------------------------------------------|
```

### Pseudocódigo de Goroutines Clave

```go
// GOROUTINE 1: consumeFrames (core/consumer.go)
func (o *Orion) consumeFrames(ctx context.Context) {
    for frame := range o.stream.Frames() {
        if o.isPaused() { continue }

        // Lógica de ROI y selección de modelo
        processedFrame := o.roiProcessor.ProcessFrame(frame)

        // Distribución no bloqueante a todos los workers
        o.frameBus.Distribute(ctx, processedFrame)
    }
}

// GOROUTINE 2: processFrames (worker/person_detector_python.go)
func (w *PythonPersonDetector) processFrames() {
    for frame := range w.input { // Canal con buffer y descarte en el envío
        // Envía frame al subproceso Python vía stdin (con MsgPack)
        w.sendFrame(frame)
    }
}

// GOROUTINE 3: consumeInferences (core/consumer.go)
func (o *Orion) consumeInferences(ctx context.Context) {
    for worker := range o.workers {
        go func(w types.InferenceWorker) {
            for inference := range w.Results() {
                // Bucle de retroalimentación para auto-foco
                if roi := inference.SuggestedROI(); roi != nil {
                    o.roiProcessor.UpdateSuggestedROI(roi)
                }
                // Publicar en MQTT
                o.emitter.Publish(inference)
            }
        }(worker)
    }
}
```

---

## 4. Vista de Desarrollo (Development View)

Esta vista describe cómo se organiza el código fuente.

*   **`cmd/`**: Puntos de entrada de los ejecutables (`oriond`, `orion-viz`). El `main.go` aquí es simple, su única función es instanciar y ejecutar el objeto `Orion` de `internal/core`.
*   **`internal/`**: El corazón de la lógica de Orión.
    *   `core/`: Contiene el orquestador `Orion`, el ciclo de vida y los consumidores de alto nivel.
    *   `config/`: Carga y validación de la configuración `orion.yaml`.
    *   `control/`: El manejador del plano de control MQTT, desacoplado del core mediante callbacks.
    *   `stream/`: Proveedores de stream (RTSP, Mock). Lógica de GStreamer encapsulada aquí.
    *   `framebus/`: El distribuidor de fotogramas (fan-out) con política de descarte.
    *   `worker/`: La implementación del worker de Python, incluyendo la gestión del subproceso y la comunicación IPC.
    *   `emitter/`: El cliente MQTT para publicar inferencias y estado.
    *   `types/`: Definiciones de datos clave (Frame, Inference, etc.) usadas en todo el sistema.
*   **`models/`**: Contiene los workers de Python (`person_detector.py`), los modelos `.onnx` y los scripts para ejecutarlos (`run_worker.sh`).
*   **`config/`**: Archivos de configuración por defecto (`orion.yaml`, `go2rtc.yaml`, `mosquitto.conf`).
*   **`Makefile`**: Define los comandos de construcción, test y ejecución (`make run`, `make build`).

---

## 5. Vista Física (Physical View)

Esta vista muestra cómo se despliega el sistema en el hardware.

### Diagrama de Despliegue Típico (Edge)

```
                                     ┌───────────────────┐
                                     │   MQTT Broker     │
                                     │ (Cloud o Local)   │
                                     └─────────▲─────────┘
                                               │
                                               │ MQTT (Control y Datos)
                                               │
┌──────────┐     ┌─────────────────────────────┴─────────────────────────────┐
│  Cámara  │     │                      Host de Borde                        │
│   IP     ├─────┤ (Ej: i7 iGPU OpenVino NUC Local - Arch)                       │
│          │RTSP │                                                           │
└──────────┘     │   ┌───────────────────────────────────────────────────┐   │
                 │   │                 Contenedor Docker                 │   │
                 │   │   ┌────────────────────┐     ┌──────────────────┐ │   │
                 │   │   │  Orquestador Go    │     │  Worker Python   │ │   │
                 │   │   │     (oriond)       │◄───►│ (person_detector)│ │   │
                 │   │   └────────────────────┘ IPC └──────────────────┘ │   │
                 │   │                                                   │   │
                 │   └───────────────────────────────────────────────────┘   │
                 │                                                           │
                 └───────────────────────────────────────────────────────────┘
```

Un despliegue típico consiste en:
1.  Una o más **cámaras IP** que emiten video vía RTSP.
2.  Un **Host de Borde** (un dispositivo como i7 iGpu Open Vino o un servidor) que ejecuta el servicio Orión, generalmente dentro de un contenedor Docker.
3.  El contenedor de Orión contiene el **proceso principal de Go** y uno o más **subprocesos de Python**.
4.  Un **Broker MQTT** (que puede estar en la nube o en la misma red local) al que Orión se conecta para recibir comandos y emitir datos.

---

## 6. Escenarios (Scenarios View)

Esta vista ilustra cómo las otras vistas trabajan juntas para cumplir con los casos de uso clave.

### Escenario 1: Cambio de Modelo de IA en Caliente (Hot-Reload)

1.  **Actor Externo** publica un mensaje en el topic MQTT `care/control/{instance_id}`:
    ```json
    {"command": "set_model_size", "params": {"size": "m"}}
    ```
2.  **Vista de Procesos:** El `Control Handler` (Go) recibe el mensaje.
3.  **Vista Lógica:** El `Control Handler` invoca el callback `OnSetModelSize`.
4.  **Vista de Desarrollo:** La implementación de este callback está en `core/commands.go` (`setModelSize`), que a su vez llama a `worker.SetModelSize("m")`.
5.  **Vista de Procesos:** El `PythonPersonDetector` (Go) escribe un comando de control JSON especial en el `stdin` del subproceso Python.
6.  **Vista de Procesos:** El worker de Python recibe el comando, descarga el modelo `yolo11m` de su memoria y carga el nuevo, sin reiniciar el proceso.
7.  **Resultado:** El sistema ahora usa un modelo más pesado y preciso, con una interrupción de servicio nula o mínima.

### Escenario 2: Un Worker se Cuelga

1.  **Fallo:** El subproceso Python entra en un bucle infinito y deja de procesar fotogramas y leer de `stdin`.
2.  **Vista de Procesos:** El `PythonPersonDetector` (Go) intenta escribir en `stdin` (`sendFrame`). La escritura se bloquea y el `timeout` de 2 segundos se activa. Loguea un error `stdin write timeout`.
3.  **Vista Lógica:** El `watchWorkers` (en `core/orion.go`) se ejecuta. Comprueba la métrica `LastSeenAt` del worker.
4.  **Vista de Desarrollo:** Como el worker no ha emitido inferencias, `LastSeenAt` es antiguo. La condición `time.Since(metrics.LastSeenAt) > minTimeout` se cumple.
5.  **Vista de Procesos:** El `watchdog` llama a `worker.Stop()` (que mata al proceso colgado) y luego a `worker.Start()` (que crea un nuevo subproceso Python).
6.  **Resultado:** El worker se recupera automáticamente. El sistema experimentó una degradación temporal pero no una falla total, cumpliendo con la **Decisión Arquitectónica AD-3 (KISS Auto-Recovery)**.
