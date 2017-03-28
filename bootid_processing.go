package phonelab

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar"
	"github.com/gurupras/go-easyfiles"
)

type PhonelabSourceProcessor struct {
	Path      string
	Device    string
	Bootid    string
	bootFiles []string
	ErrHandler
}

func NewPhonelabSourceProcessor(path, device, bootid string, errHandler ErrHandler) (*PhonelabSourceProcessor, error) {
	bootPath := filepath.Join(path, device, bootid)
	bootFiles, err := doublestar.Glob(filepath.Join(bootPath, "*.gz"))
	if err != nil {
		return nil, err
	}
	sort.Strings(bootFiles)
	return &PhonelabSourceProcessor{path, device, bootid, bootFiles, errHandler}, nil
}

func (psp *PhonelabSourceProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	go func() {
		for _, bootFile := range psp.bootFiles {
			file, err := easyfiles.Open(bootFile, os.O_RDONLY, easyfiles.GZ_UNKNOWN)
			if err != nil {
				if psp.ErrHandler != nil {
					psp.ErrHandler(err)
				} else {
					panic(fmt.Sprintf("Failed to Open '%v': %v", bootFile, err))
				}
			}
			reader, err := file.Reader(0)
			if err != nil {
				if psp.ErrHandler != nil {
					psp.ErrHandler(err)
				} else {
					panic(fmt.Sprintf("Failed to get reader to '%v': %v", bootFile, err))
				}
			}
			reader.Split(bufio.ScanLines)
			for reader.Scan() {
				line := reader.Text()
				outChan <- line
			}
		}
		close(outChan)
	}()
	return outChan
}

type PhonelabSourceGenerator struct {
	devicePaths map[string][]string
	ErrHandler
}

func NewPhonelabSourceGenerator(devicePaths map[string][]string, errHandler ErrHandler) *PhonelabSourceGenerator {
	return &PhonelabSourceGenerator{devicePaths, errHandler}
}

func (psg *PhonelabSourceGenerator) Process() <-chan *PipelineSourceInstance {
	sourceChan := make(chan *PipelineSourceInstance)

	go func() {
		for device, basePaths := range psg.devicePaths {
			for _, basePath := range basePaths {
				infoJsonPath := filepath.Join(basePath, device, "info.json")
				if data, err := ioutil.ReadFile(infoJsonPath); err != nil {
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

						psp, err := NewPhonelabSourceProcessor(basePath, device, bootid, psg.ErrHandler)
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
