package storage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mp3 "github.com/hajimehoshi/go-mp3"
)

const (
	defaultWelcomeSoundDir = "./data/uploads/welcome-sounds"
	welcomeSoundExtension  = ".mp3"
	welcomeSoundURLPrefix  = "/uploads/welcome-sounds/"

	// Clips quieter than this carry no audible sound for the room.
	welcomeSoundSilenceFloorLufs = -50.0

	// go-mp3 always decodes to 16-bit little-endian stereo.
	decodedBytesPerFrame = 4
	decodedSampleScale   = 32768.0
	decodeChunkBytes     = 16 * 1024
)

// Rejection reasons. Like avatar errors, their messages go back to the uploader as they are.
var (
	ErrWelcomeSoundTooLarge    = errors.New("sound file is too large")
	ErrWelcomeSoundUnsupported = errors.New("file is not an MP3 sound")
	ErrWelcomeSoundSampleRate  = errors.New("export the sound at 16, 24, 32, 44.1 or 48 kHz")
	ErrWelcomeSoundTooLong     = errors.New("sound is too long")
	ErrWelcomeSoundSilent      = errors.New("sound is silent")
	ErrWelcomeSoundPeakTooHigh = errors.New("sound peaks too high; lower the volume and export it again")
	ErrWelcomeSoundTooLoud     = errors.New("sound is too loud; lower the volume and export it again")
)

// Sample rates go-mp3 v0.3.4 decodes in line with ffmpeg. It mis-measures 22.05 kHz clips at
// 32 kbps and below by up to 8 dB of peak, and cannot decode 8, 11.025 or 12 kHz at all.
var welcomeSoundSampleRates = map[int]bool{16000: true, 24000: true, 32000: true, 44100: true, 48000: true}

var welcomeSoundDir = defaultWelcomeSoundDir

// WelcomeSoundLimits are the effective upload limits from system settings.
type WelcomeSoundLimits struct {
	MaxBytes           int
	MaxDurationSeconds int
	MaxPeakDbfs        float64
	MaxLoudnessLufs    float64
}

// WelcomeSoundAnalysis is what one decode pass measured on an accepted clip.
type WelcomeSoundAnalysis struct {
	Duration     time.Duration
	PeakDbfs     float64
	LoudnessLufs float64
}

func WelcomeSoundDir() string {
	return welcomeSoundDir
}

// SetWelcomeSoundDirForTest overrides the welcome sound directory; restore() reverts it.
func SetWelcomeSoundDirForTest(dir string) (restore func()) {
	previous := welcomeSoundDir
	welcomeSoundDir = dir
	return func() { welcomeSoundDir = previous }
}

// looksLikeMP3 checks for an ID3 tag or an MPEG audio frame sync at the start of the file.
func looksLikeMP3(data []byte) bool {
	if bytes.HasPrefix(data, []byte("ID3")) {
		return true
	}
	return len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0
}

// decodeWelcomeSound decodes at most maxSeconds of audio and reports whether the clip ran past it,
// so an overlong clip never decodes in full. A mono MP3 comes back as two identical channels,
// so those are folded into one.
func decodeWelcomeSound(data []byte, maxSeconds int) (channels [][]float64, sampleRate int, truncated bool, err error) {
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, 0, false, ErrWelcomeSoundUnsupported
	}
	sampleRate = decoder.SampleRate()
	if sampleRate <= 0 {
		return nil, 0, false, ErrWelcomeSoundUnsupported
	}
	if !welcomeSoundSampleRates[sampleRate] {
		return nil, 0, false, ErrWelcomeSoundSampleRate
	}
	maxFrames := maxSeconds * sampleRate

	var left, right []float64
	identical := true
	buffer := make([]byte, decodeChunkBytes)
	for {
		read, readErr := io.ReadFull(decoder, buffer)
		for offset := 0; offset+decodedBytesPerFrame <= read; offset += decodedBytesPerFrame {
			if len(left) == maxFrames {
				return nil, 0, true, nil
			}
			leftSample := int16(binary.LittleEndian.Uint16(buffer[offset:]))
			rightSample := int16(binary.LittleEndian.Uint16(buffer[offset+2:]))
			identical = identical && leftSample == rightSample
			left = append(left, float64(leftSample)/decodedSampleScale)
			right = append(right, float64(rightSample)/decodedSampleScale)
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return nil, 0, false, ErrWelcomeSoundUnsupported
		}
	}
	if len(left) == 0 {
		return nil, 0, false, ErrWelcomeSoundUnsupported
	}
	if identical {
		return [][]float64{left}, sampleRate, false, nil
	}
	return [][]float64{left, right}, sampleRate, false, nil
}

// AnalyzeWelcomeSound decodes the clip once and checks it against every limit.
func AnalyzeWelcomeSound(data []byte, limits WelcomeSoundLimits) (WelcomeSoundAnalysis, error) {
	if len(data) > limits.MaxBytes {
		return WelcomeSoundAnalysis{}, ErrWelcomeSoundTooLarge
	}
	if !looksLikeMP3(data) {
		return WelcomeSoundAnalysis{}, ErrWelcomeSoundUnsupported
	}

	channels, sampleRate, truncated, err := decodeWelcomeSound(data, limits.MaxDurationSeconds)
	if err != nil {
		return WelcomeSoundAnalysis{}, err
	}
	if truncated {
		return WelcomeSoundAnalysis{}, ErrWelcomeSoundTooLong
	}

	analysis := WelcomeSoundAnalysis{
		Duration:     time.Duration(len(channels[0])) * time.Second / time.Duration(sampleRate),
		PeakDbfs:     samplePeakDbfs(channels),
		LoudnessLufs: integratedLoudness(channels, sampleRate),
	}
	switch {
	case math.IsInf(analysis.LoudnessLufs, -1) || analysis.LoudnessLufs < welcomeSoundSilenceFloorLufs:
		return analysis, ErrWelcomeSoundSilent
	case analysis.PeakDbfs >= limits.MaxPeakDbfs:
		return analysis, ErrWelcomeSoundPeakTooHigh
	case analysis.LoudnessLufs > limits.MaxLoudnessLufs:
		return analysis, ErrWelcomeSoundTooLoud
	}
	return analysis, nil
}

// SaveUserWelcomeSound validates the clip and stores it as the user's only welcome sound.
func SaveUserWelcomeSound(userID string, data []byte, limits WelcomeSoundLimits) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user id required")
	}
	if _, err := AnalyzeWelcomeSound(data, limits); err != nil {
		return "", err
	}

	dir := WelcomeSoundDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create welcome sound dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, userID+welcomeSoundExtension), data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write welcome sound: %w", err)
	}

	version := strconv.FormatInt(time.Now().Unix(), 10)
	return welcomeSoundURLPrefix + userID + welcomeSoundExtension + "?v=" + version, nil
}

func DeleteUserWelcomeSoundFile(userID string) error {
	if userID == "" {
		return nil
	}
	err := os.Remove(filepath.Join(WelcomeSoundDir(), userID+welcomeSoundExtension))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ResolveWelcomeSoundFile(pathParam string) (string, error) {
	pathParam = strings.Split(pathParam, "?")[0]
	if pathParam == "" || strings.Contains(pathParam, "..") {
		return "", fmt.Errorf("invalid path")
	}
	dir := filepath.Clean(WelcomeSoundDir())
	resolved := filepath.Join(dir, pathParam)
	if !strings.HasPrefix(resolved, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path")
	}
	return resolved, nil
}
