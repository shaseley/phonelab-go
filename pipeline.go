package phonelab

const (
	DEFAULT_MAX_CONCURRENCY = 0
)

type PipelineSourceGenerator interface {
	Process() <-chan Processor
}

// Conceptually, a pipeline is network graph with a single source and multiple
// sinks. The sinks represent processing paths; for example, if a DataProcessor
// computes some data and statistics, there may be two sinks. DataProcessors
// that only focus on one task will probably have only one sink, which we
// imagine is the common case.
type PipelineSink interface {
	GetSource() Processor
	OnData(interface{})
	OnFinish()
}

// Our representation of a pipeline is a sink-centric one. Each sink has a
// source (Processor) that represents its parent in the graph and generates
// data. This processor will have its own parent processor, all the way up
// to the root.
type Pipeline []PipelineSink

type PipelineBuilder interface {
	// Return the first node in the pipeline - closest to the source - and
	// a list of sinks.
	BuildPipeline(source Processor) Pipeline
}

// DataProcessors don't know anything about the source of their data, but can
// assume that they are processing at the right granularity. For example, the
// same processor logic should work if data is read from a single, local
// logcat file, gzipped log files, boot_id processor, or streamed over a
// network.
//
// The DataProcessor will be connected to a source through a broker.
type DataProcessor interface {
	PipelineBuilder
	Finish()
}

type Runner struct {
	Source         PipelineSourceGenerator
	DataProcessor  DataProcessor
	MaxConcurrency int
}

func NewRunner(gen PipelineSourceGenerator, dp DataProcessor) *Runner {
	return &Runner{
		Source:         gen,
		DataProcessor:  dp,
		MaxConcurrency: DEFAULT_MAX_CONCURRENCY,
	}
}

func (r *Runner) runOne(source Processor, done chan int) {
	sinks := r.DataProcessor.BuildPipeline(source)
	doneSink := make(chan int)

	for _, s := range sinks {
		go func(sink PipelineSink) {
			proc := sink.GetSource()
			resChan := proc.Process()
			for res := range resChan {
				sink.OnData(res)
			}
			sink.OnFinish()
			doneSink <- 1
		}(s)
	}

	for i := 0; i < len(sinks); i++ {
		<-doneSink
	}

	done <- 1
}

// Synchronsously run the processor for all data sources.
func (runner *Runner) Run() {
	running := 0
	sourceChan := runner.Source.Process()
	done := make(chan int)

	for source := range sourceChan {
		// Do we have a spot?
		if runner.MaxConcurrency > 0 && running == runner.MaxConcurrency {
			// No. Wait for something to finish.
			<-done
			running -= 1
		}
		running += 1
		go runner.runOne(source, done)
	}

	for running > 0 {
		<-done
		running -= 1
	}

	runner.DataProcessor.Finish()
}
