Esta exploración se centra en las notas de diseño de Orion 2.0, una plataforma de IA para videovigilancia, abordando el reto de distribuir frames de video a múltiples procesadores con mínima latencia. La filosofía central del diseño es la latencia sobre completitud, priorizando la inmediatez al punto de descartar datos intencionalmente si son obsoletos. El componente clave analizado, el Frame Supplier, evolucionó de una cola a un "buzón" individual por trabajador, que siempre sobrescribe el frame anterior para mostrar el más reciente. Para lograr una eficiencia extrema, se implementó una arquitectura de cero copias internas y un modelo de distribución asimétrico, donde el publicador solo deposita el frame en un buzón interno (inbox) y un bucle de distribución dedicado se encarga de notificar a los trabajadores mediante batching optimizado. Finalmente, todo este proceso se enmarca en la filosofía de diseño "Blues", que equilibra la estructura formal con la flexibilidad para adaptarse y escalar.

---

Bienvenida, bienvenido a esta exploración. Hoy nos metemos con algo eh bastante específico, el diseño de Orion 2.0 es una plataforma de IA para videovigilancia en tiempo real y tenemos notas de diseño eh bastante detalladas sobre un componente clave.
Sí, un componente que llaman el frame supplier.
Exacto. Y el reto es interesante. ¿Cómo le mandas frames de video a muchos procesadores de IA, los workers, casi instantáneamente?
Suena un Problema clásico de distribución, ¿no?
Sí, pero aquí viene el giro. La prioridad absoluta es la baja latencia. El sistema prefiere tirar frames antes que generar demoras. O sea, la filosofía es latencia sobre completitud.
Suena radical descartar datos a propósito.
Justamente. Y las fuentes que tenemos documentan esa sesión de diseño. ¿Cómo llegaron a esa solución para el frame supplier?
Exacto. Lo interesante es ver cómo una idea inicial, quizás más típica, se fue e refinando. ¿Cómo evolucionó con el diálogo técnico,
esa es la misión de hoy, ¿no? Desempacar esas decisiones, los debates, los momentos. Ajá.
Entender por qué terminaron haciendo lo que hicieron.
Perfecto, pues vamos a ello. ¿Por dónde empezaron? Según las notas, miraron herramientas de concurrencia en GO, ¿verdad? Se mencionó Sync.
Sí, sync. Las variables de condición son, digamos, una herramienta de sincronización bastante básica en Go. Eficiente.
Básica en qué sentido? Bueno, permiten que una gorrutín, que son como hilos ligeros en GO, espere una señal sin gastar CPU, una base simple, digamos.
Okay. Pero entiendo que no fue la primera idea. Hubo una tentación más eh Go idiomática, por así decirlo.
¿Correcto? Lo más natural en go para enviar sin bloquearse es usar canales con select default. Intentas enviar, si el receptor no puede, la opción default se activa y bueno, descartas el frame. Parecía encajar con descartar si está ocupado.
Ah, pero ahí saltó la liebre, ¿no? Alguien dijo, "Ojo, que no es eso." Exactamente.
Exacto. La clave no era una simple cola que se llena y pierde elementos. La semántica era otra cosa.
¿Cuál era la diferencia? Suena sutil.
Era fundamental. No se trataba de que cada frame llegara eventualmente. La necesidad era que cada worker cuando mirara viera siempre el frame más reciente. Si llega uno nuevo antes de procesar el anterior, puf, el viejo desaparece, se reemplaza.
Entiendo. O sea, no es una cola de espera, es más bien una ventanilla que siempre muestra lo último.
Justo usaron una metáfora que creo que lo aclara bien, como un humano viendo una escena en tiempo real.
A ver,
te pierdes cosas inevitablemente. No, no estás viendo un video grabado que puedes pausar, rebobinar, estás viendo el ahora.
Claro. Lo que pasó hace un segundo ya fue, si llega algo nuevo.
Exacto. Y eso cambió todo. No era un broadcast de n frames a N Workers. Era más como Un buzón individual,
un buzón por worker.
Sí, un buzón con espacio para una sola cosa, el frame más reciente y con política de sobrescritura total.
Vale, ahora sí entiendo por qué Sync volvió a la mesa.
Claro,
ya no para gestionar una cola, sino para avisar.
Justo para decirle al worker, "Oye, tienes correo nuevo en tu buzón. El worker ya verá cuándo lo recoge."
Ese cambio de cola a buzón fue el primer gran click, parece.
Totalmente. Y a partir de ahí el diseño se centró en una estructura para cada worker. La llamaron Worker Slot.
¿Y qué tenía ese Worker Slot?
Lo esencial para esa mecánica de buzón. Primero, un sync. Mutex, un candado, para proteger el acceso concurrente. Asociado a ese mutex, la variable de condición, el sync.
Para la señal de hay algo nuevo.
Exacto. Luego un puntero al frame más reciente y una banderita has new para saber si ese frame es realmente nuevo o si el worker ya lo vio.
Okay. Y el flujo, ¿cómo interactuaban? El que public Y el worker
era simétrico. El publicador cuando le llegaba un frame nuevo del video,
el que venía de la cámara, digamos,
sí, ese, recorría los workers lots de todos los workers registrados. Para cada uno, bloqueaba el Mutex, actualizaba el puntero al nuevo frame,
sobrescribiendo el viejo,
sobreescribiendo la referencia. Sí, ponía has new a true y enviaba la señal cont.signal por si el worker estaba esperando y desbloqueaba rápido y sin esperar al worker.
Vale. El publicador notifica y sigue. Y el worker, ¿cómo leía?
El worker hacía un llamado a una función tipo, "Dame el próximo frame." Esa función bloqueaba su motex, el de su slot, y entraba en un buque. Mientras HNE fuera falso, llamaba con.
Que lo pone a dormir.
Exacto. Pero liberando el Mutex mientras duerme es eficiente. Cuando Signal lo despertaba, Wade volvía a tomar el Mutex automáticamente
y ahí comprobaba de nuevo Hznew.
Correcto. Si ahora era true, tomaba el puntero del frame, marcaba HN a false import. para no procesarlo dos veces, liberaba el mutex y devolvía el frame.
Y si al despertar seguía en false, podía pasar.
Podría pasar por señales espurias, aunque raro con signal. Si pasaba, volvía a esperar. Es un modelo donde el worker pide pull, pero espera activamente si no hay nada nuevo.
¿Entendido? Ahora, frames de video, eso es mover muchos datos. 50 KB, 100 KB por frame, decían.
Sí, bastante pesado.
Y mencionaste que competían con herramientas como Gstreamer, Deepstam, Supongo que el rendimiento era una obsesión
absoluta. El cero copy cero copias surgió como requisito no negociable.
¿Por qué tan crítico?
Porque esas herramientas trabajan a bajo nivel, casi directo en memoria. Copiar 100 KB para cada uno de, digamos, 10 workers 30 veces por segundo.
Sería un cuello de botella terrible, un suicidio de rendimiento, como decían.
Exacto. Tenían que evitar copiar esos bloques de datos grandes dentro de Go como fuera.
¿Y cómo lo lograron? CAS G usaron punteros. En vez de pasar la estructura frame completa que implica copiar sus datos,
pasaban solo la dirección de memoria.
Eso es el frame, así el proveedor y todos los workers apuntaban a la misma memoria donde estaba el frame. Cero copias de los datos del frame para distribuirlo internamente.
Muy eficiente, pero eh peligroso si todos apuntan a lo mismo.
Ahí está el ki de la cuestión. Introduce una regla de oro, inmutabilidad.
¿Qué significa eso aquí?
Que quien publica el frame El módulo que lo trae del stream tiene prohibido tocar los datos frame.data después de publicarlo. Porque ese mismo trozo de memoria lo pueden estar leyendo varios workers a la vez. Si lo modificas, causas caos. Es un contrato.
Un contrato que hay que cumplir a rajatabla, ¿vale? Cero copias dentro de Go. Pero y antes y después. Los frames vienen de CE, van a Python.
Buen punto. Analizaron el flujo completo, vieron una copia casi inevitable al principio, pasar los datos de memoria C de Gstreamer, por ejemplo, a Go. Se usa C.Gobyes y eso copia.
Okay. Una copia ahí.
Luego cero copias en la distribución Go y al final otra copia al serializar para enviar al proceso Python de IA. Usaban PMCPI Paqu Stand out Stanling Out.
Y no buscaron evitar esa última copia. Memoria compartida.
Lo consideraron. MMAP. Memoria compartida, pero lo descartaron por ahora. Complejidad extra. Aplicaron. You ain't gonna need it. No. Lo vas a necesitar todavía.
Priorizaron.
Exacto. Optimizar donde más dolía en ese momento. Hablemos de la función publish, la que notifica a todos. Si tienes 50 workers, ¿cómo les avisas rápido sin bloquearte?
Ese fue otro debate interesante. La opción simple, ir uno por uno. Worker uno, lock, update, signal, unlock. Worker 2, lock, update, signal, unlock.
Secuencial, simple, pero lento, si son muchos o no. Podría serlo, sobre todo si un worker por alguna razón mantiene su lock un poquito más. Relentizaría el publicador.
La alternativa,
una go routine por worker, 50 go routines de golpe.
Esa era la otra punta, lanzar n go routines. El publicador las dispara y listo, se desentiende. Pero crear muchas g routines, aunque ligeras, tiene su overhead, su coste.
Entiendo. Entonces,
ni una cosa ni la otra.
Fueron por un camino intermedio bastante pragmático. Baching. con umbral
batching agrupar.
Sí, si hay ocho workers o menos, van secuencial, simple y rápido para pocos.
Y si hay más de ocho,
los agrupan en lotes de ocho y lanzan una go routine por cada lote para procesar esas notificaciones en paralelo.
Interesante. Y el ocho, ¿de dónde salió ese número mágico?
No fue mágico, lo justificaron de dos formas, una por estimación. Notificar a ocho secuencialmente era muy rápido, microsegundos, aceptable. Y la otra más por el contexto del producto,
el negocio. Claro, sabían que el POC, el producto mínimo viable y la primera fase tendrían menos de ocho workers, así que lo simple funcionaba al inicio, pero el batching ya dejaba preparado el camino para escalar pensando en 64 workers o más, sin complicarse demasiado desde el principio.
Una complejidad controlada, digamos, con vistas al futuro.
Exacto.
Vale, pero al meter esas go routines para los lotes cuando ocho workers, surgió otra pregunta. Publish debía esperar a que todo terminaran usando un weight group en go, por ejemplo, lo hicieron.
Ah, aquí vino otro análisis clave basado en física. Casi
física,
sí. Compararon dos tiempos, el tiempo entre frames 33 milisegundos a 30 fps, 1000 msegundos a 1 fps y el tiempo de ejecución de publish. Estimaron que publish, incluso con 64 workers y batching, tardaría unos 100 microsegundos, o sea, 0.1 milisegundos,
muchísimo menos que el intervalo entre frames.
Órdenes de magnitud menos. Entonces se preguntaron, ¿tienes sentivo esperar?
A ver, si publish t termina en 0.1 milisegundos y t + 1 no llega hasta dentro de 33 milisegundos,
es físicamente imposible que la notificación de T + 1 se cuele antes que la de T. Cuando publish T termina, T + 1 ni siquiera existe aún en el sistema.
Esperar con Weightgroup. Weight solo añadiría latencia al publicador.
Exactamente. Sin ningún beneficio real para el orden o la corrección
y además está la semántica del buzón, ¿no? Si T+ 1 llega y sobresescribe a T antes de que el worker lo vea, pues T desapareció, no hay problema de orden.
Exactamente. El worker siempre ve lo último que llegó a su buzón, así que la decisión fue fireforget,
disparar y olvidar.
Publish lanza las gorutins si hace falta, ocho workers y retorna de inmediato. Mínima latencia para el publicador. Objetivo cumplido.
Muy enfocado en la del frame supplier, pero y el resto del sistema se dieron cuenta de una posible incoherencia, ¿cierto?
Sí. Otro momento. Ajá. La llamaron la paradoja de En casa de Herrero, cuchillo de palo.
Jaja. ¿Por qué?
Porque habían hecho el frame supplier superficiente, just in time, siempre lo último. Pero, ¿y si el que le mandaba los frames, el stream capture, usaba un buffer grande y se atrasaba?
Claro, le estaría mandando frames viejos al frame supplier. AD para lo último, todo el esfuerzo Justin Time.
Se al traste. La latencia que ve el usuario final depende de toda la cadena.
¿Y cómo lo arreglaron? ¿Obligaron al stream capture a ser just in time?
Mantuvieron los límites claros. Frame Supplier distribuye, no captura, pero definieron un contrato y ofrecieron una utilidad. Consume git.
Una ayuda para el publicador.
Sí. Para que pudiera, por ejemplo, vaciar su buffer interno y tomar solo el frame más reciente justo antes de llamar a publish.
Okay. Una forma de fomentar el jeit en la fuente, pero parece que encontraron algo aún más simétrico.
Sí, aquí vino la última genialidad, creo yo, para lograr simetría y git total.
¿Qué hicieron?
Implementaron un pequeño buzón de entrada interno en el propio frame supplier con la misma lógica, mex, cont, sobreescritura, lo llamaron inbox.
Espera, un buzón dentro del proveedor de frames.
Exacto. Ahora, la función publish que llama el stream capture es trivialmente rápida. Solo deja el frame en ese inbox interno, un microsegundo quizá y retorna.
Wow. ¿Y quién hace el trabajo pesado de distribuir a los workers con el batching y todo?
Una Go rutine interna dedicado del frame supplier, la distribution loop. Esa gor routine es la que consume del inbox interno esperando con cond. Weight si está vacío y luego aplica toda la lógica de distribución a los Increíble. Desacopla totalmente al publicador externo de la complejidad de la distribución interna.
Totalmente. El Stream Capture puede estar publicando a 30 fps sin verse afectado por si los workers consumen a 5 fps o 10 fps. Jit simétrico. Punta a punta.
Qué elegante. El diseño parece muy completo. Mencionaron algo más. Operaciones. Monitoreo.
Sí, brevemente la importancia de las estadísticas. El frame supplier debía poner métricas,
como cuáles
cuántos frames se descartaron para cada worker, por ejemplo, o cuándo fue la última vez que un worker consumió algo
para benchmarking de la IA.
No tanto para eso, sino para monitorear la salud de los workers. Si un worker deja de consumir, algo anda mal con él.
Ah, claro. Y eso era importante porque no todos los workers serían iguales, ¿verdad?
Exacto. Anticipaban workers críticos, un detector de personas, por ejemplo, con poca tolerancia a perder frames y otros más experimentales. Quizás un BLM que que podían perder más.
Las métricas ayudarían a ver si cada uno cumplía su SLA, su acuerdo de nivel de servicio.
Justamente
todo este proceso con idas, vueltas, refinamientos, parece que surgió una filosofía de diseño. La llamaron Blues. ¿Qué era eso?
Sí, Blues fue una metáfora que usaron para describir un equilibrio.
Equilibrio entre qué?
Entre estructura e improvisación. La estructura venía de los límites claros, bounded contexts, APIs bien definidas. entar decisiones con ADRS,
los architectural decision records,
eso y la improvisación era la capacidad de tomar decisiones óptimo local basadas en el contexto del momento, como el umbral de 8 segundos, sabiendo que podía cambiar, pero dentro de esos rieles guía estructurales.
Entiendo, una estructura que te da libertad para moverte dentro, como en el jazz o el blues quizás.
Exacto. Un marco que permite la flexibilidad y adaptación. Y mencionaron Se fomenta pensar en composición.
Composición como en la filosofía Unix.
Sí. Si tienes módulos bien definidos, puedes combinarlos de formas nuevas. En lugar de hacer un módulo gigante que hace todo y se vuelve inmanejable. Gestionar la complejidad
tiene mucho sentido. Entonces, para resumir, empezaron con el reto de latencia, corrigieron la semántica clave, es un buzón, no una cola. Se obsesionaron con el rendimiento, cero copia interno. Buscaron pragmatismo para escalar, batching. Usaron la física para simplificar, no esperar innecesariamente y lograron consistencia punta a punta jit simétrico y todo bajo esa filosofía blues de estructura flexible. El resultado es este frame supplier.
A mí lo que más me llama la atención es cómo entender bien la semántica, el buzón y las restricciones físicas simplificó tanto las cosas.
Totalmente. Evitó añadir complejidad donde no hacía falta. A veces entender el problema a fondo es la mejor optimización,
sin duda. Y nos deja con esa reflexión sobre Blues, ¿no? Cómo equilibrar el diseño para hoy, documentando bien el por qué, con la certeza de que mañana habrá cambios. Exacto. Y también la idea de diseñar sistemas que a propósito descartan información.
Sí, descartar inteligentemente para priorizar la inmediatez. ¿Qué vale más? ¿Tenerlo todo aunque sea tarde o tenerlo relevante ahora mismo?
Una pregunta que va más allá de este sistema, ciertamente abre todo una línea de pensamiento. sobre la información en tiempo real.
