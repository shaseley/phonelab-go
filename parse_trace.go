package phonelab

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// Top-level Kernel-Trace tag parser

// Everything that the KernelTraceParser parses implements this interface
type TraceInterface interface {
	Tag() string
	GetTrace() *Trace
	SetTrace(trace *Trace)
}

var TRACE_PATTERN = regexp.MustCompile(`` +
	`\s*(?P<thread>.*?)` +
	`\s+\[(?P<cpu>\d+)\]` +
	`\s+(?P<unknown>.{4})` +
	`\s+(?P<timestamp>\d+\.\d+)` +
	`: ` +
	`(?P<message>(?P<tag>.*?):` +
	`\s+(?P<text>.*)` +
	`)`)

type Trace struct {
	Thread    string    `logcat:"thread"`
	Cpu       int       `logcat:"cpu"`
	Unknown   string    `logcat:"unknown"`
	Timestamp float64   `logcat:"timestamp"`
	Tag       string    `logcat:"tag"`
	Datetime  time.Time `logcat:"-"`
	Logline   *Logline  `logcat:"-"`
}

type KernelTraceParser struct {
	RegexParser *RegexParser
	Subparsers  map[string]Parser

	// Parameters
	ErrOnUnknownTag bool
}

func NewKernelTraceParser() *KernelTraceParser {
	parser := &KernelTraceParser{ErrOnUnknownTag: true}
	parser.RegexParser = NewRegexParser(parser)

	// TODO: This should be ad hoc
	parser.Subparsers = map[string]Parser{
		"sched_cpu_hotplug":                   NewRegexParser(&SchedCPUHotplugParser{}),
		"phonelab_num_online_cpus":            NewRegexParser(&NumOnlineCpusParser{}),
		"thermal_temp":                        NewRegexParser(&ThermalTempParser{}),
		"cpu_frequency":                       NewRegexParser(&CpuFrequencyParser{}),
		"phonelab_proc_foreground":            NewRegexParser(&ProcForegroundParser{}),
		"phonelab_periodic_ctx_switch_info":   NewRegexParser(&PeriodicCtxSwitchInfoParser{}),
		"phonelab_periodic_ctx_switch_marker": NewRegexParser(&PeriodicCtxSwitchMarkerParser{}),
	}

	return parser
}

func (p *KernelTraceParser) New() interface{} {
	return &Trace{}
}

func (p *KernelTraceParser) Regex() *regexp.Regexp {
	return TRACE_PATTERN
}

func (p *KernelTraceParser) Parse(line string) (interface{}, error) {

	var trace *Trace

	// Parse trace using regex parser
	if obj, err := p.RegexParser.Parse(line); err != nil {
		return nil, err
	} else {
		trace = obj.(*Trace)
	}

	// TODO: Fix
	// This one doesn't come from the payload
	// trace.Datetime = logline.Datetime

	// Uncomment this line if you want to add Logline information
	// Or add it manually where required
	//trace.Logline = logline

	// Parse the payload
	parser, ok := p.Subparsers[trace.Tag]
	if !ok {
		if p.ErrOnUnknownTag {
			return nil, fmt.Errorf("No parser defined for trace tag '%v'", trace.Tag)
		} else {
			return trace, nil
		}
	}

	if obj, err := parser.Parse(p.RegexParser.LastMap["text"]); err != nil {
		return nil, err
	} else {
		ti := obj.(TraceInterface)
		ti.SetTrace(trace)
		return ti, err
	}
}

var TraceParser = NewKernelTraceParser()

///////////////////////////////////////////////////////////////////////////////
// Sched CPU Hotplug

/* Format: cpu 1 offline error=0 */
var SCHED_CPU_HOTPLUG_PATTERN = regexp.MustCompile(`` +
	`\s*cpu` +
	`\s+(?P<cpu>\d+)` +
	`\s+(?P<state>[a-zA-Z0-9_]+)` +
	`\s+error=(?P<error>-?\d+)`)

