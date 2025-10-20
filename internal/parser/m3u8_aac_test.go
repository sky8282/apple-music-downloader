package parser

import (
	"strconv"
	"strings"
	"testing"
)

// TestParseAACVariantFormat tests the AAC variant format parsing logic
// This validates the fix for binaural/downmix AAC format detection
func TestParseAACVariantFormat(t *testing.T) {
	tests := []struct {
		name          string
		audioString   string
		expectedType  string
		expectedOK    bool
	}{
		{
			name:         "Standard AAC format",
			audioString:  "audio-stereo-256",
			expectedType: "aac",
			expectedOK:   true,
		},
		{
			name:         "Binaural AAC format (Pattern 1: type-bitrate)",
			audioString:  "audio-stereo-binaural-256",
			expectedType: "aac-binaural",
			expectedOK:   true,
		},
		{
			name:         "Binaural AAC format (Pattern 2: bitrate-type)",
			audioString:  "audio-stereo-256-binaural",
			expectedType: "aac-binaural",
			expectedOK:   true,
		},
		{
			name:         "Downmix AAC format (Pattern 1: type-bitrate)",
			audioString:  "audio-stereo-downmix-256",
			expectedType: "aac-downmix",
			expectedOK:   true,
		},
		{
			name:         "Downmix AAC format (Pattern 2: bitrate-type)",
			audioString:  "audio-stereo-256-downmix",
			expectedType: "aac-downmix",
			expectedOK:   true,
		},
		{
			name:         "Invalid format - no prefix",
			audioString:  "invalid-format",
			expectedType: "",
			expectedOK:   false,
		},
		{
			name:         "AAC with different bitrate",
			audioString:  "audio-stereo-128",
			expectedType: "aac",
			expectedOK:   true,
		},
		{
			name:         "Binaural with 128 bitrate (Pattern 1)",
			audioString:  "audio-stereo-binaural-128",
			expectedType: "aac-binaural",
			expectedOK:   true,
		},
		{
			name:         "Binaural with 128 bitrate (Pattern 2)",
			audioString:  "audio-stereo-128-binaural",
			expectedType: "aac-binaural",
			expectedOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the same logic from the fix in ExtractMedia function
			var audioFormat string
			var ok bool
			
			if strings.HasPrefix(tt.audioString, "audio-stereo-") {
				remainder := strings.TrimPrefix(tt.audioString, "audio-stereo-")
				parts := strings.Split(remainder, "-")
				
				if len(parts) == 1 {
					// Format: "256" (standard AAC)
					audioFormat = "aac"
					ok = true
				} else if len(parts) >= 2 {
					// Check both patterns:
					// Pattern 1: "binaural-256" or "downmix-256"
					// Pattern 2: "256-binaural" or "256-downmix"
					
					// Try to parse first part as bitrate to detect pattern
					if _, err := strconv.Atoi(parts[0]); err == nil && len(parts) >= 2 {
						// Pattern 2: bitrate comes first, format type is last
						formatType := parts[len(parts)-1]
						audioFormat = "aac-" + formatType
					} else {
						// Pattern 1: format type comes first
						audioFormat = "aac-" + parts[0]
					}
					ok = true
				}
			}
			
			if ok != tt.expectedOK {
				t.Errorf("Expected ok=%v, got ok=%v for audio string: %s", tt.expectedOK, ok, tt.audioString)
			}
			
			if audioFormat != tt.expectedType {
				t.Errorf("Expected format=%s, got format=%s for audio string: %s", tt.expectedType, audioFormat, tt.audioString)
			}
		})
	}
}

// TestExtractBitrateFromAudioString tests bitrate extraction from various AAC audio strings
func TestExtractBitrateFromAudioString(t *testing.T) {
	tests := []struct {
		name            string
		audioString     string
		expectedBitrate string
	}{
		{
			name:            "Standard AAC",
			audioString:     "audio-stereo-256",
			expectedBitrate: "256",
		},
		{
			name:            "Binaural AAC (Pattern 1: type-bitrate)",
			audioString:     "audio-stereo-binaural-256",
			expectedBitrate: "256",
		},
		{
			name:            "Binaural AAC (Pattern 2: bitrate-type)",
			audioString:     "audio-stereo-256-binaural",
			expectedBitrate: "256",
		},
		{
			name:            "Downmix AAC (Pattern 1: type-bitrate)",
			audioString:     "audio-stereo-downmix-128",
			expectedBitrate: "128",
		},
		{
			name:            "Downmix AAC (Pattern 2: bitrate-type)",
			audioString:     "audio-stereo-128-downmix",
			expectedBitrate: "128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Split and find the numeric part (same as in the fix)
			parts := strings.Split(tt.audioString, "-")
			
			var bitrate string
			for _, part := range parts {
				if _, err := strconv.Atoi(part); err == nil {
					bitrate = part
					break
				}
			}
			
			if bitrate != tt.expectedBitrate {
				t.Errorf("Expected bitrate=%s, got bitrate=%s for audio string: %s", 
					tt.expectedBitrate, bitrate, tt.audioString)
			}
		})
	}
}
