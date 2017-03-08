package phonelab

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

type passThroughHandler struct{}

func (p *passThroughHandler) Handle(log interface{}) interface{} { return log }
func (p *passThroughHandler) Finish()                            {}

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
	processor := NewSimpleProcessor(&emitter{100}, handler)

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

func TestStringFilter(t *testing.T) {
	assert := assert.New(t)

	filters := []StringFilter{
		func(log string) bool {
			return strings.Index(log, "foo:") >= 0
		},
		func(log string) bool {
			return strings.Index(log, "bar:") >= 0
		},
	}

	var tests = []struct {
		log      string
		expected bool
	}{
		{"foo: 1234", true},
		{"bar: 1234", true},
		{"foo: bar: 1234", true},
		{"nope: 1234", false},
	}

	handler := &StringFilterHandler{
		filters,
	}

	for _, test := range tests {
		res := handler.Handle(test.log)
		if test.expected {
			assert.Equal(test.log, res)
		} else {
			assert.Nil(res)
		}
	}
}

/*
func TestStringFilterProcessor(t *testing.T) {
	assert := assert.New(t)

	filters := []StringFilter{
		func(log string) bool {
			return strings.Index(log, "foo:") >= 0
		},
		func(log string) bool {
			return strings.Index(log, "bar:") >= 0
		},
		func(log string) bool {
			return strings.Index(log, "somelongtag:") >= 0 ||
				strings.Index(log, "othertag")
		},
	}

	var tests = []struct {
		log      string
		expected bool
	}{
		{"foo: 1234", true},
		{"bar: 1234", true},
		{"foo: bar: 1234", true},
		{"somelongtag: 1234", true},
		{"othertag: 1234", true},
		{"nope: 1234", false},
		{"othertag?: 1234", false},
	}

	handler := &StringFilterHandler{
		filters,
	}

	for _, test := range tests {
		res := handler.Handle(test.log)
		if test.expected {
			assert.Equal(test.log, res)
		} else {
			assert.Nil(res)
		}
	}

}
*/
