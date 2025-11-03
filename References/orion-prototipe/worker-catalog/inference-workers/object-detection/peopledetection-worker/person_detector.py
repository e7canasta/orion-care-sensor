#!/usr/bin/env python3
"""
═══════════════════════════════════════════════════════════════════════════════
PERSON DETECTOR WORKER - ONNX RUNTIME (PYTHON)
═══════════════════════════════════════════════════════════════════════════════

PURPOSE:
  Subprocess that runs YOLO11 ONNX inference for person detection.
  Communicates with Go worker via stdin (frames) and stdout (results).
  Designed to be spawned and managed by person_detector_python.go

ARCHITECTURE:
  ┌──────────────┐  stdin   ┌─────────────────┐  ONNX    ┌──────────┐
  │  Go Worker   │ ──JSON─> │ Python Process  │ ───────> │ ONNX RT  │
  │ (parent)     │          │  (this file)    │          │ (C++)    │
  └──────────────┘          └─────────────────┘          └──────────┘
         ^                         │
         │ stdout (JSON results)   │
         └─────────────────────────┘

DATA FLOW:
  1. Go worker → JSON line to stdin: {frame_data: base64, width, height, meta}
  2. main() loop → parse JSON → extract frame_data, meta
  3. detector.infer() → select model (320 or 640) based on roi_processing
  4. preprocess() → decode base64 → numpy array → letterbox resize → normalize
  5. ONNX session.run() → inference on GPU/CPU
  6. postprocess() → parse YOLO output → filter by confidence → NMS
  7. build result JSON → write to stdout (flush)
  8. Go worker reads stdout → parse JSON → send to results channel

MULTI-MODEL SYSTEM (ROI Attention):
  - PRIMARY MODEL (640x640):  Used for full-resolution frames
  - SECONDARY MODEL (320x320): Used for ROI-attention cropped frames
  
  MODEL SELECTION LOGIC:
    IF frame_meta["roi_processing"]["target_size"] == 320:
      → use session_320 (faster, less accurate)
    ELSE:
      → use session_640 (default, higher accuracy)
  
  BENEFITS:
    • 320 model: ~2-3x faster inference for ROI crops
    • 640 model: higher accuracy for full frames
    • Both loaded at startup (no reload delay)

LIFECYCLE:
  INIT:
    PersonDetector.__init__(model_path, confidence, model_path_320)
      → load primary model (640):
          • ort.InferenceSession(model_path)
          • extract input/output names, shapes
          • detect model size (n/s/m/l/x) from filename
      → IF model_path_320 provided:
          • load secondary model (320)
          • enable multi_model_enabled flag
      → configure providers: CPUExecutionProvider (TODO: GPU)
      → initialize counters: inferences_320, inferences_640

  RUNNING:
    main() loop:
      FOR EACH line in sys.stdin:
        request = json.loads(line)
        request_type = request.get("type", "frame")
        
        IF request_type == "command":
          handle_command(request):
            IF command == "set_model_size":
              → hot-reload detector with new model
              → glob search: yolo11{size}_*.onnx
              → reinitialize PersonDetector
              → log: "Model reloaded: {old} → {new}"
        
        ELSE:  # frame inference (default)
          frame_b64 = request["frame_data"]
          frame_data = base64.b64decode(frame_b64)
          
          result = detector.infer(frame_data, width, height, meta):
            # Model selection
            roi_processing = meta.get("roi_processing")
            IF roi_processing AND multi_model_enabled:
              target_size = roi_processing["target_size"]
              IF target_size == 320:
                session = session_320
                model_input_size = 320
                inferences_320 += 1
              ELSE:
                session = session_640
                model_input_size = 640
                inferences_640 += 1
            ELSE:
              session = session_640  # default
              model_input_size = 640
            
            # Preprocessing
            input_tensor = preprocess(frame_data, width, height, model_input_size):
              • np.frombuffer(frame_data) → numpy array
              • reshape to (height, width, 3)
              • letterbox_resize(image, model_input_size):
                  - calculate scale to fit target_size
                  - resize with aspect ratio maintained
                  - pad with gray (114, 114, 114)
                  - return (target_size, target_size, 3)
              • normalize: float32 / 255.0 → [0, 1]
              • HWC → CHW: transpose(2, 0, 1)
              • add batch dim: (1, 3, H, W)
              • return input_tensor
            
            # Inference (ONNX Runtime)
            outputs = session.run(output_names, {input_name: input_tensor})
              → returns [(1, 84, 8400)] for YOLO11
            
            # Postprocessing
            detections = postprocess(outputs, width, height, model_input_size):
              • remove batch dim: output[0] → (84, 8400)
              • transpose: (8400, 84)
              • VECTORIZED FILTERING:
                  person_confs = output[:, 4]  # class 0 (person)
                  mask = person_confs >= confidence_threshold
                  filtered_output = output[mask]
              • VECTORIZED BBOX CONVERSION:
                  x_centers, y_centers, widths, heights = filtered_output[:, 0:4]
                  scale_x = orig_width / model_input_size
                  scale_y = orig_height / model_input_size
                  x1 = (x_centers - widths/2) * scale_x
                  y1 = (y_centers - heights/2) * scale_y
                  x2 = (x_centers + widths/2) * scale_x
                  y2 = (y_centers + heights/2) * scale_y
                  clip to [0, orig_width] and [0, orig_height]
              • build detections list: [{bbox, confidence}, ...]
              • NMS (Non-Maximum Suppression):
                  IF opencv available:
                    cv2.dnn.NMSBoxes (C++ backend, ~3-5x faster)
                  ELSE:
                    Python fallback (pure numpy)
              • return filtered detections
            
            # Build result
            result = {
              "type": "person_detection",
              "instance_id": meta["instance_id"],
              "room_id": meta["room_id"],
              "timestamp": datetime.utcnow().isoformat(),
              "frame_seq": meta["seq"],
              "data": {
                "detections": [{bbox, confidence}, ...],
                "count": len(detections),
                "metadata": {
                  "img_size": "640x640" or "320x320",
                  "model_size": "n" / "s" / "m" / "l" / "x",
                  "model_selected": "320" or "640",
                  "roi_attention": True/False,
                  "processing_time_ms": 45.2
                }
              },
              "timing": {
                "preprocess_ms": 1.5,
                "inference_ms": 42.1,
                "postprocess_ms": 1.6,
                "total_ms": 45.2
              }
            }
            return result
          
          # Write to stdout (Go worker reads this)
          print(json.dumps(result), flush=True)

HOT-RELOAD (set_model_size command):
  INPUT: {"type": "command", "command": "set_model_size", "params": {"size": "m"}}
  
  PROCESS:
    1. Extract size parameter: n/s/m/l/x
    2. Glob search in model_dir:
       patterns = ["yolo11{size}.onnx", "yolo11{size}_*.onnx", ...]
    3. Find first match
    4. Reinitialize detector:
       detector = PersonDetector(new_model_path, confidence_threshold)
    5. Log: "Model reloaded: yolo11n_fp32_640.onnx → yolo11m_fp32_640.onnx"
    6. Continue processing frames with new model

JSON PROTOCOL (stdin ← Go):
  {
    "frame_data": "<base64 encoded RGB numpy bytes>",
    "width": 1280,
    "height": 720,
    "meta": {
      "instance_id": "node_001",
      "room_id": "room_A",
      "seq": 12345,
      "timestamp": "2025-10-09T14:30:00.123456789Z",
      "roi_processing": {              // Optional, for multi-model selection
        "target_size": 320,            // Tells Python which model to use
        "crop_applied": false,
        "num_rois": 2
      }
    }
  }

JSON PROTOCOL (stdout → Go):
  {
    "type": "person_detection",
    "instance_id": "node_001",
    "room_id": "room_A",
    "timestamp": "2025-10-09T14:30:00.123456789Z",
    "frame_seq": 12345,
    "data": {
      "detections": [
        {
          "bbox": {"x": 100, "y": 200, "width": 150, "height": 300},
          "confidence": 0.87
        }
      ],
      "count": 1,
      "metadata": {
        "processing_time_ms": 45.2,
        "frame_width": 1280,
        "frame_height": 720,
        "img_size": "640x640",
        "model_size": "n",
        "imgsz": 640,
        "model_selected": "640",
        "roi_attention": false
      }
    },
    "timing": {
      "preprocess_ms": 1.5,
      "inference_ms": 42.1,
      "postprocess_ms": 1.6,
      "total_ms": 45.2
    }
  }

LOGGING (stderr → Go logStderr goroutine):
  - All logs to stderr (stdout reserved for results)
  - Format: "timestamp [LEVEL] message"
  - Levels: INFO, WARNING, ERROR, CRITICAL, DEBUG
  - Go maps levels: [ERROR]/[CRITICAL] → slog.Error
                    [WARNING] → slog.Warn
                    [INFO]/[DEBUG] → slog.Debug

YOLO11 OUTPUT FORMAT:
  - Shape: (1, 84, 8400) for YOLO11n/s/m
  - 84 channels: [x_center, y_center, width, height, class_0_conf, ..., class_79_conf]
  - 8400 proposals: grid predictions from 3 detection heads
  - Person class: 0 (COCO dataset)
  - Coordinates: relative to model input size (320 or 640)

OPTIMIZATIONS:
  1. VECTORIZED FILTERING (numpy):
     - Filter 8400 proposals with single mask operation
     - ~10-20x faster than Python loop
  
  2. VECTORIZED BBOX CONVERSION (numpy):
     - Convert all bboxes at once with numpy operations
     - No Python loops for coordinate transformations
  
  3. OPENCV NMS (C++ backend):
     - cv2.dnn.NMSBoxes uses native C++ implementation
     - ~3-5x faster than Python NMS
     - Fallback to Python if OpenCV unavailable
  
  4. LETTERBOX RESIZE:
     - Maintains aspect ratio (no distortion)
     - Pads with gray (114, 114, 114) to avoid edge artifacts
     - Uses cv2.resize (C++ backend) if available

PERFORMANCE CHARACTERISTICS:
  - YOLO11n (640x640): ~30-50ms per frame on CPU
  - YOLO11n (320x320): ~10-20ms per frame on CPU (2-3x faster)
  - YOLO11m (640x640): ~80-120ms per frame on CPU
  - Preprocessing: ~1-2ms (numpy + cv2)
  - Postprocessing: ~1-3ms (vectorized numpy + cv2 NMS)
  - Memory: ~500MB per loaded model
  - Multi-model overhead: ~1GB total RAM

DEPENDENCIES:
  - numpy: array operations, preprocessing
  - onnxruntime: ONNX model inference (CPU/GPU)
  - opencv-python (optional): faster resize + NMS
  - PIL (fallback): resize if opencv unavailable

ERROR HANDLING:
  1. JSON parse error → log to stderr, skip line, continue
  2. Missing frame_data/width/height → log error, skip, continue
  3. ONNX inference error → log with traceback, skip frame
  4. Model reload failure → log error, keep old model, continue
  5. KeyboardInterrupt → clean shutdown, log info
  6. Fatal error → log with traceback, sys.exit(1)

INTEGRATION POINTS:
  - Spawned by: models/run_worker.sh (activates venv, passes args)
  - Managed by: internal/worker/person_detector_python.go
  - Communication: stdin/stdout (JSON lines), stderr (logs)
  - Models: models/yolo11{n,s,m,l,x}_fp32_{320,640}.onnx

COMMAND LINE ARGS:
  --model PATH          Primary ONNX model (required, usually 640x640)
  --model-320 PATH      Secondary ONNX model (optional, 320x320 for ROI)
  --confidence FLOAT    Confidence threshold (default: 0.5)

USAGE:
  # Single-model mode
  python person_detector.py --model models/yolo11n_fp32_640.onnx --confidence 0.5
  
  # Multi-model mode (ROI attention)
  python person_detector.py \
    --model models/yolo11n_fp32_640.onnx \
    --model-320 models/yolo11n_fp32_320.onnx \
    --confidence 0.5

STARTUP SEQUENCE:
  1. Parse command line arguments
  2. Initialize PersonDetector:
     - Load primary model (640)
     - Load secondary model (320) if provided
     - Detect model size from filename
     - Configure ONNX providers
  3. Log: "Person detector worker ready. Waiting for frames..."
  4. Enter main loop: read stdin, process, write stdout
  5. On KeyboardInterrupt/EOF: clean shutdown

SHUTDOWN:
  - Go worker closes stdin → Python receives EOF
  - main() loop exits
  - Log: "Worker shutting down"
  - ONNX sessions automatically cleaned up (garbage collection)
  - Process exits with code 0

═══════════════════════════════════════════════════════════════════════════════
"""

