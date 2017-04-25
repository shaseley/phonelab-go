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
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var (
	hdfsAddr = flag.String("hdfs-addr", "", "Address of HDFS server")
)

func TestHDFSSerialize(t *testing.T) {
	if strings.Compare(*hdfsAddr, "") == 0 {
		t.Skip(fmt.Sprintf("HDFS address not specified"))
	}

	addr := parseHDFSAddr(*hdfsAddr)
	log.Debugf("addr=%v\n", addr)
	fs := easyhdfs.NewHDFSFileSystem(addr)

	require := require.New(t)

	// Add an extra directory just to test mkdirAll
	sanitizedPath := *hdfsAddr
	sanitizedPath = sanitizedPath[strings.Index(sanitizedPath, addr)+len(addr):]
	log.Debugf("sanitizedPath=%v\n", sanitizedPath)
	outdir := filepath.Join(sanitizedPath, "test-hdfs-serialize")
	if exists, _ := fs.Exists(outdir); exists {
		err := fs.RemoveAll(outdir)
		require.Nil(err)
	}
	filePath := filepath.Join(outdir, "test-serialize.gz")

	data := []string{"Hello", "World"}

	serializer := NewHDFSSerializer(*hdfsAddr)
	serializePath := fmt.Sprintf("hdfs://%v%v", addr, filePath)
	log.Debugf("serializePath=%v\n", serializePath)
	err := serializer.Serialize(data, serializePath)
	require.Nil(err)
	defer fs.Remove(outdir)

	// Now check the data
	// Sanitize filePath since it contains hdfs://hdfsAddr/
	outPath, err := serializer.OutPath(serializePath)
	require.Nil(err)

	f, err := fs.Open(outPath, os.O_RDONLY, easyfiles.GZ_TRUE)
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

	serializer := NewHDFSSerializer(*hdfsAddr)

	err := serializer.Serialize(nil, "")
	require.NotNil(err)
}
