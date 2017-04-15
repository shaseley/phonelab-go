package phonelab

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhonelabSourceProcessor(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	sourceInfo := &PhonelabSourceInfo{}
	sourceInfo.Path = "./test/phonelab_source"
	sourceInfo.DeviceId = "test-device-1"
	sourceInfo.BootId = "bootid-0"
	info, err := GetInfoFromFile("./test/phonelab_source/test-device-1/")
	require.Nil(err)
	sourceInfo.StitchInfo = info

	processor, err := NewPhonelabSourceProcessor(sourceInfo, func(e error) {
		t.Log("Error: ", e)
		t.FailNow()
	})
	assert.Nil(err, "Failed to create phonelab source processor")

	logChan := processor.Process()
	logs := 0
	for log := range logChan {
		assert.True(len(log.(string)) > 0)
		//fmt.Printf("Counting ...%d\n", logs)
		logs += 1
	}
	assert.Equal(10000, logs)
}

func TestMultPhonelabSourceProcessor(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	devicePaths := make(map[string][]string)
	devicePaths["test-device-1"] = []string{"./test/phonelab_source"}
	devicePaths["test-device-2"] = []string{"./test/phonelab_source"}

	gen := NewPhonelabSourceGenerator(devicePaths, nil, func(e error) {
		t.Log("Error: ", e)
		t.FailNow()
	})

	sourceChan := gen.Process()
	pos := 0

	for sourceInst := range sourceChan {
		pos += 1

		tp, ok := sourceInst.Info["type"].(string)
		assert.True(ok)
		assert.Equal("phonelab-device", tp)

		sourceInfo, ok := sourceInst.Info["source_info"].(*PhonelabSourceInfo)
		assert.True(ok)

		expected := 0

		switch sourceInfo.DeviceId {
		case "test-device-1":
			expected = 10000
		case "test-device-2":
			expected = 20000
		default:
			t.Fatal("Unexpected device: " + sourceInfo.DeviceId)
		}

		logs := 0
		logChan := sourceInst.Processor.Process()
		for log := range logChan {
			assert.True(len(log.(string)) > 0)
			logs += 1
		}
		assert.Equal(expected, logs)
	}
	assert.Equal(4, pos)
}

// Make sure we can open the file multiple times
func TestPhonelabSourceProcessorMux(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)

	const NUM_OPENS = 10

	sourceInfo := &PhonelabSourceInfo{}
	sourceInfo.Path = "./test/phonelab_source"
	sourceInfo.DeviceId = "test-device-1"
	sourceInfo.BootId = "bootid-0"
	info, err := GetInfoFromFile("./test/phonelab_source/test-device-1/")
	require.Nil(err)
	sourceInfo.StitchInfo = info

	processor, err := NewPhonelabSourceProcessor(sourceInfo, func(e error) {
		t.Log("Error: ", e)
		t.FailNow()
	})
	assert.Nil(err, "Failed to create phonelab source processor")

	done := make(chan int)

	for i := 0; i < NUM_OPENS; i++ {
		go func() {
			logChan := processor.Process()
			logs := 0
			for log := range logChan {
				assert.True(len(log.(string)) > 0)
				logs += 1
			}
			done <- logs
		}()
	}

	allLogs := 0
	for i := 0; i < NUM_OPENS; i++ {
		allLogs += <-done
	}

	assert.Equal(10000*NUM_OPENS, allLogs)
}

// Make sure stuff works while parsing yamls
func TestPhonelabYaml(t *testing.T) {
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
  type: phonelab
  sources: ["./test/phonelab_source/**/info.json"]
processors:
  - name: counter
    has_logstream: true
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

	assert.Equal(10000, manager.counts["test-device-1->bootid-0"], fmt.Sprintf("%v", manager.counts))
	assert.Equal(10000, manager.counts["test-device-1->bootid-1"], fmt.Sprintf("%v", manager.counts))

	assert.Equal(20000, manager.counts["test-device-2->bootid-0"], fmt.Sprintf("%v", manager.counts))
	assert.Equal(20000, manager.counts["test-device-2->bootid-1"], fmt.Sprintf("%v", manager.counts))
}

func TestPhonelabDateRange(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	assert := assert.New(t)

	manager := &countingResultsManager{
		counts: make(map[string]int),
	}

	env := NewEnvironment()
	env.Processors["counter"] = &countingProcessorGen{manager}

	// This is essentially the same as TestPhonelabYaml but with daterange
	// Test device 2 should return 0
	confString := `
source:
  type: phonelab
  sources: ["./test/phonelab_source/test-device-2/info.json"]
  args:
    daterange: "19700101 - 20170101"
processors:
  - name: counter
    has_logstream: true
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

	assert.Equal(10000, manager.counts["test-device-2->bootid-0"], fmt.Sprintf("%v", manager.counts))
	assert.Equal(0, manager.counts["test-device-2->bootid-1"], fmt.Sprintf("%v", manager.counts))
}