type SchedCpuHotplug struct {
	Trace *Trace `logcat:"-"`
	Cpu   int    `logcat:"cpu"`
	State string `logcat:"state"`
	Error int    `logcat:"error"`
}

func (t *SchedCpuHotplug) Tag() string {
	return t.Trace.Tag
}

func (t *SchedCpuHotplug) GetTrace() *Trace {
	return t.Trace
}

func (t *SchedCpuHotplug) SetTrace(trace *Trace) {
	t.Trace = trace
}

type SchedCPUHotplugParser struct {
}

func (s *SchedCPUHotplugParser) New() interface{} {
	return &SchedCpuHotplug{}
}

func (s *SchedCPUHotplugParser) Regex() *regexp.Regexp {
	return SCHED_CPU_HOTPLUG_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// Thermal Temp

/* Format: sensor_id=5 temp=59 */
var THERMAL_TEMP_PATTERN = regexp.MustCompile(`` +
	`\s*sensor_id=(?P<sensor_id>\d+)` +
	`\s+temp=(?P<temp>\d+).*`)

type ThermalTemp struct {
	Trace    *Trace `logcat:"-"`
	SensorId int    `logcat:"sensor_id"`
	Temp     int    `logcat:"temp"`
}

func (t *ThermalTemp) Tag() string {
	return t.Trace.Tag
}

func (t *ThermalTemp) GetTrace() *Trace {
	return t.Trace
}

func (t *ThermalTemp) SetTrace(trace *Trace) {
	t.Trace = trace
}

type ThermalTempParser struct {
}

func (s *ThermalTempParser) New() interface{} {
	return &ThermalTemp{}
}

func (s *ThermalTempParser) Regex() *regexp.Regexp {
	return THERMAL_TEMP_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// CPU Frequency

/* Format: cpu_frequency: state=2265600 cpu_id=0 */
var CPU_FREQUENCY_PATTERN = regexp.MustCompile(`` +
	`\s*state=(?P<state>\d+)` +
	`\s+cpu_id=(?P<cpu_id>\d+)`)

type CpuFrequency struct {
	Trace *Trace `logcat:"-"`
	State int    `logcat:"state"`
	CpuId int    `logcat:"cpu_id"`
}

func (cf *CpuFrequency) Tag() string {
	return cf.Trace.Tag
}

func (t *CpuFrequency) GetTrace() *Trace {
	return t.Trace
}

func (t *CpuFrequency) SetTrace(trace *Trace) {
	t.Trace = trace
}

type CpuFrequencyParser struct {
}

func (s *CpuFrequencyParser) New() interface{} {
	return &CpuFrequency{}
}

func (s *CpuFrequencyParser) Regex() *regexp.Regexp {
	return CPU_FREQUENCY_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// # Online CPUs

/* Format: phonelab_num_online_cpus: num_online_cpus=4 */
var PHONELAB_NUM_ONLINE_CPUS_PATTERN = regexp.MustCompile(`` +
	`\s*num_online_cpus=(?P<num_online_cpus>\d+)`)

type PhonelabNumOnlineCpus struct {
	Trace         *Trace `logcat:"-"`
	NumOnlineCpus int    `logcat:"num_online_cpus"`
}

func (ti *PhonelabNumOnlineCpus) Tag() string {
	return ti.Trace.Tag
}

func (t *PhonelabNumOnlineCpus) GetTrace() *Trace {
	return t.Trace
}

func (t *PhonelabNumOnlineCpus) SetTrace(trace *Trace) {
	t.Trace = trace
}

type NumOnlineCpusParser struct {
}

func (s *NumOnlineCpusParser) New() interface{} {
	return &PhonelabNumOnlineCpus{}
}

func (s *NumOnlineCpusParser) Regex() *regexp.Regexp {
	return PHONELAB_NUM_ONLINE_CPUS_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// Proc Foreground

/* Format: phonelab_proc_foreground: pid=13759 tgid=13759 comm=.android.dialer */
var PHONELAB_PROC_FOREGROUND_PATTERN = regexp.MustCompile(`` +
	`\s*pid=(?P<pid>\d+) tgid=(?P<tgid>\d+) comm=(?P<comm>\S+)`)

type PhonelabProcForeground struct {
	Trace *Trace `logcat:"-"`
	Pid   int    `logcat:"pid"`
	Tgid  int    `logcat:"tgid"`
	Comm  string `logcat:"comm"`
}

func (ti *PhonelabProcForeground) Tag() string {
	return ti.Trace.Tag
}

func (t *PhonelabProcForeground) GetTrace() *Trace {
	return t.Trace
}

func (t *PhonelabProcForeground) SetTrace(trace *Trace) {
	t.Trace = trace
}

type ProcForegroundParser struct {
}

func (s *ProcForegroundParser) New() interface{} {
	return &PhonelabProcForeground{}
}

func (s *ProcForegroundParser) Regex() *regexp.Regexp {
	return PHONELAB_PROC_FOREGROUND_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// Periodic context switch info

/* Format: phonelab_periodic_ctx_switch_info: cpu=0 pid=3 tgid=3 nice=0 comm=ksoftirqd/0 utime=0 stime=0 rtime=1009429 bg_utime=0 bg_stime=0 bg_rtime=0 s_run=0 s_int=17 s_unint=0 s_oth=0 log_idx=933300 rx=0 tx=0 */
var PHONELAB_PERIODIC_CTX_SWITCH_INFO_PATTERN = regexp.MustCompile(`` +
	`\s*cpu=(?P<cpu>\d+)` +
	`\s*pid=(?P<pid>\d+)` +
	`\s*tgid=(?P<tgid>\d+)` +
	`\s*nice=(?P<nice>-?\d+)` +
	`\s*comm=(?P<comm>.*?)` +
	`\s*utime=(?P<utime>\d+)` +
	`\s*stime=(?P<stime>\d+)` +
	`\s*rtime=(?P<rtime>\d+)` +
	`\s*bg_utime=(?P<bg_utime>\d+)` +
	`\s*bg_stime=(?P<bg_stime>\d+)` +
	`\s*bg_rtime=(?P<bg_rtime>\d+)` +
	`\s*s_run=(?P<s_run>\d+)` +
	`\s*s_int=(?P<s_int>\d+)` +
	`\s*s_unint=(?P<s_unint>\d+)` +
	`\s*s_oth=(?P<s_oth>\d+)` +
	`\s*log_idx=(?P<log_idx>\d+)` +
	`(` +
	`\s*rx=(?P<rx>\d+)` +
	`\s*tx=(?P<tx>\d+)` +
	`)?`)

type PhonelabPeriodicCtxSwitchInfo struct {
	Trace   *Trace `logcat:"-"`
	Cpu     int    `logcat:"cpu"`
	Pid     int    `logcat:"pid"`
	Tgid    int    `logcat:"tgid"`
	Nice    int    `logcat:"nice"`
	Comm    string `logcat:"comm"`
	Utime   int64  `logcat:"utime"`
	Stime   int64  `logcat:"stime"`
	Rtime   int64  `logcat:"rtime"`
	BgUtime int64  `logcat:"bg_utime"`
	BgStime int64  `logcat:"bg_stime"`
	BgRtime int64  `logcat:"bg_rtime"`
	SRun    int64  `logcat:"s_run"`
	SInt    int64  `logcat:"s_int"`
	SUnint  int64  `logcat:"s_unint"`
	SOth    int64  `logcat:"s_oth"`
	LogIdx  int64  `logcat:"log_idx"`
	Rx      int64  `logcat:"rx"`
	Tx      int64  `logcat:"tx"`
}

func (ti *PhonelabPeriodicCtxSwitchInfo) Tag() string {
	return ti.Trace.Tag
}

func (t *PhonelabPeriodicCtxSwitchInfo) GetTrace() *Trace {
	return t.Trace
}

func (t *PhonelabPeriodicCtxSwitchInfo) SetTrace(trace *Trace) {
	t.Trace = trace
}

type PeriodicCtxSwitchInfoParser struct {
}

func (s *PeriodicCtxSwitchInfoParser) New() interface{} {
	return &PhonelabPeriodicCtxSwitchInfo{}
}

func (s *PeriodicCtxSwitchInfoParser) Regex() *regexp.Regexp {
	return PHONELAB_PERIODIC_CTX_SWITCH_INFO_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// Periodic context switch marker

var PHONELAB_PERIODIC_CTX_SWITCH_MARKER_PATTERN = regexp.MustCompile(`` +
	`\s*(?P<state>BEGIN|END)` +
	`\s+cpu=(?P<cpu>\d+)` +
	`\s+count=(?P<count>\d+)` +
	`\s+log_idx=(?P<log_idx>\d+)`)

type PPCSMState string

const (
	PPCSMBegin PPCSMState = "BEGIN"
	PPCSMEnd   PPCSMState = "END"
)

type PhonelabPeriodicCtxSwitchMarker struct {
	Trace  *Trace     `logcat:"-"`
	State  PPCSMState `logcat:"state"`
	Cpu    int        `logcat:"cpu"`
	Count  int        `logcat:"count"`
	LogIdx int64      `logcat:"log_idx"`
}

func (ti *PhonelabPeriodicCtxSwitchMarker) Tag() string {
	return ti.Trace.Tag
}

func (t *PhonelabPeriodicCtxSwitchMarker) GetTrace() *Trace {
	return t.Trace
}

func (t *PhonelabPeriodicCtxSwitchMarker) SetTrace(trace *Trace) {
	t.Trace = trace
}

type PeriodicCtxSwitchMarkerParser struct {
}

func (s *PeriodicCtxSwitchMarkerParser) New() interface{} {
	return &PhonelabPeriodicCtxSwitchMarker{}
}

func (s *PeriodicCtxSwitchMarkerParser) Regex() *regexp.Regexp {
	return PHONELAB_PERIODIC_CTX_SWITCH_MARKER_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// Legacy

func ParseTraceFromLoglinePayload(logline *Logline) TraceInterface {
	if logline == nil {
		return nil
	}

	obj, err := TraceParser.Parse(logline.Payload)
	if err != nil {
		return nil
	}

	trace := obj.(TraceInterface)

	// This one doesn't come from the payload
	trace.GetTrace().Datetime = logline.Datetime

	// Uncomment this line if you want to add Logline information
	// Or add it manually where required
	//trace.Logline = logline

	return trace
}

///////////////////////////////////////////////////////////////////////////////

type PeriodicCtxSwitchInfo struct {
	Start *PhonelabPeriodicCtxSwitchMarker
	Info  []*PhonelabPeriodicCtxSwitchInfo
	End   *PhonelabPeriodicCtxSwitchMarker
}

func (pcsi *PeriodicCtxSwitchInfo) TotalTime() int64 {
	total_time := int64(0)
	for _, info := range pcsi.Info {
		total_time += info.Rtime
	}
	return total_time
}

func (pcsi *PeriodicCtxSwitchInfo) Busyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, info := range pcsi.Info {
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.Rtime
		}
	}
	return float64(busy_time) / float64(total_time)
}

func (pcsi *PeriodicCtxSwitchInfo) FgBusyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, info := range pcsi.Info {
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.Rtime
			busy_time -= info.BgRtime
		}
	}
	return float64(busy_time) / float64(total_time)
}

func (pcsi *PeriodicCtxSwitchInfo) BgBusyness() float64 {
	total_time := pcsi.TotalTime()
	busy_time := int64(0)

	if total_time == 0 {
		return 0.0
	}

	for _, info := range pcsi.Info {
		if !strings.Contains(info.Comm, "swapper") {
			busy_time += info.BgRtime
		}
	}
	return float64(busy_time) / float64(total_time)
}
