# AAC Binaural/Downmix Fix Validation Guide

## Overview
This fix resolves the bitstream parsing error that occurred when downloading Apple Music tracks in `aac-binaural` or `aac-downmix` formats.

## The Problem
**Error Message:**
```
General compliance : Bitstream parsing ran out of data to read before the end of the syntax was reached, 
most probably the bitstream is malformed (conf, offset 0x180DA)
```

**Root Cause:**
The AAC variant selection logic used a regex pattern `audio-stereo-\d+` that only matched standard AAC format (`audio-stereo-256`). It failed to match binaural and downmix formats:
- `audio-stereo-binaural-256`
- `audio-stereo-downmix-256`

This caused the downloader to fall back to an incompatible variant, resulting in malformed bitstream errors.

## The Solution
The fix replaces regex matching with prefix-based string parsing that correctly handles all AAC format variants:
1. Standard AAC: `audio-stereo-256` → matches `aac`
2. Binaural AAC: `audio-stereo-binaural-256` → matches `aac-binaural`
3. Downmix AAC: `audio-stereo-downmix-256` → matches `aac-downmix`

## How to Validate

### 1. Using Debug Mode
Enable debug mode to see which AAC variants are available and which one is selected:

```bash
./main --aac --aac-type aac-binaural --debug [APPLE_MUSIC_URL]
```

You should see output like:
```
Debug: All Available Variants:
┌──────────┬────────────────────────────┬───────────┐
│  CODEC   │           AUDIO            │ BANDWIDTH │
├──────────┼────────────────────────────┼───────────┤
│ mp4a.40.2│ audio-stereo-256           │ 256000    │
│ mp4a.40.2│ audio-stereo-binaural-256  │ 256000    │
│ mp4a.40.2│ audio-stereo-downmix-256   │ 256000    │
└──────────┴────────────────────────────┴───────────┘

AAC Variants Found:
  - audio-stereo-256
  - audio-stereo-binaural-256
  - audio-stereo-downmix-256

Selected AAC variant: [URL] (audio: audio-stereo-binaural-256, type: aac-binaural)
```

### 2. Download Test
Try downloading with each AAC format type:

```bash
# Standard AAC (should work as before)
./main --aac --aac-type aac [APPLE_MUSIC_URL]

# Binaural AAC (now fixed)
./main --aac --aac-type aac-binaural [APPLE_MUSIC_URL]

# Downmix AAC (now fixed)
./main --aac --aac-type aac-downmix [APPLE_MUSIC_URL]
```

### 3. Verify Output File
After download, verify the file is not corrupted:

```bash
# Check file integrity with ffmpeg
ffmpeg -v error -i output.m4a -f null - 2>&1

# Or get detailed info
ffmpeg -i output.m4a 2>&1 | grep -E "Stream|Duration|Audio"

# Or use mp4info
mp4info output.m4a
```

If the file is valid, you should see no errors or warnings about malformed bitstreams.

### 4. Run Unit Tests
The fix includes comprehensive unit tests:

```bash
# Run AAC variant parsing tests
go test -v ./internal/parser -run TestParseAACVariantFormat

# Run bitrate extraction tests
go test -v ./internal/parser -run TestExtractBitrateFromAudioString

# Run all parser tests
go test -v ./internal/parser/...
```

All tests should pass:
```
=== RUN   TestParseAACVariantFormat
--- PASS: TestParseAACVariantFormat (0.00s)
=== RUN   TestExtractBitrateFromAudioString
--- PASS: TestExtractBitrateFromAudioString (0.00s)
PASS
```

## Expected Behavior After Fix

### Before the Fix
- `--aac-type aac-binaural` → Falls back to first variant → Bitstream parsing error
- `--aac-type aac-downmix` → Falls back to first variant → Bitstream parsing error

### After the Fix
- `--aac-type aac-binaural` → Correctly selects binaural variant → Download succeeds
- `--aac-type aac-downmix` → Correctly selects downmix variant → Download succeeds
- `--aac-type aac` → Works as before → Download succeeds

## Error Handling

### If Format Not Available
If you request a format that's not available in the playlist, you'll see a warning:

```
Warning: Requested AAC type 'aac-binaural' not found in playlist. Available AAC variants:
  - audio-stereo-256
  - audio-stereo-downmix-256
Falling back to first available variant. This may cause bitstream parsing errors.
```

This helps diagnose issues when specific formats aren't available for a track.

## Technical Details

### Files Modified
- `internal/parser/m3u8.go`: Fixed AAC variant detection logic (lines 277-333)
- `internal/parser/m3u8_aac_test.go`: Added comprehensive unit tests

### Code Changes
The key change is replacing:
```go
// Old (broken) code
aacregex := regexp.MustCompile(`audio-stereo-\d+`)
replaced := aacregex.ReplaceAllString(variant.Audio, "aac")
if replaced == *core.Aac_type {
    // ... select variant
}
```

With:
```go
// New (fixed) code
var audioFormat string
if strings.HasPrefix(variant.Audio, "audio-stereo-") {
    remainder := strings.TrimPrefix(variant.Audio, "audio-stereo-")
    parts := strings.Split(remainder, "-")
    
    if len(parts) >= 2 {
        // Format: "binaural-256" or "downmix-256"
        audioFormat = "aac-" + parts[0]
    } else if len(parts) == 1 {
        // Format: "256" (standard AAC)
        audioFormat = "aac"
    }
}

if audioFormat == *core.Aac_type {
    // ... select variant
}
```

## Security
- CodeQL analysis: 0 vulnerabilities found ✅
- No new dependencies added
- No security-sensitive code paths modified

## Troubleshooting

### Issue: Still getting bitstream errors
1. Enable debug mode to see which variant is being selected
2. Verify the track actually has the format you're requesting
3. Check if you're using the latest version with the fix

### Issue: Format not available warning
This is expected if the track doesn't have that specific format. Try:
- Standard AAC: `--aac-type aac`
- Check available formats in debug mode

### Issue: Tests fail
Make sure you're in the repository root and have Go installed:
```bash
go version  # Should show Go 1.16 or later
go test -v ./internal/parser/...
```

## Questions or Issues?
If you encounter any issues with this fix, please provide:
1. The command you ran (with `--debug` flag)
2. The debug output showing available variants
3. Any error messages
4. The output file information from `ffmpeg -i`
