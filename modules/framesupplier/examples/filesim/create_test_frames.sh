#!/bin/bash
# Create synthetic test frames using ImageMagick (if available) or just dummy files

FRAMES_DIR="data/frames"
mkdir -p "$FRAMES_DIR"

# Check if ImageMagick available
if command -v convert &> /dev/null; then
    echo "Creating synthetic frames with ImageMagick..."
    for i in {1..10}; do
        filename=$(printf "frame_%03d.png" $i)
        convert -size 640x480 xc:blue \
                -pointsize 72 -fill white \
                -draw "text 200,240 'Frame $i'" \
                "$FRAMES_DIR/$filename"
        echo "Created $filename"
    done
else
    echo "ImageMagick not found, creating dummy PNG files..."
    # Create minimal valid PNG (1x1 transparent)
    for i in {1..10}; do
        filename=$(printf "frame_%03d.png" $i)
        # Minimal PNG header + IEND
        printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82' > "$FRAMES_DIR/$filename"
        echo "Created $filename (dummy)"
    done
fi

echo "Done! Created 10 frames in $FRAMES_DIR"
ls -lh "$FRAMES_DIR"
