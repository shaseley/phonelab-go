package phonelab

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

type testDataProcessor struct {
	sinks []*testSink
	sync.Mutex
}

func (t *testDataProcessor) BuildPipeline(source *PipelineSourceInstance) Pipeline {
	// Normally, there will be at least one node before the source.
	// We'll fake that with a pass through handler.
	handler := &passThroughHandler{}
	processor := NewSimpleProcessor(source.Processor, handler)

	sink := &testSink{
		Source: processor,
		Data:   make([]interface{}, 0),
	}

	t.Lock()
	t.sinks = append(t.sinks, sink)
	t.Unlock()
	return []PipelineSink{sink}
}

func (t *testDataProcessor) Finish() {}

// Simple data sink that collects everything
type testSink struct {
	Source Processor
	Data   []interface{}
}

func (t *testSink) GetSource() Processor    { return t.Source }
func (t *testSink) OnData(data interface{}) { t.Data = append(t.Data, data) }
func (t *testSink) OnFinish()               {}

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

	dataProc := &testDataProcessor{
		sinks: make([]*testSink, 0),
	}

	runner := NewRunner(
		&emitterGenerator{
			sizes: []int{10, 20, 50},
		},
		dataProc,
	)
	runner.Run()
	for _, exp := range dataProc.sinks {
		assert.NotNil(exp)
		t.Log(len(exp.Data))
	}
}
