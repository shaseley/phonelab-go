package phonelab

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func confToTextFile(conf string) (string, error) {
	// Same thing, but from a file
	tempfile, err := ioutil.TempFile("", "pl-go")
	if err != nil {
		return "", err
	}
	defer tempfile.Close()

	name := tempfile.Name()

	_, err = tempfile.Write([]byte(conf))

	return name, err
}

func TestRunnerConfFromString(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	confString := `
max_concurrency: 5
data_collector: "resultsCollector"
source:
  type: files
  sources:
    - "path/to/some/file"
    - "path/to/some/other/file"
processors:
  - name: proc1
    description: "Test processor"
    inputs: []
    parsers: ["tag1", "tag2"]
    filters:
      - type: "simple"
        filter: "foo&&bar"
      - type: "custom"
        filter: "baz"
      - type: "regex"
        filter: "^sometext.*othertext.*$"
    has_logstream: true
sink:
  name: proc1
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	expected := &RunnerConf{
		MaxConcurrency: 5,
		DataCollector:  "resultsCollector",
		SourceConf: &PipelineSourceConf{
			Type: PipelineSourceFile,
			Sources: []string{
				"path/to/some/file",
				"path/to/some/other/file",
			},
		},
		Processors: []*ProcessorConf{
			&ProcessorConf{
				Name:         "proc1",
				Description:  "Test processor",
				Inputs:       []*ProcessorInputConf{},
				HasLogstream: true,
				Parsers:      []string{"tag1", "tag2"},

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
			},
		},
		Sink: &ProcessorInputConf{Name: "proc1"},
	}

	require.True(reflect.DeepEqual(expected.SourceConf, conf.SourceConf))
	require.True(reflect.DeepEqual(expected.Processors, conf.Processors))
	require.True(reflect.DeepEqual(expected, conf))

	// Same thing, but from a file
	name, err := confToTextFile(confString)
	if len(name) == 0 {
		defer os.Remove(name)
	}
	require.Nil(err)

	conf, err = RunnerConfFromFile(name)
	require.NotNil(conf)
	require.Nil(err)

	require.True(reflect.DeepEqual(expected.SourceConf, conf.SourceConf))
	require.True(reflect.DeepEqual(expected.Processors, conf.Processors))
	require.True(reflect.DeepEqual(expected, conf))
}

func TestRunnerConfYamlErr(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	confString := `
sink:
  name: foo
processors missing colon`

	conf, err := RunnerConfFromString(confString)
	require.NotNil(err)
	require.Nil(conf)

	// Same thing, but from a file
	name, err := confToTextFile(confString)
	if len(name) == 0 {
		defer os.Remove(name)
	}
	require.Nil(err)

	conf, err = RunnerConfFromFile(name)
	assert.Nil(conf)
	assert.NotNil(err)

	conf, err = RunnerConfFromFile("fooooo.yml")
	assert.Nil(conf)
	assert.NotNil(err)
}

func TestRunnerConfDependencies(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	confString := `
processors:
  - name: proc1
    inputs: [{name: proc2}]
  - name: proc2
    inputs: [{name: proc3}]
  - name: proc3
    inputs: [{name: proc4}]
  - name: proc4
sink:
  name: proc1
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	env := NewEnvironment()

	dg, err := conf.dependencyGraph(env)
	require.Nil(err)
	require.NotNil(dg)

	assert.Equal(4, len(dg.NodeMap))

	var ok bool

	_, ok = dg.NodeMap["proc1"]
	assert.True(ok)
	_, ok = dg.NodeMap["proc2"]
	assert.True(ok)
	_, ok = dg.NodeMap["proc3"]
	assert.True(ok)
	_, ok = dg.NodeMap["proc4"]
	assert.True(ok)

	topSort, err := dg.TopSort()
	require.Nil(err)
	require.Equal(4, len(topSort))

	// There's only one ordering here.
	expected := []string{
		"proc1",
		"proc2",
		"proc3",
		"proc4",
	}
	assert.Equal(expected, topSort)
}

