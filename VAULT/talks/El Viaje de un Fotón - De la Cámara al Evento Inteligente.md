# El Viaje de un Fotón: De la Cámara al Evento Inteligente

### Introducción: Dos Mentes, Una Misión

En el mundo de los sistemas inteligentes, transformar la información visual cruda —lo que una cámara ve— en conocimiento útil y accionable es un desafío complejo. Este documento desglosa, paso a paso, cómo un sistema avanzado logra esta hazaña. Para ello, seguiremos el viaje de los datos a través de dos componentes principales, dos "mentes" que trabajan en equipo para una sola misión: el cuidado inteligente.

Los protagonistas de esta historia son **Orion** y **Sala**. La mejor manera de entender su relación es con una analogía médica. Imagina un equipo de radiología:

- **Orion** es el técnico experto que opera la máquina de rayos X. Su trabajo es obtener la imagen más clara posible y describir objetivamente lo que ve en ella: "Veo una sombra en el pulmón izquierdo que mide 2cm de diámetro". No especula, no diagnostica; simplemente reporta los hechos observables.
- **Sala** es el médico radiólogo especialista. Toma el informe objetivo del técnico y lo combina con su conocimiento y el historial del paciente para llegar a una conclusión: "Basado en la forma, densidad y ubicación de esta sombra, es un nódulo que requiere seguimiento". Sala interpreta los hechos para generar un diagnóstico.

Esta separación de responsabilidades es la piedra angular de todo el sistema.

**Principio Clave:** Orion ve, no interpreta. Sala interpreta los hechos que Orion reporta.

--------------------------------------------------------------------------------

## 1. El Observador: ¿Qué es y Cómo Funciona Orion?

El viaje de los datos comienza con **Orion**, el "sensor inteligente" del sistema. Orion es un servicio que actúa como los ojos de la operación, observando constantemente el entorno a través de una cámara. Su trabajo no es entender el _significado_ de lo que ocurre, sino describirlo con una precisión impecable.

Las responsabilidades principales de Orion se pueden resumir en tres tareas clave:

- **Procesar video:** Se conecta directamente a una cámara (generalmente a través de un protocolo llamado RTSP) y analiza el flujo de video en tiempo real, cuadro por cuadro.
- **Ejecutar modelos de IA:** Utiliza "cerebros" de inteligencia artificial pre-entrenados para identificar elementos como personas y sus posturas, y analizar su interacción con **regiones de interés (ROIs)** predefinidas en la escena.
- **Reportar hechos:** Una vez que ha identificado algo, su trabajo final es empaquetar estas observaciones en mensajes estructurados y objetivos. Estos mensajes, llamados "inferencias", son los hechos crudos que enviará para que otros los interpreten.

El trabajo de Orion termina en el momento en que envía su reporte. No sabe si una postura es peligrosa o si un movimiento es preocupante; simplemente informa: "He detectado una persona con estas coordenadas y esta postura". Para que este informe llegue a su destino, necesita un canal de comunicación y un lenguaje común.

--------------------------------------------------------------------------------

## 2. El Mensaje: Hechos Crudos Enviados por MQTT

Para que Sala pueda entender a Orion, ambos deben hablar el mismo idioma y utilizar el mismo "servicio de mensajería". Este servicio es un protocolo llamado **MQTT**, diseñado para ser extremadamente eficiente y confiable. Desde una perspectiva de arquitectura, su modelo de publicación-suscripción es fundamental porque **desacopla** a los componentes: Orion publica sus hallazgos sin necesidad de saber quién los escucha, y Sala se suscribe a los hechos que le interesan sin conocer qué instancia específica de Orion los está produciendo.

En MQTT, los mensajes se publican en "canales" o "topics", que funcionan como direcciones postales. La estructura de estos topics es jerárquica y descriptiva, lo que permite saber de inmediato quién habla y sobre qué. Por ejemplo, un topic como `care/inferences/orion-nuc-001/person_detection` nos dice:

- `**care/inferences**`: Define el espacio de nombres. Todos los mensajes dentro de `care` pertenecen al dominio del cuidado, y `inferences` agrupa los reportes de hechos de los sensores.
- `**orion-nuc-001**`: El emisor es la instancia de Orion identificada como "nuc-001".
- `**person_detection**`: El tema del mensaje es la detección de una persona.

Orion emite diferentes tipos de mensajes (inferencias) en diferentes topics, cada uno diseñado para responder una pregunta muy específica sobre la escena. Los cuatro tipos más importantes para nuestros casos de uso se resumen en la siguiente tabla:

|   |   |   |
|---|---|---|
|Tipo de Inferencia|Pregunta que Responde|Ejemplo de Uso para Sala|
|`person_detection`|¿Hay alguien en la escena y en qué región (ROI)?|Saber si una persona está muy cerca del borde de la cama (`roi_overlap`).|
|`pose_keypoints`|¿Cuál es la postura exacta de la persona?|Calcular ángulos del torso para confirmar si alguien está realmente sentado.|
|`flow_roi`|¿Cuánto movimiento hay en áreas específicas?|Detectar movimientos sutiles en la zona de la cabeza (`microawakes`).|
|`multi_person`|¿Hay más de una persona presente?|Identificar la presencia de un cuidador para silenciar alertas temporalmente.|

Ahora que entendemos el formato y el contenido de los mensajes que Orion envía, es el turno de ver cómo el cerebro del sistema, Sala, los recibe y les da sentido.

--------------------------------------------------------------------------------

## 3. El Intérprete: ¿Qué Hace Sala con los Hechos?

Si Orion son los ojos, **Sala** es el "cerebro experto" del sistema. Su función principal es **escuchar** atentamente los canales de MQTT a los que Orion publica sus inferencias.

