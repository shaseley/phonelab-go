package phonelab

// TimeweaverProcessor weaves together two streams of data based on timestamp.
// The logs coming down the source channels must implement the
// MonotonicTimestamper interface, otherwise there will be a panic on a bad
// type assertion.
type TimeweaverProcessor struct {
	lhs Processor
	rhs Processor
}

type MonotonicTimestamper interface {
	MonotonicTimestamp() float64
}

func NewTimeweaverProcessor(lhs, rhs Processor) *TimeweaverProcessor {
	return &TimeweaverProcessor{
		lhs: lhs,
		rhs: rhs,
	}
}

// State for tracking timeweaver sources
type timeweaverState struct {
	source <-chan interface{}
	get    bool
	ok     bool
	obj    interface{}
}

func newTimeweaverState(source Processor) *timeweaverState {
	return &timeweaverState{
		source: source.Process(),
		get:    true,
		ok:     true,
		obj:    nil,
	}
}

func (state *timeweaverState) updateIfneeded() {
	if state.get {
		state.obj, state.ok = <-state.source
		state.get = false
	}
}

func (state *timeweaverState) drain(outChan chan interface{}) {
	if state.obj != nil {
		outChan <- state.obj
	}
	for log := range state.source {
		outChan <- log
	}
}

func (state *timeweaverState) eof() bool {
	return !state.ok
}

func (state *timeweaverState) timestamp() float64 {
	return (state.obj.(MonotonicTimestamper)).MonotonicTimestamp()
}

func (state *timeweaverState) send(outChan chan interface{}) {
	outChan <- state.obj
	state.get = true
}

func (tw *TimeweaverProcessor) Process() <-chan interface{} {

	lhs := newTimeweaverState(tw.lhs)
	rhs := newTimeweaverState(tw.rhs)

	outChan := make(chan interface{})

	// Process
	go func() {
		for {
			lhs.updateIfneeded()
			rhs.updateIfneeded()

			if lhs.eof() {
				rhs.drain(outChan)
				break
			} else if rhs.eof() {
				lhs.drain(outChan)
				break
			} else {
				if lhs.timestamp() <= rhs.timestamp() {
					lhs.send(outChan)
				} else {
					rhs.send(outChan)
				}
			}
		}

		close(outChan)
	}()

	return outChan
}
