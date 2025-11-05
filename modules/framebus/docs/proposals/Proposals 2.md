 - El modelo de estadísticas pierde contexto cuando hay churn de suscriptores: Stats() suma TotalSent y TotalDropped sólo para los canales activos al momento de la llamada (internal/bus/bus.go:203). Cuando un  
 worker se da de baja, sus contadores desaparecen y la tasa global cae artificialmente. Propongo conservar un acumulado histórico y, si hace falta, separar métricas “lifetime” vs “active” en una ADR dedicada.  
 - Tras Close(), tanto Subscribe como Unsubscribe devuelven ErrBusClosed (internal/bus/bus.go:139). Esto obliga a orquestadores a desenlazar todos los subscriptores antes de cerrar; si ocurre un shutdown de  
 emergencia, se bloquea la limpieza de registros. Recomiendo discutir en una ADR la conveniencia de permitir Unsubscribe idempotente post-cierre (y quizá Subscribe como no-op) para simplificar la coreografía  
 de apagado.  
 - Publish incrementa totalPublished antes de verificar el flag closed (internal/bus/bus.go:165). Si alguien publica por error tras cerrar, se lanza el pánico previsto, pero el contador ya subió, rompiendo la  
 invariancia TotalSent + TotalDropped == TotalPublished × subscribers. Mover el Add(1) después de la comprobación (o revertirlo en la rama de error) mantendría la consistencia incluso bajo llamadas erróneas.  
  
 Siguientes pasos sugeridos  
  
 1. Registrar ADR sobre la estrategia de métricas acumuladas vs activas y ajustar Stats() en consecuencia.  
 2. Revisar la política post-Close() y documentarla en un ADR antes de modificar la API.  
 3. Corregir el orden del totalPublished y añadir pruebas que cubran publish/close race para evitar regresiones.