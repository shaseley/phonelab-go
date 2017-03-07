package phonelab

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTextFileProcessor(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)

	processor := NewTextFileProcessor("./test/test.log", func(e error) {
		t.Log("Error: ", e)
		t.FailNow()
	})

	logChan := processor.Process()
	logs := 0
	for log := range logChan {
		assert.True(len(log.(string)) > 0)
		logs += 1
	}
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

		tp, ok := sourceInst.Info["type"].(string)
		assert.True(ok)
		assert.Equal("file", tp)

		name, ok := sourceInst.Info["file_name"].(string)
		assert.True(ok)

		expected := 0

		switch name {
		case "test/test.log":
			expected = 5000
		case "test/test.10000.log":
			expected = 10000
		default:
			t.Fatal("Unexpected file: " + name)
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
