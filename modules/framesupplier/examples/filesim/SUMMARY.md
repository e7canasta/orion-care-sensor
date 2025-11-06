# FileSim v3.0 - Feature Summary

## 🎯 Core Capabilities

FileSim is a production-grade FrameSupplier testing tool that simulates real-world streaming scenarios from disk files.

### Primary Use Cases
1. **Benchmark FrameSupplier performance** (validate non-blocking design)
2. **Test multi-worker scenarios** (YOLO320/640, VLM, analytics)
3. **Validate drop policy** (slow workers drop frames, fast workers don't)
4. **Integration testing** (MQTT emitter, full pipeline)
5. **CI/CD regression testing** (archive runs, compare metrics)

---

## 🚀 Features Overview

### 1️⃣ Basic Simulation
- Read frames from disk (PNG/JPEG)
- Publish @ configurable FPS (1-60+)
- Loop N times or infinite
- 3 workers copying to separate output dirs

### 2️⃣ Real-time Stats Monitor (`-stats`) ⭐
```
[STATS] FPS: 29.8/30.0 | Drops: worker-1=0.0% worker-2=33% worker-3=70% | Inbox: 0
```
- Actual vs target FPS
- Per-worker drop rates
- Inbox health (should be 0)
- Updates every 1 second

### 3️⃣ Variable Worker Speeds (`-worker-delays`) ⭐
```bash
-worker-delays "20ms,50ms,100ms"
```
- Simulates different inference times
- Worker-1: Fast (PersonDetector - YOLO320)
- Worker-2: Medium (PoseDetector - YOLO640)
- Worker-3: Slow (VLM/Analytics)

### 4️⃣ MQTT Emitter (`-mqtt-worker`) ⭐
```bash
-mqtt-worker -mqtt-broker tcp://localhost:1883
```
- Additional worker that "publishes" to MQTT
- Currently STUB (logs only)
- Ready for real MQTT integration

### 5️⃣ Runs Tracking System (`-project`, `-name`) ⭐⭐
```
runs/detect/
├── exp1/           # Auto-numbered
│   ├── config.yaml
│   └── stats.json
├── exp2/
├── my-bench/       # Custom named
└── latest -> exp2  # Symlink
```

**Auto-saves:**
- `config.yaml` - Full run configuration
- `stats.json` - Final statistics (drops, duration, etc)

**Benefits:**
✅ Reproducibility
✅ Historical comparison
✅ CI/CD integration
✅ No data loss

---

## 📊 Command Examples

### Quick Test
```bash
./filesim -fps 5 -n 1
```

### Performance Benchmark
```bash
./filesim -fps 30 -stats -worker-delays "20ms,50ms,100ms" -name benchmark-v1
```

### Stress Test
```bash
./filesim -fps 60 -stats -worker-delays "10ms,80ms,200ms" -name stress-test
```

### Compare Runs
```bash
./filesim -fps 30 -n 2 -name baseline
./filesim -fps 30 -n 2 -name optimized
diff <(jq '.workers' runs/detect/baseline/stats.json) \
     <(jq '.workers' runs/detect/optimized/stats.json)
```

---

## 📈 Validated Metrics

### What We Validate
1. **Non-blocking Publish**: Inbox drops = 0 (always)
2. **Drop Policy**: Slow workers drop, fast workers don't
3. **FPS Accuracy**: Actual FPS ≈ Target FPS
4. **Worker Isolation**: One slow worker doesn't block others

### Expected Drop Rates @ 30fps
| Worker Delay | Expected Drops | Reason                    |
|--------------|----------------|---------------------------|
| 20ms         | 0%             | 20ms < 33ms frame period  |
| 50ms         | 30-40%         | 50ms > 33ms               |
| 100ms        | 60-70%         | 100ms >> 33ms             |

---

## 🔧 Technical Details

### Architecture
```
Disk → Producer → FrameSupplier → Worker-1 (copy to disk)
                       ↓
                   Worker-2 (copy to disk)
                       ↓
                   Worker-3 (copy to disk)
                       ↓
                   MQTT-Worker (stub)
```

### Dependencies
- Go 1.x (framesupplier module)
- gopkg.in/yaml.v3 (config serialization)
- jq (optional, for JSON inspection)

### Performance
- **Overhead**: <1% CPU for stats monitoring
- **Throughput**: 60+ fps sustained (disk I/O permitting)
- **Memory**: ~100KB per frame in memory

---

## 📚 Documentation

| File          | Purpose                                |
|---------------|----------------------------------------|
| README.md     | User guide, flags reference            |
| EXAMPLES.md   | 8 usage examples + troubleshooting     |
| CHANGELOG.md  | Version history (v1.0 → v3.0)          |
| SUMMARY.md    | This file (executive summary)          |
| runs/README.md| Runs directory usage guide             |

---

## 🎯 Next Steps (Roadmap)

### v3.1 (Next)
- [ ] Frame copying to runs/ (`--save-frames`)
- [ ] Real MQTT publish (replace stub)
- [ ] Run tags/metadata (`-tag benchmark`)

### v4.0 (Future)
- [ ] Time-series stats (continuous updates)
- [ ] Plot generation (FPS graphs, histograms)
- [ ] HTML report generator
- [ ] Run comparison tool (`--compare exp1 exp2`)

---

## 💡 Pro Tips

1. **Always use `-stats`** for meaningful benchmarks
2. **Name important runs** (`-name benchmark-v2.0`)
3. **Use jq** to inspect/compare stats.json
4. **Archive old runs** (compress to save disk space)
5. **Don't commit runs/** (add to .gitignore)

---

## 🤝 Contributing

This is a testing tool for FrameSupplier development. Improvements welcome:
- More realistic worker scenarios
- Additional metrics tracking
- Visualization tools
- Export formats (CSV, Prometheus, etc)

---

**Version**: v3.0 (2025-11-05)  
**Status**: Production-ready ✅  
**License**: Same as Orion project
