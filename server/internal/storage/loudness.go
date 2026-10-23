package storage

import "math"

// ITU-R BS.1770-4 constants. The K-weighting coefficients are derived per sample rate
// from the analog prototypes, as libebur128 does, so any MP3 sample rate measures correctly.
const (
	kWeightingShelfFrequency  = 1681.974450955533
	kWeightingShelfGainDb     = 3.999843853973347
	kWeightingShelfQ          = 0.7071752369554196
	kWeightingShelfVbExponent = 0.4996667741545416
	kWeightingHighPassFreq    = 38.13547087602444
	kWeightingHighPassQ       = 0.5003270373238773

	loudnessBlockSeconds        = 0.4
	loudnessBlockStepSeconds    = 0.1
	loudnessOffsetLu            = -0.691
	loudnessAbsoluteGateLufs    = -70.0
	loudnessRelativeGateDeltaLu = -10.0
)

// biquad is a second-order IIR filter in direct form I.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

func (filter *biquad) process(sample float64) float64 {
	output := filter.b0*sample + filter.b1*filter.x1 + filter.b2*filter.x2 - filter.a1*filter.y1 - filter.a2*filter.y2
	filter.x2, filter.x1 = filter.x1, sample
	filter.y2, filter.y1 = filter.y1, output
	return output
}

// kWeightingFilters returns the high-shelf and high-pass stages of the BS.1770 K-weighting.
func kWeightingFilters(sampleRate int) (*biquad, *biquad) {
	rate := float64(sampleRate)

	shelfK := math.Tan(math.Pi * kWeightingShelfFrequency / rate)
	shelfVh := math.Pow(10, kWeightingShelfGainDb/20)
	shelfVb := math.Pow(shelfVh, kWeightingShelfVbExponent)
	shelfA0 := 1 + shelfK/kWeightingShelfQ + shelfK*shelfK
	shelf := &biquad{
		b0: (shelfVh + shelfVb*shelfK/kWeightingShelfQ + shelfK*shelfK) / shelfA0,
		b1: 2 * (shelfK*shelfK - shelfVh) / shelfA0,
		b2: (shelfVh - shelfVb*shelfK/kWeightingShelfQ + shelfK*shelfK) / shelfA0,
		a1: 2 * (shelfK*shelfK - 1) / shelfA0,
		a2: (1 - shelfK/kWeightingShelfQ + shelfK*shelfK) / shelfA0,
	}

	highPassK := math.Tan(math.Pi * kWeightingHighPassFreq / rate)
	highPassA0 := 1 + highPassK/kWeightingHighPassQ + highPassK*highPassK
	highPass := &biquad{
		b0: 1,
		b1: -2,
		b2: 1,
		a1: 2 * (highPassK*highPassK - 1) / highPassA0,
		a2: (1 - highPassK/kWeightingHighPassQ + highPassK*highPassK) / highPassA0,
	}
	return shelf, highPass
}

func blockLoudness(power float64) float64 {
	return loudnessOffsetLu + 10*math.Log10(power)
}

// samplePeakDbfs returns the highest absolute sample across all channels in dBFS.
func samplePeakDbfs(channels [][]float64) float64 {
	peak := 0.0
	for _, channel := range channels {
		for _, sample := range channel {
			peak = math.Max(peak, math.Abs(sample))
		}
	}
	return 20 * math.Log10(peak)
}

// integratedLoudness measures gated BS.1770 loudness in LUFS for front channels
// (weight 1 each). It returns -Inf when nothing rises above the absolute gate.
func integratedLoudness(channels [][]float64, sampleRate int) float64 {
	if len(channels) == 0 || len(channels[0]) == 0 {
		return math.Inf(-1)
	}
	length := len(channels[0])

	weighted := make([][]float64, len(channels))
	for channelIndex, channel := range channels {
		shelf, highPass := kWeightingFilters(sampleRate)
		weighted[channelIndex] = make([]float64, length)
		for sampleIndex, sample := range channel {
			weighted[channelIndex][sampleIndex] = highPass.process(shelf.process(sample))
		}
	}

	blockLength := int(loudnessBlockSeconds * float64(sampleRate))
	stepLength := int(loudnessBlockStepSeconds * float64(sampleRate))
	// A clip shorter than one gating block is measured as a single block.
	if length < blockLength {
		blockLength = length
	}

	var blockPowers []float64
	for start := 0; start+blockLength <= length; start += stepLength {
		power := 0.0
		for _, channel := range weighted {
			sum := 0.0
			for _, sample := range channel[start : start+blockLength] {
				sum += sample * sample
			}
			power += sum / float64(blockLength)
		}
		blockPowers = append(blockPowers, power)
	}

	gatedMean := func(threshold float64) (float64, int) {
		sum, count := 0.0, 0
		for _, power := range blockPowers {
			if blockLoudness(power) > threshold {
				sum += power
				count++
			}
		}
		if count == 0 {
			return 0, 0
		}
		return sum / float64(count), count
	}

	absoluteMean, absoluteCount := gatedMean(loudnessAbsoluteGateLufs)
	if absoluteCount == 0 {
		return math.Inf(-1)
	}
	relativeGate := blockLoudness(absoluteMean) + loudnessRelativeGateDeltaLu
	relativeMean, _ := gatedMean(math.Max(relativeGate, loudnessAbsoluteGateLufs))
	return blockLoudness(relativeMean)
}