Al recibir un mensaje, lo primero que hace Sala es un riguroso control de calidad para garantizar la robustez del sistema. Valida que el mensaje tenga el formato correcto, que no sea demasiado antiguo, y que provenga de un sensor (`instance_id`) conocido y asignado a una habitación (`room_id`) válida.

Pero la verdadera magia de Sala no reside en procesar un único mensaje, sino en su capacidad para tejer una narrativa a partir de múltiples hechos a lo largo del tiempo.

La misión de Sala no es solo escuchar, sino **sintetizar**. Combina múltiples hechos a lo largo del tiempo para entender la historia completa de lo que está sucediendo en la habitación.

Un solo mensaje de Orion es una fotografía instantánea. Una secuencia de mensajes es una película. Sala es el director que ve la película completa y entiende la trama. La mejor manera de comprender este proceso de síntesis es a través de ejemplos concretos.

--------------------------------------------------------------------------------

## 4. La Síntesis: De Hechos Aislados a Eventos Significativos

Aquí es donde la separación de responsabilidades brilla. Orion proporciona las piezas del rompecabezas, y Sala las ensambla para ver la imagen completa.

### Caso de Uso 1: Detectando un "Microawake"

- **Objetivo:** Detectar cuando un residente dormido se mueve ligeramente en la cama (por ejemplo, solo la cabeza), lo que podría indicar una interrupción del sueño.

Sala logra esto combinando información de uno o más tipos de inferencia para construir confianza en su conclusión:

1. **Observa el Movimiento:** Sala está escuchando el topic `flow_roi`. De repente, recibe un mensaje que indica un aumento de la "energía" de movimiento en la región de interés (ROI) definida sobre la cabecera de la cama (`BED_HEAD.energy > 0.15`). Este es el primer hecho.
2. **Busca Estabilidad:** Este movimiento por sí solo no significa mucho. Sala revisa simultáneamente, en los mismos mensajes de `flow_roi`, la energía de movimiento en la región del cuerpo (`BED_BODY.energy < 0.05`). Este segundo hecho confirma que el tronco del residente permanece quieto.
3. **Añade Evidencia (Opcional):** Para una mayor certeza, si este patrón de movimiento activó una inferencia de mayor calidad, Sala puede buscar un tercer hecho en el topic `pose_keypoints`. Un mensaje que confirme una rotación significativa de la cabeza (`head_orientation_yaw > 12°`) refuerza la hipótesis.
4. **Concluye y Actúa:** Al observar que el patrón de movimiento aislado en la cabeza persiste durante unos segundos (p. ej., 3 segundos), Sala tiene suficiente evidencia correlacionada para interpretar la situación. Emite un evento de dominio significativo: **"Microawake detectado"**.

### Caso de Uso 2: Confirmando "Sentado al Borde de la Cama"

- **Objetivo:** Confirmar con alta fiabilidad que un residente está sentado al borde de la cama, un evento que puede preceder a una caída y requiere atención.

Para un evento tan crítico, Sala requiere múltiples piezas de evidencia de diferentes tipos de inferencias, creando una cadena de confirmación lógica:

1. **Primera Pista (Detección de Persona):** Sala recibe un mensaje en el topic `person_detection`. El dato clave es que la caja que delimita a la persona tiene una alta superposición con el ROI del borde de la cama (`roi_overlap.BED_RIGHT_EDGE > 0.5`). Esto es una sospecha inicial.
2. **Segunda Pista (Postura):** Casi de inmediato, llega un mensaje más detallado en `pose_keypoints`. Este hecho es mucho más rico: informa que el ángulo del torso es de 62 grados (`torso_angle_deg=62`) y que la cadera está a solo 11 cm del borde (`hip_to_edge_cm=11`). La evidencia se fortalece.
3. **Confirmación Final (Persistencia):** Sala no se precipita. Espera una fracción de segundo (1.2 segundos en este caso) y recibe un _segundo_ mensaje de `pose_keypoints` que confirma que la postura se mantiene estable. La sospecha se ha convertido en una certeza.
4. **Concluye y Actúa:** Con tres piezas de evidencia consistentes y complementarias, Sala tiene la confianza necesaria para interpretar la escena. Emite el evento de dominio de alta criticidad: **"Edge of Bed Confirmed"**.

--------------------------------------------------------------------------------

## Conclusión: Una Alianza Perfecta para un Cuidado Inteligente

El viaje de los datos, desde un simple fotón que impacta el sensor de la cámara hasta un evento de dominio procesable, es posible gracias a una arquitectura elegante basada en la especialización.

El flujo es una historia cohesiva: **Orion**, el observador objetivo, traduce la realidad visual en hechos crudos y estructurados. Estos hechos viajan a través de **MQTT**, un sistema de mensajería rápido y fiable. Finalmente, **Sala**, el intérprete experto, recibe estos hechos, los valida, los correlaciona en el tiempo y los sintetiza para comprender el significado profundo de la escena.

Esta separación de responsabilidades es crucial. Permite que el sistema sea, a la vez, **robusto** y **flexible**. Es robusto porque sus decisiones se basan en hechos objetivos y verificables reportados por Orion. Y es flexible porque toda la inteligencia interpretativa reside en Sala, cuyo "conocimiento" puede ser actualizado y mejorado sin necesidad de modificar el sensor (Orion) que le proporciona la información. Esta división del trabajo crea un sistema **desacoplado, mantenible y escalable**, donde la inteligencia de negocio puede evolucionar en Sala sin alterar los sensores de campo, un principio fundamental para cualquier plataforma de cuidado inteligente robusta.