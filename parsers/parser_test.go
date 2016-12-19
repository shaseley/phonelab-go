package parsers

import (
	"bufio"
	"github.com/stretchr/testify/assert"
	"os"
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
	t.Parallel()
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
	t.Parallel()
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

func TestLoglineParser(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	parser := NewLoglineParser()
	parser.AddKnownTags()

	file, err := os.Open("./test/test.10000.log")
	assert.NotNil(file)
	assert.Nil(err)
	if err != nil {
		t.FailNow()
	}

	printk, power, thermal, cpu_freq, unknown := 0, 0, 0, 0, 0

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		obj, err := parser.Parse(line)

		// Everyting in the testfile is well formed. Also,
		// all subparsers are not set to error on unknown lines.
		assert.NotNil(obj)
		assert.Nil(err)
		if err != nil {
			t.FailNow()
		}

		log := obj.(*Logline)

		switch log.Payload.(type) {
		case *PrintkLog:
			printk += 1
		case *PLPowerBatteryLog:
			power += 1
		case *ThermalTemp:
			thermal += 1
		case *CpuFrequency:
			cpu_freq += 1
		case string:
			unknown += 1
		default:
			t.Fatalf("Unexpected type: %T", log)
		}
	}

	assert.True(printk > 0)
	assert.True(power > 0)
	assert.True(thermal > 0)
	assert.True(cpu_freq > 0)
	assert.True(unknown > 0)

	t.Log(printk)
	t.Log(power)
	t.Log(thermal)
	t.Log(cpu_freq)
	t.Log(unknown)
}

func TestLoglineParserSingleTag(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	parser := NewLoglineParser()

	// We'll only add a printk parser that doesn't know how to
	// parse any payloads, but doesn't error.
	pkParser := NewPrintkParser()
	pkParser.ErrOnUnknownTag = false
	pkParser.Subparsers = make([]*PrintkSubparser, 0)

	parser.SetParser("KernelPrintk", pkParser)

	file, err := os.Open("./test/test.10000.log")
	assert.NotNil(file)
	assert.Nil(err)
	if err != nil {
		t.FailNow()
	}

	printk, unknown := 0, 0

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		obj, err := parser.Parse(line)

		// Everyting in the testfile is well formed. Also,
		// all subparsers are not set to error on unknown lines.
		assert.NotNil(obj)
		assert.Nil(err)
		if err != nil {
			t.FailNow()
		}

		log := obj.(*Logline)

		switch log.Payload.(type) {
		case *PrintkLog:
			printk += 1
		case string:
			unknown += 1
		default:
			t.Fatalf("Unexpected type: %T", log)
		}
	}

	assert.True(printk > 0)
	assert.True(unknown > 0)

	t.Log(printk)
	t.Log(unknown)
}
