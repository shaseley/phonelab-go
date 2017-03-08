package phonelab

// The environment maintains state about what we know how to create/run
type ParserGen func() Parser

type ProcessorGen interface {
	GenerateProcessor(info *PipelineSourceInstance) Processor
}

type Environment struct {
	// Parsers we know about
	Parsers    map[string]ParserGen
	Processors map[string]ProcessorGen
	Filters    map[string]StringFilter
}

func NewEnvironment() *Environment {
	env := &Environment{
		Parsers:    make(map[string]ParserGen),
		Processors: make(map[string]ProcessorGen),
		Filters:    make(map[string]StringFilter),
	}

	env.RegisterKnownParsers()

	return env
}

// Add a generator for all of the parsers we know about.
func (env *Environment) RegisterKnownParsers() {
	env.RegisterParserGenerator(TAG_PRINTK,
		func() Parser {
			pk := NewPrintkParser()
			pk.ErrOnUnknownTag = false
			return pk
		})

	env.RegisterParserGenerator(TAG_TRACE,
		func() Parser {
			tparser := NewKernelTraceParser()
			tparser.ErrOnUnknownTag = false
			return tparser
		})

	env.RegisterParserGenerator(TAG_PL_POWER_BATTERY,
		NewPLPowerBatteryParser)

	env.RegisterParserGenerator(TAG_QOE_LIFECYCLE,
		NewQoEActivityLifecycleParser)
}

// Add a parser generator for a given log tag.
// By default, all known parsers are registered when the environment is created.
// Client code can register any custom parsers
func (env *Environment) RegisterParserGenerator(tag string, gen ParserGen) {
	env.Parsers[tag] = gen
}