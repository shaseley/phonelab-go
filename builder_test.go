package phonelab

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"sync"
	"testing"
)

func TestRunnerConfFromString(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	specString := `
max_concurrency: 5
source:
  type: files
  sources:
    - "path/to/some/file"
    - "path/to/some/other/file"
pipeline:
  filters:
    - type: "simple"
      filter: "foo&&bar"
    - type: "custom"
      filter: "baz"
    - type: "regex"
      filter: "^sometext.*othertext.*$"
  parsers: ["tag1", "tag2"]
  processors: ["proc1"]
`
	spec, err := RunnerConfFromString(specString)
	require.Nil(err)
	require.NotNil(spec)

	expected := &PipelineRunnerConf{
		MaxConcurrency: 5,
		SourceConf: &PipelineSourceConf{
			Type: PipelineSourceFile,
			Sources: []string{
				"path/to/some/file",
				"path/to/some/other/file",
			},
		},
		PipelineConf: &PipelineConf{
			Filters: []*FilterConf{
				&FilterConf{
					Type:   FilterTypeSimple,
					Filter: "foo&&bar",
				},
				&FilterConf{
					Type:   FilterTypeCustom,
					Filter: "baz",
				},
				&FilterConf{
					Type:   FilterTypeRegex,
					Filter: "^sometext.*othertext.*$",
				},
			},
			Parsers:    []string{"tag1", "tag2"},
			Processors: []string{"proc1"},
		},
	}

	require.True(reflect.DeepEqual(expected.SourceConf, spec.SourceConf))
	require.True(reflect.DeepEqual(expected.PipelineConf, spec.PipelineConf))
	require.True(reflect.DeepEqual(expected, spec))
}

func TestRunnerConfWithPreProcFromString(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	specString := `
max_concurrency: 5
source:
  type: files
  sources:
    - "path/to/some/file"
    - "path/to/some/other/file"
preprocessors:
  - filters:
      - type: "simple"
        filter: "sf1"
    parsers: ["tag3", "tag4"]
    processors: ["preproc1"]
pipeline:
  filters:
    - type: simple
      filter: "foo&&bar"
    - type: custom
      filter: "baz"
  parsers: ["tag1", "tag2"]
  processors: ["proc1"]
`
	spec, err := RunnerConfFromString(specString)
	require.NotNil(spec)
	require.Nil(err)

	expected := &PipelineRunnerConf{
		MaxConcurrency: 5,
		SourceConf: &PipelineSourceConf{
			Type: PipelineSourceFile,
			Sources: []string{
				"path/to/some/file",
				"path/to/some/other/file",
			},
		},
		Preprocessors: []*PipelineConf{
			&PipelineConf{
				Filters: []*FilterConf{
					&FilterConf{
						Type:   FilterTypeSimple,
						Filter: "sf1",
					},
				},
				Parsers:    []string{"tag3", "tag4"},
				Processors: []string{"preproc1"},
			},
		},
		PipelineConf: &PipelineConf{
			Filters: []*FilterConf{
				&FilterConf{
					Type:   FilterTypeSimple,
					Filter: "foo&&bar",
				},
				&FilterConf{
					Type:   FilterTypeCustom,
					Filter: "baz",
				},
			},
			Parsers:    []string{"tag1", "tag2"},
			Processors: []string{"proc1"},
		},
	}

	assert.True(reflect.DeepEqual(expected.Preprocessors, spec.Preprocessors))
	assert.True(reflect.DeepEqual(expected.SourceConf, spec.SourceConf))
	assert.True(reflect.DeepEqual(expected.PipelineConf, spec.PipelineConf))
	assert.True(reflect.DeepEqual(expected, spec))
}

////////////////////////////////////////////////////////////////////////////////
// Processor for counting lines

// Generate processors
type countingProcessorGen struct {
	manager *countingResultsManager
}

func (c *countingProcessorGen) GenerateProcessor(source *PipelineSourceInstance) Processor {
	return NewSimpleProcessor(source.Processor, &countingHandler{
		count:    0,
		filename: source.Info["file_name"].(string),
		manager:  c.manager,
	})
}

// Collect all the results
type countingResultsManager struct {
	counts map[string]int
	sync.Mutex
}

func (m *countingResultsManager) Finish(filename string, count int) {
	m.Lock()
	m.counts[filename] = count
	m.Unlock()
}

// Count lines in files
type countingHandler struct {
	filename string
	count    int
	manager  *countingResultsManager
}

func (proc *countingHandler) Handle(log interface{}) interface{} {
	proc.count += 1
	return nil
}

func (proc *countingHandler) Finish() {
	proc.manager.Finish(proc.filename, proc.count)
}

////////////////////////////////////////////////////////////////////////////////
// Processor for skipping every other line

// Generate processors
type skipProcessorGen struct{}

func (s *skipProcessorGen) GenerateProcessor(source *PipelineSourceInstance) Processor {
	return NewSimpleProcessor(source.Processor, &skipProcessor{})
}

// Count lines in files
type skipProcessor struct {
	odd bool
}

func (proc *skipProcessor) Handle(log interface{}) interface{} {
	proc.odd = !proc.odd
	if proc.odd {
		return log
	} else {
		return nil
	}
}

