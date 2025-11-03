
Orión es un servicio de inferencia de IA en tiempo real diseñado para el monitoreo de pacientes mediante visión por computadora. Su arquitectura se fundamenta en un patrón de **canalización de streaming (streaming pipeline)**, con un mecanismo de **control basado en eventos (event-driven control)** para la gestión remota. Se basa en una orquestación principal en **Go** que gestiona _workers_ de inferencia en **Python**, una separación tecnológica clave que influye en muchas de las decisiones de diseño.

Las características arquitectónicas que definen el sistema son las siguientes:

- **Streaming Pipeline**: Procesamiento continuo de fotogramas con tasas de inferencia configurables.
- **Fan-Out Distribution**: Distribución de un único stream de video a múltiples _workers_ (procesos de inferencia) en paralelo.
- **Non-Blocking Channels**: Una política de descarte de fotogramas que previene bloqueos y mantiene la latencia baja.
- **Inter-Process Communication**: Integración entre el orquestador principal (escrito en Go) y los _workers_ de IA (escritos en Python) mediante JSON sobre las tuberías estándar (stdin/stdout).
- **Event-Driven Control**: Control remoto y capacidad de recarga de configuraciones en caliente (_hot-reload_) basado en el protocolo MQTT.
- **Graceful Degradation**: Monitoreo de la salud y recuperación automática de _workers_ para asegurar la robustez del servicio.

Este conjunto de características es el resultado directo de las decisiones de diseño que se analizan en detalle a continuación.

### 3. Análisis Detallado de Decisiones Arquitectónicas Clave (AD)

Esta sección constituye el núcleo de este memorándum. Cada subsección analiza una decisión crítica, detallando su justificación, los compromisos evaluados (_trade-offs_) y las alternativas que fueron consideradas y descartadas. El objetivo es proporcionar una visión transparente y completa del proceso de diseño que ha dado forma a Orión.

#### 3.1. AD-1: Canales No Bloqueantes con Política de Descarte

- **Decisión:** Utilizar canales con búfer de 1 a 5 y un patrón `select { case <- chan: ... default: drop }` para descartar datos (fotogramas) de manera explícita cuando los procesos consumidores están ocupados.
- **Justificación Estratégica:** En un sistema de streaming en tiempo real como Orión, **la latencia es más crítica que la completitud de los datos**. Es preferible descartar fotogramas ocasionalmente para mantener un rendimiento predecible y de baja latencia, en lugar de permitir que un _worker_ lento bloquee toda la canalización. Esta decisión asegura que el sistema siempre procese la información más reciente posible. Críticamente, los fotogramas descartados son rastreados y registrados para la observabilidad, lo que nos permite monitorear la salud del sistema sin sacrificar el rendimiento.
- **Análisis de Trade-offs:**

|   |   |
|---|---|
|Ventajas|Desventajas|
|✅ Evita bloqueos en la canalización|❌ Se pueden perder algunos fotogramas/inferencias|
|✅ Latencia predecible y acotada|❌ Requiere monitoreo de las tasas de descarte|

- **Alternativas Rechazadas:**
    - **Canales bloqueantes:** Rechazado porque un consumidor lento provocaría un "bloqueo de cabecera de línea" (_head-of-line blocking_), deteniendo todo el procesamiento.
    - **Búferes ilimitados:** Rechazado por el alto riesgo de un consumo de memoria ilimitado y la eventual caída del sistema si un consumidor se detiene o ralentiza permanentemente.

#### 3.2. AD-2: Comunicación Inter-Procesos (IPC) Go-Python vía JSON sobre stdin/stdout

- **Decisión:** Implementar la comunicación entre el orquestador de Go y los _workers_ de Python utilizando un subproceso con tuberías estándar (stdin/stdout) y serialización JSON para el intercambio de datos.
- **Justificación Estratégica:** Los factores clave fueron la **simplicidad, el aislamiento de procesos y el rendimiento**. Para la comunicación en la misma máquina, el uso de tuberías es significativamente más rápido que el TCP loopback, aproximándose a una comunicación de copia cero (_zero-copy_) y evitando la complejidad de una pila de red completa (gRPC/HTTP). Además, el aislamiento garantiza que una falla en el subproceso de Python no provoque la caída del orquestador principal de Go.
- **Análisis de Trade-offs:**

|   |   |
|---|---|
|Ventajas|Desventajas|
|✅ Baja latencia (overhead de serialización de ~1ms)|❌ Sin validación de esquema (JSON no es fuertemente tipado)|
|✅ Depuración sencilla (formato de texto)|❌ Overhead de codificación Base64 para fotogramas (~33% aumento)|
|✅ Aislamiento robusto de procesos||

- **Alternativas Rechazadas:**
    - **Memoria compartida:** Rechazada por la complejidad en la gestión del ciclo de vida de los segmentos de memoria y la sincronización.
    - **gRPC:** Rechazado por el overhead de la capa de red y la complejidad adicional de definir y compilar esquemas con Protobuf.
    - **HTTP:** Rechazado por ser excesivamente pesado (_overhead_) para las llamadas de alta frecuencia requeridas en el procesamiento de video.

#### 3.3. AD-3: Auto-Recuperación de Workers con Estrategia "KISS" (Keep It Simple Stupid)

- **Decisión:** Implementar un supervisor (_watchdog_) que intente reiniciar un _worker_ fallido **UNA SOLA VEZ**. Si este reinicio falla, el sistema deja de intentarlo y requiere intervención manual.
- **Justificación Estratégica:** Esta decisión se rige por el principio de simplicidad para prevenir fallos sistémicos. Un fallo en el reinicio probablemente indica un problema subyacente más profundo (ej. un archivo de modelo corrupto, una dependencia faltante) que los reintentos automáticos no resolverían. En lugar de ocultar el problema con bucles de reinicio, es más robusto y seguro alertar a un operador para que investigue la causa raíz.
- **Análisis de Trade-offs:**

