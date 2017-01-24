package processing

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTextFileProcessor(t *testing.T) {
	assert := assert.New(t)

	processor := NewTextFileProcessor("../test/test.log", func(e error) {
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
