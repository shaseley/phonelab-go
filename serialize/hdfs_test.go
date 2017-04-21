package serialize

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-easyfiles/easyhdfs"
	"github.com/stretchr/testify/require"
)

var (
	hdfsAddr = flag.String("hdfs-addr", "", "Address of HDFS server")
	hdfsPath = flag.String("hdfs-path", "/test", "Base path under which serialization is tested")
)

func TestHDFSSerialize(t *testing.T) {
	if strings.Compare(*hdfsAddr, "") == 0 {
		t.Skip(fmt.Sprintf("HDFS address not specified"))
	}

	fs := easyhdfs.NewHDFSFileSystem(*hdfsAddr)

	require := require.New(t)

	// Add an extra directory just to test mkdirAll
	outdir := filepath.Join(*hdfsPath, "test-hdfs-serialize")
	filePath := filepath.Join(outdir, "test-serialize.gz")

	data := []string{"Hello", "World"}

	serializer := &HDFSSerializer{*hdfsAddr}
	err := serializer.Serialize(data, filePath)
	require.Nil(err)
	defer fs.Remove(outdir)

	// Now check the data
	f, err := fs.Open(filePath, os.O_RDONLY, easyfiles.GZ_TRUE)
	require.Nil(err)
	reader, err := f.RawReader()
	require.Nil(err)
	var got []string
	err = json.NewDecoder(reader).Decode(&got)
	require.Nil(err)

	require.True(reflect.DeepEqual(data, got))
}

func TestHDFSSerializerBadArgs(t *testing.T) {
	if strings.Compare(*hdfsAddr, "") == 0 {
		t.Skip(fmt.Sprintf("HDFS address not specified"))
	}

	require := require.New(t)

	serializer := &HDFSSerializer{*hdfsAddr}

	err := serializer.Serialize(nil, "")
	require.NotNil(err)
}
