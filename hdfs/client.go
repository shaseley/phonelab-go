package hdfs

import (
	"strings"

	"github.com/colinmarc/hdfs"
)

type HDFSClient struct {
	*hdfs.Client
}

func NewHDFSClient(hdfsAddr string) (*HDFSClient, error) {
	if strings.Compare(hdfsAddr, "") == 0 {
		return nil, nil
	}
	client, err := hdfs.New(hdfsAddr)
	if err != nil {
		return nil, err
	}
	ret := &HDFSClient{client}
	return ret, nil
}
