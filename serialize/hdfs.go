package serialize

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-easyfiles/easyhdfs"
	log "github.com/sirupsen/logrus"
)

type HDFSSerializer struct {
	Addr string
}

func NewHDFSSerializer(addr string) *HDFSSerializer {
	return &HDFSSerializer{parseHDFSAddr(addr)}
}

func parseHDFSAddr(path string) string {
	path = stripHDFSPrefix(path)
	// Address is up to first '/'
	idx := strings.Index(path, "/")
	if idx == -1 {
		idx = len(path)
	}
	return path[:idx]
}

func stripHDFSPrefix(addr string) string {
	if strings.HasPrefix(addr, "hdfs://") {
		addr = addr[7:]
	}
	return addr
}

func (h *HDFSSerializer) OutPath(path string) (string, error) {
	hostnameIdx := strings.Index(path, parseHDFSAddr(h.Addr))
	if hostnameIdx == -1 {
		return "", fmt.Errorf("Path does not contain HDFS address prefix '%v'", h.Addr)
	} else {
		hostnameIdx += len(h.Addr)
	}
	return path[hostnameIdx:], nil
}

func (h *HDFSSerializer) Serialize(obj interface{}, filename string) error {
	// FIXME: We should use a pool of connections
	// This will blow up the number of connections if there are a large
	// number of goroutines.
	outPath, err := h.OutPath(filename)
	if err != nil {
		return err
	}
	log.Debugf("OutPath=%v\n", outPath)

	fileType := easyfiles.GZ_FALSE
	if strings.HasSuffix(outPath, ".gz") {
		fileType = easyfiles.GZ_TRUE
	}

	fs := easyhdfs.NewHDFSFileSystem(h.Addr)
	//Mkdirs
	outdir := path.Dir(outPath)
	if exists, _ := fs.Exists(outdir); !exists {
		err := fs.Makedirs(outdir)
		if err != nil {
			return fmt.Errorf("Failed to create directory: %v: %v", outdir, err)
		}
	}

	file, err := fs.Open(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileType)
	if err != nil {
		return fmt.Errorf("Failed to open file: %v: %v", outPath, err)
	}
	defer file.Close()

	writer, err := file.Writer(0)
	if err != nil {
		return fmt.Errorf("Failed to get writer to file: %v: %v", outPath, err)
	}
	defer writer.Close()
	defer writer.Flush()

	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return fmt.Errorf("Failed to marshal object to json: %v", err)
	}

	if _, err := writer.Write(b); err != nil {
		return fmt.Errorf("Failed to write json bytes to file: %v: %v", outPath, err)
	}
	return nil
}
