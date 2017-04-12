package phonelab

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPhonelabSourceProcessor(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	processor, err := NewPhonelabSourceProcessor("./test/phonelab_source", "test-device-1", "bootid-0", "", func(e error) {
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

	gen := NewPhonelabSourceGenerator(devicePaths, "", func(e error) {
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

		device, ok := sourceInst.Info["deviceid"].(string)
		assert.True(ok)

		expected := 0

		switch device {
		case "test-device-1":
			expected = 10000
		case "test-device-2":
			expected = 20000
		default:
			t.Fatal("Unexpected device: " + device)
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

	const NUM_OPENS = 10

	processor, err := NewPhonelabSourceProcessor("./test/phonelab_source", "test-device-1", "bootid-0", "", func(e error) {
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
