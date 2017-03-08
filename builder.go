package phonelab

import (
	"errors"
	"fmt"
	"github.com/bmatcuk/doublestar"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
)

// Build a pipeline from a yaml file.

// TODO:
//	Should we support multiple piplines?
//	Expand things to include data from device id formatted files

type PipelineRunnerConf struct {
	// Runner configuration
	MaxConcurrency uint `yaml:"max_concurrency"`

	SourceConf   *PipelineSourceConf `yaml:"source"`
	PipelineConf *PipelineConf       `yaml:"pipeline"`
}

type PipelineSourceType string

const (
	PipelineSourceFile PipelineSourceType = "files"
)

type PipelineSourceConf struct {
	Type    PipelineSourceType `yaml:"type"`
	Sources []string           `yaml:"sources"`
}

type SimpleFilterConf struct {
	Substrings []string `yaml:"substrings"`
}

type PipelineConf struct {
	SimpleFilters  []*SimpleFilterConf `yaml:"simple_filters"`
	ComplexFilters []string            `yaml:"complex_filters"`
	Parsers        []string            `yaml:"parsers"`
	Processors     []string            `yaml:"processors"`
}

func RunnerConfFromString(text string) (*PipelineRunnerConf, error) {
	spec := &PipelineRunnerConf{}

	err := yaml.Unmarshal([]byte(text), spec)

	if err != nil {
		return nil, err
	}

	return spec, nil
}

func RunnerConfFromFile(file string) (*PipelineRunnerConf, error) {
	var err error

	if _, err = os.Stat(file); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading file %v: %v", file, err)
	}

	return RunnerConfFromString(string(data))
}

// Convert the source specification into something that can generate loglines.
func (conf *PipelineSourceConf) ToPipelineSourceGenerator() (PipelineSourceGenerator, error) {

	switch conf.Type {
	default:
		return nil, errors.New("Invalid type specification: " + string(conf.Type))
	case PipelineSourceFile:
		if len(conf.Sources) == 0 {
			return nil, errors.New("Missing sources specification in runner conf.")
		}

		allFiles := make([]string, 0)

		for _, source := range conf.Sources {
			if len(source) == 0 {
				return nil, errors.New("Invalid source file: empty name")
			}
			if files, err := doublestar.Glob(source); err != nil {
				return nil, fmt.Errorf("Error globbing files: %v", err)
			} else {
				allFiles = append(allFiles, files...)
			}
		}

		errHandler := func(err error) {
			panic(err)
		}

		return NewTextFileSourceGenerator(allFiles, errHandler), nil
	}
}

func (conf *PipelineConf) validate(env *Environment) error {

	if len(conf.SimpleFilters) > 0 {
		for _, filterSpec := range conf.SimpleFilters {
			if len(filterSpec.Substrings) == 0 {
				return errors.New("Simple filter spec must contain at least one condition.")
			} else {
				for _, cond := range filterSpec.Substrings {
					if len(cond) == 0 {
						return errors.New("Invalid filter spec string: string cannot be empty.")
					}
				}
			}
		}
	}

	if len(conf.ComplexFilters) > 0 {
		for _, filterName := range conf.ComplexFilters {
			if len(filterName) == 0 {
				return errors.New("Invalid filter name: name cannot be empty.")
			} else if _, ok := env.Filters[filterName]; !ok {
				return errors.New("Unknown filter: " + filterName)
			}
		}
	}

	if len(conf.Parsers) > 0 {
		for _, parser := range conf.Parsers {
			if len(parser) == 0 {
				return errors.New("Invalid tag: tag cannot be empty.")
			}
		}
	}

	if len(conf.Processors) > 0 {
		for _, procName := range conf.Processors {
			if len(procName) == 0 {
				return errors.New("Invalid filter name: name cannot be empty.")
			} else if _, ok := env.Processors[procName]; !ok {
				return errors.New("Unknown Processor: " + procName)
			}
		}
	} else {
		return errors.New("Must specify at least one processor")
	}

	return nil
}

func (conf *PipelineRunnerConf) ToRunner(env *Environment) (*Runner, error) {
	gen, err := conf.SourceConf.ToPipelineSourceGenerator()
	if err != nil {
		return nil, err
	}

	if err = conf.PipelineConf.validate(env); err != nil {
		return nil, err
	}

	proc := NewRunnerConfProcssor(conf, env)

	return NewRunner(gen, proc), nil
}

////////////////////////////////////////////////////////////////////////////////
// DataProcessor built from PipelineRunnerConf

type RunnerConfProcessor struct {
	Conf      *PipelineConf
	Env       *Environment
	validated bool
}

func NewRunnerConfProcssor(conf *PipelineRunnerConf, env *Environment) *RunnerConfProcessor {
	return &RunnerConfProcessor{
		Conf: conf.PipelineConf,
		Env:  env,
	}
}

func makeStringFilterFunc(substrings []string) StringFilter {
	return func(check string) bool {
		for _, s := range substrings {
			if strings.Index(check, s) < 0 {
				return false
			}
		}
		return true
	}
}

// This assumes the configuration makes sense
// FIXME: This probably isn't the best assumption.
func (proc *RunnerConfProcessor) BuildPipeline(sourceInst *PipelineSourceInstance) Pipeline {

	filters := make([]StringFilter, 0)
	source := sourceInst.Processor

	// Filters first
	for _, filterSpec := range proc.Conf.SimpleFilters {
		if len(filterSpec.Substrings) > 0 {
			valid := []string{}
			for _, cond := range filterSpec.Substrings {
				if len(cond) > 0 {
					valid = append(valid, cond)
				}
			}
			if len(valid) > 0 {
				filters = append(filters, makeStringFilterFunc(valid))
			}
		}
	}

	// Complex filters
	for _, filterName := range proc.Conf.ComplexFilters {
		if filter, ok := proc.Env.Filters[filterName]; ok {
			filters = append(filters, filter)
		}
	}

	if len(filters) > 0 {
		source = NewStringFilterProcessor(source, filters)
	}

	// Parsers: Get these from the environment instead of the conf; we'll parse
	// anything we know how to.
	parser := NewLoglineParser()
	for _, tag := range proc.Conf.Parsers {
		if parserGen, ok := proc.Env.Parsers[tag]; ok {
			parser.SetParser(tag, parserGen())
		}
	}

	source = NewLoglineProcessor(source, parser)

	for _, procName := range proc.Conf.Processors {
		if procGen, ok := proc.Env.Processors[procName]; !ok {
			panic("Cannot find processor " + procName)
		} else {
			source = procGen.GenerateProcessor(&PipelineSourceInstance{
				Info:      sourceInst.Info,
				Processor: source,
			})
		}
	}

	return []PipelineSink{
		&RunnerProcSink{
			Source: source,
		},
	}
}

func (proc *RunnerConfProcessor) Finish() {}

////////////////////////////////////////////////////////////////////////////////
// Simple sink built from PipelineRunnerConf

// Simple data sink that collects everything
type RunnerProcSink struct {
	Source Processor
}

func (sink *RunnerProcSink) GetSource() Processor    { return sink.Source }
func (sink *RunnerProcSink) OnData(data interface{}) {}
func (sink *RunnerProcSink) OnFinish()               {}
