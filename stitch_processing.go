package phonelab

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/set"
	"github.com/gurupras/go-daterange"
	"github.com/gurupras/go-easyfiles"
	"github.com/gurupras/go-easyfiles/easyhdfs"
	log "github.com/sirupsen/logrus"
)

type PhonelabRawProcessor struct {
	*PhonelabRawInfo
	Files []string
	ErrHandler
}

type PhonelabRawInfo struct {
	*StitchInfo
	FSInterface   easyfiles.FileSystemInterface
	Path          string
	ProcessedPath string
	DeviceId      string
	HdfsAddr      string
	DateRange     *daterange.DateRange
}

func (info *PhonelabRawInfo) Type() string {
	return "phonelab-raw"
}

func (info *PhonelabRawInfo) Context() string {
	return info.DeviceId
}

func NewPhonelabRawProcessor(sourceInfo *PhonelabRawInfo, files []string, errHandler ErrHandler) (*PhonelabRawProcessor, error) {
	return &PhonelabRawProcessor{sourceInfo, files, errHandler}, nil
}

func (prp *PhonelabRawProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	var filteredFiles []string
	if prp.PhonelabRawInfo.DateRange != nil {
		dateRange := prp.PhonelabRawInfo.DateRange
		// Files are in the format time/YYYY/mm/dd.out.gz
		filteredFiles = make([]string, 0)
		for _, file := range prp.Files {
			tmp := path.Base(file)[:2]
			day, err := strconv.Atoi(tmp)
			if err != nil {
				panic(fmt.Sprintf("Failed to convert '%v' to int", tmp))
			}
			tmp = path.Base(path.Dir(file))
			month, err := strconv.Atoi(tmp)
			if err != nil {
				panic(fmt.Sprintf("Failed to convert '%v' to int", tmp))
			}
			tmp = path.Base(path.Dir(path.Dir(file)))
			year, err := strconv.Atoi(tmp)
			if err != nil {
				panic(fmt.Sprintf("Failed to convert '%v' to int", tmp))
			}
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			if dateRange.ContainsDate(date) {
				filteredFiles = append(filteredFiles, file)
			}
		}
	} else {
		filteredFiles = prp.Files
	}
	log.Infof("Files: %v", filteredFiles)

	go func() {
		for _, f := range filteredFiles {
			outChan <- f
		}
		close(outChan)
	}()
	return outChan
}

type PhonelabRawGenerator struct {
	devicePaths map[string]string
	Args        map[string]interface{}
	ErrHandler
}

func NewPhonelabRawGenerator(devicePaths map[string]string, args map[string]interface{}, errHandler ErrHandler) *PhonelabRawGenerator {
	return &PhonelabRawGenerator{devicePaths, args, errHandler}
}

