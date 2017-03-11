package phonelab

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

// A processor that totals up the loglines and emits it.
type totalProcessor struct {
	source Processor
}

func (proc *totalProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	go func() {
		inChan := proc.source.Process()
		total := 0
		for _ = range inChan {
			total += 1
		}
		outChan <- total
		close(outChan)
	}()

	return outChan
}

// A data processor that collects the totals from all the sources.
type totalDataCollector struct {
	results []int
	sync.Mutex
}

func newTestDataCollector() *totalDataCollector {
	return &totalDataCollector{
		results: make([]int, 0),
	}
}

func (t *totalDataCollector) BuildPipeline(source *PipelineSourceInstance) *Pipeline {
	// Normally, there will be at least one node before the source.
	// We'll fake that with a pass through handler.
	processor := &totalProcessor{source.Processor}

	return &Pipeline{
		LastHop: processor,
	}
}

func (t *totalDataCollector) OnData(data interface{}) {
	t.Lock()
	t.results = append(t.results, data.(int))
	t.Unlock()
}

func (t *totalDataCollector) Finish() {}

// Simple Generator
type emitterGenerator struct {
	sizes []int
}

func (e *emitterGenerator) Process() <-chan *PipelineSourceInstance {
	outChan := make(chan *PipelineSourceInstance)

	go func() {
		for _, val := range e.sizes {
			outChan <- &PipelineSourceInstance{
				Processor: &emitter{val},
				Info:      make(PipelineSourceInfo),
			}
		}
		close(outChan)
	}()

	return outChan
}

func TestPipeline(t *testing.T) {
	assert := assert.New(t)

	dataProc := newTestDataCollector()

	runner := NewRunner(
		&emitterGenerator{
			sizes: []int{10, 20, 50},
		},
		dataProc,
		dataProc,
	)

	runner.Run()

	totals := make(map[int]bool)

	for _, total := range dataProc.results {
		totals[total] = true
	}

	assert.Equal(3, len(totals))
	assert.True(totals[10])
	assert.True(totals[20])
	assert.True(totals[50])
}
