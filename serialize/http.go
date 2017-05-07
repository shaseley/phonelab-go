package serialize

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/fatih/set"
	"github.com/gorilla/mux"
	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-stoppable-net-listener"
	"github.com/labstack/gommon/log"
	"github.com/parnurzeal/gorequest"
)

type HTTPSerializer struct {
}

func (h *HTTPSerializer) OutPath(path string) (string, error) {
	return path, nil
}

func (h *HTTPSerializer) Serialize(obj interface{}, url string) error {
	request := gorequest.New()
	resp, _, errors := request.Post(url).Send(obj).End()
	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Status)
	}
	return nil
}

type HTTPReceiver struct {
	BasePath  string
	callbacks set.Interface
}

func NewHTTPReceiver(path string) *HTTPReceiver {
	return &HTTPReceiver{path, set.New()}
}

type HTTPCallback interface {
	Data() []byte
	Request() *http.Request
}

type httpCallbackData struct {
	data    *bytes.Buffer
	request *http.Request
}

func (h *httpCallbackData) Data() []byte {
	return h.data.Bytes()
}

func (h *httpCallbackData) Request() *http.Request {
	return h.request
}

func (h *HTTPReceiver) AddCallback(cb chan HTTPCallback) {
	if !h.callbacks.Has(cb) {
		h.callbacks.Add(cb)
	}
}

func (h *HTTPReceiver) RemoveCallback(cb chan HTTPCallback) {
	if h.callbacks.Has(cb) {
		h.callbacks.Remove(cb)
	}
}

func httpSerialize(basePath string, data []byte, r *http.Request) {
	vars := mux.Vars(r)
	relpath := vars["relpath"]
	log.Debugf("Received file '%v'", relpath)

	dir := path.Dir(relpath)
	outdir := filepath.Join(basePath, dir)
	filename := path.Base(relpath)
	filePath := filepath.Join(outdir, filename)
	log.Debugf("Writing '%v' > '%v'", relpath, filePath)

	if !easyfiles.Exists(outdir) {
		if err := easyfiles.Makedirs(outdir); err != nil {
			log.Errorf("Failed to create directory: %v: %v", outdir, err)
			return
		}
	}

	f, err := easyfiles.Open(filePath, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_UNKNOWN)
	if err != nil {
		log.Errorf("Failed to open file: %v: %v", filePath, err)
		return
	}
	defer f.Close()

	writer, err := f.Writer(0)
	if err != nil {
		log.Errorf("Failed to get writer to file: %v: %v", filePath, err)
		return
	}
	defer writer.Close()
	defer writer.Flush()

	if _, err = writer.Write(data); err != nil {
		log.Errorf("Failed to write data to file: %v: %v", filePath, err)
	}
}

func (h *HTTPReceiver) AddHTTPSerializeCallback() {
	c := make(chan HTTPCallback)
	go func() {
		for cbData := range c {
			httpSerialize(h.BasePath, cbData.Data(), cbData.Request())
		}
	}()
	h.AddCallback(c)
}

func (h *HTTPReceiver) Handle(w http.ResponseWriter, r *http.Request) {

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to copy body: %v", err)))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

	httpData := &httpCallbackData{buf, r}
	for _, obj := range h.callbacks.List() {
		c := obj.(chan HTTPCallback)
		c <- httpData
	}
}

func (h *HTTPReceiver) RunHTTPReceiver(port int) error {
	r := mux.NewRouter()
	r.HandleFunc("/upload/{relpath:[\\S+/]+}", h.Handle)
	http.Handle("/", r)

	snl, err := stoppablenetlistener.New(port)
	if err != nil {
		return err
	}
	snl.Timeout = 100 * time.Millisecond

	go func() {
		server := http.Server{}
		server.Serve(snl)
	}()
	return nil
}