func TestRunnerConfDependencyCycle(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	confString := `
processors:
  - name: proc1
    inputs: [{name: proc2}]
  - name: proc2
    inputs: [{name: proc3}]
  - name: proc3
    inputs: [{name: proc4}]
  - name: proc4
    inputs: [{name: proc2}]
sink:
  name: proc1
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	env := NewEnvironment()

	dg, err := conf.dependencyGraph(env)
	require.Nil(err)
	require.NotNil(dg)

	t.Log(dg.NodeMap)
	assert.Equal(4, len(dg.NodeMap))

	_, err = dg.TopSort()
	require.NotNil(err)
}

////////////////////////////////////////////////////////////////////////////////
// Processor for counting lines

// Generate processors
type countingProcessorGen struct {
	manager *countingResultsManager
}

func (c *countingProcessorGen) GenerateProcessor(source *PipelineSourceInstance, kwargs map[string]interface{}) Processor {

	increment := 1
	if v, ok := kwargs["increment"]; ok {
		increment = v.(int)
	}

	return NewSimpleProcessor(source.Processor, &countingHandler{
		count:     0,
		increment: increment,
		filename:  source.Info["file_name"].(string),
		manager:   c.manager,
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
	filename  string
	count     int
	increment int
	manager   *countingResultsManager
}

func (proc *countingHandler) Handle(log interface{}) interface{} {
	proc.count += proc.increment
	return nil
}

func (proc *countingHandler) Finish() {
	proc.manager.Finish(proc.filename, proc.count)
}

////////////////////////////////////////////////////////////////////////////////
// Processor for skipping every other line

// Generate processors
type skipProcessorGen struct{}

func (s *skipProcessorGen) GenerateProcessor(source *PipelineSourceInstance,
	kwargs map[string]interface{}) Processor {
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
processors:
  - name: counter
    has_logstream: true
source:
  type: files
  sources: ["./test/*.log"]
sink:
  name: "counter"
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	errs := runner.Run()
	assert.Equal(0, len(errs))

	t.Log(manager.counts)

	assert.Equal(5000, manager.counts["test/test.log"])
	assert.Equal(10000, manager.counts["test/test.10000.log"])
}

// Test a two-node pipeline.  The first node rejects every other line, and the
// next node counts the lines.
func TestBuilderPipelinePreprocessors(t *testing.T) {
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
processors:
  - name: main
    generator: "counter"
    preprocessors:
      - name: "skip_odd"
    has_logstream: true
sink:
  name: main
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

func (c *passThroughProcessorGen) GenerateProcessor(source *PipelineSourceInstance,
	kwargs map[string]interface{}) Processor {
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

func (gen *checkProcessorGen) GenerateProcessor(source *PipelineSourceInstance,
	kwargs map[string]interface{}) Processor {
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
func TestBuilderPipelineMultInputs(t *testing.T) {
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
processors:
  - name: checker
    has_logstream: true
    inputs:
      - name: pp1
      - name: pp2
      - name: pp3
  - name: pp1
    generator: "passthrough"
    has_logstream: true
  - name: pp2
    generator: "passthrough"
    has_logstream: true
  - name: pp3
    generator: "passthrough"
    has_logstream: true
sink:
  name: checker
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
processors:
  - name: main
    generator: passthrough
    has_logstream: true
sink:
  name: main
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

func TestBuilderProcessorConf(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	confString := `
- name: "test"
  description: "A test processor"
  inputs:
    - name: A
      args:
        foo: 1
        bar: "baz"
    - name: B
  has_logstream: true
  filters:
    - type: "simple"
      filter: "foo&&bar"
    - type: "custom"
      filter: "baz"
    - type: "regex"
      filter: "^sometext.*othertext.*$"
  parsers: ["Some-Tag", "Some-Other-Tag"]
`
	confString2 := `
name: "test"
description: "A test processor"
inputs:
  - name: A
    args:
      foo: 1
      bar: "baz"
  - name: B
has_logstream: true
filters:
  - type: "simple"
    filter: "foo&&bar"
  - type: "custom"
    filter: "baz"
  - type: "regex"
    filter: "^sometext.*othertext.*$"
parsers: ["Some-Tag", "Some-Other-Tag"]
`

	expected := &ProcessorConf{
		Name:        "test",
		Description: "A test processor",
		Inputs: []*ProcessorInputConf{
			&ProcessorInputConf{
				Name: "A",
				Args: map[string]interface{}{
					"foo": 1,
					"bar": "baz",
				},
			},
			&ProcessorInputConf{
				Name: "B",
				Args: nil,
			},
		},
		HasLogstream: true,
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
		Parsers: []string{"Some-Tag", "Some-Other-Tag"},
	}

	var err error
	var confs []*ProcessorConf
	var name string

	for i := 0; i < 4; i++ {
		switch i {
		case 0:
			confs, err = ProcessorConfsFromString(confString)
			name = ""
		case 1:
			confs, err = ProcessorConfsFromString(confString2)
			name = ""
		case 2:
			name, err := confToTextFile(confString)
			if err == nil {
				confs, err = ProcessorConfsFromFile(name)
			}
		case 3:
			name, err := confToTextFile(confString2)
			if err == nil {
				confs, err = ProcessorConfsFromFile(name)
			}
		}

		if len(name) == 0 {
			defer os.Remove(name)
		}

		require.Nil(err)
		require.NotNil(confs)
		require.Equal(1, len(confs))

		assert.True(reflect.DeepEqual(expected, confs[0]))
	}

	confs, err = ProcessorConfsFromFile("fooooo.yml")
	assert.Equal(0, len(confs))
	assert.NotNil(err)
}

func TestIsYamlList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	tests := []struct {
		conf     string
		expected bool
	}{
		{"foo: bar", false},
		{"- foo: bar", true},
		{`# this is a list
- foo: bar"`, true},
		{`
# this also
# is a list
- foo: bar"`, true},
		{`
# But not this
foo: bar"`, false},
		{`
# Or this`, false},
	}

	for _, test := range tests {
		assert.Equal(test.expected, isYamlList(test.conf))
	}

}