func (proc *skipProcessor) Finish() {}

////////////////////////////////////////////////////////////////////////////////

// Test building and running a simple pipeling.  The pipeline consists of a
// single processing node that simply counts lines.  The sources are two log
// files with known counts.
func TestBuilderPipeline(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["counter"] = &countingProcessorGen{manager}

	confString := `
source:
  type: files
  sources: ["./test/*.log"]
pipeline:
  processors: ["counter"]
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	runner.Run()
	t.Log(manager.counts)

	assert.Equal(5000, manager.counts["test/test.log"])
	assert.Equal(10000, manager.counts["test/test.10000.log"])
}

// Test a two-node pipeline.  The first node rejects every other line, and the
// next node counts the lines.
func TestBuilderPipelineMult(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["counter"] = &countingProcessorGen{manager}
	env.Processors["skip_odd"] = &skipProcessorGen{}

	confString := `
source:
  type: files
  sources: ["./test/*.log"]
pipeline:
  processors: ["skip_odd", "counter"]
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	runner.Run()
	t.Log(manager.counts)

	assert.Equal(2500, manager.counts["test/test.log"])
	assert.Equal(5000, manager.counts["test/test.10000.log"])
}

////////////////////////////////////////////////////////////////////////////////

// Generate pass through processors
type passThroughProcessorGen struct{}

func (c *passThroughProcessorGen) GenerateProcessor(source *PipelineSourceInstance) Processor {
	return NewSimpleProcessor(source.Processor, &passThroughHandler{})
}

// A handler to check if the loglines are coming in the right order. There is a
// slight complication here because of the timing of when the timestamp and
// token are set. It's possible the thread logging the entry could be preempted
// in between setting the timestamp and token. In this case, we may have a case
// where line B has a larger token and comes after A, but A has a timestamp
// after B.
type checkProcessorHandler struct {
	lastLine  int64
	lastTime  float64
	lineCount int
	t         *testing.T
	manager   *countingResultsManager
	filename  string
}

func (proc *checkProcessorHandler) Handle(iLog interface{}) interface{} {
	ll := iLog.(*Logline)
	proc.lineCount += 1

	if proc.lastLine > 0 {
		assert.True(proc.t, ll.LogcatToken > proc.lastLine || ll.TraceTime > proc.lastTime)
		proc.lastLine = ll.LogcatToken
		proc.lastTime = ll.TraceTime
	}

	return nil
}

func (proc *checkProcessorHandler) Finish() {
	proc.manager.Finish(proc.filename, proc.lineCount)
}

type checkProcessorGen struct {
	t       *testing.T
	manager *countingResultsManager
}

func (gen *checkProcessorGen) GenerateProcessor(source *PipelineSourceInstance) Processor {
	cp := &checkProcessorHandler{
		lastLine:  0,
		lineCount: 0,
		lastTime:  0.0,
		t:         gen.t,
		manager:   gen.manager,
		filename:  source.Info["file_name"].(string),
	}

	return NewSimpleProcessor(source.Processor, cp)
}

// Test a pipeline with multiple preprocessors. The preprocessors don't do
// much - they simply pass the line on. However, because everything is hooked
// up with Timeweavers, there is an expected order. Also, we should each
// logline N times.
func TestBuilderPipelinePreproc(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["passthrough"] = &passThroughProcessorGen{}
	env.Processors["checker"] = &checkProcessorGen{t, manager}

	const multiplier = 4

	confString := `
source:
  type: files
  sources: ["./test/*.log"]
preprocessors:
  - processors: ["passthrough"]
  - processors: ["passthrough"]
  - processors: ["passthrough"]
pipeline:
  processors: ["checker"]
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	// The processors handle the checking.
	runner.Run()

	assert.Equal(5000*multiplier, manager.counts["test/test.log"])
	assert.Equal(10000*multiplier, manager.counts["test/test.10000.log"])
}

////////////////////////////////////////////////////////////////////////////////

// Custom DataCollector that counts the number of lines it sees and compares it
// with the expected number on Finish().
type testDataCollector struct {
	totalLines int
	expected   int
	t          *testing.T
	sync.Mutex
}

func (dc *testDataCollector) OnData(data interface{}) {
	dc.Lock()
	dc.totalLines += 1
	dc.Unlock()
}

func (dc *testDataCollector) Finish() {
	dc.t.Log(dc.totalLines)
	assert.Equal(dc.t, dc.expected, dc.totalLines)
}

// Test a custom DataCollector by diverting all lines to it and count the number
// of lines. We'd expect the total to be the sum of the lines in all files.
func TestBuilderDataCollector(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	env := NewEnvironment()
	env.Processors["passthrough"] = &passThroughProcessorGen{}

	collectorGen := func() DataCollector {
		return &testDataCollector{
			totalLines: 0,
			expected:   15000,
			t:          t,
		}
	}
	env.DataCollectors["test"] = collectorGen

	confString := `
data_collector: "test"
source:
  type: files
  sources: ["./test/*.log"]
pipeline:
  processors: ["passthrough"]
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	assert.Equal("test", conf.DataCollector)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	// The processors handle the checking.
	runner.Run()
}
