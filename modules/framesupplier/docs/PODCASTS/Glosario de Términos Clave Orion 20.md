Glosario de Términos Clave: Orion 2.0

Este documento es una guía de referencia rápida diseñada para ayudar a los nuevos miembros del equipo a comprender la terminología, las metáforas y la filosofía de diseño que impulsan el proyecto Orion 2.0. Familiarizarse con estos conceptos es fundamental para tomar decisiones de arquitectura coherentes y efectivas.

1. Metáforas y Filosofía de Diseño ("El Blues")

Para contribuir eficazmente a la arquitectura de Orion 2.0, es esencial comprender primero el "alma" del proyecto: El Blues. Esta sección define nuestra mentalidad de diseño, los principios básicos que responden al porqué detrás de nuestras elecciones técnicas. Entender esta filosofía es el primer paso para poder innovar de manera coherente y pragmática.

1.1. Blues

Es la filosofía de diseño central del proyecto. El "Blues" representa el equilibrio ideal entre una estructura rígida y una improvisación caótica. Es una forma de pragmatismo informado, donde se innova dentro de un conjunto de reglas conocidas (las "escalas"), adaptándose al contexto específico del momento para crear la mejor solución posible.

Pura Estructura → Blues (Ideal) → Pura Improvisación Rigidez → Pragmatismo → Caos

En el contexto de Orion: "Tocar blues" significa tomar decisiones de diseño basadas en el contexto actual, como elegir un mecanismo de apagado (graceful shutdown) más simple porque se ajusta a las restricciones presentes. Esta improvisación es segura precisamente porque está anclada a los "rieles guía" estructurales de los Bounded Contexts y a la "memoria de decisiones" que proporcionan los ADRs.

1.2. Casa de Herrero, cuchillo de acero

Es un principio de consistencia arquitectónica que invierte el dicho popular para significar "practicar lo que se predica". Si un componente o sistema impone una cierta filosofía (por ejemplo, "Just-In-Time"), debe adherirse a esa misma filosofía tanto internamente como en sus interacciones con sus proveedores.

En el contexto de Orion: Este principio fue clave para corregir una "inconsistencia sistémica" en el rediseño del módulo FrameSupplier. Dado que este módulo ofrece una semántica "Just-In-Time" (JIT) de "solo el frame más reciente" a sus consumidores (los workers), se determinó que también debía exigir semántica JIT a su propio proveedor (stream-capture). Este insight evitó que el FrameSupplier procesara frames obsoletos y llevó a la creación de un "buzón de entrada" (inbox mailbox) interno para garantizar la consistencia de punta a punta.

1.3. Óptimo Local

No se considera una trampa técnica o un atajo, sino una elección de diseño pragmática y consciente que representa la mejor solución para el conjunto actual de restricciones y contexto. No es una "chapuza" cuando la razón detrás de la decisión está documentada y se entiende que puede (y debe) evolucionar si el contexto cambia. Esta elección se hace con pleno conocimiento de alternativas futuras más complejas, las cuales se difieren conscientemente hasta que el contexto las exija.

En el contexto de Orion: La discusión sobre el diseño para múltiples streams de video es un ejemplo perfecto. La solución de componer varias instancias de FrameSupplier, un enfoque alineado con la "filosofía Unix" de componer herramientas simples y cohesivas, se identificó como un Óptimo Local válido para manejar una cantidad pequeña de streams. Esta decisión, documentada en un ADR, preserva la pureza del Bounded Context del módulo, con el entendimiento de que podría ser reemplazada por una solución consolidada si futuras restricciones de rendimiento a gran escala lo hicieran necesario.

1.4. Synapse

Una "sinapsis" es una idea o conexión creativa que emerge durante una discusión colaborativa y que no existía previamente en la mente de ninguna persona de forma individual. Representa un momento de genuina co-creación, donde el diálogo entre dos o más personas genera un entendimiento superior a la suma de sus partes.

En el contexto de Orion: Las sinapsis a menudo ocurren en momentos de reflexión ("modo café") después de que se ha tomado una decisión formal. Un ejemplo fue la conexión que surgió entre la complejidad interna del FrameSupplier y la "filosofía Unix" de componer herramientas simples. Esta idea no fue propuesta por una sola persona, sino que nació orgánicamente de la conversación y refinó la comprensión del equipo sobre el diseño del módulo.

2. Prácticas y Patrones de Arquitectura ("Las Escalas")

Si "El Blues" es nuestra filosofía, "Las Escalas" son el andamiaje que la hace posible en la práctica. Esta sección define las herramientas, patrones y artefactos concretos que proporcionan la estructura necesaria para improvisar de manera segura y coherente, permitiéndonos aplicar nuestra mentalidad de diseño de forma sostenible.

2.1. ADR (Architectural Decision Record)

Un Registro de Decisión Arquitectónica (ADR) es un documento conciso que captura una decisión de arquitectura importante, el contexto que la rodeó, las alternativas que se consideraron y las consecuencias de la elección final. Su propósito principal es actuar como la "memoria de decisiones" del proyecto.

En el contexto de Orion: Los ADRs son la herramienta que hace sostenible la filosofía del Óptimo Local. Al documentar el porqué detrás de una decisión (por ejemplo, ADR-003 explicando la lógica detrás del umbral de batching de 8, una decisión que equilibra el rendimiento para un despliegue de prueba de concepto (≤8 workers) con la escalabilidad futura), el equipo puede revisar y evolucionar esa decisión de forma consciente y segura en el futuro cuando el contexto cambie, evitando así la acumulación de deuda técnica por ignorancia.

2.2. Bounded Context (Contexto Delimitado)

Es un principio central de la arquitectura de software que impone una clara separación de responsabilidades. Como principio rector, establece que "Cada módulo debe tener una, y solo una, razón para cambiar". Actúa como un "riel guía" que proporciona la estructura necesaria para innovar de forma segura dentro de límites bien definidos, evitando el scope creep (expansión descontrolada de la funcionalidad).

En el contexto de Orion: El contexto delimitado del módulo FrameSupplier es estrictamente la distribución de frames. Esta frontera clara evita que asuma responsabilidades que no le corresponden, como la gestión del ciclo de vida de los workers o la priorización de tareas, las cuales pertenecen a otros módulos con sus propios contextos delimitados.

2.3. Point Silla (Punto Silla)

Un Punto Silla es un punto de entrada estratégico para una sesión de descubrimiento de diseño. Típicamente, es una pregunta técnica con amplias implicaciones arquitectónicas que se formula intencionadamente para abrir múltiples caminos de exploración en lugar de reducir las opciones a una única solución prematura.

En el contexto de Orion: La pregunta "¿sync.Cond como base?" actuó como el Point Silla para la sesión de diseño del FrameSupplier. Esta simple pregunta inició toda la exploración de semánticas fundamentales como "buzón vs. cola", los trade-offs entre diferentes primitivas de concurrencia en Go y, en última instancia, condujo a la arquitectura final del módulo.
