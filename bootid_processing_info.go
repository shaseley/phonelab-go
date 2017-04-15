package phonelab

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type StitchFileInfo struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
type StitchInfo struct {
	BootInfo map[string]map[string]*StitchFileInfo `json:"boot_info"`
	Files    []string                              `json:"files"`
}

func NewStitchInfo() *StitchInfo {
	info := &StitchInfo{}
	info.BootInfo = make(map[string]map[string]*StitchFileInfo)
	info.Files = make([]string, 0)
	return info
}

func (s *StitchInfo) BootIds() []string {
	res := make([]string, len(s.BootInfo))
	idx := 0
	for k, _ := range s.BootInfo {
		res[idx] = k
		idx++
	}
	return res
}

func (s *StitchInfo) String() string {
	b, _ := json.MarshalIndent(s, "", "    ")
	return string(b)
}

func GetInfoFromFile(path string) (info *StitchInfo, err error) {
	var fpath string
	var bytes []byte

	fpath = filepath.Join(path, "info.json")
	if bytes, err = ioutil.ReadFile(fpath); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read file:", fpath, ":", err)
		return
	}
	return GetInfoFromBytes(bytes)
}

func GetInfoFromBytes(b []byte) (*StitchInfo, error) {
	info := &StitchInfo{}
	if err := json.Unmarshal(b, &info); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to unmarshal bytes:", err)
		return nil, err
	}
	return info, nil
}
