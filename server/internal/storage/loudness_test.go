package storage

import (
	"math"
	"testing"
)

// sineWave returns one channel of a sine at the given frequency, amplitude and length.
func sineWave(frequency, amplitude float64, sampleRate int, seconds float64) []float64 {
	samples := make([]float64, int(seconds*float64(sampleRate)))
	for index := range samples {
		samples[index] = amplitude * math.Sin(2*math.Pi*frequency*float64(index)/float64(sampleRate))
	}
	return samples
}

func TestIntegratedLoudness(t *testing.T) {
	t.Run("should read a full-scale 997 Hz mono sine as -3.01 LUFS", func(t *testing.T) {
		// ITU-R BS.1770 defines this reference point for mono.
		tone := sineWave(997, 1, 48000, 2)

		loudness := integratedLoudness([][]float64{tone}, 48000)

		if math.Abs(loudness-(-3.01)) > 0.05 {
			t.Fatalf("expected -3.01 LUFS, got %.3f", loudness)
		}
	})

	t.Run("should add 3 LU when the same tone plays on two channels", func(t *testing.T) {
		tone := sineWave(997, 0.1, 48000, 2)

		mono := integratedLoudness([][]float64{tone}, 48000)
		stereo := integratedLoudness([][]float64{tone, tone}, 48000)

		if math.Abs(stereo-mono-3.01) > 0.05 {
			t.Fatalf("expected stereo 3.01 LU above mono, got mono %.3f stereo %.3f", mono, stereo)
		}
	})

	t.Run("should measure a clip shorter than one gating block", func(t *testing.T) {
		tone := sineWave(997, 1, 48000, 0.2)

		loudness := integratedLoudness([][]float64{tone}, 48000)

		if math.Abs(loudness-(-3.01)) > 0.1 {
			t.Fatalf("expected about -3.01 LUFS, got %.3f", loudness)
		}
	})

	t.Run("should report negative infinity for silence", func(t *testing.T) {
		silence := make([]float64, 48000)

		loudness := integratedLoudness([][]float64{silence}, 48000)

		if !math.IsInf(loudness, -1) {
			t.Fatalf("expected -Inf, got %.3f", loudness)
		}
	})
}
