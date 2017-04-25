package serialize

import (
	"fmt"
	"strings"
)

func DetectSerializer(path string) (serializer Serializer, err error) {
	if strings.HasPrefix(path, "hdfs://") {
		serializer = NewHDFSSerializer(path)
	} else if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		serializer = &HTTPSerializer{}
	} else if strings.HasPrefix(path, "file://") {
		// Local
		serializer = &LocalSerializer{}
	} else {
		err = fmt.Errorf("Unknown protocol in path: %v", path)
	}
	return
}
