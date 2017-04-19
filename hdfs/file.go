package hdfs

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-easyfiles/easyhdfs"
	log "github.com/sirupsen/logrus"
)

func OpenFile(path string, mode int, gz easyfiles.FileType, client *HDFSClient) (*easyfiles.File, error) {
	if client == nil {
		return easyfiles.Open(path, mode, gz)
	} else {
		// First, make sure gz is not UNKNOWN. We don't run the usual .gz extension tests
		// that exist in easyfiles.Open(). Thus we only handle GZ_(TRUE|FALSE)
		if gz == easyfiles.GZ_UNKNOWN {
			return nil, fmt.Errorf("easyhdfs cannot handle GZ_UNKNOWN. Must be GZ_TRUE or GZ_FALSE.")
		}
		hdfsFile := &easyhdfs.HdfsFile{path, nil, nil, client.Client}
		// Check if file exists
		// If a file does not exist, this throws an error
		stat, err := client.Stat(path)
		_ = stat
		truncCreate := false
		if err == nil {
			if stat == nil {
				// That's weird ...
				log.Fatalf("stat and err are nil for file: %v", path)
			}
			// We have a file that exists
			// Do we need to truncate it?
			log.Debugf("stat is not nil.. File exists. Check if we need to truncate: %v", path)
			if mode&os.O_TRUNC > 0 || (mode&os.O_WRONLY > 0 && mode&os.O_APPEND == 0) {
				log.Debugf("Truncating file: %v", path)
				// If O_TRUNC is set, then truncate
				// Otherwise, if in write mode and
				// no O_APPEND is set, truncate
				err = client.Remove(path)
				if err != nil {
					return nil, fmt.Errorf("%v", err)
				}
				truncCreate = true
			}
		}
		if err != nil || truncCreate {
			// File does not exist
			// Check if we have to create it
			log.Debugf("Check if we have to create file: %v", path)
			if mode&os.O_CREATE > 0 || truncCreate {
				log.Debugf("Creating empty file: %v", path)
				if err := client.CreateEmptyFile(path); err != nil {
					return nil, err
				}
			}
		}
		// At this point, we're sure the file exists
		// Get reader to file
		log.Debugf("Opening file with read: %v", path)
		f, err := client.Open(path)
		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}
		hdfsFile.FileReader = f

		if mode&os.O_WRONLY > 0 || mode&os.O_RDWR > 0 {
			// We don't need to check for O_APPEND, because that's
			// the only mode we support
			log.Debugf("Opening file with write: %v", path)
			w, err := client.Append(path)
			if err != nil {
				return nil, fmt.Errorf("%v", err)
			}
			hdfsFile.Writer = w
		}
		file := &easyfiles.File{path, hdfsFile, mode, gz}
		return file, nil
	}
}

func ReadFile(path string, client *HDFSClient) ([]byte, error) {
	if client != nil {
		return client.ReadFile(path)
	} else {
		return ioutil.ReadFile(path)
	}
}
