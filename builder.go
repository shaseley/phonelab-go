package phonelab

import (
	"errors"
	"fmt"
	"github.com/bmatcuk/doublestar"
	"github.com/shaseley/depgraph"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
)

// Build a pipeline from a yaml file.

// TODO:
//	Expand things to include data from device id formatted files

////////////////////////////////////////////////////////////////////////////////
// Runner Configuration

type RunnerConf struct {
	MaxConcurrency uint                `yaml:"max_concurrency"` // Number of concurrent pipelines
	DataCollector  string              `yaml:"data_collector"`  // DataCollector to hook up to the sink
	SourceConf     *PipelineSourceConf `yaml:"source"`          // Source specification
	Processors     []*ProcessorConf    `yaml:"processors"`      // Custom processors that are defined here (as opposed to in a separate file).
	SinkName       string              `yaml:"sink_name"`       // Name of the sink/last-hop processor.
}

////////////////////////////////////////////////////////////////////////////////
// Pipeline source

type PipelineSourceType string

const (
	PipelineSourceFile PipelineSourceType = "files"
)

type PipelineSourceConf struct {
	Type    PipelineSourceType `yaml:"type"`
	Sources []string           `yaml:"sources"`
}

////////////////////////////////////////////////////////////////////////////////
// Logline Filters

type FilterType string

const (
	FilterTypeSimple FilterType = "simple"
	FilterTypeRegex             = "regex"
	FilterTypeCustom            = "custom"
)

type FilterConf struct {
	Type   FilterType `yaml:"type"`
	Filter string     `yaml:"filter"`
}

////////////////////////////////////////////////////////////////////////////////
// Processor Configuration

type ProcessorConf struct {
	// Metadata
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Pipleine config
	Inputs        []string      `yaml:"inputs"`        // A list of dependencies, i.e. input sources
	HasLogstream  bool          `yaml:"has_logstream"` // Whether or not it requires parsed loglines
	Filters       []*FilterConf `yaml:"filters"`       // Filters to apply to log strings
	Preprocessors []string      `yaml:"preprocessors"` // A list of preprocessor node names
	Parsers       []string      `yaml:"parsers"`       // A list of parsers to use
	Generator     string        `yaml:"generator"`     // The generator name for the processor. If empty, use name.
}

////////////////////////////////////////////////////////////////////////////////

func RunnerConfFromString(text string) (*RunnerConf, error) {
	spec := &RunnerConf{}

	err := yaml.Unmarshal([]byte(text), spec)

	if err != nil {
		return nil, err
	}

	return spec, nil
}

