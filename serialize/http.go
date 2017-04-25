package serialize

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-stoppable-net-listener"
	"github.com/parnurzeal/gorequest"
	log "github.com/sirupsen/logrus"
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
	BasePath string
}

func NewHTTPReceiver(path string) *HTTPReceiver {
	return &HTTPReceiver{path}
}

func (h *HTTPReceiver) Handle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	relpath := vars["relpath"]
	log.Debugf("Received file '%v'", relpath)

	dir := path.Dir(relpath)
	outdir := filepath.Join(h.BasePath, dir)
	filename := path.Base(relpath)
	filePath := filepath.Join(outdir, filename)
	log.Debugf("Writing '%v' > '%v'", relpath, filePath)

	if !easyfiles.Exists(outdir) {
		if err := easyfiles.Makedirs(outdir); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Failed to create directory: %v: %v", outdir, err)))
			return
		}
	}

	f, err := easyfiles.Open(filePath, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_UNKNOWN)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to open file: %v: %v", filePath, err)))
		return
	}
	defer f.Close()

	writer, err := f.Writer(0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to get writer to file: %v: %v", filePath, err)))
		return
	}
	defer writer.Close()
	defer writer.Flush()

	if _, err := io.Copy(writer, r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to get write to file: %v: %v", filePath, err)))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *HTTPReceiver) RunHTTPReceiver(port int) error {
	r := mux.NewRouter()
	r.HandleFunc("/upload/{relpath:[\\S+/]+}", h.Handle)
	http.Handle("/", r)

	snl, err := stoppablenetlistener.New(41121)
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