////////////////////////////////////////////////////////////////////////////////
// Processor that outputs the current line count

type lineCount struct {
	Count int
}

func (lc *lineCount) MonotonicTimestamp() float64 {
	return float64(lc.Count)
}

// Generate processors
type lineCountProcessorGen struct{}

func (lc *lineCountProcessorGen) GenerateProcessor(source *PipelineSourceInstance,
	kwargs map[string]interface{}) Processor {

	return NewSimpleProcessor(source.Processor, &lineCountProcessor{})
}

// Count lines in files
type lineCountProcessor struct {
	count int
}

func (proc *lineCountProcessor) Handle(log interface{}) interface{} {
	proc.count += 1
	return &lineCount{proc.count}
}

func (proc *lineCountProcessor) Finish() {}

// Processor that receives the counts

// Custom DataCollector that counts the number of lines it sees and compares it
// with the expected number on Finish().
type lcDataCollector struct {
	Counts   map[*lineCount]int
	expected int
	t        *testing.T
	sync.Mutex
}

func (dc *lcDataCollector) OnData(data interface{}) {
	dc.Lock()
	dc.Counts[data.(*lineCount)] += 1
	dc.Unlock()
}

func (dc *lcDataCollector) Finish() {
	assert.Equal(dc.t, dc.expected, len(dc.Counts))
	for _, c := range dc.Counts {
		assert.Equal(dc.t, 4, c)
	}
}