func RunnerConfFromFile(file string) (*RunnerConf, error) {
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

func isYamlList(text string) bool {
	pos := 0
	isComment := false

	for pos < len(text) {
		char := text[pos : pos+1]

		if isComment {
			if char == "\n" {
				isComment = false
			}
		} else if char == "#" {
			isComment = true
		} else if char == "-" {
			return true
		} else if len(strings.TrimSpace(char)) > 0 {
			return false
		}
		pos += 1
	}
	return false
}

func ProcessorConfsFromString(text string) ([]*ProcessorConf, error) {
	if isYamlList(text) {
		var confs []*ProcessorConf
		if err := yaml.Unmarshal([]byte(text), &confs); err != nil {
			return nil, err
		} else {
			return confs, nil
		}
	} else {
		conf := &ProcessorConf{}
		if err := yaml.Unmarshal([]byte(text), conf); err != nil {
			return nil, err
		} else {
			return []*ProcessorConf{conf}, nil
		}
	}
}

func ProcessorConfsFromFile(file string) ([]*ProcessorConf, error) {
	var err error

	if _, err = os.Stat(file); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading file %v: %v", file, err)
	}

	return ProcessorConfsFromString(string(data))
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

func (conf *ProcessorConf) GeneratorName() string {
	if len(conf.Generator) > 0 {
		return conf.Generator
	} else {
		return conf.Name
	}
}

// Validate the ProcessorConf fields EXCEPT for the Inputs - those are validated
// separately when the input processor conf is validated.
func (conf *ProcessorConf) validate(env *Environment) error {

	// Processor generator name
	genName := conf.GeneratorName()
	if len(strings.TrimSpace(genName)) == 0 {
		return errors.New("Invalid processor name: name cannot be empty")
	} else if _, ok := env.Processors[genName]; !ok {
		return errors.New("Unknown Processor: " + genName)
	}

	// Preprocessors are "dumb" and don't require a configuration. These are
	// designed to be chained together linearly, rather than as a tree.
	for _, list := range [][]string{conf.Preprocessors} {
		for _, depName := range list {
			if len(depName) == 0 {
				return errors.New("Invalid preprocessor name: name cannot be empty.")
			} else if _, ok := env.Processors[depName]; !ok {
				return errors.New("Unknown Processor: " + depName)
			}
		}
	}

	// Filters
	if len(conf.Filters) > 0 {
		for _, filterSpec := range conf.Filters {
			if len(filterSpec.Filter) == 0 {
				return errors.New("Filter must not be empty.")
			} else {
				switch filterSpec.Type {
				default:
					return errors.New("Invalid filter type: " + string(filterSpec.Type))
				case FilterTypeSimple:
					// OK
					break
				case FilterTypeRegex:
					// Looking for side effects.
					_ = makeRegexFilter(filterSpec.Filter)
					break
				case FilterTypeCustom:
					// Check if we have a function already
					if _, ok := env.Filters[filterSpec.Filter]; !ok {
						return errors.New("Unknown custom filter: " + filterSpec.Filter)
					}
				}
			}
		}
	}

	// Parsers
	if len(conf.Parsers) > 0 {
		for _, parser := range conf.Parsers {
			if len(parser) == 0 {
				return errors.New("Invalid tag: tag cannot be empty.")
			} else if _, ok := env.Parsers[parser]; !ok {
				return errors.New("Unknown parser: " + parser)
			}
		}
	}

	return nil
}

func (conf *ProcessorConf) buildFilterProc(env *Environment, source Processor) Processor {
	filters := make([]StringFilter, 0)

	if len(conf.Filters) > 0 {
		for _, filterSpec := range conf.Filters {
			if len(filterSpec.Filter) > 0 {
				switch filterSpec.Type {
				case FilterTypeSimple:
					{
						valid := []string{}
						for _, cond := range strings.Split(filterSpec.Filter, "&&") {
							if len(cond) > 0 {
								valid = append(valid, cond)
							}
						}
						if len(valid) > 0 {
							filters = append(filters, makeStringFilterFunc(valid))
						}
						break
					}
				case FilterTypeRegex:
					{
						filters = append(filters, makeRegexFilter(filterSpec.Filter))
						break
					}
				case FilterTypeCustom:
					if filter, ok := env.Filters[filterSpec.Filter]; ok {
						filters = append(filters, filter)
					}
				}
			}
		}
	}

	if len(filters) > 0 {
		return NewStringFilterProcessor(source, filters)
	} else {
		return nil
	}
}

func (conf *ProcessorConf) buildParserProc(env *Environment, source Processor) Processor {

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

// Build a logline input pipeline processor. This processor is a simple chain
// consting of the followin:
//		source (disk) -> string filters -> parser -> [preprocessor p1, p2, ..., pn]
func (conf *ProcessorConf) buildLoglineSource(env *Environment, source Processor,
	info PipelineSourceInfo) (Processor, error) {

	// Build the string filters, if any.
	if filter := conf.buildFilterProc(env, source); filter != nil {
		source = filter
	}

	// We'll have at least one parser for loglines
	source = conf.buildParserProc(env, source)

	// Add on any preprocessors
	for _, procName := range conf.Preprocessors {
		if procGen, ok := env.Processors[procName]; !ok {
			return nil, errors.New("Cannot find processor " + procName)
		} else {
			source = procGen.GenerateProcessor(&PipelineSourceInstance{
				Info:      info,
				Processor: source,
			})
		}
	}

	return source, nil
}

func (conf *RunnerConf) findProcessor(name string) *ProcessorConf {
	// TODO: Where are we storing the existing configurations?
	// This is something the environment should handle.
	// To bootstrap things, just look at what is embedded.

	for _, proc := range conf.Processors {
		if proc.Name == name {
			return proc
		}
	}
	return nil
}

func (p *ProcessorConf) Key() string {
	return p.Name
}

func (conf *RunnerConf) dependencyGraph(env *Environment) (*depgraph.DependencyGraph, error) {
	seen := make(map[string]bool)

	root := conf.findProcessor(conf.SinkName)
	if root == nil {
		return nil, errors.New("Cannot find sink processor '" + conf.SinkName + "'.")
	}

	graph := depgraph.New(make([]depgraph.Keyer, 0))

	// Add root
	toProcess := []*ProcessorConf{root}
	seen[root.Key()] = true
	graph.AddNode(root)

	// First pass, just all get the nodes
	for len(toProcess) > 0 {
		// Pop a node from the stack
		n := toProcess[len(toProcess)-1]
		toProcess = toProcess[0 : len(toProcess)-1]

		// Add it to the graph if needed
		if _, ok := graph.NodeMap[n.Key()]; !ok {
			graph.AddNode(n)
		}

		// Handle its sources
		for _, dep := range n.Inputs {
			if !seen[dep] {
				proc := conf.findProcessor(dep)
				if proc == nil {
					return nil, fmt.Errorf("Cannot find input processor '%v' for processor '%v'", dep, n.Key())
				}
				seen[dep] = true
				toProcess = append(toProcess, proc)

				// Add it to the graph if needed
				if _, ok := graph.NodeMap[dep]; !ok {
					graph.AddNode(proc)
				}
			}

			// Add the edge
			if err := graph.AddDependency(&depgraph.Dependency{
				Dependent:   n.Key(),
				DependentOn: dep,
			}); err != nil {
				return nil, err
			}
		}
	}

	return graph, nil
}

func validateProcessorConfs(graph *depgraph.DependencyGraph, env *Environment) error {
	for key, node := range graph.NodeMap {
		pconf := node.Value.(*ProcessorConf)
		if err := pconf.validate(env); err != nil {
			return fmt.Errorf("Error in processor %v: %v", key, err)
		}
	}
	return nil
}

func (conf *RunnerConf) ToRunner(env *Environment) (*Runner, error) {

	// This validates that we have all of processor confs for input sources.
	// We still need to validate each one.
	graph, err := conf.dependencyGraph(env)
	if err != nil {
		return nil, err
	}

	// Check for cycles
	if _, err = graph.TopSort(); err != nil {
		return nil, errors.New("Cycle detected in the pipeline dependency graph!")
	}

	// Now, validate each processor conf
	if err = validateProcessorConfs(graph, env); err != nil {
		return nil, err
	}

	// Sources
	gen, err := conf.SourceConf.ToPipelineSourceGenerator()
	if err != nil {
		return nil, err
	}

	proc := NewRunnerConfProcssor(conf, env, graph)

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

	// At this point the runner *should* work, though we haven't built an
	// actual pipeline. But, we've validated that we can find each processor and
	// its configuration and that there are no cycles, so we're in OK
	// shape.
	return NewRunner(gen, collector, proc), nil
}

////////////////////////////////////////////////////////////////////////////////
// DataCollector built from PipelineRunnerConf

type RunnerConfProcessor struct {
	Conf     *RunnerConf
	Env      *Environment
	DepGraph *depgraph.DependencyGraph
}

func NewRunnerConfProcssor(conf *RunnerConf, env *Environment,
	graph *depgraph.DependencyGraph) *RunnerConfProcessor {
	return &RunnerConfProcessor{
		Conf:     conf,
		Env:      env,
		DepGraph: graph,
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

type plBuilderState struct {
	procMap    map[string]Processor
	sourceInst *PipelineSourceInstance
	env        *Environment
	graph      *depgraph.DependencyGraph
}

// Stich multiple (input) processors into a single processor
func stitchInputs(processors []Processor) Processor {
	if len(processors) == 0 {
		return nil
	} else if len(processors) == 1 {
		return processors[0]
	} else {
		// Combine the inputs using TimeWeaverProcessors. They get combined
		// like this: (((0 1) 2) 3).
		proc := processors[0]
		for i := 1; i < len(processors); i++ {
			proc = NewTimeweaverProcessor(proc, processors[i])
		}
		return proc
	}
}

// Build the processor specified by the conf, building any other processors
// needed.
func (conf *ProcessorConf) buildProcessor(state *plBuilderState) (Processor, error) {

	// Is it cached?
	if proc, ok := state.procMap[conf.Key()]; ok {
		return proc, nil
	}

	// Processors may get input from loglines and/or other processors.  When we
	// have more than one processor, we need to take a slightly complicated
	// approach. Processors may emit data at any rate, and may need to emit
	// data that goes backwards compared to the timestamps of their input
	// stream. Our approach is to create multiple input streams at the file
	// level, and merge them back together as one, ordered stream.
	//
	// So, we need to do the following:
	//  1) Build the main pipeline filter/parser
	//	2) Build a pipeline for each preprocessor stream. We do this by
	//	   recursively calling buildProcessor for each input.
	//	3) Merge all of these streams together
	//	4) Build and pass the data to the main processor.
	//  5) If this processor is passed to multiple processors, put a Muxer
	//     behind it to multiplex our output stream.

	// For files, we can just invoke Process() multiple times.  We can't
	// use a muxer because that sends the same object, which would be fine
	// if we had unlimited memory! But we dont, so we leave it on disk
	// until we can process it. TODO: Can we do better?

	inputs := make([]Processor, 0)

	// (1) Build the logline input processing chain for _this_ processor.
	// This will get stitches with other input (if needed) later.
	if conf.HasLogstream {
		if logPipeline, err := conf.buildLoglineSource(state.env, state.sourceInst.Processor,
			state.sourceInst.Info); err != nil {
			return nil, err
		} else {
			inputs = append(inputs, logPipeline)
		}
	}

	// (2) Get each other processor we depend on
	for _, depName := range conf.Inputs {
		if node, ok := state.graph.NodeMap[depName]; !ok {
			return nil, fmt.Errorf("Cannot find processor conf for '%v'", depName)
		} else if otherProc, err := node.Value.(*ProcessorConf).buildProcessor(state); err != nil {
			return nil, err
		} else {
			inputs = append(inputs, otherProc)
		}
	}

	// (3) Combine the log pipeline (if we have one) with any other inputs.
	input := stitchInputs(inputs)
	if input == nil {
		return nil, fmt.Errorf("No inputs and no log processor for '%v'", conf.Name)
	}

	// (4) Finally, make an instance of our processor with the newly stitched inputs.
	genName := conf.GeneratorName()

	procGen, ok := state.env.Processors[genName]
	if !ok {
		return nil, errors.New("Cannot find processor " + genName)
	}

	proc := procGen.GenerateProcessor(&PipelineSourceInstance{
		Info:      state.sourceInst.Info,
		Processor: input,
	})

	// (5) One last thing: we might need to multiplex our output. If we have
	// more than one in edge in the dependency graph, then our output goes to
	// more than one processor, so, we'll wrap it in muxer.
	if node, ok := state.graph.NodeMap[conf.Key()]; !ok {
		return nil, fmt.Errorf("Cannot find processor conf for '%v'", conf.Key())
	} else if len(node.EdgesIn) > 1 {
		// yep, we need to put a multiplexer in front of the output
		proc = NewMuxer(proc, len(node.EdgesIn))
	}

	// Lastly, cache it
	state.procMap[conf.Key()] = proc

	return proc, nil
}

func (proc *RunnerConfProcessor) BuildPipeline(sourceInst *PipelineSourceInstance) (*Pipeline, error) {
	// First, get the sink processor conf. We'll build the actual pipeline graph
	// from there.
	sinkProc := proc.Conf.findProcessor(proc.Conf.SinkName)
	if sinkProc == nil {
		return nil, fmt.Errorf("Cannot find sink processor '%v'", proc.Conf.SinkName)
	}

	// Heavy lifting is done by buildProcessor; we just provide the context.
	source, err := sinkProc.buildProcessor(&plBuilderState{
		procMap:    make(map[string]Processor),
		sourceInst: sourceInst,
		env:        proc.Env,
		graph:      proc.DepGraph,
	})

	if err != nil {
		return nil, err
	}

	return &Pipeline{
		LastHop: source,
	}, nil
}

func (proc *RunnerConfProcessor) OnData(data interface{}) {}
func (proc *RunnerConfProcessor) Finish()                 {}
