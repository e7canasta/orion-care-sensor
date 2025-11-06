# Runbooks - Alerta Care Operational Knowledge

**Purpose**: Operational procedures for production incidents and maintenance
**Audience**: On-call engineers, DevOps, senior developers
**Philosophy**: Life-critical system (fall detection) → zero tolerance for prolonged downtime

---

## 🚨 Critical Context

**Alerta Care is life-critical**:
- Monitors geriatric patients for falls
- Detection failure = patient unattended (life-threatening)
- Downtime SLA: <5 minutes (restart/failover must be fast)
- False negatives worse than false positives (miss fall = critical)

**Operational Priority**:
1. **Restore service** (get monitoring back ASAP)
2. **Investigate root cause** (postmortem AFTER restoration)
3. **Learn and prevent** (update ADRs, runbooks, alerts)

---

## Quick Reference

| Symptom                          | Runbook                      | Severity | Max Response Time |
|----------------------------------|------------------------------|----------|-------------------|
| No inferences received           | [No Inferences](#no-inferences-received) | 🔴 P0    | <2 minutes        |
| High frame drop rate (>80%)      | [High Drops](#high-frame-drop-rate) | 🟡 P1    | <10 minutes       |
| Worker not responding            | [Worker Stuck](#worker-not-responding) | 🟡 P1    | <10 minutes       |
| FrameSupplier crash/restart loop | [Supplier Crash](#framesupplier-crash) | 🔴 P0    | <2 minutes        |
| Graceful shutdown hangs          | [Shutdown Hang](#graceful-shutdown-hangs) | 🟢 P2    | <30 minutes       |

---

## Runbook Template

Each runbook follows this structure:

```markdown
## [Symptom]

**Severity**: P0 (critical) | P1 (high) | P2 (medium) | P3 (low)

### Symptoms
- Observable indicators (logs, metrics, alerts)

### Immediate Actions (Restore Service)
1. Step-by-step commands to restore service
2. Prioritize speed (detailed investigation comes later)

### Investigation (Post-Restoration)
- Root cause analysis steps
- References to ADRs (architectural context)

### Prevention
- Alerts to add
- ADR updates needed
- Code changes proposed

### Related
- Links to ADRs, architecture docs, postmortems
```

---

## 🔴 P0: Critical (Service Down)

### No Inferences Received

**Severity**: 🔴 P0 (Life-critical - patients unmonitored)

#### Symptoms
- MQTT topic `care/inferences/{instance_id}` silent for >30 seconds
- Alert: "No inferences received from {instance_id}"
- Dashboard shows last inference timestamp >30s old

#### Immediate Actions (Restore Service)

**1. Check if Orion process is running**:
```bash
# SSH to edge device
ssh pi@alerta-care-{room_id}

# Check process
ps aux | grep oriond

# If not running, check why it stopped
journalctl -u oriond -n 50
```

**2. Restart Orion service** (if crashed):
```bash
sudo systemctl restart oriond

# Verify restart
sudo systemctl status oriond

# Tail logs
journalctl -u oriond -f
```

**3. Check MQTT broker connectivity**:
```bash
# Test MQTT connection
mosquitto_pub -t test/ping -m "test" -h {mqtt_broker}

# If fails, check network
ping {mqtt_broker}
```

**4. If restart doesn't help, failover to backup device** (if available):
```bash
# Activate standby device for this room
./scripts/failover.sh --room {room_id} --to backup-device
```

**Expected restoration time**: <2 minutes

---

#### Investigation (Post-Restoration)

**Check logs for crash reason**:
```bash
journalctl -u oriond --since "10 minutes ago" | grep -i "panic\|fatal\|error"
```

**Common root causes**:
1. **GStreamer pipeline failure** (camera unreachable)
   - Check: `cat /var/log/oriond/gstreamer.log`
   - Related: Stream-Capture module

2. **Python worker crash** (model loading failed)
   - Check: `cat /var/log/oriond/worker-person-detector.log`
   - Related: Worker-Lifecycle module

3. **MQTT broker unreachable** (network issue)
   - Check: `ping {mqtt_broker}`, network topology

4. **Out of memory** (edge device resource exhaustion)
   - Check: `dmesg | grep -i oom`
   - Related: Memory leak investigation

**References**:
- Architecture: `/VAULT/arquitecture/ARCHITECTURE.md`
- Control Plane: `/VAULT/arquitecture/wiki/3-mqtt-control-plane.md`

---

#### Prevention

**Add alerts** (if not already present):
```yaml
# Prometheus alert
- alert: NoInferencesReceived
  expr: time() - last_inference_timestamp_seconds > 30
  severity: critical
  annotations:
    summary: "No inferences from {{ $labels.instance_id }}"
    runbook: "RUNBOOKS.md#no-inferences-received"
```

**ADR updates**:
- If root cause is architectural (e.g., FrameSupplier doesn't handle camera disconnect), create ADR or update existing

**Postmortem**:
- Document incident (template in `/docs/postmortems/TEMPLATE.md`)
- Action items tracked in GitHub issues

---

### FrameSupplier Crash

**Severity**: 🔴 P0 (Service down, patients unmonitored)

#### Symptoms
- Logs show: `panic: runtime error` in FrameSupplier code
- Orion service restart loop (crash → restart → crash)
- MQTT inferences stop

#### Immediate Actions (Restore Service)

**1. Check panic stack trace**:
```bash
journalctl -u oriond --since "5 minutes ago" | grep -A 20 "panic:"
```

**2. If panic is known bug** (check GitHub issues):
```bash
# Apply hotfix patch
cd /opt/orion
git fetch origin hotfix/issue-123
git checkout hotfix/issue-123
sudo systemctl restart oriond
```

**3. If panic is unknown, rollback to last stable version**:
```bash
cd /opt/orion
git checkout v1.0.2  # Last known stable
make build
sudo systemctl restart oriond
```

**4. If rollback doesn't work, failover**:
```bash
./scripts/failover.sh --room {room_id} --to backup-device
```

**Expected restoration time**: <2 minutes (rollback) or <5 minutes (failover)

---

#### Investigation (Post-Restoration)

**Analyze panic stack trace**:
```bash
# Extract full stack trace
journalctl -u oriond --since "10 minutes ago" > /tmp/panic.log
```

**Common panic sources** (FrameSupplier module):

1. **Double-close panic** (ADR-005 related):
   ```
   panic: close of closed channel
   ```
   - Check: Unsubscribe called after Stop()?
   - Related: ADR-005 § Idempotency

2. **Nil pointer dereference** (slot not initialized):
   ```
   panic: runtime error: invalid memory address
   ```
   - Check: Subscribe() called during Stop()?
   - Related: ADR-005 § Race Condition

3. **Goroutine leak** (distributionLoop doesn't exit):
   ```
   panic: too many goroutines
   ```
   - Check: Stop() called but wg.Wait() hangs?
   - Related: ADR-005 § Graceful Shutdown

**References**:
- ADR-005: Graceful Shutdown Semantics
- ADR-001: sync.Cond (if panic in cond.Wait)
- Architecture: FrameSupplier concurrency model

---

#### Prevention

**Add panic recovery** (if not present):
```go
// In critical goroutines
defer func() {
    if r := recover(); r != nil {
        log.Error("Panic in distributionLoop", "panic", r, "stack", debug.Stack())
        // Report to monitoring
        metrics.PanicsTotal.Inc()
    }
}()
```

**Regression test**:
- Add test that reproduces panic (before fix)
- Verify test passes after fix
- Add to CI/CD

**Postmortem**:
- Document panic scenario
- Update ADR if architectural change needed

---

## 🟡 P1: High (Service Degraded)

### High Frame Drop Rate

**Severity**: 🟡 P1 (Service degraded, some inferences missed)

#### Symptoms
- Metrics: `framesupplier_drops_total` increasing rapidly
- Alert: "Drop rate >80% on {instance_id}"
- Inferences still arriving, but lower frequency than expected

#### Immediate Actions (Restore Service)

**1. Check if drops are expected** (JIT semantics):
```bash
# Get FrameSupplier stats
mosquitto_pub -t care/control/{instance_id} -m '{"command":"get_status"}' && \
mosquitto_sub -t care/status/{instance_id} -C 1

# Check output:
# - inbox_drops: Should be ~0 (if high, distribution loop slow)
# - worker_drops: Expected if worker slow (JIT semantics)
```

**2. If inbox drops high** (distribution loop bottleneck):
```bash
# Check CPU usage
top -p $(pgrep oriond)

# If CPU maxed, check batching threshold (ADR-003)
# Expected: <100µs per distribution @ 64 workers
```

**3. If worker drops high** (worker too slow):
```bash
# Check worker inference time
cat /var/log/oriond/worker-person-detector.log | grep "inference_ms"

# Expected: YOLO320 ~20ms, YOLO640 ~50ms
# If >100ms, worker is bottleneck (not FrameSupplier issue)
```

**Expected restoration**: Often NOT an issue (JIT design), but verify

---

#### Investigation (Post-Restoration)

**Understand drop semantics** (ADR-004):
- FrameSupplier uses JIT (Just-In-Time) semantics
- Drops are FEATURE (not bug) when:
  - Source FPS > Worker FPS (e.g., 30fps camera, 1fps inference)
  - Expected: ~29 drops per second @ 30fps → 1fps
- Drops are BUG when:
  - Inbox drops >0 (distribution loop can't keep up)
  - Worker drops when worker faster than source

**Thresholds**:
```python
# Expected drops (JIT semantics)
expected_drops_per_sec = source_fps - worker_fps

# Example: 30fps source, 1fps worker
# Expected: 29 drops/sec (normal)

# Inbox drops (distribution bottleneck)
inbox_drops_threshold = 0  # Should always be ~0
```

**References**:
- ADR-004: Symmetric JIT Architecture
- ADR-003: Batching (distribution latency)
- Architecture: Drop policy rationale

---

#### Prevention

**Tune alerts** (distinguish JIT drops from bottleneck):
```yaml
# Alert on INBOX drops (bad)
- alert: InboxDropsHigh
  expr: framesupplier_inbox_drops_total > 0
  severity: warning
  annotations:
    summary: "Distribution loop bottleneck"
    runbook: "RUNBOOKS.md#high-frame-drop-rate"

# Alert on WORKER drops (informational)
- alert: WorkerDropsHigh
  expr: rate(framesupplier_worker_drops_total[1m]) > 10
  severity: info
  annotations:
    summary: "Worker slow (expected in JIT)"
```

**Dashboard**: Add drop rate graph (inbox vs worker, separate)

---

### Worker Not Responding

**Severity**: 🟡 P1 (Inferences degraded, one worker down)

#### Symptoms
- Logs: "Worker {worker_id} hasn't consumed frame in 30s"
- Metrics: `framesupplier_worker_idle_duration_seconds` > threshold
- MQTT inferences still arriving, but specific detection missing

#### Immediate Actions (Restore Service)

**1. Check if worker is stuck**:
```bash
# Check worker process
ps aux | grep person-detector.py

# If running, check if blocked
# (This requires goroutine profiling, future work)
```

**2. Restart worker** (Worker-Lifecycle will handle this in future):
```bash
# Manual restart for now (MVP phase)
pkill -f person-detector.py
# Orion will auto-restart (KISS auto-recovery)
```

**3. If auto-recovery fails, restart Orion**:
```bash
sudo systemctl restart oriond
```

**Expected restoration**: <1 minute (auto-recovery) or <2 minutes (manual restart)

---

#### Investigation (Post-Restoration)

**Check worker logs**:
```bash
cat /var/log/oriond/worker-person-detector.log | tail -100
```

**Common root causes**:
1. **Model loading failed** (ONNX runtime error)
2. **Inference timeout** (GPU hang, rare)
3. **Deadlock** (worker blocked in readFunc, FrameSupplier issue)

**If deadlock suspected** (related to ADR-005):
- Check: Did Stop() close slot correctly?
- Check: Worker blocked in cond.Wait()?
- Create postmortem, update ADR-005 if architectural issue

**References**:
- ADR-005: Graceful Shutdown (if worker stuck in readFunc)
- Worker-Lifecycle: Future module (restart policies)

---

#### Prevention

**Add worker watchdog** (if not present):
```go
// In FrameSupplier
go watchWorkers(ctx) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ticker.C:
            checkWorkerIdle()  // Log if worker hasn't consumed in 30s
        case <-ctx.Done():
            return
        }
    }
}
```

**Future**: Worker-Lifecycle module handles restart policies (ADR-006?)

---

## 🟢 P2: Medium (Service Functional, Issue Logged)

### Graceful Shutdown Hangs

**Severity**: 🟢 P2 (Deployment delayed, not production impact)

#### Symptoms
- `sudo systemctl stop oriond` hangs for >10 seconds
- Logs: "Waiting for distributionLoop to exit..."
- Eventually times out (systemd kills process)

#### Immediate Actions

**1. Force kill if needed** (deployment blocked):
```bash
sudo systemctl kill -s SIGKILL oriond
sudo systemctl start oriond
```

**2. For investigation, allow graceful shutdown to complete**:
```bash
# Increase systemd timeout temporarily
sudo systemctl edit oriond
# Add: TimeoutStopSec=60s

sudo systemctl stop oriond
# Observe logs
journalctl -u oriond -f
```

---

#### Investigation

**Check ADR-005 implementation**:
- Does Stop() call inboxCond.Broadcast()?
- Does Stop() close worker slots?
- Does distributionLoop check ctx.Done()?

**Common root causes**:
1. **distributionLoop doesn't exit** (missing ctx.Done check)
2. **wg.Wait() hangs** (goroutine leak)
3. **Worker Unsubscribe() never called** (slots not closed by Stop)

**References**:
- ADR-005: Graceful Shutdown Semantics
- ADR-001: sync.Cond (if cond.Wait doesn't wake on Broadcast)

---

#### Prevention

**Add test** (from ADR-005):
```go
func TestGracefulShutdown(t *testing.T) {
    supplier := NewSupplier()
    supplier.Start()

    // Subscribe worker (no frames published)
    readFunc := supplier.Subscribe("worker1")
    go func() {
        frame := readFunc()  // Blocks here
        assert.Nil(t, frame)  // Should return nil after Stop
    }()

    time.Sleep(100 * time.Millisecond)

    // Call Stop (should unblock worker)
    done := make(chan struct{})
    go func() {
        supplier.Stop()
        close(done)
    }()

    select {
    case <-done:
        // Success: Stop completed
    case <-time.After(1 * time.Second):
        t.Fatal("Stop() hung, worker didn't exit")
    }
}
```

**Regression**: Ensure TestGracefulShutdown in CI/CD

---

## 📋 Operational Procedures

### Daily Health Checks

**Every morning** (automated, cron job):
```bash
#!/bin/bash
# /opt/orion/scripts/health-check.sh

# Check all instances
for room in room-101 room-102 room-103; do
    echo "Checking $room..."

    # 1. Check last inference timestamp
    last_inference=$(redis-cli GET "alerta:$room:last_inference")
    age=$(($(date +%s) - last_inference))

    if [ $age -gt 60 ]; then
        echo "❌ $room: No inferences in $age seconds"
        # Alert on-call
        ./alert.sh "$room no inferences"
    else
        echo "✅ $room: Healthy (last inference ${age}s ago)"
    fi

    # 2. Check drop rate
    # (Query Prometheus or logs)

    # 3. Check worker health
    # (Query metrics)
done
```

**Alerts**: Send to Slack, PagerDuty, or similar

---

### Weekly Maintenance

**Every Sunday 2am** (low-traffic window):
```bash
# 1. Restart all Orion instances (fresh start)
ansible-playbook playbooks/rolling-restart.yml

# 2. Clear old logs (keep last 7 days)
find /var/log/oriond -name "*.log" -mtime +7 -delete

# 3. Backup configuration
rsync -av /etc/orion/ backup-server:/backups/orion/$(date +%Y%m%d)/

# 4. Update models (if new version available)
./scripts/update-models.sh --version latest
```

---

### Incident Response Template

**When alert fires**:

1. **Acknowledge** (PagerDuty, Slack, etc.)
2. **Assess severity** (P0/P1/P2/P3 from table above)
3. **Open runbook** (this document)
4. **Execute immediate actions** (restore service)
5. **Communicate** (notify team, stakeholders)
6. **Investigate** (root cause analysis)
7. **Document** (postmortem)
8. **Prevent** (update ADRs, runbooks, alerts)

---

## 🔗 Integration with ADRs

**Runbooks complement ADRs**:

| Document Type | Purpose                          | Example                                     |
|---------------|----------------------------------|---------------------------------------------|
| **ADRs**      | Design decisions (why built this way) | ADR-005: Why Stop() closes slots        |
| **Runbooks**  | Operational procedures (how to run) | What to do when Stop() hangs            |
| **Postmortems** | Incident learnings (what went wrong) | Why FrameSupplier crashed 2025-03-15   |

**Cross-references**:
- Runbook → ADR (architectural context for root cause)
- ADR → Runbook (operational implications of design)
- Postmortem → ADR (if incident reveals architectural gap)

**Example**:
```
Incident: Worker stuck in readFunc after Stop()

1. Runbook: "Worker Not Responding" → immediate actions (restart)
2. Investigation: References ADR-005 (graceful shutdown semantics)
3. Root cause: ADR-005 not implemented correctly (Stop() didn't close slots)
4. Fix: Implement ADR-005 § Implementation Checklist
5. Postmortem: Document incident, link to ADR-005 update
6. Prevention: Add TestGracefulShutdown to CI/CD
```

---

## 📈 Evolution Plan

**MVP Phase** (current, 6 months):
- Foundational runbooks (this document)
- Manual incident response (SSH, systemctl)
- Weekly health checks (automated script)

**Production Phase** (6-12 months):
- Automated remediation (restart on alert)
- Centralized logging (ELK stack, Grafana)
- Chaos engineering (test failover, graceful degradation)

**Scale Phase** (12-24 months):
- Multi-site deployment (geographic redundancy)
- Self-healing (auto-failover, circuit breakers)
- Predictive alerts (ML-based anomaly detection)

**Document growth**:
- Add runbooks as new scenarios emerge (production incidents)
- Snapshot runbooks at releases (like ADRs)
- Archive deprecated runbooks (when automation replaces manual)

---

## 🎯 Success Criteria

**For On-Call Engineer**:
- P0 incident → runbook → service restored in <2 minutes
- P1 incident → runbook → issue triaged in <10 minutes
- No guessing (runbook has exact commands, references)

**For Senior Dev**:
- Postmortem → runbook updated (prevent recurrence)
- ADR change → runbook updated (operational implications)
- New deployment → runbook reviewed (pre-prod checklist)

**For System**:
- MTTR (Mean Time To Recovery): <5 minutes (P0), <30 minutes (P1)
- MTBF (Mean Time Between Failures): >7 days (target)
- Postmortem completion: Within 48 hours of incident

---

## 📚 References

### Internal Docs
- `/VAULT/arquitecture/ARCHITECTURE.md` - System architecture
- `/docs/ADR/` - Architecture decision records
- `/PAIR_DISCOVERY_PROTOCOL.md` - How decisions are made
- `/docs/postmortems/` - Incident learnings (future)

### External Resources
- GStreamer debugging: https://gstreamer.freedesktop.org/documentation/gstreamer/gstdebugutils.html
- MQTT troubleshooting: https://mosquitto.org/man/mosquitto-8.html
- ONNX Runtime: https://onnxruntime.ai/docs/

---

## Meta: About This Document

**Purpose**: Foundational runbooks for Alerta Care operational knowledge

**Maintenance**:
- Update after every production incident (postmortem → runbook)
- Review quarterly (are runbooks still accurate?)
- Snapshot at releases (like ADRs)

**Ownership**:
- Primary: On-call rotation (DevOps + senior devs)
- Updates: Anyone who resolves incident (pair with postmortem)
- Review: Ernesto (architect) ensures alignment with ADRs

---

**Last Updated**: 2025-01-05 (Foundational version, pre-production)
**Next Update**: After first production deployment (6 months)
**Maintainer**: Update runbooks after incidents, quarterly review
