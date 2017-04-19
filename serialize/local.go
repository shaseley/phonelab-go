package serialize

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/gurupras/go-easyfiles"
)

type LocalSerializer struct {
}

type LocalSerializerArgs struct {
	Filename string
}

func (h *LocalSerializer) Serialize(obj interface{}, args interface{}) error {
	localArgs, ok := args.(*LocalSerializerArgs)
	if !ok {
		return fmt.Errorf("Invalid args type.\nExpecting: %t\nGot: %t\n", LocalSerializerArgs{}, args)
	}

	dir := path.Dir(localArgs.Filename)
	if !easyfiles.Exists(dir) {
		if err := easyfiles.Makedirs(dir); err != nil {
			return err
		}
	}

	if b, err := json.MarshalIndent(obj, "", "    "); err != nil {
		return err
	} else {
		f, err := easyfiles.Open(localArgs.Filename, os.O_CREATE|os.O_WRONLY, easyfiles.GZ_UNKNOWN)
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