import sys
import json
import base64
import argparse
import logging
import time
import struct
from typing import Dict, List, Any
from datetime import datetime

import numpy as np
import onnxruntime as ort
import msgpack

# Configure logging to stderr (stdout reserved for inference results)
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    stream=sys.stderr
)
logger = logging.getLogger(__name__)


class PersonDetector:
    """YOLO11n person detector with ONNX Runtime - supports multi-model for ROI attention"""

    def __init__(self, model_path: str, confidence_threshold: float = 0.5, model_path_320: str = None):
        self.model_path = model_path
        self.confidence_threshold = confidence_threshold
        self.multi_model_enabled = model_path_320 is not None

        # Use CPUExecutionProvider for now (TODO: CUDAExecutionProvider for GPU)
        providers = ['CPUExecutionProvider']

        # Load primary model (usually 640)
        logger.info(f"Loading primary ONNX model: {model_path}")
        self.session_640 = ort.InferenceSession(model_path, providers=providers)

        self.input_name_640 = self.session_640.get_inputs()[0].name
        self.input_shape_640 = self.session_640.get_inputs()[0].shape
        self.output_names_640 = [out.name for out in self.session_640.get_outputs()]
        self.model_input_size_640 = self.input_shape_640[2]

        logger.info(f"Primary model loaded - input size: {self.model_input_size_640}")

        # Load secondary model (320) if provided for multi-model ROI attention
        if self.multi_model_enabled:
            logger.info(f"Loading secondary ONNX model: {model_path_320}")
            self.session_320 = ort.InferenceSession(model_path_320, providers=providers)

            self.input_name_320 = self.session_320.get_inputs()[0].name
            self.input_shape_320 = self.session_320.get_inputs()[0].shape
            self.output_names_320 = [out.name for out in self.session_320.get_outputs()]
            self.model_input_size_320 = self.input_shape_320[2]

            logger.info(f"Secondary model loaded - input size: {self.model_input_size_320}")
            logger.info(f"✅ Multi-model enabled: 320x320 and {self.model_input_size_640}x{self.model_input_size_640}")
        else:
            self.session_320 = None
            logger.info(f"Single-model mode: {self.model_input_size_640}x{self.model_input_size_640}")

        # Extract model size from filename (e.g., "yolo11n.onnx" or "yolo11x_fp32_640.onnx" -> "x")
        import os
        import re
        model_filename = os.path.basename(model_path)

        # Extract size from pattern: yolo11{size}[_fp32_640].onnx or yolov8{size}[_fp32_640].onnx
        # Match patterns: yolo11n, yolo11x_fp32_640, yolov8s, etc.
        match = re.search(r'yolo(?:11|v8)([nsmxl])', model_filename.lower())
        if match:
            self.model_size = match.group(1)  # Extract just the size letter
        else:
            self.model_size = "unknown"

        logger.info(f"Model size detected: {self.model_size}")

        # Stats for multi-model selection
        self.inferences_320 = 0
        self.inferences_640 = 0

    def preprocess(self, frame_data: bytes, width: int, height: int, model_input_size: int) -> np.ndarray:
        """
        Preprocess frame for YOLO input

        Input: RGB frame (width x height x 3)
        Output: Normalized tensor (1, 3, model_input_size, model_input_size)
        """
        # Decode frame bytes to numpy array
        frame = np.frombuffer(frame_data, dtype=np.uint8)
        frame = frame.reshape((height, width, 3))

        # Resize to model input size (letterbox resize for YOLO)
        frame_resized = self.letterbox_resize(frame, model_input_size)

        # Convert RGB to normalized float32 [0, 1]
        frame_norm = frame_resized.astype(np.float32) / 255.0

        # HWC → CHW (channels first)
        frame_chw = np.transpose(frame_norm, (2, 0, 1))

        # Add batch dimension: (1, 3, H, W)
        frame_batch = np.expand_dims(frame_chw, axis=0)

        return frame_batch

    def letterbox_resize(self, image: np.ndarray, target_size: int) -> np.ndarray:
        """
        Resize image with letterbox (maintain aspect ratio, pad with gray)
        """
        h, w = image.shape[:2]

        # Calculate scale to fit target_size
        scale = min(target_size / h, target_size / w)
        new_h, new_w = int(h * scale), int(w * scale)

        # Resize
        resized = self.resize_image(image, new_w, new_h)

        # Create padded canvas (gray background)
        canvas = np.full((target_size, target_size, 3), 114, dtype=np.uint8)

        # Center the resized image
        top = (target_size - new_h) // 2
        left = (target_size - new_w) // 2
        canvas[top:top+new_h, left:left+new_w] = resized

        return canvas

    def resize_image(self, image: np.ndarray, width: int, height: int) -> np.ndarray:
        """Simple bilinear resize (numpy implementation)"""
        # For production, use cv2.resize for better performance
        # For MVP, use numpy
        try:
            import cv2
            return cv2.resize(image, (width, height), interpolation=cv2.INTER_LINEAR)
        except ImportError:
            # Fallback to PIL if cv2 not available
            from PIL import Image
            img = Image.fromarray(image)
            img = img.resize((width, height), Image.BILINEAR)
            return np.array(img)

    def postprocess(self, outputs: List[np.ndarray], orig_width: int, orig_height: int, model_input_size: int) -> List[Dict]:
        """
        Postprocess YOLO outputs to person detections

        YOLO11 output format:
        - Shape: (1, 84, 8400) for YOLOv11n
        - 84 channels: [x, y, w, h, class_0_conf, class_1_conf, ..., class_79_conf]
        - Person class: 0 (COCO dataset)
        """
        output = outputs[0]  # (1, 84, 8400)

        # Remove batch dimension
        output = output[0]  # (84, 8400)

        # Transpose to (8400, 84)
        output = output.T  # (8400, 84)

        # === OPTIMIZATION: Vectorized filtering with numpy ===
        # Extract person confidences (class 0) for all 8400 proposals
        person_confs = output[:, 4]  # Shape: (8400,)

        # Filter by confidence threshold (vectorized, much faster than Python loop)
        mask = person_confs >= self.confidence_threshold
        filtered_output = output[mask]  # Keep only high-confidence detections
        filtered_confs = person_confs[mask]

        if len(filtered_output) == 0:
            return []

        # === OPTIMIZATION: Vectorized bbox conversion ===
        # Extract bbox coordinates (all at once)
        x_centers = filtered_output[:, 0]
        y_centers = filtered_output[:, 1]
        widths = filtered_output[:, 2]
        heights = filtered_output[:, 3]

        # Convert from center format to corner format (vectorized)
        scale_x = orig_width / model_input_size
        scale_y = orig_height / model_input_size

        x1 = ((x_centers - widths/2) * scale_x).astype(int)
        y1 = ((y_centers - heights/2) * scale_y).astype(int)
        x2 = ((x_centers + widths/2) * scale_x).astype(int)
        y2 = ((y_centers + heights/2) * scale_y).astype(int)

        # Clip to image bounds (vectorized)
        x1 = np.clip(x1, 0, orig_width)
        y1 = np.clip(y1, 0, orig_height)
        x2 = np.clip(x2, 0, orig_width)
        y2 = np.clip(y2, 0, orig_height)

        # Build detections list
        detections = []
        for i in range(len(filtered_output)):
            detections.append({
                "bbox": {
                    "x": int(x1[i]),
                    "y": int(y1[i]),
                    "width": int(x2[i] - x1[i]),
                    "height": int(y2[i] - y1[i])
                },
                "confidence": float(filtered_confs[i])
            })

        # NMS (Non-Maximum Suppression) to remove duplicate detections
        detections = self.nms(detections, iou_threshold=0.45)

        return detections

    def nms(self, detections: List[Dict], iou_threshold: float = 0.45) -> List[Dict]:
        """
        Non-Maximum Suppression to remove duplicate detections

        Uses OpenCV's optimized NMS if available (C++ backend, ~3-5x faster),
        otherwise falls back to Python implementation.
        """
        if len(detections) == 0:
            return []

        # === OPTIMIZATION: Try OpenCV NMS (C++ backend, much faster) ===
        try:
            import cv2

            # Convert to OpenCV format: bboxes (x, y, w, h), confidences
            bboxes = []
            confidences = []
            for det in detections:
                bbox = det['bbox']
                bboxes.append([bbox['x'], bbox['y'], bbox['width'], bbox['height']])
                confidences.append(float(det['confidence']))

            # Run OpenCV NMS (native C++ implementation)
            indices = cv2.dnn.NMSBoxes(
                bboxes,
                confidences,
                score_threshold=self.confidence_threshold,
                nms_threshold=iou_threshold
            )

            # Extract kept detections
            if len(indices) > 0:
                return [detections[i] for i in indices.flatten()]
            else:
                return []

        except (ImportError, Exception):
            # Fallback to Python implementation if OpenCV not available
            pass

        # === FALLBACK: Python implementation (slower but always works) ===
        # Sort by confidence (descending)
        detections = sorted(detections, key=lambda x: x['confidence'], reverse=True)

        keep = []

        while detections:
            # Keep highest confidence detection
            best = detections.pop(0)
            keep.append(best)

            # Remove overlapping detections
            detections = [
                det for det in detections
                if self.iou(best['bbox'], det['bbox']) < iou_threshold
            ]

        return keep

    def iou(self, bbox1: Dict, bbox2: Dict) -> float:
        """Calculate Intersection over Union"""
        x1_min = bbox1['x']
        y1_min = bbox1['y']
        x1_max = x1_min + bbox1['width']
        y1_max = y1_min + bbox1['height']

        x2_min = bbox2['x']
        y2_min = bbox2['y']
        x2_max = x2_min + bbox2['width']
        y2_max = y2_min + bbox2['height']

        # Calculate intersection
        inter_x_min = max(x1_min, x2_min)
        inter_y_min = max(y1_min, y2_min)
        inter_x_max = min(x1_max, x2_max)
        inter_y_max = min(y1_max, y2_max)

        if inter_x_max < inter_x_min or inter_y_max < inter_y_min:
            return 0.0

        inter_area = (inter_x_max - inter_x_min) * (inter_y_max - inter_y_min)

        # Calculate union
        bbox1_area = bbox1['width'] * bbox1['height']
        bbox2_area = bbox2['width'] * bbox2['height']
        union_area = bbox1_area + bbox2_area - inter_area

        if union_area == 0:
            return 0.0

        return inter_area / union_area

    def infer(self, frame_data: bytes, width: int, height: int, frame_meta: Dict) -> Dict:
        """
        Run inference on a single frame with intelligent model selection

        Returns:
            Inference result as JSON-serializable dict
        """
        start_time = time.time()

        # Select model based on ROI processing metadata (if available)
        roi_processing = frame_meta.get("roi_processing")
        target_size = None

        if roi_processing and self.multi_model_enabled:
            target_size = roi_processing.get("target_size", self.model_input_size_640)

            # Select appropriate model session
            if target_size == 320 and self.session_320 is not None:
                session = self.session_320
                input_name = self.input_name_320
                output_names = self.output_names_320
                model_input_size = self.model_input_size_320
                self.inferences_320 += 1
                model_selected = "320"
            else:
                session = self.session_640
                input_name = self.input_name_640
                output_names = self.output_names_640
                model_input_size = self.model_input_size_640
                self.inferences_640 += 1
                model_selected = "640"
        else:
            # Fallback to primary model (640) if no ROI metadata or single-model mode
            session = self.session_640
            input_name = self.input_name_640
            output_names = self.output_names_640
            model_input_size = self.model_input_size_640
            self.inferences_640 += 1
            model_selected = "640"

        # Preprocess
        input_tensor = self.preprocess(frame_data, width, height, model_input_size)
        preprocess_time = time.time() - start_time

        # Run inference
        inference_start = time.time()
        outputs = session.run(output_names, {input_name: input_tensor})
        inference_time = time.time() - inference_start

        # Postprocess
        postprocess_start = time.time()
        detections = self.postprocess(outputs, width, height, model_input_size)
        postprocess_time = time.time() - postprocess_start

        total_time = time.time() - start_time

        # === HYBRID AUTO-FOCUS: Compute suggested ROI for NEXT frame ===
        suggested_roi = self.compute_suggested_roi(detections, width, height)

        # Build inference result
        result = {
            "type": "person_detection",
            "instance_id": frame_meta.get("instance_id", "unknown"),
            "room_id": frame_meta.get("room_id", "unknown"),
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "frame_seq": frame_meta.get("seq", 0),
            "data": {
                "detections": detections,
                "count": len(detections),
                "metadata": {
                    "processing_time_ms": round(total_time * 1000, 2),
                    "frame_width": width,
                    "frame_height": height,
                    # Model inference metadata (Python → Go → MQTT)
                    "img_size": f"{model_input_size}x{model_input_size}",
                    "model_size": self.model_size,
                    "imgsz": model_input_size,
                    "model_selected": model_selected,  # NEW: which model was used
                    "roi_attention": roi_processing is not None,  # NEW: was ROI attention active
                    # Echo back ROI processing metadata (Go needs this for MQTT)
                    "roi_processing": frame_meta.get("roi_processing") if frame_meta.get("roi_processing") else None
                }
            },
            "timing": {
                "preprocess_ms": round(preprocess_time * 1000, 2),
                "inference_ms": round(inference_time * 1000, 2),
                "postprocess_ms": round(postprocess_time * 1000, 2),
                "total_ms": round(total_time * 1000, 2)
            },
            # === NEW: Suggested ROI for next frame (hybrid auto-focus) ===
            "suggested_roi": suggested_roi
        }

        logger.debug(f"Frame {frame_meta.get('seq', 0)}: {len(detections)} person(s) detected in {total_time*1000:.1f}ms (model: {model_selected}, suggested_roi: {suggested_roi is not None})")

        return result

    def compute_suggested_roi(self, detections: List[Dict], frame_width: int, frame_height: int) -> Dict:
        """
        Compute suggested ROI for NEXT frame based on current detections.
        Uses NumPy for fast vectorized computation.

        Returns normalized ROI (0.0-1.0) or None if no detections.

        HYBRID AUTO-FOCUS:
          - Python computes ROI suggestion (fast with NumPy)
          - Go decides whether to use it (priority: external > suggested > full)
          - Stateless: no history buffer, only current detections
        """
        if len(detections) == 0:
            return None

        # Extract bboxes as numpy array for vectorized operations
        bboxes = []
        for det in detections:
            bbox = det['bbox']
            # Convert to normalized coordinates [0.0, 1.0]
            x_norm = bbox['x'] / frame_width
            y_norm = bbox['y'] / frame_height
            w_norm = bbox['width'] / frame_width
            h_norm = bbox['height'] / frame_height
            bboxes.append([x_norm, y_norm, w_norm, h_norm])

        bboxes = np.array(bboxes)

        # Vectorized min/max computation (NumPy magic - 10x faster than Go loops)
        min_x = np.min(bboxes[:, 0])
        min_y = np.min(bboxes[:, 1])
        max_x = np.max(bboxes[:, 0] + bboxes[:, 2])
        max_y = np.max(bboxes[:, 1] + bboxes[:, 3])

        # Compute merged bounding box
        merged_width = max_x - min_x
        merged_height = max_y - min_y

        # Expand by 15% margin to catch motion (configurable in future)
        expansion_pct = 0.15
        expansion_x = merged_width * expansion_pct
        expansion_y = merged_height * expansion_pct

        # Apply expansion
        suggested_x = max(0.0, min_x - expansion_x / 2)
        suggested_y = max(0.0, min_y - expansion_y / 2)
        suggested_width = min(1.0 - suggested_x, merged_width + expansion_x)
        suggested_height = min(1.0 - suggested_y, merged_height + expansion_y)

        # Clamp to [0, 1] bounds
        suggested_roi = {
            "x": float(suggested_x),
            "y": float(suggested_y),
            "width": float(suggested_width),
            "height": float(suggested_height)
        }

        logger.debug(f"Suggested ROI computed from {len(detections)} detections: {suggested_roi}")

        return suggested_roi


