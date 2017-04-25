package serialize

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/gurupras/go-easyfiles"
)

type LocalSerializer struct {
}

func (h *LocalSerializer) OutPath(path string) (string, error) {
	// Remove file://
	if strings.HasPrefix(path, "file://") {
		return path[7:], nil
	} else {
		return "", fmt.Errorf("Path '%v' does not contain 'file://'", path)
	}
}

func (h *LocalSerializer) Serialize(obj interface{}, filename string) error {
	outPath, err := h.OutPath(filename)
	if err != nil {
		return err
	}
	dir := path.Dir(outPath)
	if !easyfiles.Exists(dir) {
		if err := easyfiles.Makedirs(dir); err != nil {
			return err
		}
	}

	if b, err := json.MarshalIndent(obj, "", "    "); err != nil {
		return err
	} else {
		f, err := easyfiles.Open(outPath, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_UNKNOWN)
		if err != nil {
			return err
		}
		defer f.Close()

		writer, err := f.Writer(0)
		if err != nil {
			return err
		}
		defer writer.Close()
		defer writer.Flush()

		writer.Write(b)
	}
	return nil
}
