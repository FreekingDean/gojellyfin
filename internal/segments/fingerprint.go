package segments

import (
	"math"
	"math/bits"
	"math/cmplx"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
)

const (
	frameSize = 4096
	hopSize   = 512
	bandCount = 33
	lowestHz  = 300.0
	highestHz = 2000.0

	framesPerSecond = float64(ffmpeg.AudioSampleRate) / hopSize
)

type spectrum struct {
	window  []float64
	twiddle []complex128
	edges   []int
}

func newSpectrum() *spectrum {
	window := make([]float64, frameSize)
	for i := range window {
		window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(frameSize-1))
	}

	twiddle := make([]complex128, frameSize/2)
	for i := range twiddle {
		twiddle[i] = cmplx.Exp(complex(0, -2*math.Pi*float64(i)/float64(frameSize)))
	}

	edges := make([]int, bandCount+1)
	for i := range edges {
		hz := lowestHz * math.Pow(highestHz/lowestHz, float64(i)/bandCount)
		edges[i] = int(math.Round(hz * frameSize / float64(ffmpeg.AudioSampleRate)))
	}

	return &spectrum{window: window, twiddle: twiddle, edges: edges}
}

func fingerprint(samples []int16) []uint32 {
	if len(samples) < frameSize {
		return nil
	}

	bands := newSpectrum()
	frames := (len(samples)-frameSize)/hopSize + 1
	prints := make([]uint32, 0, frames)
	buffer := make([]complex128, frameSize)
	previous := make([]float64, bandCount)
	current := make([]float64, bandCount)

	for frame := range frames {
		bands.energies(samples[frame*hopSize:frame*hopSize+frameSize], buffer, current)
		if frame > 0 {
			prints = append(prints, bitsOf(previous, current))
		}
		previous, current = current, previous
	}

	return prints
}

func bitsOf(previous, current []float64) uint32 {
	var print uint32
	for band := range bandCount - 1 {
		now := current[band] - current[band+1]
		before := previous[band] - previous[band+1]
		if now-before > 0 {
			print |= 1 << band
		}
	}

	return print
}

func (s *spectrum) energies(frame []int16, buffer []complex128, into []float64) {
	for i, sample := range frame {
		buffer[i] = complex(float64(sample)*s.window[i], 0)
	}
	s.transform(buffer)

	for band := range bandCount {
		energy := 0.0
		for bin := s.edges[band]; bin < s.edges[band+1]; bin++ {
			energy += real(buffer[bin])*real(buffer[bin]) + imag(buffer[bin])*imag(buffer[bin])
		}
		into[band] = energy
	}
}

func (s *spectrum) transform(values []complex128) {
	size := len(values)
	shift := 64 - bits.TrailingZeros(uint(size))
	for i := range size {
		j := int(bits.Reverse64(uint64(i)) >> shift)
		if j > i {
			values[i], values[j] = values[j], values[i]
		}
	}

	for width := 2; width <= size; width <<= 1 {
		half, step := width/2, size/width
		for start := 0; start < size; start += width {
			for k := range half {
				odd := values[start+k+half] * s.twiddle[k*step]
				values[start+k+half] = values[start+k] - odd
				values[start+k] += odd
			}
		}
	}
}
