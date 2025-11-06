package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
)

// FrameSaver handles saving frames to disk (optional feature).
//
// Converts RGB raw data to PNG or JPEG format.
// Thread-safe: can be called from multiple workers concurrently.
type FrameSaver struct {
	outputDir    string
	format       string
	jpegQuality  int
	framesSaved  atomic.Uint64
	framesDropped atomic.Uint64
}

// NewFrameSaver creates a frame saver with given output directory and format.
//
// Format: "png" or "jpeg"
// JPEGQuality: 1-100 (only used for JPEG)
func NewFrameSaver(outputDir, format string, jpegQuality int) (*FrameSaver, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Validate format
	if format != "png" && format != "jpeg" {
		return nil, fmt.Errorf("unsupported format: %s (must be png or jpeg)", format)
	}

	return &FrameSaver{
		outputDir:   outputDir,
		format:      format,
		jpegQuality: jpegQuality,
	}, nil
}

// SaveFrame saves a frame to disk as PNG or JPEG.
//
// Filename format: frame_{seq:06d}_{timestamp}.{ext}
// Example: frame_000042_20251105_234517.123.png
//
// Thread-safe: safe to call from multiple goroutines.
func (fs *FrameSaver) SaveFrame(frame *framesupplier.Frame) error {
	// Convert RGB to image.RGBA (add alpha channel)
	img, err := rgbToRGBA(frame)
	if err != nil {
		fs.framesDropped.Add(1)
		return fmt.Errorf("RGB conversion failed: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("frame_%06d_%s.%s",
		frame.Seq,
		frame.Timestamp.Format("20060102_150405.000"),
		fs.format)
	filepath := filepath.Join(fs.outputDir, filename)

	// Create output file
	file, err := os.Create(filepath)
	if err != nil {
		fs.framesDropped.Add(1)
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Encode based on format
	switch fs.format {
	case "png":
		if err := png.Encode(file, img); err != nil {
			fs.framesDropped.Add(1)
			return fmt.Errorf("PNG encode failed: %w", err)
		}
	case "jpeg":
		if err := jpeg.Encode(file, img, &jpeg.Options{Quality: fs.jpegQuality}); err != nil {
			fs.framesDropped.Add(1)
			return fmt.Errorf("JPEG encode failed: %w", err)
		}
	}

	fs.framesSaved.Add(1)
	return nil
}

// rgbToRGBA converts RGB raw bytes (3 bytes/pixel) to image.RGBA (4 bytes/pixel).
//
// Adds alpha channel with value 255 (fully opaque).
func rgbToRGBA(frame *framesupplier.Frame) (*image.RGBA, error) {
	// Validate input dimensions
	expectedSize := frame.Width * frame.Height * 3
	if len(frame.Data) != expectedSize {
		return nil, fmt.Errorf("invalid RGB data size: got %d, expected %d",
			len(frame.Data), expectedSize)
	}

	// Create RGBA image
	img := image.NewRGBA(image.Rect(0, 0, frame.Width, frame.Height))

	// Convert RGB to RGBA (add alpha = 255)
	for i := 0; i < frame.Width*frame.Height; i++ {
		img.Pix[i*4+0] = frame.Data[i*3+0] // R
		img.Pix[i*4+1] = frame.Data[i*3+1] // G
		img.Pix[i*4+2] = frame.Data[i*3+2] // B
		img.Pix[i*4+3] = 255               // A (opaque)
	}

	return img, nil
}

// Stats returns current save statistics.
func (fs *FrameSaver) Stats() (saved, dropped uint64) {
	return fs.framesSaved.Load(), fs.framesDropped.Load()
}
