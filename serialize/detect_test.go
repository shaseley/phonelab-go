package serialize

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func test(t *testing.T, str string, obj Serializer) {
	require := require.New(t)
	s, err := DetectSerializer(str)
	require.Nil(err)
	require.Equal(reflect.TypeOf(s), reflect.TypeOf(obj))
}

func TestDetectHDFS(t *testing.T) {
	// Base
	str := "hdfs://example.com/"
	test(t, str, &HDFSSerializer{})

	// With port
	str = "hdfs://example.com:31121/"
	test(t, str, &HDFSSerializer{})

	// Base with path
	str = "hdfs://example.com/test"
	test(t, str, &HDFSSerializer{})

	// Port with path
	str = "hdfs://example.com:31121/test"
	test(t, str, &HDFSSerializer{})
}

func TestDetectHTTP(t *testing.T) {
	str := "http://example.com/"
	test(t, str, &HTTPSerializer{})

	// With port
	str = "http://example.com:31121/"
	test(t, str, &HTTPSerializer{})

	// Base with path
	str = "http://example.com/test"
	test(t, str, &HTTPSerializer{})

	// Port with path
	str = "http://example.com:31121/test"
	test(t, str, &HTTPSerializer{})

	// Now HTTPS
	str = "https://example.com/"
	test(t, str, &HTTPSerializer{})

	// With port
	str = "https://example.com:31121/"
	test(t, str, &HTTPSerializer{})

	// Base with path
	str = "https://example.com/test"
	test(t, str, &HTTPSerializer{})

	// Port with path
	str = "https://example.com:31121/test"
	test(t, str, &HTTPSerializer{})
}

func TestDetectLocal(t *testing.T) {
	str := "file://test"
	test(t, str, &LocalSerializer{})

	str = "file://test/"
	test(t, str, &LocalSerializer{})

	str = "file:///test"
	test(t, str, &LocalSerializer{})

	str = "file:///test/"
	test(t, str, &LocalSerializer{})
}
