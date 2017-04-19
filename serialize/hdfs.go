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
}

type HDFSSerializerArgs struct {
	*hdfs.HDFSClient
	Filename string
	FileType
}

func (h *HDFSSerializer) Serialize(obj interface{}, args interface{}) error {
	var err error

	hdfsArgs, ok := args.(*HDFSSerializerArgs)
	if !ok {
		return fmt.Errorf("Invalid args type.\nExpecting: %t\nGot: %t\n", HDFSSerializerArgs{}, args)
	}

	//Mkdirs
	outdir := path.Dir(hdfsArgs.Filename)
	err = hdfsArgs.MkdirAll(outdir, 0775)
	if err != nil {
		return fmt.Errorf("Failed to create directory: %v: %v", outdir, err)
	}

	filePath := hdfsArgs.Filename

	file, err := hdfs.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, easyfiles.FileType(hdfsArgs.FileType), hdfsArgs.HDFSClient)
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
