package serialize

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-stoppable-net-listener"
	"github.com/stretchr/testify/require"
)

func TestHTTPSerialize(t *testing.T) {
	require := require.New(t)

	httpReceiver := NewHTTPReceiver("test")

	r := mux.NewRouter()
	r.HandleFunc("/upload/{relpath:[\\S+/]+}", httpReceiver.Handle)
	http.Handle("/", r)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		server := http.Server{}
		snl, err := stoppablenetlistener.New(41121)
		require.Nil(err)
		snl.Timeout = 100 * time.Millisecond
		server.Serve(snl)
	}()

	time.Sleep(100 * time.Millisecond)

	// Now upload some data
	data := []string{"Hello", "World"}

	url := "http://127.0.0.1:41121/upload/dummyPath/http-upload-test.gz"

	serializer := &HTTPSerializer{}
	err := serializer.Serialize(data, url)
	require.Nil(err)
	defer os.RemoveAll("test/dummyPath/")

	// Now check the data
	f, err := easyfiles.Open("test/dummyPath/http-upload-test.gz", os.O_RDONLY, easyfiles.GZ_UNKNOWN)
	require.Nil(err)
	defer f.Close()

	reader, err := f.RawReader()
	require.Nil(err)

	var got []string
	err = json.NewDecoder(reader).Decode(&got)
	require.Nil(err)

	require.True(reflect.DeepEqual(data, got))
}

func TestHTTPSerializerBadArgs(t *testing.T) {
	require := require.New(t)

	serializer := &HTTPSerializer{}

	err := serializer.Serialize(nil, "")
	require.NotNil(err)
}
