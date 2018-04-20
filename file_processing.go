package phonelab

import (
	"fmt"
	"os"
	"strings"

	"github.com/gurupras/go-easyfiles"
)

type TextFileProcessor struct {
	Filename string
	ErrHandler
}

type TextFileSourceInfo struct {
	Filename string
}

func (info *TextFileSourceInfo) Type() string {
	return "file"
}

func (info *TextFileSourceInfo) Context() string {
	return info.Filename
}

type ErrHandler func(error)

func NewTextFileProcessor(file string, errHandler ErrHandler) *TextFileProcessor {
	return &TextFileProcessor{
		Filename:   file,
		ErrHandler: errHandler,
	}
}

func (p *TextFileProcessor) processFile(outChan chan interface{}) {
	gz := easyfiles.GZ_FALSE
	if strings.HasSuffix(p.Filename, ".gz") || strings.HasSuffix(p.Filename, ".tgz") {
		gz = easyfiles.GZ_TRUE
	}

	file, err := easyfiles.LocalFS.Open(p.Filename, os.O_RDONLY, gz)
	if err != nil {
		if p.ErrHandler != nil {
			p.ErrHandler(err)
		} else {
			panic(fmt.Sprintf("Error opening file: %v", err))
		}
	}

	scanner, err := file.Reader(0)
	if err != nil {
		if p.ErrHandler != nil {
			p.ErrHandler(err)
		} else {
			panic(fmt.Sprintf("Failed to get scanner to '%v': %v", p.Filename, err))
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		outChan <- line
	}

	if err = scanner.Err(); err != nil {
		if p.ErrHandler != nil {
			p.ErrHandler(err)
		} else {
			panic(fmt.Sprintf("Error scanning file: %v", err))
		}
	}
}

func (p *TextFileProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	go func() {
		p.processFile(outChan)
		close(outChan)
	}()

	return outChan
}

// A source generator that generates one TextFileProcessor for each filename.
type TextFileSourceGenerator struct {
	Files      []string
	ErrHandler ErrHandler
}

func NewTextFileSourceGenerator(files []string, errFunc ErrHandler) *TextFileSourceGenerator {
	return &TextFileSourceGenerator{
		Files:      files,
		ErrHandler: errFunc,
	}
}

func (tf *TextFileSourceGenerator) Process() <-chan *PipelineSourceInstance {
	sourceChan := make(chan *PipelineSourceInstance)

	go func() {
		for _, file := range tf.Files {
			info := &TextFileSourceInfo{
				Filename: file,
			}

			sourceChan <- &PipelineSourceInstance{
				Processor: NewTextFileProcessor(file, tf.ErrHandler),
				Info:      info,
			}
		}
		close(sourceChan)
	}()

	return sourceChan
}
