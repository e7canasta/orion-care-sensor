# ADR-001: TCP-Only Transport for RTSP

**Status:** Accepted  
**Date:** 2025-01-04  
**Deciders:** Ernesto (Visiona), Claude (AI Architect)  
**Technical Story:** stream-capture design review

---

## Context and Problem Statement

RTSP supports multiple transport protocols (UDP, TCP, HTTP tunneling, multicast) for streaming video. We need to decide which transport protocol(s) to support in stream-capture for maximum reliability and compatibility with our deployment environment (edge devices behind NATs/firewalls).

## Decision Drivers

* **Network compatibility**: Must work behind NATs and firewalls (common in edge deployments)
* **Integration with go2rtc**: Our RTSP proxy prefers reliable transports
* **Simplicity**: Minimize protocol negotiation complexity
* **Performance**: Balance latency vs reliability
* **Error handling**: Prefer connection-oriented semantics over packet loss handling

## Considered Options

1. **UDP-only** (RTP over UDP) - Lowest latency, multicast support
2. **TCP-only** (RTP over TCP) - Maximum reliability and compatibility
3. **Auto-negotiate** (UDP with TCP fallback) - Best of both worlds

## Decision Outcome

**Chosen option: "TCP-only" (Option 2)**, because it provides maximum compatibility with our deployment constraints (NAT/firewall traversal) and simplifies error handling (TCP connection state vs UDP packet loss).

### Positive Consequences

* ✅ Works behind NATs and firewalls (outbound TCP connections)
* ✅ Simpler error handling (TCP connection state instead of RTP packet loss)
* ✅ Compatible with go2rtc default configuration
* ✅ Predictable behavior (no UDP port negotiation failures)
* ✅ No multicast configuration required

### Negative Consequences

* ❌ Higher latency than UDP (~100-200ms TCP overhead)
* ❌ No multicast support (acceptable for 1:1 camera streams)
* ❌ Slightly higher CPU usage (TCP retransmissions)

## Pros and Cons of the Options

### Option 1: UDP-only

RTP packets sent over UDP with no reliability guarantees.

* ✅ Good, because lowest latency (~50ms less than TCP)
* ✅ Good, because supports multicast (efficient for 1-to-many streaming)
* ✅ Good, because lower CPU usage (no retransmissions)
* ❌ Bad, because often blocked by firewalls
* ❌ Bad, because requires complex NAT traversal (STUN/TURN)
* ❌ Bad, because packet loss requires application-level handling
* ❌ Bad, because port negotiation can fail

### Option 2: TCP-only

RTP packets sent over TCP with guaranteed delivery.

* ✅ Good, because works behind NATs/firewalls (outbound TCP)
* ✅ Good, because simpler error handling (connection state)
* ✅ Good, because compatible with go2rtc defaults
* ✅ Good, because predictable behavior (no port negotiation)
* ❌ Bad, because higher latency than UDP (~100-200ms)
* ❌ Bad, because no multicast support
* ❌ Bad, because TCP retransmissions can cause jitter

### Option 3: Auto-negotiate (UDP with TCP fallback)

Try UDP first, fall back to TCP if UDP fails.

* ✅ Good, because benefits from UDP when available
* ✅ Good, because TCP fallback for restrictive networks
* ❌ Bad, because complex negotiation logic
* ❌ Bad, because unpredictable latency (depends on negotiation result)
* ❌ Bad, because harder to debug (which transport was used?)
* ❌ Bad, because increased startup time (UDP timeout + TCP retry)

## Implementation

GStreamer `rtspsrc` element configuration:

```go
rtspsrc.SetProperty("protocols", 4) // 4 = TCP only (GST_RTSP_LOWER_TRANS_TCP)
```

**Protocol values:**
- `1` = UDP unicast
- `2` = UDP multicast
- `4` = TCP
- `8` = HTTP (tunneling)

## Links

* [GStreamer rtspsrc documentation](https://gstreamer.freedesktop.org/documentation/rtsp/rtspsrc.html)
* Related code: [internal/rtsp/pipeline.go:61](../../internal/rtsp/pipeline.go#L61)
* Mentioned in: [ARCHITECTURE.md - Design Decisions](../ARCHITECTURE.md#ad-5-tcp-only-transport-rtsp)

---

**Co-authored-by:** Gaby de Visiona <noreply@visiona.app>
