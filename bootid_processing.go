package phonelab

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar"
	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-hdfs-doublestar"
	"github.com/shaseley/phonelab-go/hdfs"
	log "github.com/sirupsen/logrus"
)

type PhonelabSourceProcessor struct {
	Path      string
	Device    string
	Bootid    string
	HdfsAddr  string
	bootFiles []string
	ErrHandler
}

func NewPhonelabSourceProcessor(path, device, bootid, hdfsAddr string, errHandler ErrHandler) (*PhonelabSourceProcessor, error) {
	bootPath := filepath.Join(path, device, bootid)
	client, err := hdfs.NewHdfsClient(hdfsAddr)
	if err != nil {
		return nil, err
	}
	var bootFiles []string
	if client != nil {
		bootFiles, err = hdfs_doublestar.Glob(client, filepath.Join(bootPath, "*.gz"))
	} else {
		bootFiles, err = doublestar.Glob(filepath.Join(bootPath, "*.gz"))
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(bootFiles)
	return &PhonelabSourceProcessor{path, device, bootid, hdfsAddr, bootFiles, errHandler}, nil
}

func (psp *PhonelabSourceProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	client, err := hdfs.NewHdfsClient(psp.HdfsAddr)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to HDFS namenode at '%v': %v", psp.HdfsAddr, err))
	}

	go func() {
		for _, bootFile := range psp.bootFiles {
			file, err := hdfs.OpenFile(bootFile, os.O_RDONLY, easyfiles.GZ_TRUE, client)
			if err != nil {
				if psp.ErrHandler != nil {
					psp.ErrHandler(err)
				} else {
					panic(fmt.Sprintf("Failed to Open '%v': %v", bootFile, err))
				}
			}
			scanner, err := file.Reader(0)
			if err != nil {
				if psp.ErrHandler != nil {
					psp.ErrHandler(err)
				} else {
					panic(fmt.Sprintf("Failed to get scanner to '%v': %v", bootFile, err))
				}
			}
			scanner.Split(bufio.ScanLines)
			for scanner.Scan() {
				line := scanner.Text()
				outChan <- line
			}
		}
		close(outChan)
	}()
	return outChan
}

type PhonelabSourceGenerator struct {
	devicePaths map[string][]string
	hdfsAddr    string
	ErrHandler
}

func NewPhonelabSourceGenerator(devicePaths map[string][]string, hdfsAddr string, errHandler ErrHandler) *PhonelabSourceGenerator {
	return &PhonelabSourceGenerator{devicePaths, hdfsAddr, errHandler}
}

func (psg *PhonelabSourceGenerator) Process() <-chan *PipelineSourceInstance {
	sourceChan := make(chan *PipelineSourceInstance)

	client, err := hdfs.NewHdfsClient(psg.hdfsAddr)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to HDFS namenode at '%v': %v", psg.hdfsAddr, err))
	}

	log.Debugf("Paths: %v", psg.devicePaths)

	go func() {
		for device, basePaths := range psg.devicePaths {
			for _, basePath := range basePaths {
				infoJsonPath := filepath.Join(basePath, device, "info.json")
				if data, err := hdfs.ReadFile(infoJsonPath, client); err != nil {
					if psg.ErrHandler != nil {
						psg.ErrHandler(err)
					} else {
						panic(fmt.Sprintf("Error reading '%v': %v", infoJsonPath, err))
					}
				} else {
					infoJson := make(map[string][]string)
					if err := json.Unmarshal(data, &infoJson); err != nil {
						if psg.ErrHandler != nil {
							psg.ErrHandler(err)
						} else {
							panic(fmt.Sprintf("Error unmarshaling '%v': %v", infoJsonPath, err))
						}
					}
					bootids := infoJson["bootids"]
					for _, bootid := range bootids {
						info := make(PipelineSourceInfo)
						info["type"] = "phonelab-device"
						info["deviceid"] = device
						info["bootid"] = bootid
						info["basePath"] = basePath
						info["hdfsAddr"] = psg.hdfsAddr

						psp, err := NewPhonelabSourceProcessor(basePath, device, bootid, psg.hdfsAddr, psg.ErrHandler)
						if err != nil {
							if psg.ErrHandler != nil {
								psg.ErrHandler(err)
							} else {
								panic(fmt.Sprintf("Error creating new PhonelabSourceProcessor: %v", err))
							}
						}
						sourceChan <- &PipelineSourceInstance{
							Processor: psp,
							Info:      info,
						}
					}
				}
			}
		}
		close(sourceChan)
	}()
	return sourceChan
}