|   |   |
|---|---|
|Ventajas|Desventajas|
|✅ Implementación simple y predecible|❌ Requiere intervención manual después del primer fallo|
|✅ Evita bucles de reinicio infinitos||
|✅ Genera señales claras para el operador||

#### 3.4. AD-4: Fase de Calentamiento para Medición de FPS Real

- **Decisión:** Introducir una fase de "calentamiento" de 5 segundos al inicio del sistema. Durante este período, se mide la tasa de fotogramas por segundo (FPS) real del stream de video para ajustar dinámicamente la tasa de inferencia.
- **Justificación Estratégica:** Esta decisión optimiza el uso de recursos al adaptar el sistema a las condiciones reales de la fuente de video. Previene el **sobre-muestreo (CPU desperdiciado)** o el **sub-muestreo (detecciones perdidas)** que ocurrirían si dependiéramos de valores de configuración estáticos. El sistema se auto-ajusta para procesar a una tasa óptima basada en el rendimiento real del stream.
- **Análisis de Trade-offs:**

|   |   |
|---|---|
|Ventajas|Desventajas|
|✅ Limitación de tasa precisa y eficiente|❌ Añade ~5 segundos al tiempo de inicio|
|✅ Se adapta a las características reales del stream||

#### 3.5. AD-5: Uso de MQTT para el Plano de Control

- **Decisión:** Utilizar el protocolo de mensajería pub/sub MQTT para los comandos de control remoto, en lugar de una API RESTful sobre HTTP.
- **Justificación Estratégica:** La elección de MQTT responde a una alineación estratégica con los patrones de despliegue en el borde (_edge_) y del Internet de las Cosas (IoT). Su naturaleza asíncrona, bidireccional y su capacidad para operar detrás de firewalls y NAT (ya que las conexiones son salientes desde el dispositivo) lo convierten en la opción ideal para dispositivos desplegados en redes corporativas o restringidas.
- **Análisis de Trade-offs:**

|   |   |
|---|---|
|Ventajas|Desventajas|
|✅ Protocolo estándar de IoT|❌ Requiere una dependencia de un broker MQTT|
|✅ Funciona detrás de NAT/firewalls||
|✅ Asíncrono y orientado a eventos por diseño||

- **Alternativas Rechazadas:**
    - **REST/HTTP:** Descartado porque su modelo de solicitud-respuesta requeriría que el dispositivo de borde exponga un puerto de entrada para recibir comandos, lo cual es un anti-patrón operativo y de seguridad en redes restringidas.

Estas decisiones, aunque analizadas individualmente, forman colectivamente una arquitectura coherente y estratégica que posiciona al sistema para el éxito.


### 4. Implicaciones Estratégicas y Visión a Futuro

Esta sección sintetiza cómo el conjunto de decisiones arquitectónicas posiciona al sistema Orión para el éxito actual y el crecimiento futuro. Las elecciones de diseño no son aisladas; contribuyen de manera sinérgica a los objetivos generales del sistema.

- **Rendimiento en Tiempo Real:** Las decisiones de usar **canales no bloqueantes (AD-1)** y una **comunicación inter-procesos de baja latencia (AD-2)** son fundamentales para la capacidad del sistema de procesar video en tiempo real, manteniendo la latencia al mínimo.
- **Mantenibilidad y Robustez:** La estrategia de **auto-recuperación simple (AD-3)** y el **aislamiento de procesos (AD-2)** crean un sistema más fácil de operar y depurar. Los fallos están contenidos y son claramente señalados, reduciendo el tiempo medio de reparación (MTTR).
- **Escalabilidad y Flexibilidad:** La arquitectura actual está explícitamente diseñada para crecer en múltiples dimensiones, proporcionando una base sólida para futuras funcionalidades:
    - **Escalado Horizontal:** La adición de nuevos tipos de _workers_ (ej. detección de posturas, reconocimiento facial) está soportada de forma nativa. El patrón de distribución _fan-out_ del `FrameBus` permite que nuevos _workers_ se registren para recibir el mismo stream de video sin necesidad de rediseñar la canalización.
    - **Escalado Vertical:** La abstracción de IPC Go-Python (AD-2) es la clave para la aceleración por hardware. El orquestador de Go es agnóstico al proveedor de inferencia, lo que significa que se puede añadir aceleración por GPU enteramente dentro del proceso de Python, sin requerir cambios en el código de Go.
    - **Procesamiento Multi-Stream:** Aunque actualmente maneja un solo stream, el diseño permite una futura expansión para procesar múltiples cámaras. Esto requerirá cambios menores, como añadir un `stream_id` a los metadatos de los fotogramas y actualizar las métricas de los _workers_ para que sean por stream.
    - **Despliegue Distribuido:** El **diseño sin estado (**_**stateless**_**)** y la configuración a través de archivos YAML son elecciones deliberadas que hacen al sistema inherentemente compatible con plataformas de orquestación de contenedores como Kubernetes.

### 5. Conclusión

La arquitectura del sistema Orión no es un producto accidental, sino el resultado de un conjunto de decisiones deliberadas y sopesadas. Cada decisión fue tomada con una clara comprensión de sus ventajas, desventajas y las alternativas disponibles. La arquitectura resultante equilibra eficazmente los compromisos necesarios para cumplir con los requisitos inmediatos de rendimiento y resiliencia, al tiempo que proporciona una base sólida, flexible y escalable para la evolución futura del sistema y su adaptación a nuevos desafíos.