func TestBuilderInputMuxing(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	env := NewEnvironment()
	env.Processors["passthrough"] = &passThroughProcessorGen{}
	env.Processors["lineCounter"] = &lineCountProcessorGen{}

	collectorGen := func() DataCollector {
		return &lcDataCollector{
			Counts:   make(map[*lineCount]int),
			expected: 15000,
			t:        t,
		}
	}
	env.DataCollectors["test"] = collectorGen

	confString := `
data_collector: "test"
source:
  type: files
  sources: ["./test/*.log"]
processors:
  - name: lc
    generator: lineCounter
    has_logstream: true

  - name: p1
    inputs: [{name: lc}]
    generator: passthrough

  - name: p2
    inputs: [{name: lc}]
    generator: passthrough

  - name: p3
    inputs: [{name: lc}]
    generator: passthrough

  - name: p4
    inputs: [{name: lc}]
    generator: passthrough

  - name: main
    generator: passthrough
    inputs:
      - name: p1
      - name: p2
      - name: p3
      - name: p4

sink:
  name:  main
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
	errs := runner.Run()
	assert.Equal(0, len(errs))
	t.Log(errs)
}

func TestPipelineSourceErrors(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	confs := []*PipelineSourceConf{
		&PipelineSourceConf{
			Type: "foo",
			Sources: []string{
				"some/file",
			},
		},
		&PipelineSourceConf{
			Type:    "files",
			Sources: []string{},
		},
		&PipelineSourceConf{
			Type:    "files",
			Sources: nil,
		},
		&PipelineSourceConf{
			Type: "files",
			Sources: []string{
				"",
			},
		},
	}

	for _, conf := range confs {
		gen, err := conf.ToPipelineSourceGenerator()
		assert.Nil(gen)
		assert.NotNil(err)
	}

}

func TestProcessorConfErrors(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	confs := []*ProcessorConf{
		// No generator
		&ProcessorConf{
			Name:      "",
			Generator: "",
		},
		// Processor doesn't exist
		&ProcessorConf{
			Name: "foo",
		},
		// Preprocessor doesn't exist
		&ProcessorConf{
			Name:         "Test1",
			Generator:    "passthrough",
			HasLogstream: true,
			Preprocessors: []*ProcessorInputConf{
				&ProcessorInputConf{
					Name: "foo",
				},
			},
		},
		// Preprocessor blank
		&ProcessorConf{
			Name:         "Test2",
			Generator:    "passthrough",
			HasLogstream: true,
			Preprocessors: []*ProcessorInputConf{
				&ProcessorInputConf{
					Name: "",
				},
			},
		},
		////// Filters //////
		// Empty filter
		&ProcessorConf{
			Name:         "Test3",
			Generator:    "passthrough",
			HasLogstream: true,
			Filters: []*FilterConf{
				&FilterConf{
					Type:   "simple",
					Filter: "",
				},
			},
		},
		// Invalid filter type
		&ProcessorConf{
			Name:         "Test4",
			Generator:    "passthrough",
			HasLogstream: true,
			Filters: []*FilterConf{
				&FilterConf{
					Type:   "foo",
					Filter: "bar",
				},
			},
		},
		// Invalid custom filter
		&ProcessorConf{
			Name:         "Test5",
			Generator:    "passthrough",
			HasLogstream: true,
			Filters: []*FilterConf{
				&FilterConf{
					Type:   "custom",
					Filter: "foo",
				},
			},
		},
		///// Parsers ////
		// Blank parser
		&ProcessorConf{
			Name:         "Test6",
			Generator:    "passthrough",
			HasLogstream: true,
			Parsers:      []string{""},
		},
		// Invalid
		&ProcessorConf{
			Name:         "Test7",
			Generator:    "passthrough",
			HasLogstream: true,
			Parsers:      []string{"foo"},
		},
	}

	env := NewEnvironment()
	env.Processors["passthrough"] = &passThroughProcessorGen{}

	for _, conf := range confs {
		err := conf.validate(env)
		assert.NotNil(err)
		if err == nil {
			t.Log(conf)
		}
	}
}

func TestBuilderFilters(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["counter"] = &countingProcessorGen{manager}
	env.Filters["thermal"] = func(s string) bool {
		return strings.Contains(s, "thermal_temp: sensor_id")
	}

	confStrings := []string{
		`
processors:
  - name: counter
    has_logstream: true
    filters:
      - type: simple
        filter: "thermal_temp: sensor_id"
source:
  type: files
  sources: ["./test/*.log"]
sink:
  name: "counter"
`,
		`
processors:
  - name: counter
    has_logstream: true
    filters:
      - type: regex
        filter: "^.*thermal_temp: sensor_id.*$"
source:
  type: files
  sources: ["./test/*.log"]
sink:
  name: "counter"
`,
		`
processors:
  - name: counter
    has_logstream: true
    filters:
      - type: custom
        filter: thermal
source:
  type: files
  sources: ["./test/*.log"]
sink:
  name: "counter"
`,
	}

	for _, confString := range confStrings {

		manager.counts = make(map[string]int)

		conf, err := RunnerConfFromString(confString)
		require.Nil(err)
		require.NotNil(conf)

		runner, err := conf.ToRunner(env)
		require.Nil(err)
		require.NotNil(runner)

		errs := runner.Run()
		assert.Equal(0, len(errs))

		t.Log(manager.counts)

		c1 := manager.counts["test/test.log"]
		c2 := manager.counts["test/test.10000.log"]
		assert.Equal(1416, c1+c2)
	}
}

func TestBuilderArgPassing1(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["counter"] = &countingProcessorGen{manager}
	env.Processors["passthrough"] = &passThroughProcessorGen{}

	confString := `
processors:
  - name: counter
    has_logstream: true

  - name: main
    generator: passthrough
    inputs:
      - name: counter
        args:
          increment: 5
source:
  type: files
  sources: ["./test/*.log"]
sink:
  name: "main"
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	errs := runner.Run()
	assert.Equal(0, len(errs))

	t.Log(manager.counts)

	assert.Equal(5000*5, manager.counts["test/test.log"])
	assert.Equal(10000*5, manager.counts["test/test.10000.log"])
}