def main():
    parser = argparse.ArgumentParser(description='Person Detector Worker with Multi-Model ROI Attention')
    parser.add_argument('--model', required=True, help='Path to primary ONNX model (usually 640x640)')
    parser.add_argument('--model-320', help='Path to secondary ONNX model (320x320) for ROI attention (optional)')
    parser.add_argument('--confidence', type=float, default=0.5, help='Confidence threshold')
    args = parser.parse_args()

    # Initialize detector
    try:
        detector = PersonDetector(
            model_path=args.model,
            confidence_threshold=args.confidence,
            model_path_320=args.model_320
        )
    except Exception as e:
        logger.error(f"Failed to initialize detector: {e}")
        sys.exit(1)

    if detector.multi_model_enabled:
        logger.info("Person detector worker ready (MULTI-MODEL MODE). Waiting for frames and commands on stdin...")
    else:
        logger.info("Person detector worker ready (SINGLE-MODEL MODE). Waiting for frames and commands on stdin...")

    # Main loop: read frames/commands from stdin (MsgPack with length-prefix framing)
    try:
        while True:
            try:
                # Read length prefix (4 bytes, big-endian)
                length_bytes = sys.stdin.buffer.read(4)
                if len(length_bytes) < 4:
                    # EOF or incomplete read
                    logger.info("stdin closed (EOF)")
                    break

                # Unpack length
                msg_length = struct.unpack('>I', length_bytes)[0]

                # Read exact msgpack message
                msgpack_data = sys.stdin.buffer.read(msg_length)
                if len(msgpack_data) < msg_length:
                    logger.error(f"Incomplete msgpack message: expected {msg_length} bytes, got {len(msgpack_data)}")
                    break

                # Unpack msgpack
                request = msgpack.unpackb(msgpack_data, raw=False)

                request_type = request.get("type", "frame")

                # Handle control commands
                if request_type == "command":
                    command = request.get("command")
                    params = request.get("params", {})

                    if command == "set_model_size":
                        # Hot-reload model with different size
                        size = params.get("size")
                        if not size or size not in ["n", "s", "m", "l", "x"]:
                            logger.error(f"Invalid model size: {size}")
                            continue

                        logger.info(f"Hot-reloading model with size: {size}")

                        # Find model file with size pattern (supports yolo11{size}.onnx or yolo11{size}_*.onnx)
                        import os
                        import glob
                        model_dir = os.path.dirname(detector.model_path)

                        # Try multiple patterns
                        patterns = [
                            f"yolo11{size}.onnx",
                            f"yolo11{size}_*.onnx",
                            f"yolov8{size}.onnx",
                            f"yolov8{size}_*.onnx"
                        ]

                        new_model_path = None
                        for pattern in patterns:
                            matches = glob.glob(os.path.join(model_dir, pattern))
                            if matches:
                                new_model_path = matches[0]  # Take first match
                                break

                        if not new_model_path or not os.path.exists(new_model_path):
                            logger.error(f"Model not found for size '{size}' in {model_dir}")
                            logger.error(f"Tried patterns: {patterns}")
                            continue

                        # Reload detector
                        try:
                            old_model = detector.model_path
                            detector = PersonDetector(new_model_path, detector.confidence_threshold)
                            logger.info(f"✅ Model reloaded: {os.path.basename(old_model)} → {os.path.basename(new_model_path)}")

                        except Exception as e:
                            logger.error(f"Failed to reload model: {e}", exc_info=True)

                    else:
                        logger.warning(f"Unknown command: {command}")

                # Handle frame inference (default)
                else:
                    # Extract frame data (NO base64 decoding needed! MsgPack sends raw bytes)
                    frame_data = request.get("frame_data")
                    width = request.get("width")
                    height = request.get("height")
                    meta = request.get("meta", {})

                    if frame_data is None or not width or not height:
                        logger.error("Invalid request: missing frame_data, width, or height")
                        continue

                    # frame_data is already bytes (no base64 decode needed!)

                    # Run inference
                    result = detector.infer(frame_data, width, height, meta)

                    # Write result to stdout as MsgPack with length-prefix framing
                    packed_result = msgpack.packb(result, use_bin_type=True)

                    # Write length prefix (4 bytes, big-endian)
                    length_prefix = struct.pack('>I', len(packed_result))
                    sys.stdout.buffer.write(length_prefix)

                    # Write msgpack data
                    sys.stdout.buffer.write(packed_result)
                    sys.stdout.buffer.flush()

            except Exception as e:
                logger.error(f"Error processing frame: {e}", exc_info=True)

    except KeyboardInterrupt:
        logger.info("Worker interrupted by user")
    except Exception as e:
        logger.error(f"Fatal error: {e}", exc_info=True)
        sys.exit(1)

    logger.info("Worker shutting down")


if __name__ == "__main__":
    main()
