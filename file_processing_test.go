package phonelab

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TextFileProcessorTestCommon(filename string) (int, error) {
	var err error

	processor := NewTextFileProcessor(filename, DEFAULT_MAX_CONCURRENCY, func(e error) {
		err = e
	})

	logChan := processor.Process()
	logs := 0
	for log := range logChan {
		if len(log.(string)) > 0 {
			logs += 1
		}
	}

	return logs, err
}

func TestTextFileProcessor(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	logs, err := TextFileProcessorTestCommon("./test/test.log")

	assert.Nil(err)
	assert.Equal(5000, logs)
}

func TestTextFileProcessorGZ(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	logs, err := TextFileProcessorTestCommon("./test/test.log.gz")

	assert.Nil(err)
	assert.Equal(5000, logs)
}

func TestMultTextFileProcessor(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	files := []string{
		"test/test.log",
		"test/test.10000.log",
	}

	gen := NewTextFileSourceGenerator(files, func(e error) {
		t.Log("Error: ", e)
		t.FailNow()
	})

	sourceChan := gen.Process()
	pos := 0

	for sourceInst := range sourceChan {
		pos += 1

		assert.Equal("file", sourceInst.Info.Type())

		info, ok := sourceInst.Info.(*TextFileSourceInfo)
		assert.True(ok)

		expected := 0

		switch info.Filename {
		case "test/test.log":
			expected = 5000
		case "test/test.10000.log":
			expected = 10000
		default:
			t.Fatal("Unexpected file: " + info.Filename)
		}

		logs := 0
		logChan := sourceInst.Processor.Process()
		for log := range logChan {
			assert.True(len(log.(string)) > 0)
			logs += 1
		}
		assert.Equal(expected, logs)
	}

	assert.Equal(2, pos)
}

// Make sure we can open the file multiple times
func TestTextFileProcessorMux(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	const NUM_OPENS = 10

	processor := NewTextFileProcessor("./test/test.log", DEFAULT_MAX_CONCURRENCY, func(e error) {
		t.Log("Error: ", e)
		t.FailNow()
	})

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

	assert.Equal(5000*NUM_OPENS, allLogs)
}
