package phonelab

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type sequenceNum int

func (sn sequenceNum) MonotonicTimestamp() float64 {
	return float64(int(sn))
}

type sequenceEmitter struct {
	numbers []sequenceNum
}

func (s *sequenceEmitter) Process() <-chan interface{} {
	outChan := make(chan interface{})

	go func() {
		for _, sn := range s.numbers {
			outChan <- sn
		}
		close(outChan)
	}()

	return outChan
}

func TestTimeweaverOddEven(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	odds := make([]sequenceNum, 0)
	for i := 1; i <= 100; i += 2 {
		odds = append(odds, sequenceNum(i))
	}
	oddEmitter := &sequenceEmitter{odds}

	evens := make([]sequenceNum, 0)
	for i := 2; i <= 100; i += 2 {
		evens = append(evens, sequenceNum(i))
	}
	evenEmitter := &sequenceEmitter{evens}

	results := make([]sequenceNum, 0)
	tw := NewTimeweaverProcessor(oddEmitter, evenEmitter)

	outChan := tw.Process()
	for sn := range outChan {
		results = append(results, sn.(sequenceNum))
	}

	// Check
	require.Equal(100, len(results))

	for i := 0; i < 100; i++ {
		assert.Equal(i+1, int(results[i]))
	}
}

func TestTimeweaverChunks(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	lhsEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 1; i <= 25; i++ {
		lhsEmitter.numbers = append(lhsEmitter.numbers, sequenceNum(i))
	}
	for i := 51; i <= 75; i++ {
		lhsEmitter.numbers = append(lhsEmitter.numbers, sequenceNum(i))
	}

	rhsEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 26; i <= 50; i++ {
		rhsEmitter.numbers = append(rhsEmitter.numbers, sequenceNum(i))
	}
	for i := 76; i <= 100; i++ {
		rhsEmitter.numbers = append(rhsEmitter.numbers, sequenceNum(i))
	}

	results := make([]sequenceNum, 0)
	tw := NewTimeweaverProcessor(lhsEmitter, rhsEmitter)

	outChan := tw.Process()
	for sn := range outChan {
		results = append(results, sn.(sequenceNum))
	}

	// Check
	require.Equal(100, len(results))

	for i := 0; i < 100; i++ {
		assert.Equal(i+1, int(results[i]))
	}
}

func TestTimeweaverSame(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	lhsEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 1; i <= 100; i++ {
		lhsEmitter.numbers = append(lhsEmitter.numbers, sequenceNum(i))
	}

	rhsEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 1; i <= 100; i++ {
		rhsEmitter.numbers = append(rhsEmitter.numbers, sequenceNum(i))
	}

	results := make([]sequenceNum, 0)
	tw := NewTimeweaverProcessor(lhsEmitter, rhsEmitter)

	outChan := tw.Process()
	for sn := range outChan {
		results = append(results, sn.(sequenceNum))
	}

	// Check
	require.Equal(200, len(results))

	expected := 1
	pos := 0

	for pos < 200 {
		assert.Equal(expected, int(results[pos]))
		pos += 1
		assert.Equal(expected, int(results[pos]))
		pos += 1
		expected += 1
	}
}

func TestTimeweaverMult(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	// Chunks
	lhsEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 1; i <= 25; i++ {
		lhsEmitter.numbers = append(lhsEmitter.numbers, sequenceNum(i))
	}
	for i := 51; i <= 75; i++ {
		lhsEmitter.numbers = append(lhsEmitter.numbers, sequenceNum(i))
	}

	rhsEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 26; i <= 50; i++ {
		rhsEmitter.numbers = append(rhsEmitter.numbers, sequenceNum(i))
	}
	for i := 76; i <= 100; i++ {
		rhsEmitter.numbers = append(rhsEmitter.numbers, sequenceNum(i))
	}

	results := make([]sequenceNum, 0)
	chunkProc := NewTimeweaverProcessor(lhsEmitter, rhsEmitter)

	// Now, repeat all in a separate emiiter
	fullEmitter := &sequenceEmitter{make([]sequenceNum, 0)}
	for i := 1; i <= 100; i++ {
		fullEmitter.numbers = append(fullEmitter.numbers, sequenceNum(i))
	}

	tw := NewTimeweaverProcessor(chunkProc, fullEmitter)

	outChan := tw.Process()
	for sn := range outChan {
		results = append(results, sn.(sequenceNum))
	}

	// Check
	require.Equal(200, len(results))
	expected := 1
	pos := 0

	for pos < 200 {
		assert.Equal(expected, int(results[pos]))
		pos += 1
		assert.Equal(expected, int(results[pos]))
		pos += 1
		expected += 1
	}

}
