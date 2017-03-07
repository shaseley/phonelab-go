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
  simple_filters:
    - substrings: ["foo", "bar"]
    - substrings: ["baz"]
  complex_filters: []
  tags: ["tag1", "tag2"]
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
		PipelineConf: &PipelineConf{
			SimpleFilters: []*SimpleFilterConf{
				&SimpleFilterConf{
					Substrings: []string{"foo", "bar"},
				},
				&SimpleFilterConf{
					Substrings: []string{"baz"},
				},
			},
			ComplexFilters: []string{},
			Tags:           []string{"tag1", "tag2"},
			Processors:     []string{"proc1"},
		},
	}

	require.True(reflect.DeepEqual(expected.SourceConf, spec.SourceConf))
	require.True(reflect.DeepEqual(expected.PipelineConf, spec.PipelineConf))
	require.True(reflect.DeepEqual(expected, spec))
}

////////////////////////////////////////////////////////////////////////////////
// Processor for counting lines

// Generate processors
type countingProcessorGen struct {
	manager *countingResultsManager
}

func (c *countingProcessorGen) GenerateProcessor(source *PipelineSourceInstance) Processor {
	return newCountingProc(source, c.manager)
}

// Collect all the results
type countingResultsManager struct {
	counts map[string]int
	sync.Mutex
}

func (m *countingResultsManager) onFinish(filename string, count int) {
	m.Lock()
	m.counts[filename] = count
	m.Unlock()
}

// Count lines in files
type countingProcessor struct {
	filename string
	count    int
	source   Processor
	manager  *countingResultsManager
}

func newCountingProc(source *PipelineSourceInstance, manager *countingResultsManager) Processor {
	return &countingProcessor{
		count:    0,
		filename: source.Info["file_name"].(string),
		source:   source.Processor,
		manager:  manager,
	}
}

func (proc *countingProcessor) Process() <-chan interface{} {

	outChan := make(chan interface{})

	go func() {
		inChan := proc.source.Process()
		for _ = range inChan {
			proc.count += 1
		}
		close(outChan)
		proc.manager.onFinish(proc.filename, proc.count)
	}()

	return outChan
}

////////////////////////////////////////////////////////////////////////////////
// Processor for skipping every other line

// Generate processors
type skipProcessorGen struct{}

func (s *skipProcessorGen) GenerateProcessor(source *PipelineSourceInstance) Processor {
	return newSkipProc(source)
}

// Count lines in files
type skipProcessor struct {
	source Processor
}

func newSkipProc(source *PipelineSourceInstance) Processor {
	return &skipProcessor{
		source: source.Processor,
	}
}

func (proc *skipProcessor) Process() <-chan interface{} {

	outChan := make(chan interface{})
	odd := true

	go func() {
		inChan := proc.source.Process()
		for log := range inChan {
			if !odd {
				outChan <- log
			}
			odd = !odd
		}
		close(outChan)
	}()

	return outChan
}

////////////////////////////////////////////////////////////////////////////////

// Test building and running a simple pipeling.
// The pipeline consists of a single processing node that simply counts lines.
// The sources are two log files with known counts.
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

// Test a two-node pipeline.
// The first node rejects every other line, and the next node
// counts the lines.
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
