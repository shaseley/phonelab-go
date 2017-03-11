package phonelab

const (
	DEFAULT_MAX_CONCURRENCY = 0
)

// Key-value pair Information pertaining to the source of pipeline data.
type PipelineSourceInfo map[string]interface{}

type PipelineSourceInstance struct {
	Processor Processor
	Info      PipelineSourceInfo
}

type PipelineSourceGenerator interface {
	Process() <-chan *PipelineSourceInstance
}

// Conceptually, a pipeline is network graph with a single source and single
// sink. They aren't necessarily a flat list of processing nodes; the input
// may fork into multiple streams that are recombined by timestamps or in some
// other way. Since multiple paths can always be demuxed into a single
// processing node, having a single sink is not limiting. The same holds for
// input and muxing.
//
// Our representation of a pipeline is a sink-centric one. The pipeline is
// described by the last-hop Processor, of which we'll invoke its Process()
// method to kick off processing.
//
// The last hop can either be the sink, or the DataCollector can play that
// role. By letting the DataCollector act as the sink, you can get more
// reusability out of processors, theoretically anyways.
type Pipeline struct {
	LastHop Processor
}

// Build a Pipeline configured to get its input from source.
type PipelineBuilder interface {
	// Instantiate the pipline using the given source info.
	BuildPipeline(source *PipelineSourceInstance) *Pipeline
}

// DataCollectors generally don't know anything about the source of their data,
// but can assume that they are processing at the right granularity. For
// example, the same processor logic should work if data is read from a single,
// local logcat file, gzipped log files, boot_id processor, or streamed over a
// network.
type DataCollector interface {
	OnData(interface{})
	Finish()
}

// A Runner manages running the processors. Its job is to facilitate building
// pipelines once for each source, kicking off the processing, and passing
// results to the DataCollector.
type Runner struct {
	Source         PipelineSourceGenerator
	Collector      DataCollector
	Builder        PipelineBuilder
	MaxConcurrency int
}

func NewRunner(gen PipelineSourceGenerator, dc DataCollector, plb PipelineBuilder) *Runner {
	return &Runner{
		Source:         gen,
		Collector:      dc,
		Builder:        plb,
		MaxConcurrency: DEFAULT_MAX_CONCURRENCY,
	}
}

func (r *Runner) runOne(source *PipelineSourceInstance, done chan int) {
	// Build it
	pipeline := r.Builder.BuildPipeline(source)

	// Start the processing
	resChan := pipeline.LastHop.Process()

	// Drain the results and forward them to the DataCollector.
	for res := range resChan {
		r.Collector.OnData(res)
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

	runner.Collector.Finish()
}
