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
	"github.com/shaseley/phonelab-go/hdfs"
	"github.com/stretchr/testify/require"
)

type hdfsTestConf struct {
	Addr string `yaml:"addr"`
	Path string `yaml:"path"`
}

var (
	hdfsAddr = flag.String("hdfs-addr", "", "Address of HDFS server")
	hdfsPath = flag.String("hdfs-path", "/test", "Base path under which serialization is tested")
)

func TestHDFSSerialize(t *testing.T) {
	if strings.Compare(*hdfsAddr, "") == 0 {
		t.Skip(fmt.Sprintf("HDFS address not specified"))
	}

	require := require.New(t)

	conf := hdfsTestConf{*hdfsAddr, *hdfsPath}

	client, err := hdfs.NewHDFSClient(conf.Addr)
	require.Nil(err)
	require.NotNil(client)

	// Add an extra directory just to test mkdirAll
	outdir := filepath.Join(conf.Path, "test-hdfs-serialize")
	filePath := filepath.Join(outdir, "test-serialize.gz")
	hdfsArgs := &HDFSSerializerArgs{client, filePath, GZ_TRUE}

	data := []string{"Hello", "World"}

	serializer := &HDFSSerializer{}
	err = serializer.Serialize(data, hdfsArgs)
	require.Nil(err)
	defer client.Remove(outdir)

	// Now check the data
	f, err := hdfs.OpenFile(filePath, os.O_RDONLY, easyfiles.GZ_TRUE, client)
	require.Nil(err)
	reader, err := f.RawReader()
	require.Nil(err)
	var got []string
	err = json.NewDecoder(reader).Decode(&got)
	require.Nil(err)

	require.True(reflect.DeepEqual(data, got))
}

func TestHDFSSerializerBadArgs(t *testing.T) {
	require := require.New(t)

	serializer := &HDFSSerializer{}

	args := struct{}{}
	err := serializer.Serialize(nil, args)
	require.NotNil(err)
}
