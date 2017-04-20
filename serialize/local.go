package serialize

import (
	"encoding/json"
	"os"
	"path"

	"github.com/gurupras/go-easyfiles"
)

type LocalSerializer struct {
}

func (h *LocalSerializer) Serialize(obj interface{}, filename string) error {
	dir := path.Dir(filename)
	if !easyfiles.Exists(dir) {
		if err := easyfiles.Makedirs(dir); err != nil {
			return err
		}
	}

	if b, err := json.MarshalIndent(obj, "", "    "); err != nil {
		return err
	} else {
		f, err := easyfiles.Open(filename, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_UNKNOWN)
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
