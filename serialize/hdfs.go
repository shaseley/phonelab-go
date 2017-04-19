package serialize

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/gurupras/go-easyfiles"
	"github.com/shaseley/phonelab-go/hdfs"
)

type HDFSSerializer struct {
	Addr string
}

func NewHDFSSerializer(addr string) *HDFSSerializer {
	return &HDFSSerializer{addr}
}

type HDFSSerializerArgs struct {
	Filename string
	FileType
}

func (h *HDFSSerializer) Serialize(obj interface{}, args interface{}) error {
	var err error

	hdfsArgs, ok := args.(*HDFSSerializerArgs)
	if !ok {
		return fmt.Errorf("Invalid args type.\nExpecting: %t\nGot: %t\n", HDFSSerializerArgs{}, args)
	}

	// FIXME: We should use a pool of connections
	// This will blow up the number of connections if there are a large
	// number of goroutines.
	client, err := hdfs.NewHDFSClient(h.Addr)
	if err != nil {
		return fmt.Errorf("Failed to initialize HDFS client: %v", err)
	}

	//Mkdirs
	outdir := path.Dir(hdfsArgs.Filename)
	err = client.MkdirAll(outdir, 0775)
	if err != nil {
		return fmt.Errorf("Failed to create directory: %v: %v", outdir, err)
	}

	filePath := hdfsArgs.Filename

	file, err := hdfs.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, easyfiles.FileType(hdfsArgs.FileType), client)
	if err != nil {
		return fmt.Errorf("Failed to open file: %v: %v", filePath, err)
	}
	defer file.Close()

	writer, err := file.Writer(0)
	if err != nil {
		return fmt.Errorf("Failed to get writer to file: %v: %v", filePath, err)
	}
	defer writer.Close()
	defer writer.Flush()

	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return fmt.Errorf("Failed to marshal object to json: %v", err)
	}

	if _, err := writer.Write(b); err != nil {
		return fmt.Errorf("Failed to write json bytes to file: %v: %v", filePath, err)
	}
	return nil
}
