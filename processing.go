package phonelab

// package processing contains interfaces and functionality for building
// log-processing pipelines. The basic interface is Processor, which abstracts
// any processing component that can generate data. Concrete implementations
// are provided for a muxer, demuxer, simple I/O processor, and multiple
// log sources.

import (
	"fmt"
	"log"
	"sync"
)

// The basic interface that all PhoneLab pipeline operators implement. When the
// Process() function is invoked, the Processor should create and return a
// channel that logs will be transmitted on.
type Processor interface {
	Process() <-chan interface{}
}

// LogHandlers accept callbacks when a new log arrives and is ready to be
// processed.
type LogHandler interface {
	Handle(log interface{}) interface{}
}

// SimpleProcessor is a Processor with a single source and a single output
// channel. Furthermore, for each log on the in-channel, the log is passed to
// the processor's LogHandler, which performs some operation on the the log.
// If the LogHandler returns a non-nil value, this value will be written on the
// output channel.
type SimpleProcessor struct {
	Handler LogHandler
	Source  Processor
}

// Create a new SimpleOperator with a single source and handler
func NewSimpleProcessor(source Processor, handler LogHandler) *SimpleProcessor {
	return &SimpleProcessor{
		Handler: handler,
		Source:  source,
	}
}

func (proc *SimpleProcessor) Process() <-chan interface{} {
	outChan := make(chan interface{})

	if proc.Handler == nil {
		panic("SimpleProcessor handler cannot be nil!")
	}
	if proc.Source == nil {
		panic("SimpleProcessor source cannot be nil!")
	}

	go func() {
		inChan := proc.Source.Process()
		for log := range inChan {
			if res := proc.Handler.Handle(log); res != nil {
				outChan <- res
			}
		}
		close(outChan)
	}()

	return outChan
}

// Muxer multiplexes log lines/objects onto multiple output channels from a
// single source.
type Muxer struct {
	Source  Processor
	dest    []chan interface{}
	numDest int
	l       sync.Mutex
}

func NewMuxer(source Processor, numDest int) *Muxer {
	return &Muxer{
		Source:  source,
		dest:    make([]chan interface{}, 0),
		numDest: numDest,
	}
}

func (m *Muxer) Process() <-chan interface{} {
	// This is going to be invoked multiple times, once for each output
	// processor, but we need to give each one their own channel. And, we want
	// to wait until all the channels have been created to start processing.
	m.l.Lock()
	defer m.l.Unlock()

	outChan := make(chan interface{})
	m.dest = append(m.dest, outChan)

	if len(m.dest) > m.numDest {
		panic("Muxer: More invocations than destinations")
	} else if len(m.dest) < m.numDest {
		// Not there yet
		return outChan
	}

	// Good to go.
	go func() {
		inChan := m.Source.Process()

		for log := range inChan {

			// Multiplex current message. For now, blocking non-concurrent sends.
			for _, c := range m.dest {
				c <- log
			}
		}

		for _, c := range m.dest {
			close(c)
		}
	}()

	return outChan
}

// Demuxer takes input from multiple sources and funnels it down a single
// output channel.
type Demuxer struct {
	Sources []Processor
}

func NewDemuxer(sources []Processor) *Demuxer {
	return &Demuxer{
		Sources: sources,
	}
}

func (dm *Demuxer) Process() <-chan interface{} {
	outChan := make(chan interface{})
	done := make(chan int)

	var runOne = func(p Processor) {
		res := p.Process()
		for log := range res {
			outChan <- log
		}
		done <- 1
	}

	// Process
	go func() {
		for _, proc := range dm.Sources {
			go runOne(proc)
		}
		for i := 0; i < len(dm.Sources); i++ {
			<-done
		}
		close(outChan)
	}()

	return outChan
}

type StringFilter func(string) bool

type StringFilterHandler struct {
	Filters []StringFilter
}

func (p *StringFilterHandler) Handle(log interface{}) interface{} {
	switch t := log.(type) {
	case string:
		for _, filter := range p.Filters {
			if filter(t) {
				return log
			}
		}
	default:
		panic(fmt.Sprintf("String filter got non-string object: %T", log))
	}

	// Didn't pass the filters
	return nil
}

func NewStringFilterProcessor(source Processor, filters []StringFilter) Processor {
	return NewSimpleProcessor(source, &StringFilterHandler{filters})
}

// Log
type LoglineProcessorHandler struct {
	Parser *LoglineParser
}

func (p *LoglineProcessorHandler) Handle(logline interface{}) interface{} {
	line := logline.(string)
	ll, err := p.Parser.Parse(line)
	if err != nil {
		log.Printf("Error parsing line: %v\n", err)
	}
	return ll
}

func NewLoglineProcessor(source Processor, parser *LoglineParser) Processor {
	return NewSimpleProcessor(source, &LoglineProcessorHandler{parser})
}
