package phonelab

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pbnjay/strptime"
)

type MsmThermalState int

const (
	MSM_THERMAL_STATE_OFFLINE MsmThermalState = iota
	MSM_THERMAL_STATE_ONLINE
)

type PowerManagementState int

const (
	PM_SUSPEND_ENTRY PowerManagementState = iota
	PM_SUSPEND_EXIT
)

///////////////////////////////////////////////////////////////////////////////
// Printk common parser

// Printk pattern example: <6>[   21.512807] msm_thermal: Allow Online CPU3 Temp: 66
// Unfortunately, there's no set pattern in the payload for identifying the
// subparser. Some start with <tag>:, but not every message. We'll use a prefix
// approach for looking up these messages.
var PRINTK_PATTERN_STRING = `^` +
	`\s*<(?P<loglevel>\d+)>?` +
	`\s*\[\s*(?P<timestamp>\d+\.\d+)\]` +
	`\s*(?P<payload>.*)$`

// 12,1547,5703232,-;healthd: battery [l=19 v=4020 t=32.1 h=2 st=2] c=1757 chg=a 1970-05-31 01:42:28.105726458 UTC
var PRINTK_PATTERN_STRING_NEW = `^` +
	`\s*(?P<loglevel>\d+),` +
	`(?P<sequence>\d+),` +
	`(?P<timestamp_us>\d+),` +
	`.;` +
	`(?P<payload>.*)$`

type PrintkLog struct {
	LogLevel    int
	Timestamp   float64 `logcat:"timestamp"`
	TimestampUs int64   `logcat:"timestamp_us"`
	Sequence    int64   `logcat:"sequence"`
}

type PrintkSubmessage interface {
	GetPrintk() *PrintkLog
	SetPrintk(log *PrintkLog)
}

type PrintkSubparser struct {
	Prefix string
	Parser
}

type PrintkParser struct {
	RegexParser *MultRegexParser
	Subparsers  []*PrintkSubparser

	re []*regexp.Regexp

	// Parameters
	ErrOnUnknownTag bool
}

func NewPrintkParser() *PrintkParser {
	parser := &PrintkParser{
		Subparsers: []*PrintkSubparser{
			&PrintkSubparser{
				"msm_thermal:",
				NewMsmThermalParser(),
			},
			&PrintkSubparser{
				"PM: suspend e",
				NewPMManagementParser(),
			},
			&PrintkSubparser{
				"healthd:",
				NewHealthdParser(),
			},
			&PrintkSubparser{
				"acpuclk-8974 qcom,acpuclk.30: ACPU PVS:",
				NewPvsBinParser(),
			},
		},
		re: []*regexp.Regexp{
			regexp.MustCompile(PRINTK_PATTERN_STRING),
			regexp.MustCompile(PRINTK_PATTERN_STRING_NEW),
		},
		ErrOnUnknownTag: true,
	}
	parser.RegexParser = NewMultRegexParser(parser)
	return parser
}

func (p *PrintkParser) New() interface{} {
	return &PrintkLog{}
}

func (p *PrintkParser) Regex() []*regexp.Regexp {
	return p.re
}

func (p *PrintkParser) Parse(line string) (interface{}, error) {
	var printk *PrintkLog

	// For these add-ons, we'll just ignore them and return the string.
	// This has the same effect as not invoking the subparser.
	// FIXME: is there a better way to do this?
	if strings.HasPrefix(line, "SUBSYSTEM=") || strings.HasPrefix(line, "DEVICE=") {
		return line, nil
	}

	// Parse using regex parser
	if obj, err := p.RegexParser.Parse(line); err != nil {
		return nil, err
	} else {
		printk = obj.(*PrintkLog)
	}

	// Timestamp (new)
	if printk.TimestampUs > 0 {
		printk.Timestamp = float64(printk.TimestampUs) / float64(1000000)
	}

	payload := p.RegexParser.LastMap["payload"]

	// Find the parser by prefix
	var parser Parser = nil
	for _, pkp := range p.Subparsers {
		if strings.HasPrefix(payload, pkp.Prefix) {
			parser = pkp.Parser
			break
		}
	}

	if parser == nil {
		if p.ErrOnUnknownTag {
			return nil, fmt.Errorf("No parser defined for printk payload: '%v'", payload)
		} else {
			return printk, nil
		}
	}

	if obj, err := parser.Parse(payload); err != nil {
		return nil, err
	} else {
		log := obj.(PrintkSubmessage)
		log.SetPrintk(printk)
		return log, err
	}

}

