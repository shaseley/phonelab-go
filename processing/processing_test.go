package processing

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type passThroughHandler struct{}

func (p *passThroughHandler) Handle(log interface{}) interface{} { return log }

type emitter struct {
	HowMany int
}

func (e *emitter) Process() <-chan interface{} {
	dest := make(chan interface{})

	go func() {
		for i := 0; i < e.HowMany; i++ {
			dest <- i
		}
		close(dest)
	}()
	return dest
}

func TestSimpleOperator(t *testing.T) {

	assert := assert.New(t)

	handler := &passThroughHandler{}
	processor := NewSimpleOperator(&emitter{100}, handler)

	expected := 0
	resChan := processor.Process()
	for val := range resChan {
		assert.Equal(expected, val.(int))
		expected += 1
	}
	assert.Equal(expected, 100)
}

func TestMuxer(t *testing.T) {
	assert := assert.New(t)

	const nmux = 10
	const iter = 100

	processor := NewMuxer(&emitter{iter}, nmux)

	wait := make(chan int)

	for i := 0; i < nmux; i++ {
		go func() {
			expected := 0
			resChan := processor.Process()
			for val := range resChan {
				assert.Equal(expected, val.(int))
				expected += 1
			}
			assert.Equal(expected, 100)
			t.Log("Muxer Done!")
			wait <- 1
		}()
	}

	for i := 0; i < nmux; i++ {
		<-wait
	}

}

func TestDemuxer(t *testing.T) {
	assert := assert.New(t)

	const nemit = 10
	const iter = 100

	sources := make([]Processor, nemit)

	for i := 0; i < nemit; i++ {
		sources[i] = &emitter{iter}
	}

	processor := NewDemuxer(sources)

	results := make([]int, iter)
	resChan := processor.Process()

	for val := range resChan {
		ival := val.(int)
		assert.True(ival >= 0)
		assert.True(ival < iter)
		results[ival] += 1
	}

	for _, val := range results {
		assert.Equal(nemit, val)
	}

}
