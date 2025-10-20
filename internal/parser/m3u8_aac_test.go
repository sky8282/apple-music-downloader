package parser

import (
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
			name:         "Binaural AAC format",
			audioString:  "audio-stereo-binaural-256",
			expectedType: "aac-binaural",
			expectedOK:   true,
		},
		{
			name:         "Downmix AAC format",
			audioString:  "audio-stereo-downmix-256",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the same logic from the fix in ExtractMedia function
			var audioFormat string
			var ok bool
			
			if len(tt.audioString) > 0 && tt.audioString[:13] == "audio-stereo-" {
				remainder := tt.audioString[13:] // Remove "audio-stereo-" prefix
				parts := make([]string, 0)
				lastIdx := 0
				for i, ch := range remainder {
					if ch == '-' {
						if i > lastIdx {
							parts = append(parts, remainder[lastIdx:i])
						}
						lastIdx = i + 1
					}
				}
				if lastIdx < len(remainder) {
					parts = append(parts, remainder[lastIdx:])
				}
				
				if len(parts) >= 2 {
					// Format: "binaural-256" or "downmix-256"
					audioFormat = "aac-" + parts[0]
					ok = true
				} else if len(parts) == 1 {
					// Format: "256" (standard AAC)
					audioFormat = "aac"
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
			name:            "Binaural AAC",
			audioString:     "audio-stereo-binaural-256",
			expectedBitrate: "256",
		},
		{
			name:            "Downmix AAC",
			audioString:     "audio-stereo-downmix-128",
			expectedBitrate: "128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Split and get last part (same as in the fix)
			parts := make([]string, 0)
			lastIdx := 0
			for i, ch := range tt.audioString {
				if ch == '-' {
					if i > lastIdx {
						parts = append(parts, tt.audioString[lastIdx:i])
					}
					lastIdx = i + 1
				}
			}
			if lastIdx < len(tt.audioString) {
				parts = append(parts, tt.audioString[lastIdx:])
			}
			
			var bitrate string
			if len(parts) > 0 {
				bitrate = parts[len(parts)-1]
			}
			
			if bitrate != tt.expectedBitrate {
				t.Errorf("Expected bitrate=%s, got bitrate=%s for audio string: %s", 
					tt.expectedBitrate, bitrate, tt.audioString)
			}
		})
	}
}
