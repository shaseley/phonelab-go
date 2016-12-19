package parsers

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

type parseComparison struct {
	line          string
	parser        Parser
	logParseFails bool
	subParseFails bool
	expected      interface{}
	deep          bool
}

func commonTestParse(allconf []*parseComparison, t *testing.T) {
	assert := assert.New(t)

	var fail = func(msg string, conf *parseComparison) {
		t.Logf("Parse Failed: '%v'\n%v\n", msg, conf.line)
		t.FailNow()
	}

	for _, conf := range allconf {
		logline, err := ParseLogline(conf.line)

		// Is this supposed to fail?
		if conf.logParseFails {
			assert.Nil(logline)
			assert.NotNil(err)
			if err == nil {
				fail("Expected error parsing logline", conf)
			}
			continue
		}

		assert.NotNil(logline)
		assert.Nil(err)
		if logline == nil {
			fail("Unexpected nil logline", conf)
		}

		obj, err := conf.parser.Parse(logline.Payload.(string))

		// Is this supposed to fail?
		if conf.subParseFails {
			assert.Nil(obj)
			assert.NotNil(err)
			if err == nil {
				fail("Expected error parsing payload", conf)
				t.Log(obj)
			}
			continue
		}

		assert.NotNil(obj)
		assert.Nil(err)
		if obj == nil {
			fail("Unexpected nil payload object", conf)
		}

		// Finally, compare
		res := false
		if conf.deep {
			res = reflect.DeepEqual(conf.expected, obj)
		} else {
			res = shallowEqual(conf.expected, obj)
		}
		assert.True(res)
		if !res {
			fail("Objects are not equal", conf)
		}
	}

	t.Log(len(allconf), "tests passed")
}

// Helper function for comparing the results of parsing
func shallowEqual(obj1 interface{}, obj2 interface{}) bool {
	t1 := reflect.TypeOf(obj1)
	t2 := reflect.TypeOf(obj2)

	// Type checking
	if t1 != t2 {
		return false
	}

	for t1.Kind() == reflect.Ptr {
		t1 = t1.Elem()
	}

	for t2.Kind() == reflect.Ptr {
		t2 = t2.Elem()
	}

	if t1.Kind() != reflect.Struct {
		panic("shallowEqual is meant to be used with structs")
	}

	// Value checking
	v1 := reflect.ValueOf(obj1)
	for v1.Kind() == reflect.Ptr {
		v1 = v1.Elem()
	}

	v2 := reflect.ValueOf(obj2)
	for v2.Kind() == reflect.Ptr {
		v2 = v2.Elem()
	}

	for i := 0; i < v1.NumField(); i++ {
		f1 := v1.Field(i)
		f2 := v2.Field(i)

		switch f1.Kind() {
		case reflect.Int:
			fallthrough
		case reflect.Int8:
			fallthrough
		case reflect.Int16:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
			if f1.Int() != f2.Int() {
				return false
			}

		case reflect.Uint:
			fallthrough
		case reflect.Uint8:
			fallthrough
		case reflect.Uint16:
			fallthrough
		case reflect.Uint32:
			fallthrough
		case reflect.Uint64:
			if f1.Uint() != f2.Uint() {
				return false
			}

		case reflect.Float32:
			fallthrough
		case reflect.Float64:
			if f1.Float() != f2.Float() {
				return false
			}

		case reflect.String:
			if f1.String() != f2.String() {
				return false
			}
		}

	}

	return true
}

func TestShallowEqual(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	type testEqStruct struct {
		F1 int
		F2 int64
		F3 float32
		F4 float64
		F5 uint
		F6 uint8
		F7 string
		TP *testStruct
	}

	// Pointers should be ignored. These should be considered equal
	v1 := &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 := &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", &testStruct{}}
	assert.True(shallowEqual(v1, v2))
	assert.True(shallowEqual(v2, v1))

	// Defaults should always be equal
	v1 = &testEqStruct{}
	v2 = &testEqStruct{}
	assert.True(shallowEqual(v1, v2))
	assert.True(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{2, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{1, int64(40000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{1, int64(40000000000000000), float32(7.3155), float64(1234.5678), uint(100), uint8(6), "foo", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.0), uint(100), uint8(6), "foo", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(30), uint8(6), "foo", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(2), "foo", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

	v1 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "foo", nil}
	v2 = &testEqStruct{1, int64(40000000000000000), float32(1.3155), float64(1234.5678), uint(100), uint8(6), "bar", &testStruct{}}
	assert.False(shallowEqual(v1, v2))
	assert.False(shallowEqual(v2, v1))

}