///////////////////////////////////////////////////////////////////////////////
// MSM thermal printk messages (temperature)

var MSM_THERMAL_PRINTK_PATTERN = regexp.MustCompile(`` +
	`^\s*msm_thermal: (?P<state>(Set Offline:|Allow Online)) CPU(?P<cpu>\d+) Temp: (?P<temp>\d+)$`)

type MsmThermalPrintk struct {
	PrintkLog `logcat:"-"`
	StateStr  string          `logcat:"state"`
	State     MsmThermalState `logcat:"-"`
	Cpu       int             `logcat:"cpu"`
	Temp      int             `logcat:"temp"`
}

func (log *MsmThermalPrintk) GetPrintk() *PrintkLog {
	return &log.PrintkLog
}

func (log *MsmThermalPrintk) SetPrintk(pk *PrintkLog) {
	log.PrintkLog = *pk
}

type MsmThermalParser struct {
	RegexParser *RegexParser
}

func NewMsmThermalParser() *MsmThermalParser {
	parser := &MsmThermalParser{}
	parser.RegexParser = NewRegexParser(parser)
	return parser
}

func (p *MsmThermalParser) Parse(line string) (interface{}, error) {
	var mtp *MsmThermalPrintk

	if obj, err := p.RegexParser.Parse(line); err != nil {
		return obj, err
	} else {
		mtp = obj.(*MsmThermalPrintk)
	}

	// Set state
	if strings.Compare(mtp.StateStr, "Set Offline:") == 0 {
		mtp.State = MSM_THERMAL_STATE_OFFLINE
	} else if strings.Compare(mtp.StateStr, "Allow Online") == 0 {
		mtp.State = MSM_THERMAL_STATE_ONLINE
	} else {
		return nil, fmt.Errorf("Unknown msm_thermal state '%s'", mtp.StateStr)
	}

	return mtp, nil
}

func (p *MsmThermalParser) New() interface{} {
	return &MsmThermalPrintk{}
}

func (p *MsmThermalParser) Regex() *regexp.Regexp {
	return MSM_THERMAL_PRINTK_PATTERN
}

///////////////////////////////////////////////////////////////////////////////
// PowerManger Suspend entry and exit

/* Format:
<6>[93341.687692] PM: suspend exit 2016-04-27 04:00:00.220795560 UTC
<6>[93341.915138] PM: suspend entry 2016-04-27 04:00:00.448241184 UTC
*/

var POWER_MANAGEMENT_PRINTK_PATTERN = regexp.MustCompile(`^` +
	`\s*PM: suspend (?P<state>entry|exit)` +
	`\s+(?P<datetime>\d{4}-\d{2}-\d{2}` +
	`\s+\d{2}:\d{2}:\d{2}.\d+)` +
	`\s+(?P<timezone>\S+)$`)

type PowerManagementPrintk struct {
	PrintkLog `logcat:"-"`
	State     PowerManagementState `logcat:"-"`
	Datetime  time.Time            `logcat:"-"`
}

func (log *PowerManagementPrintk) GetPrintk() *PrintkLog {
	return &log.PrintkLog
}

func (log *PowerManagementPrintk) SetPrintk(pk *PrintkLog) {
	log.PrintkLog = *pk
}

type PMManagementParser struct {
	RegexParser *RegexParser
}

func NewPMManagementParser() *PMManagementParser {
	parser := &PMManagementParser{}
	parser.RegexParser = NewRegexParser(parser)
	return parser
}

func (p *PMManagementParser) New() interface{} {
	return &PowerManagementPrintk{}
}

