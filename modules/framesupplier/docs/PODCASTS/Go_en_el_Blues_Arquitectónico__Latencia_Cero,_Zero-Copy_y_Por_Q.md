Go_en_el_Blues_Arquitectónico__Latencia_Cero,_Zero-Copy_y_Por_Q


Esta exploración arquitectónica detalla las notas de diseño de Frame Supplier, un componente vital de un sistema de videovigilancia con IA (Orion 2.0) enfocado en la distribución de video en tiempo real. El desafío principal es priorizar la latencia sobre la completitud, implementando una filosofía de descarte de fotogramas sin piedad y no bloquear jamás al publicador. Las decisiones técnicas, como elegir sync.Cond sobre canales de Go, se basaron en una comprensión profunda del dominio, buscando que cada worker procese siempre el fotograma más reciente disponible (semántica "mailbox" o Just in Time). Además, el diseño estuvo fuertemente influenciado por las restricciones del negocio, incluyendo la necesidad innegociable de cero copia de datos y un monitoreo operacional detallado. Finalmente, las discusiones reflejan una filosofía Blues, un marco pragmático que equilibra la estructura arquitectónica con la improvisación consciente basada en el contexto y que incluso sugiere la participación de agentes de IA como colaboradores en el proceso de diseño.
Bienvenidos a una nueva exploración arquitectónica. Hoy nos metemos de lleno en las notas de diseño de Frame Supplier.
Eso es un componente eh bastante crítico para distribuir vídeo en tiempo real en Orion 2.0. Un sistema de IA.
Y el reto es de los buenos, ¿eh? Entregar fotogramas de vídeo donde cada milisegundo, vamos, es vital y con una filosofía que, bueno, choca un poco al principio. Descartar fotogramas sin piedad, nunca en colar.
Exacto. Latencia por encima de completitud. Olvídate de tenerlo todo.
Hablamos de un sistema de videovigilancia por IA. Ahí la respuesta inmediata, claro, lo es todo. Y lo interesante es que las fuentes que tenemos
son notas internas, discusiones, ¿sí? Fruto de payer programming y fíjate, posiblemente con ayuda de agentes de IA. Eso nos da una visión muy muy real del proceso. Hoy nos centraremos en las decisiones clave, primitivas de concurrencia en Go, esa filosofía de complejidad por diseño, no por accidente, y como el negocio, la colaboración, incluso, bueno, Quizás con IA va moldeando la arquitectura final.
Y esto para nuestra audiencia Arquitectos líderes CTO es, yo creo, muy relevante. No es solo go o Sync.
No, no es ver por qué se eligió Sync frente a canales, como principios como el Just in Time, el JIT, se aplicaron en todo el sistema.
Y cómo esa filosofía que llamaron blues guió las decisiones más pragmáticas. Vamos a desgranar este viaje en el diseño de alto rendimiento.
Y la regla de oro, el publicador, el que envía los frames, no No puede bloquearse nunca jamás.
Eso es. Si un worker va lento, su fotograma se descarta. Punto. Latencia cero en el origen es la prioridad número uno.
Y la primera idea que aparece en las notas fue usar sync kunde.
Las variables de condición de Go.
Parecía simple, ¿no?
Y eficiente. Sí, pero a ver, para los que conocen Go, lo más digamos idiomático para descartar suele ser usar canales con select y default. Es como lo natural para no bloquearse. Claro, es la tensión clásica. Modelo pull contra modelo push. Los canales con select default son push. El publicador intenta enviar. Si no puede, tatan, el default le deja seguir descartando si hace falta.
Exacto.
Y sync con den es más pool. Los workers esperan pasivamente con wait a que les llegue una señal, un signal para recoger el dato. Entonces, a primera vista, para nunca bloquear al publicador, el modelo push con canales parece lo más lógico. No
parecía, pero aquí es donde dieron una vuelta de tuerca interesante. No se quedaron solo en la mecánica,
profundizaron en la semántica,
en lo que realmente necesitaban.
Ahí está la clave. Se dieron cuenta de que no era un sistema de publicar suscribir genérico. No necesitaban que cada worker viera todos los fotogramas.
Entonces, ¿qué necesitaban? Necesitaban que cada worker, cuando estuviera libre, procesara siempre el fotograma más reciente disponible, el último.
Ah, vale. Como la percepción humana.
Exacto. La analogía que usaron es esa. Si miras algo y parpadeas, No intentas recuperar lo perdido, saltas a la hora.
¿Entendido? No es una película que pausas y rebobinas.
Eso es. Y esa comprensión cambió todo. La política de descarte ya no era descartar si el bufer está lleno,
sino
reemplazar el fotograma viejo no consumido por el nuevo. Sobrescribir.
Y claro, con esa semántica de siempre lo más fresco.
Exacto. Sync con de repente encajaba perfectamente. Es ideal para implementar un patrón mailbox por worker.
Un buzón por cada worker con capacidad para un solo elemento.
Exacto. Cada worker tiene su slot protegido por un mutex. Llega a un frame nuevo, el publicador bloquea el mutex, actualiza el campo latest frame, pisa lo que hubiera y lanza un signal a la cont de ese slot.
Y si el worker estaba esperando con Wave,
se despierta y consume ese latest frame. Si no estaba esperando, la señal se pierde, da igual, pero el latest frame está actualizado para la próxima vez que el worker pida.
Es una semántica de sobresescritura casi inherente al mecanismo.
Totalmente.
Qué curioso. ¿Cómo entender bien el dominio? Esa necesidad de la última foto cambia por completo. ¿Qué primitiva de concurrencia es la adecuada?
Sí, y de hecho se parece mucho a cómo funcionan cosas como el app de GSAM si lo configuras con Max buffers igual a 1 y drop igual a tru.
Ah, claro, está pensado para darte siempre el buffer más reciente.
Exactamente la misma semántica que necesitaban. Captura la esencia del tiempo real.
Pero bueno, esto no era un ejercicio teórico. Estaba el contexto del negocio. Alerta care, un sistema para detectar caídas. Y eso impuso restricciones muy reales. Las tres verdades del negocio las llamaron.
Sí. Y fueron decisivas. La primera, cero copy. Innegociable.
¿Por qué tan estricto?
Competían con soluciones en C++ como GSAM o deepstam. Copiar JPGs de 50 o 100 KB en Go una y otra vez
sería un desastre de rendimiento.
Exacto. Segunda verdad, el seguimiento operacional. No les importaba tanto medir los FPS de la IA,
sino A ver si el worker estaba vivo, si consumía frames o si estaba, digamos, atascado. Un detector de personas parado 30 segundos era un problema operacional grave.
Entiendo.
Y la tercera verdad eran los SLAS diferentes.
Eso es. No todos los workers eran igual de críticos. El detector de personas principal casi cero descartes permitidos. Pero otros como modelos VLM para análisis secundarios, lo del Ciro Copy, por ejemplo,
pues implementándolo, compartiendo punteros al frame, un frame and go entre publicador y workers,
pero eso tiene implicaciones, ¿no?
Muchas. Exige un contrato de inmutabilidad férreo. Una vez publicado un frame, nadie toca sus datos. Frame. Data, ni el publicador ni el worker.
Si no, corrompes la memoria para todos.
Claro, es el precio a pagar por el rendimiento. Es disciplina arquitectónica pura y bueno, siempre hay una copia inevitable al pasar a Python, pero dentro de Go, cero copias.
Vale, el zero copy impone disciplina y el tracking operacional.
Pues diseñaron el frame supplier para que cada worker slot tuviera métricas específicas, las consumed at, las consum, consecutive drops, métricas de salud, no de velocidad. Eso es para saber si el worker consume o está digamos muerto. Y una go routine externa, watch workers, vigilaba esto y podía reiniciar workers que se portaran mal.
Y con SLAS diferentes, ¿no se plantearon que el frame supplier priorizara a los workers críticos, al detector de personas, por ejemplo,
se planteó, sí, pero decidieron mantener el frame supplier simple. Principio quis.
Mantener la cohesión. Exacto. Su única misión, distribuir eficientemente con semántica último frame. La prioridad, el scheduling, el ciclo de vida. Eso era responsabilidad de otro módulo, un worker life cycle manager, definiendo bien los bounded contexts, los límites de responsabilidad.
Eso mismo, evitar el feature crip
tiene lógica. Hablemos de optimización. Hubo debate sobre cómo el publish notificaba a los workers, uno por uno, secuencial o en paralelo con Goru. Sí, secuencial es simple, pero la latencia del publish crece con el número de workers. Paralelo es más rápido para el publicador, pero crear go routins tiene su coste.
¿Y qué hicieron al final?
Algo pragmático basado en números y en el contexto. Un umbral.
Un umbral.
Si hay ocho workers o menos, notificación secuencial. Si hay más, usan lotes de ocho workers en paralelo.
O sea, un publish badch size de ocho. ¿Y por qué ocho?
Midieron. El coste de ocho notificaciones secuenciales, Meex, Lock Unlock Plus signal, era parecido al de lanzar una Go Roouting. A partir de ahí, el paralelismo compensaba.
Interesante. Complejidad por diseño, no por accidente. Añadirla solo cuando la escala lo justifica.
Justo eso.
Y relacionado con esto, les preocupó el orden si las notificaciones paralelas podían causar que el frame n + 1 se procesara antes que el N en algún worker. ¿Pensaron en usar Sync.group?
Sí, se discutió,
pero Aquí surgió otro concepto potente, el invariante físico del sistema.
¿El qué? Perdona.
El invariante físico. Analizaron las latencias reales. El publish, incluso notificando a muchos workers en paralelo, tardaba unos 100 microsegundos.
Micro,
¿vale? Y el intervalo entre frames
a 30 fps son 33,000 microsegundos.
33,000.
Vaya,
casi tres órdenes de magnitud de diferencia.
Exacto. Era físicamente imposible que la notificación del frame N+ 1 empezara antes de que que el ALN hubiera terminado de sobra.
El N+1 simplemente aún no había llegado de la cámara.
Claro,
el adelantamiento era imposible en la práctica.
Precisamente añadir un weight group weight solo habría metido latencia artificial para protegerse de un fantasma. Además, la semántica mailbox del worker ya garantiza que siempre ve el último entregado.
Así que fire and forget para las notificaciones.
Correcto.
Una lección importante, mirar las latencias físicas reales puede simplificar mucho las cosas.
Totalmente. Y luego otro Inside en casa de herrero, cuchillo de palo o más bien que no lo fuera.
Jaja. Sí. Si el frame supplier funciona con filosofía Jit, just in time, dame lo último, ¿qué sentido tiene si el que le llama publish le manda frames viejos que tenían un buffer?
Claro, la filosofía JIT tiene que ser coherente en toda la cadena
de principio a fin, desde Greamer hasta Python pasando por el frame supplier.
¿Y cómo lo aseguraron sin acoplón demasiado los módulos? Pues manteniendo la de responsabilidades. El frame supplier no se mete en cómo captura los frames su cliente. Definieron un contrato claro en la API de Publish, espero frames frescos. Eso y además ofrecieron una utilidad opcional consume git.
Una utilidad,
sí, una función que un publicador puede usar para envolver su canal de entrada. Esta utilidad se asegura de descartar todos los frames pendientes, menos el último antes de llamar al publish del frame supplier. Garantiza git, aguas arriba.
Ah. Bien pensado, flexibilidad y coherencia. Y esta búsqueda de simetría git llevó a un último rediseño del frame supplier, ¿no? Añadirle un buzón interno.
Sí, un inbox frame con su propio mutex y cond.
¿Por qué? ¿Para qué otro buzón?
Es crucial para asegurar que la llamada externa a publish, la que hace, por ejemplo, el módulo stream capture, sea siempre no bloqueante, idealmente menos de un microsegundo y que tenga semántica de sobrescritura.
Desacoplar totalmente al publicador de de la lógica interna de distribución a los workers.
Exacto. Esa lógica ahora vive en una goutine interna dedicada, la distribution loop. Es ella la que lee del inbox frame y notifica los workers con el batching si toca.
Se consigue una simetría git perfecta. El publicador solo deja el frame en el buzón de entrada y se va. Y además permite métricas más finas, inbox drops, que indican si la distribución loop no da abasto, un problema gordo.
Versus workers drops, que si un worker individual va lento, algo más esperado. Muy elegante. La verdad es que toda esta cadena de decisiones, tan pragmáticas, tan basadas en el contexto, no parecen casuales. Reflejan una filosofía. La llamaron blues. ¿Qué es eso?
Es una metáfora que surgió en las discusiones. La idea es que diseñar arquitectura no es seguir una partitura rígida, un dogma.
Tampoco es tocar notas al azar, el caos, no. Es como tocar blues. Tienes una estructura base, las escalas, el ritmo, que serían los bounded contexts, los principios arquitectónicos, los AD. documentados, los registros de decisión arquitectónica.
Eso es. Y dentro de esa estructura improvisas, tomas de decisiones óptimas locales basadas en el contexto del momento, sabiendo que igual mañana cambian.
¿Y cómo se aplicó eso aquí? ¿Algún ejemplo?
Pues mira, al elegir cómo gestionar el apagado del sistema, optaron por una solución más simple, adecuada para ese momento, y lo documentaron en un ADR explicando el por qué.
Sabían que si el contexto cambiaba la revisarían.
Exacto. o al decidir no meter la gestión multistream dentro del frame supplier, sino componer varias instancias estilo Unix, improvisación dentro de los límites del módulo.
O sea, que un óptimo local no es una chapuza, si es consciente y está documentado, es pragmatismo. Y los bounded context serían las escalas del blues, ¿no? Los railes guía.
Justo, te dan la estructura para poder improvisar sin descarrilar. Fomentan la composición frente al feature creep
y Y los ADRs son la memoria de esas improvisaciones. ¿Qué tocaste esa nota ahí? También mencionan un patrón para la documentación Viva, pero con snapshots por release.
Sí, para tener puntos de referencia estables mientras evoluciona.
Y por último, las fuentes insisten mucho en la colaboración. El thinking partner,
fundamental. Dicen que el diálogo, el desafío mutuo, el per discovery en modo café fue clave para llegar a insights como la simetría y git o el invariante físico.
Y aquí entra la IA que comentabas al principio. Cloud mencionan.
Sí, las notas sugieren que un agente IA no solo ayudaba con tareas, sino que participaba en ese descubrimiento.
¿Cómo?
Generando código.
Parece que iba más allá, que podía seguir protocolos de diseño, recordar contexto de discusiones pasadas, cuestionar propuestas basándose en principios,
incluso ayudar a definir la filosofía. Lo del blues.
Eso sugieren. Mencionan un clta context.md como un intento de que la IA entendiera la filosofía y el modo de trabajo del equipo.
Vaya, eso es sorprendente. Una IA como colaboradora en el pensamiento arquitectónico. Es una puerta que las fuentes dejan abierta.
Recapitulando para los arquitectos y CTO que nos escuchan claves importantes. Primero, la semántica correcta del problema es lo más importante antes que la primitiva.
Segundo, el contexto de negocio cerocoplas, operación tiene que moldear el diseño técnico.
Tercero, la potencia de aplicar como hit de forma coherente to end. Cuarto, el valor del pragmatismo informado. Óptimos locales conscientes, bien documentados en ADRs. Y quinto, esa filosofía blues como marco para equilibrar estructura y agilidad en sistemas complejos. Preguntas para reflexionar. ¿Estamos aplicando principios de diseño coherentemente en toda la pila? ¿Cómo documentamos y evolucionamos las decisiones óptimas locales? Fomentamos esa cultura de blues arquitectónico, de equilibrio entre guías y contexto, y cómo usamos la colaboración, sea humana o cóia, para ir más allá de lo obvio. Y el pensamiento final que dejan caer estas notas, si una IA puede empezar a internalizar filosofías arquitectónicas, participar en el descubrimiento de patrones, ser un sparring partner, estamos entrando en una era de jump sessions arquitectónicas humanoí.
¿Cuál será entonces el rol insustituible del arquitecto humano? Quizás definir y custodiar esas escalas los bounded context, los principios y sobre todo la intención profunda del sistema. Una pregunta abierta, sin duda, para el futuro del liderazgo técnico y con eso lo dejamos por hoy.
