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
//	Should we support multiple pipelines?
//	Expand things to include data from device id formatted files

type PipelineRunnerConf struct {
	// Runner configuration
	MaxConcurrency uint   `yaml:"max_concurrency"`
	DataCollector  string `yaml:"data_collector"`

	SourceConf    *PipelineSourceConf `yaml:"source"`
	Preprocessors []*PipelineConf     `yaml:"preprocessors"`
	PipelineConf  *PipelineConf       `yaml:"pipeline"`
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

func (conf *PipelineConf) buildFilterProc(env *Environment, source Processor) Processor {
	filters := make([]StringFilter, 0)

	// Filters first
	for _, filterSpec := range conf.SimpleFilters {
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
	for _, filterName := range conf.ComplexFilters {
		if filter, ok := env.Filters[filterName]; ok {
			filters = append(filters, filter)
		}
	}

	if len(filters) > 0 {
		return NewStringFilterProcessor(source, filters)
	} else {
		return nil
	}
}

func (conf *PipelineConf) buildParserProc(env *Environment, source Processor) Processor {

	// Parsers: Get these from the environment instead of the conf; we'll parse
	// anything we know how to.
	parser := NewLoglineParser()

	for _, tag := range conf.Parsers {
		if parserGen, ok := env.Parsers[tag]; ok {
			parser.SetParser(tag, parserGen())
		}
	}

	return NewLoglineProcessor(source, parser)
}

func (conf *PipelineConf) buildProcessors(env *Environment, source Processor,
	info PipelineSourceInfo) Processor {

	// Process the list sequentially, using the previous processor as the source
	// for the next one.
	for _, procName := range conf.Processors {
		if procGen, ok := env.Processors[procName]; !ok {
			panic("Cannot find processor " + procName)
		} else {
			source = procGen.GenerateProcessor(&PipelineSourceInstance{
				Info:      info,
				Processor: source,
			})
		}
	}
	return source
}

// Build the first 2 stages of the pipeline - the filters and parsers.
func (conf *PipelineConf) buildInitialProcessing(env *Environment, source Processor) Processor {

	// Build the string filters, if any.
	if filter := conf.buildFilterProc(env, source); filter != nil {
		source = filter
	}

	// We'll have at least one parser for loglines
	source = conf.buildParserProc(env, source)

	return source
}

func (conf *PipelineConf) buildFullPipeline(env *Environment, source Processor,
	info PipelineSourceInfo) Processor {

	source = conf.buildInitialProcessing(env, source)
	return conf.buildProcessors(env, source, info)
}

func (conf *PipelineRunnerConf) ToRunner(env *Environment) (*Runner, error) {
	gen, err := conf.SourceConf.ToPipelineSourceGenerator()
	if err != nil {
		return nil, err
	}

	if err = conf.PipelineConf.validate(env); err != nil {
		return nil, err
	}

	for _, pp := range conf.Preprocessors {
		if err = pp.validate(env); err != nil {
			return nil, err
		}
	}

	proc := NewRunnerConfProcssor(conf, env)

	// If there was a custom data collector specified, use it. Otherwise,
	// use our /dev/null version.
	var collector DataCollector = proc

	if len(conf.DataCollector) > 0 {
		if cgen, ok := env.DataCollectors[conf.DataCollector]; ok {
			collector = cgen()
		} else {
			return nil, errors.New("Unknown DataCollector: " + conf.DataCollector)
		}
	}

	return NewRunner(gen, collector, proc), nil
}

////////////////////////////////////////////////////////////////////////////////
// DataCollector built from PipelineRunnerConf

type RunnerConfProcessor struct {
	Conf          *PipelineConf
	Preprocessors []*PipelineConf
	Env           *Environment
}

func NewRunnerConfProcssor(conf *PipelineRunnerConf, env *Environment) *RunnerConfProcessor {
	return &RunnerConfProcessor{
		Conf:          conf.PipelineConf,
		Preprocessors: conf.Preprocessors,
		Env:           env,
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

// This assumes the configuration makes sense and was checked by validate().
// FIXME: This probably isn't the best assumption.
func (proc *RunnerConfProcessor) BuildPipeline(sourceInst *PipelineSourceInstance) *Pipeline {

	source := sourceInst.Processor

	if len(proc.Preprocessors) == 0 {
		// In the simple case, we have no preprocessors or state trackers.
		// Here, we don't need to split the input and we can create a linear
		// pipeline.
		source = proc.Conf.buildFullPipeline(proc.Env, source, sourceInst.Info)
	} else {
		// When we have preprocessors, we need to take a more complicated
		// approach. Preprocessors may emit data at any rate, and may need to
		// emit data that goes backwards compared to the timestamps of their
		// input stream. Our approach is to create multiple input streams at the
		// file level, and merge them back together as one, ordered stream.
		//
		// So, we need to do the following:
		//	1) Split the input for |preprocessors| + 1
		//	2) Build a pipeline for each preprocessor stream
		//	3) Build the main pipeline filter/parser
		//	4) Merge all of these streams together
		//	5) Build and Pass the data to the main pipeline.

		// For files, we can just invoke Process() multiple times.  We can't
		// use a muxer because that sends the same object, which would be fine
		// if we had unlimited memory! But we dont, so we leave it on disk
		// until we can process it. TODO: Can we do better?

		// (1) and (2)
		pipelines := make([]Processor, 0)

		for _, ppConf := range proc.Preprocessors {
			pipelines = append(pipelines,
				ppConf.buildFullPipeline(proc.Env, source, sourceInst.Info))
		}

		// (3)
		source = proc.Conf.buildInitialProcessing(proc.Env, source)
		pipelines = append(pipelines, source)

		// (4)
		source = NewTimeweaverProcessor(pipelines[0], pipelines[1])

		for i := 2; i < len(pipelines); i++ {
			source = NewTimeweaverProcessor(source, pipelines[i])
		}

		// (5)
		source = proc.Conf.buildProcessors(proc.Env, source, sourceInst.Info)
	}

	return &Pipeline{
		LastHop: source,
	}
}

func (proc *RunnerConfProcessor) OnData(data interface{}) {}
func (proc *RunnerConfProcessor) Finish()                 {}