func (p *PMManagementParser) Regex() *regexp.Regexp {
	return POWER_MANAGEMENT_PRINTK_PATTERN
}

func (p *PMManagementParser) Parse(line string) (interface{}, error) {
	var pmp *PowerManagementPrintk

	if obj, err := p.RegexParser.Parse(line); err != nil {
		return obj, err
	} else {
		pmp = obj.(*PowerManagementPrintk)
	}

	m := p.RegexParser.LastMap

	if strings.Compare(m["state"], "entry") == 0 {
		pmp.State = PM_SUSPEND_ENTRY
	} else if strings.Compare(m["state"], "exit") == 0 {
		pmp.State = PM_SUSPEND_EXIT
	} else {
		return nil, fmt.Errorf("Unknown pm state for line: %v", line)
	}

	if datetime, err := strptime.Parse(m["datetime"], "%Y-%m-%d %H:%M:%S.%f"); err != nil {
		return nil, err
	} else {
		pmp.Datetime = datetime
	}

	return pmp, nil
}

///////////////////////////////////////////////////////////////////////////////
// Healthd logs (battery stats)

// New and old pattern
// TODO: Add ending timestamp
var HEALTHD_PATTERN = regexp.MustCompile(`^` +
	`healthd:\s*battery` +
	`\s*\[?l=(?P<l>\d+)` +
	`\s+v=(?P<v>\d+)` +
	`\s+t=(?P<t>\d+\.\d+)` +
	`\s+h=(?P<h>-?\d+)\s+` +
	`st=(?P<st>-?\d+)\]?` +
	`\s+c=(?P<c>-?\d+)` +
	`\s+chg=(?P<chg>([auw]+)?)` +
	`.*$`)

type Healthd struct {
	PrintkLog `logcat:"-"`
	L         int     `logcat:"l"`
	V         int     `logcat:"v"`
	T         float64 `logcat:"t"`
	H         int     `logcat:"h"`
	St        int     `logcat:"st"`
	C         int     `logcat:"c"`
	Chg       string  `logcat:"chg"`
}

func (log *Healthd) GetPrintk() *PrintkLog {
	return &log.PrintkLog
}

func (log *Healthd) SetPrintk(pk *PrintkLog) {
	log.PrintkLog = *pk
}

type HealthdParser struct {
	RegexParser *RegexParser
}

func NewHealthdParser() *HealthdParser {
	parser := &HealthdParser{}
	parser.RegexParser = NewRegexParser(parser)
	return parser
}

func (p *HealthdParser) New() interface{} {
	return &Healthd{}
}

func (p *HealthdParser) Regex() *regexp.Regexp {
	return HEALTHD_PATTERN
}

func (p *HealthdParser) Parse(line string) (interface{}, error) {
	// Currently, this just wraps the regex parser
	return p.RegexParser.Parse(line)
}

///////////////////////////////////////////////////////////////////////////////
// PVS Bin

// TODO: Nexus 6
var PVS_BIN_PATTERN = regexp.MustCompile(`^` +
	`acpuclk-8974 qcom,acpuclk.30: ACPU PVS: (?P<pvs_bin>\d+)$`)

type PvsBin struct {
	PrintkLog `logcat:"-"`
	PvsBin    int `logcat:"pvs_bin"`
}

func (log *PvsBin) GetPrintk() *PrintkLog {
	return &log.PrintkLog
}

func (log *PvsBin) SetPrintk(pk *PrintkLog) {
	log.PrintkLog = *pk
}

type PvsBinParser struct {
	RegexParser *RegexParser
}

func NewPvsBinParser() *PvsBinParser {
	parser := &PvsBinParser{}
	parser.RegexParser = NewRegexParser(parser)
	return parser
}

func (p *PvsBinParser) New() interface{} {
	return &PvsBin{}
}

func (p *PvsBinParser) Regex() *regexp.Regexp {
	return PVS_BIN_PATTERN
}

func (p *PvsBinParser) Parse(line string) (interface{}, error) {
	// Currently, this just wraps the regex parser
	return p.RegexParser.Parse(line)
}
