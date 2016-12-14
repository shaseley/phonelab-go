package phonelab

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type testStruct struct {
	F1 int32 `logcat:"field1"`
	F2 int64 `logcat:"field2"`
	F3 uint32
	F4 uint64
	F5 string `logcat:"stringField5"`
	F6 float32
	F7 float64
	F8 int `logcat:"-"`
}

func TestUnpackLogcat(t *testing.T) {
	assert := assert.New(t)

	ts := testStruct{}
	ts.F8 = 1000

	values := map[string]string{
		"field1":       "1",
		"field2":       "2",
		"f3":           "3",
		"f4":           "4",
		"stringField5": "five",
		"f6":           "6.1111",
		"f7":           "7.50",
		"f8":           "1234", // Shouldn't change
	}

	err := UnpackLogcatEntry(&ts, values)
	assert.Nil(err)
	t.Log(ts)

	assert.Equal(int32(1), ts.F1)
	assert.Equal(int64(2), ts.F2)
	assert.Equal(uint32(3), ts.F3)
	assert.Equal(uint64(4), ts.F4)
	assert.Equal("five", ts.F5)
	assert.Equal(float32(6.1111), ts.F6)
	assert.Equal(float64(7.50), ts.F7)
	assert.Equal(int(1000), ts.F8)

}

func TestUnpackLogcatErrors(t *testing.T) {
	assert := assert.New(t)

	ts := testStruct{}

	values := map[string]string{}

	// Must be a pointer
	err := UnpackLogcatEntry(ts, values)
	assert.NotNil(err)

	// Parse errors
	values = map[string]string{
		"field1": "9999999999999999999999999999999999999999999999999999999",
	}

	err = UnpackLogcatEntry(&ts, values)
	assert.NotNil(err)

	values = map[string]string{
		"f6": "foo",
	}

	err = UnpackLogcatEntry(&ts, values)
	assert.NotNil(err)
}
