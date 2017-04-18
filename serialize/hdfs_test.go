package serialize

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/gurupras/go-easyfiles"
	"github.com/shaseley/phonelab-go/hdfs"
	"github.com/stretchr/testify/require"
)

var confFile string = "test/hdfs-conf.yaml"

type hdfsTestConf struct {
	Addr string `yaml:"addr"`
	Path string `yaml:"path"`
}

func TestHDFSSerialize(t *testing.T) {
	if !easyfiles.Exists(confFile) {
		t.Skip(fmt.Sprintf("HDFS Configuration file '%s' not found!", confFile))
	}

	require := require.New(t)

	b, err := ioutil.ReadFile(confFile)
	require.Nil(err)

	conf := hdfsTestConf{}
	err = yaml.Unmarshal(b, &conf)
	require.Nil(err)

	client, err := hdfs.NewHDFSClient(conf.Addr)
	require.Nil(err)
	require.NotNil(client)

	// Add an extra directory just to test mkdirAll
	outdir := filepath.Join(conf.Path, "test-hdfs-serialize")
	filename := "test-serialize.gz"
	filePath := filepath.Join(outdir, filename)
	hdfsArgs := &HDFSSerializerArgs{client, outdir, filename, easyfiles.GZ_TRUE}

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
