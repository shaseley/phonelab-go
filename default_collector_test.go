package phonelab

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lineMapEntry struct {
	Logline   int64   `json:"logline"`
	Timestamp float64 `json:"timestamp"`
}

type lineTimeMapProcessor struct{}

func (p *lineTimeMapProcessor) Handle(data interface{}) interface{} {
	if ll, ok := data.(*Logline); ok && ll != nil {
		return &lineMapEntry{
			Logline:   ll.LogcatToken,
			Timestamp: ll.TraceTime,
		}
	}
	return nil
}

func (p *lineTimeMapProcessor) Finish() {}

// Generate processors
type lineTimeMapGen struct{}

func (lc *lineTimeMapGen) GenerateProcessor(source *PipelineSourceInstance,
	kwargs map[string]interface{}) Processor {

	return NewSimpleProcessor(source.Processor, &lineTimeMapProcessor{})
}

// TODO: Clean up or remove this test
func TestBuildeDefaultrDataCollector(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	env := NewEnvironment()
	env.Processors["test"] = &lineTimeMapGen{}

	confString := `
data_collector:
  name: "default"
  args:
    path: "./test/default_collector_test"
    aggregate: true
    compress: true
source:
  type: files
  sources: ["./test/*.log"]
processors:
  - name: main
    generator: test
    has_logstream: true
sink:
  name: main
`
	conf, err := RunnerConfFromString(confString)
	require.Nil(err)
	require.NotNil(conf)

	require.NotNil(conf.DataCollector)
	assert.Equal("default", conf.DataCollector.Name)

	// Remove this for testing
	defer os.RemoveAll("test/default_collector_test")

	runner, err := conf.ToRunner(env)
	require.Nil(err)
	require.NotNil(runner)

	t.Log(runner.Source)

	// The processors handle the checking.
	runner.Run()
}
