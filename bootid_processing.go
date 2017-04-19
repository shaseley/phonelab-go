package phonelab

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar"
	"github.com/gurupras/go-daterange"
	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-hdfs-doublestar"
	"github.com/shaseley/phonelab-go/hdfs"
	log "github.com/sirupsen/logrus"
)

type PhonelabSourceProcessor struct {
	*PhonelabSourceInfo
	bootFiles []string
	ErrHandler
}

type PhonelabSourceInfo struct {
	*StitchInfo
	Path      string
	DeviceId  string
	BootId    string
	HdfsAddr  string
	DateRange *daterange.DateRange
}

func NewPhonelabSourceProcessor(sourceInfo *PhonelabSourceInfo, errHandler ErrHandler) (*PhonelabSourceProcessor, error) {
	path := sourceInfo.Path
	device := sourceInfo.DeviceId
	bootId := sourceInfo.BootId
	hdfsAddr := sourceInfo.HdfsAddr

	bootPath := filepath.Join(path, device, bootId)

	client, err := hdfs.NewHDFSClient(hdfsAddr)
	if err != nil {
		return nil, err
	}
	var bootFiles []string
	if client != nil {
		bootFiles, err = hdfs_doublestar.Glob(client.Client, filepath.Join(bootPath, "*.gz"))
	} else {
		bootFiles, err = doublestar.Glob(filepath.Join(bootPath, "*.gz"))
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(bootFiles)
	return &PhonelabSourceProcessor{sourceInfo, bootFiles, errHandler}, nil
}

func (psp *PhonelabSourceProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	client, err := hdfs.NewHDFSClient(psp.HdfsAddr)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to HDFS namenode at '%v': %v", psp.HdfsAddr, err))
	}

	go func() {
		var startIdx int = 0
		var endIdx int = len(psp.bootFiles)
		// Get as close to the requested daterange as possible
		bootPath := filepath.Join(psp.Path, psp.DeviceId, psp.BootId)
		if psp.DateRange != nil {
			for idx, bootFile := range psp.bootFiles {
				rel, err := filepath.Rel(bootPath, bootFile)
				if err != nil {
					if psp.ErrHandler != nil {
						psp.ErrHandler(err)
					} else {
						panic(fmt.Sprintf("Failed to get relative path to: %v: %v", bootFile, err))
					}
				}
				startTimestamp := psp.BootInfo[psp.BootId][rel].Start
				if startTimestamp < psp.DateRange.Start.Time.UnixNano() {
					startIdx = idx
				}
				if startTimestamp > psp.DateRange.End.Time.UnixNano() {
					endIdx = idx
					break
				}
			}
		}
		log.Debugf("%v->%v range=%v-%v", psp.DeviceId, psp.BootId, startIdx, endIdx)

		for _, bootFile := range psp.bootFiles[startIdx:endIdx] {
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
	Args        map[string]interface{}
	ErrHandler
}

func NewPhonelabSourceGenerator(devicePaths map[string][]string, args map[string]interface{}, errHandler ErrHandler) *PhonelabSourceGenerator {
	return &PhonelabSourceGenerator{devicePaths, args, errHandler}
}

func (psg *PhonelabSourceGenerator) Process() <-chan *PipelineSourceInstance {
	sourceChan := make(chan *PipelineSourceInstance)

	// Parse hdfs address
	var hdfsAddr string
	if v, ok := psg.Args["hdfs_addr"]; ok {
		hdfsAddr = v.(string)
	}

	// Parse date range
	var dateRange *daterange.DateRange
	if v, ok := psg.Args["daterange"]; ok {
		var err error
		if dateRange, err = ParseDateRange(v.(string)); err != nil {
			panic(fmt.Sprintf("Unable to parse daterange: %v", err))
		}
	}

	client, err := hdfs.NewHDFSClient(hdfsAddr)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to HDFS namenode at '%v': %v", hdfsAddr, err))
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
					var info *StitchInfo
					if info, err = GetInfoFromBytes(data); err != nil {
						if psg.ErrHandler != nil {
							psg.ErrHandler(err)
						} else {
							panic(fmt.Sprintf("Error unmarshaling '%v': %v", infoJsonPath, err))
						}
					}
					bootids := info.BootIds()
					for _, bootid := range bootids {
						psInfo := make(PipelineSourceInfo)
						psInfo["type"] = "phonelab-device"
						sourceInfo := &PhonelabSourceInfo{}
						sourceInfo.DeviceId = device
						sourceInfo.BootId = bootid
						sourceInfo.Path = basePath
						sourceInfo.HdfsAddr = hdfsAddr
						sourceInfo.DateRange = dateRange
						sourceInfo.StitchInfo = info
						psInfo["source_info"] = sourceInfo

						psp, err := NewPhonelabSourceProcessor(sourceInfo, psg.ErrHandler)
						if err != nil {
							if psg.ErrHandler != nil {
								psg.ErrHandler(err)
							} else {
								panic(fmt.Sprintf("Error creating new PhonelabSourceProcessor: %v", err))
							}
						}
						sourceChan <- &PipelineSourceInstance{
							Processor: psp,
							Info:      psInfo,
						}
					}
				}
			}
		}
		close(sourceChan)
	}()
	return sourceChan
}