func TestBuilderArgPassing2(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["counter"] = &countingProcessorGen{manager}

	confString := `
processors:
  - name: counter
    has_logstream: true
source:
  type: files
  sources: ["./test/*.log"]
sink:
  name: "counter"
  args:
    increment: 4
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	errs := runner.Run()
	assert.Equal(0, len(errs))

	t.Log(manager.counts)

	assert.Equal(5000*4, manager.counts["test/test.log"])
	assert.Equal(10000*4, manager.counts["test/test.10000.log"])
}

func TestExpandPipelineSourceConf(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	conf := &PipelineSourceConf{
		Type: "files",
		Sources: []string{
			"./test/*.log",
		},
	}

	sources, err := conf.Expand()
	assert.Nil(err)

	sourceMap := make(map[string]bool)
	for _, src := range sources {
		sourceMap[src] = true
	}

	require.Equal(2, len(sourceMap))
	assert.True(sourceMap["test/test.log"])
	assert.True(sourceMap["test/test.10000.log"])
}

func TestSplitRunnerConf(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	confString := `
processors:
  - name: main
    has_logstream: true
    inputs: [{name: input1}]
source:
  type: files
  sources: ["test/*.log"]
sink:
  name: "main"
  args:
    arg1: 100
    arg2: foo
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	expected := &RunnerConf{
		Processors: []*ProcessorConf{
			&ProcessorConf{
				Name: "main",
				Inputs: []*ProcessorInputConf{
					&ProcessorInputConf{Name: "input1"},
				},
				HasLogstream: true,
			},
		},
		SourceConf: &PipelineSourceConf{
			Type: PipelineSourceFile,
			Sources: []string{
				"test/*.log",
			},
		},
		Sink: &ProcessorInputConf{
			Name: "main",
			Args: map[string]interface{}{
				"arg1": 100,
				"arg2": "foo",
			},
		},
	}

	split, err := conf.ShallowSplit()
	require.Nil(err)
	require.Equal(2, len(split))

	require.True(reflect.DeepEqual(expected, conf))

	splitConfs, err := conf.ShallowSplit()
	require.Nil(err)
	require.Equal(2, len(splitConfs))

	expected.SourceConf.Sources[0] = "test/test.10000.log"
	assert.True(reflect.DeepEqual(expected, splitConfs[0]))

	expected.SourceConf.Sources[0] = "test/test.log"
	assert.True(reflect.DeepEqual(expected, splitConfs[1]))
}