func (prg *PhonelabRawGenerator) Process() <-chan *PipelineSourceInstance {
	sourceChan := make(chan *PipelineSourceInstance)

	// Parse hdfs address
	var hdfsAddr string
	if v, ok := prg.Args["hdfs_addr"]; ok {
		hdfsAddr = v.(string)
	}

	var fs easyfiles.FileSystemInterface

	if strings.Compare(hdfsAddr, "") == 0 {
		fs = easyfiles.LocalFS
	} else {
		fs = easyhdfs.NewHDFSFileSystem(hdfsAddr)
	}

	// Parse date range
	var dateRange *daterange.DateRange
	if v, ok := prg.Args["daterange"]; ok {
		var err error
		if dateRange, err = ParseDateRange(v.(string)); err != nil {
			panic(fmt.Sprintf("Unable to parse daterange: %v", err))
		}
	}

	// Get processed path
	var processedPath string
	if v, ok := prg.Args["processed_path"]; !ok {
		panic(fmt.Sprintf("No processed path defined."))
	} else {
		processedPath = v.(string)
	}
	log.Infof("Processed path: %v", processedPath)

	go func() {
		for device, basePath := range prg.devicePaths {
			currentFiles := set.NewNonTS()
			log.Infof("device=%v basePath=%v", device, basePath)
			filePattern := filepath.Join(basePath, device, "time", "**/*.out.gz")
			var files []string
			var diffSet set.Interface
			curFiles, err := fs.Glob(filePattern)
			for _, obj := range curFiles {
				currentFiles.Add(obj)
			}

			var info *StitchInfo
			// Try to pull and read info.json if it exists
			infoJsonPath := filepath.Join(processedPath, device, "info.json")
			log.Infof("infoJsonPath=%v", infoJsonPath)
			if data, err := fs.ReadFile(infoJsonPath); err == nil {
				log.Infof("Found info.json")
				// We've processed a portion of the currentFiles.
				// Don't re-process these
				if info, err = GetInfoFromBytes(data); err != nil {
					if prg.ErrHandler != nil {
						prg.ErrHandler(err)
					} else {
						panic(fmt.Sprintf("Error unmarshaling '%v': %v", infoJsonPath, err))
					}
				}

				// Make sure we clear out any files and directories that are not found in info.json
				// This ensures that any previous run that failed mid-way while adding new files
				// don't persist since info.json is only updated after everything else has succeeded.
				// First, remote any extraneous bootIDs
				log.Infof("Checking for extraneous bootIDs and files...")
				validBootIds := set.NewNonTS()
				for _, bootId := range info.BootIds() {
					validBootIds.Add(bootId)
				}
				log.Infof("Valid bootIDs: %v", validBootIds)

				bootIdsFound, err := fs.Glob(filepath.Join(processedPath, device, "*-*-*"))
				if err != nil {
					if prg.ErrHandler != nil {
						prg.ErrHandler(err)
					} else {
						panic(fmt.Sprintf("Error globbing bootIds for device '%v': %v", device, err))
					}
				}
				existingBootIds := set.NewNonTS()
				for _, bootIdPath := range bootIdsFound {
					bootId := path.Base(bootIdPath)
					existingBootIds.Add(bootId)
				}
				log.Infof("Existing bootIDs: %v", existingBootIds)

				// Get the difference
				extraneousBootIds := set.Difference(existingBootIds, validBootIds)
				log.Warnf("Extraneous bootIDs: %v", extraneousBootIds)
				// Any remaining bootID is extraneous
				for _, obj := range extraneousBootIds.List() {
					b := obj.(string)
					bootIdPath := filepath.Join(processedPath, device, b)
					log.Warnf("Deleting: %v", bootIdPath)
					if err := fs.RemoveAll(bootIdPath); err != nil {
						if prg.ErrHandler != nil {
							prg.ErrHandler(err)
						} else {
							panic(fmt.Sprintf("Failed to remove extraneous bootID '%v': %v", bootIdPath, err))
						}
					}
				}

				// Now, remove any extraneous files within valid bootIDs
				for bootId, data := range info.BootInfo {
					bootIdPath := filepath.Join(processedPath, device, bootId)
					validFiles := set.NewNonTS()
					existingFiles := set.NewNonTS()
					for file := range data {
						validFiles.Add(file)
					}
					filesFound, err := fs.Glob(filepath.Join(bootIdPath, "*.gz"))
					if err != nil {
						if prg.ErrHandler != nil {
							prg.ErrHandler(err)
						} else {
							panic(fmt.Sprintf("Error globbing bootId files '%v': %v", bootIdPath, err))
						}
					}
					for _, file := range filesFound {
						// Extract name alone
						name := path.Base(file)
						existingFiles.Add(name)
					}
					// Now, find the set difference and remove any extraneous files
					extraneousFiles := set.Difference(existingFiles, validFiles)
					for _, obj := range extraneousFiles.List() {
						f := obj.(string)
						f = filepath.Join(bootIdPath, f)
						log.Warnf("Deleting: %v", f)
						if err := fs.Remove(f); err != nil {
							if prg.ErrHandler != nil {
								prg.ErrHandler(err)
							} else {
								panic(fmt.Sprintf("Failed to remove extraneous bootID file '%v': %v", f, err))
							}
						}
					}
				}

				processedFiles := set.NewNonTS()
				for _, obj := range info.Files {
					processedFiles.Add(obj)
				}

				diffSet = set.Difference(currentFiles, processedFiles)
			} else {
				diffSet = currentFiles
			}
			files = make([]string, diffSet.Size())
			for idx, obj := range diffSet.List() {
				files[idx] = obj.(string)
			}

			sourceInfo := &PhonelabRawInfo{
				DeviceId:      device,
				Path:          basePath,
				ProcessedPath: processedPath,
				FSInterface:   fs,
				StitchInfo:    info,
				DateRange:     dateRange,
			}

			prp, err := NewPhonelabRawProcessor(sourceInfo, files, prg.ErrHandler)
			if err != nil {
				if prg.ErrHandler != nil {
					prg.ErrHandler(err)
				} else {
					panic(fmt.Sprintf("Error creating new PhonelabRawProcessor: %v", err))
				}
			}
			sourceChan <- &PipelineSourceInstance{
				Processor: prp,
				Info:      sourceInfo,
			}
		}
		close(sourceChan)
	}()
	return sourceChan
}
