package hdfs

import (
	"os"
	"testing"

	"github.com/gurupras/go-easyfiles"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func testReadData(f *easyfiles.File, require *require.Assertions) {
	reader, err := f.RawReader()
	require.Nil(err)

	// Read some known data
	b := make([]byte, 11)
	_, err = reader.Read(b)
	require.Nil(err)
	require.Equal("Hello World", string(b))
}

func TestStat(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	// Stat for a non-existant file
	stat, err := client.Stat("/does-not-exist")
	require.NotNil(err)
	require.Nil(stat)
}

func testWriteData(f *easyfiles.File, require *require.Assertions) {
	writer, err := f.Writer(0)
	require.Nil(err)

	defer writer.Close()
	defer writer.Flush()

	_, err = writer.Write([]byte("Hello World"))
	require.Nil(err)
}

func TestOpenBadPath(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	// Open a file with a bad path
	f, err := OpenFile("../test/test.log", os.O_RDONLY, easyfiles.GZ_FALSE, client)
	require.NotNil(err)
	require.Nil(f)
}

func TestOpenGzUnknown(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	file := "/hdfs-test-open"
	// Try opening with GZ_UNKNOWN. This should fail
	f, err := OpenFile(file, os.O_CREATE|os.O_RDONLY, easyfiles.GZ_UNKNOWN, client)
	require.NotNil(err)
	require.Nil(f)
}

func TestOpenCreate(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	file := "/hdfs-test-create"
	// Create and open a file
	f, err := OpenFile(file, os.O_CREATE|os.O_RDONLY, easyfiles.GZ_FALSE, client)
	require.Nil(err)
	require.NotNil(f)
	f.Close()
	client.Remove(file)
}

func TestOpenModes(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	file := "/hdfs-test-open"
	// Create and open a file
	f, err := OpenFile(file, os.O_CREATE|os.O_RDONLY, easyfiles.GZ_FALSE, client)
	require.Nil(err)
	require.NotNil(f)
	f.Close()

	// Check if the file exists
	stat, err := client.Stat(file)
	require.Nil(err)
	require.NotNil(stat)

	// Test opening without O_CREATE
	f, err = OpenFile(file, os.O_RDONLY, easyfiles.GZ_FALSE, client)
	require.Nil(err)
	require.NotNil(f)
	f.Close()

	// Test opening with write
	f, err = OpenFile(file, os.O_WRONLY, easyfiles.GZ_FALSE, client)
	require.Nil(err)
	require.NotNil(f)
	f.Close()

	// Remove this file at the end
	client.Remove(file)

}

func TestReadWrite(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	// open with write
	file := "/hdfs-open-write"
	f, err := OpenFile(file, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_FALSE, client)
	require.Nil(err)
	require.NotNil(f)
	defer client.Remove(file)

	// Write
	testWriteData(f, require)
	f.Close()

	// Now check if the write actually worked
	f, err = OpenFile(file, os.O_RDONLY, easyfiles.GZ_FALSE, client)
	require.Nil(err)
	testReadData(f, require)
}

func TestReadWriteGz(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	client, err := NewHDFSClient("dirtydeeds.cse.buffalo.edu:9000")
	require.Nil(err)
	require.NotNil(client)

	// open with write
	file := "/hdfs-open-write-gz"
	f, err := OpenFile(file, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_TRUE, client)
	require.Nil(err)
	require.NotNil(f)
	defer client.Remove(file)

	// Write
	testWriteData(f, require)
	f.Close()

	// Now check if the write actually worked
	f, err = OpenFile(file, os.O_RDONLY, easyfiles.GZ_TRUE, client)
	require.Nil(err)
	testReadData(f, require)
}

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}
