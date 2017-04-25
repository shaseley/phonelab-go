package serialize

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gurupras/go-easyfiles"
	"github.com/stretchr/testify/require"
)

func TestLocalSerialize(t *testing.T) {
	require := require.New(t)

	serializer := &LocalSerializer{}

	data := []string{"Hello", "World"}

	outdir := filepath.Join("test", "test-local")
	filename := "test-local-serializer.gz"
	filePath := filepath.Join(outdir, filename)

	err := serializer.Serialize(data, "file://"+filePath)
	require.Nil(err)
	defer os.RemoveAll(outdir)

	// Now test it
	f, err := easyfiles.Open(filePath, os.O_RDONLY, easyfiles.GZ_UNKNOWN)
	require.Nil(err)

	reader, err := f.RawReader()
	require.Nil(err)
	var got []string
	err = json.NewDecoder(reader).Decode(&got)
	require.Nil(err)

	require.True(reflect.DeepEqual(data, got))
}

func TestLocalSerializeBadArgs(t *testing.T) {
	require := require.New(t)

	serializer := &LocalSerializer{}

	err := serializer.Serialize(nil, "")
	require.NotNil(err)
}
