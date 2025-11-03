#!/bin/bash
# Download YOLO11n ONNX models for person detection

set -e

MODELS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "📦 Downloading YOLO11n ONNX model..."

# Create models directory if not exists
mkdir -p "$MODELS_DIR"

# Option 1: Download pre-exported ONNX from Ultralytics
# YOLO11n person detection (trained on COCO, includes person class)

MODEL_URL="https://github.com/ultralytics/assets/releases/download/v8.3.0/yolo11n.pt"
MODEL_NAME="yolo11n"

echo "Downloading $MODEL_NAME from Ultralytics..."

# Download .pt model (PyTorch checkpoint)
if [ ! -f "$MODELS_DIR/${MODEL_NAME}.pt" ]; then
    wget -q --show-progress "$MODEL_URL" -O "$MODELS_DIR/${MODEL_NAME}.pt"
    echo "✅ Downloaded ${MODEL_NAME}.pt"
else
    echo "✅ ${MODEL_NAME}.pt already exists"
fi

# Export to ONNX using Ultralytics CLI
if [ ! -f "$MODELS_DIR/${MODEL_NAME}.onnx" ]; then
    echo "Exporting to ONNX format..."

    # Check if yolo CLI is available
    if ! command -v yolo &> /dev/null; then
        echo "⚠️  yolo CLI not found. Installing ultralytics..."
        pip3 install ultralytics
    fi

    # Export to ONNX
    yolo export model="$MODELS_DIR/${MODEL_NAME}.pt" format=onnx simplify=True imgsz=640

    echo "✅ Exported to ${MODEL_NAME}.onnx"
else
    echo "✅ ${MODEL_NAME}.onnx already exists"
fi

echo ""
echo "📊 Model info:"
ls -lh "$MODELS_DIR/${MODEL_NAME}.onnx"

echo ""
echo "✅ Model download complete!"
echo ""
echo "Model path: $MODELS_DIR/${MODEL_NAME}.onnx"
echo ""
echo "To use this model, update config/orion.yaml:"
echo "  models:"
echo "    person_detector:"
echo "      model_path: models/${MODEL_NAME}.onnx"
echo "      confidence: 0.5"
