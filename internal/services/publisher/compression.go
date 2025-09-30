package publisher

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/golang/snappy"
	"github.com/pierrec/lz4/v4"
)

// Compressor defines the interface for message compression
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Type() string
}

// NewCompressor creates a new compressor based on the specified type
func NewCompressor(compressionType string, level int) Compressor {
	switch compressionType {
	case "snappy":
		return &snappyCompressor{}
	case "gzip":
		return &gzipCompressor{level: level}
	case "lz4":
		return &lz4Compressor{}
	case "none", "":
		return &noopCompressor{}
	default:
		return &noopCompressor{}
	}
}

// noopCompressor performs no compression
type noopCompressor struct{}

func (c *noopCompressor) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (c *noopCompressor) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

func (c *noopCompressor) Type() string {
	return "none"
}

// snappyCompressor uses Snappy compression (fast, moderate compression)
type snappyCompressor struct{}

func (c *snappyCompressor) Compress(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

func (c *snappyCompressor) Decompress(data []byte) ([]byte, error) {
	return snappy.Decode(nil, data)
}

func (c *snappyCompressor) Type() string {
	return "snappy"
}

// gzipCompressor uses GZIP compression (slower, better compression)
type gzipCompressor struct {
	level int
}

func (c *gzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	
	// Create gzip writer with specified compression level
	writer, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	
	// Write and close
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to write data: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}
	
	return buf.Bytes(), nil
}

func (c *gzipCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()
	
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	
	return decompressed, nil
}

func (c *gzipCompressor) Type() string {
	return "gzip"
}

// lz4Compressor uses LZ4 compression (very fast, good compression)
type lz4Compressor struct{}

func (c *lz4Compressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	
	writer := lz4.NewWriter(&buf)
	
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to write data: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}
	
	return buf.Bytes(), nil
}

func (c *lz4Compressor) Decompress(data []byte) ([]byte, error) {
	reader := lz4.NewReader(bytes.NewReader(data))
	
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	
	return decompressed, nil
}

func (c *lz4Compressor) Type() string {
	return "lz4"
}

// CompressionStats provides compression statistics
type CompressionStats struct {
	OriginalSize   int     `json:"original_size"`
	CompressedSize int     `json:"compressed_size"`
	CompressionRatio float64 `json:"compression_ratio"`
	Type           string  `json:"type"`
}

// GetCompressionStats calculates compression statistics
func GetCompressionStats(original, compressed []byte, compressionType string) CompressionStats {
	originalSize := len(original)
	compressedSize := len(compressed)
	
	var ratio float64
	if originalSize > 0 {
		ratio = float64(compressedSize) / float64(originalSize)
	}
	
	return CompressionStats{
		OriginalSize:     originalSize,
		CompressedSize:   compressedSize,
		CompressionRatio: ratio,
		Type:             compressionType,
	}
}

// ShouldCompress determines if data should be compressed based on size and type
func ShouldCompress(data []byte, compressionType string, minSize int) bool {
	if compressionType == "none" || compressionType == "" {
		return false
	}
	
	if len(data) < minSize {
		return false
	}
	
	// Check if data is already compressed (simple heuristic)
	if isLikelyCompressed(data) {
		return false
	}
	
	return true
}

// isLikelyCompressed performs a simple heuristic to detect if data is already compressed
func isLikelyCompressed(data []byte) bool {
	if len(data) < 10 {
		return false
	}
	
	// Check for common compression magic numbers
	
	// GZIP magic number
	if data[0] == 0x1f && data[1] == 0x8b {
		return true
	}
	
	// PNG magic number
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return true
	}
	
	// JPEG magic number
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return true
	}
	
	// ZIP magic number
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x50, 0x4B, 0x03, 0x04}) {
		return true
	}
	
	// Simple entropy check - if data has low entropy, it's likely not compressed
	entropy := calculateEntropy(data[:min(len(data), 1024)]) // Check first 1KB
	return entropy > 7.0 // High entropy suggests compressed or encrypted data
}

// calculateEntropy calculates the Shannon entropy of data
func calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	
	// Count frequency of each byte
	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}
	
	// Calculate entropy
	var entropy float64
	length := float64(len(data))
	
	for _, count := range freq {
		if count > 0 {
			p := float64(count) / length
			entropy -= p * log2(p)
		}
	}
	
	return entropy
}

// log2 calculates logarithm base 2
func log2(x float64) float64 {
	return 0.6931471805599453 * logNatural(x) // ln(2) * ln(x)
}

// Simple natural logarithm approximation
func logNatural(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Using Taylor series approximation for ln(1+x) where x is close to 0
	// For simplicity, using a basic approximation
	y := (x - 1) / (x + 1)
	y2 := y * y
	return 2 * y * (1 + y2/3 + y2*y2/5 + y2*y2*y2/7)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}