package phonelab

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
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

/* Printk pattern example: <6>[   21.512807] msm_thermal: Allow Online CPU3 Temp: 66 */
var PRINTK_PATTERN_STRING = `(<(?P<loglevel>\d+)>)?\s*\[\s*(?P<timestamp>\d+\.\d+)\]\s*`

///////////////////////////////////////////////////////////////////////////////
// MSM thermal printk messages (temperature)

var MSM_THERMAL_PRINTK_PATTERN = regexp.MustCompile(PRINTK_PATTERN_STRING +
	`\s*msm_thermal: (?P<state>(Set Offline:|Allow Online)) CPU(?P<cpu>\d+) Temp: (?P<temp>\d+)`)

type MsmThermalPrintk struct {
	Logline  *Logline        `logcat:"-"`
	StateStr string          `logcat:"state"`
	State    MsmThermalState `logcat:"-"`
	Cpu      int             `logcat:"cpu"`
	Temp     int             `logcat:"temp"`
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

	// TODO: fix
	// mtp.Logline = logline

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

func ParseMsmThermalPrintk(logline *Logline) *MsmThermalPrintk {
	parser := NewMsmThermalParser()
	if obj, err := parser.Parse(logline.Payload); err != nil {
		return nil
	} else {
		mtp := obj.(*MsmThermalPrintk)
		mtp.Logline = logline
		return mtp
	}
}

///////////////////////////////////////////////////////////////////////////////
// PowerManger Suspend entry and exit

/* Format:
<6>[93341.687692] PM: suspend exit 2016-04-27 04:00:00.220795560 UTC
<6>[93341.915138] PM: suspend entry 2016-04-27 04:00:00.448241184 UTC
*/

var POWER_MANAGEMENT_PRINTK_PATTERN = regexp.MustCompile(PRINTK_PATTERN_STRING +
	`\s*PM: suspend (?P<state>entry|exit) (?P<datetime>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}.\d+) (?P<timezone>\S+)`)

type PowerManagementPrintk struct {
	Logline  *Logline             `logcat:"-"`
	State    PowerManagementState `logcat:"-"`
	Datetime time.Time            `logcat:"-"`
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

	// TODO: Fix
	//pmp.Logline = logline

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

func ParsePowerManagementPrintk(logline *Logline) *PowerManagementPrintk {
	parser := NewPMManagementParser()
	if obj, err := parser.Parse(logline.Payload); err != nil {
		return nil
	} else {
		pmp := obj.(*PowerManagementPrintk)
		pmp.Logline = logline
		return pmp
	}
}

///////////////////////////////////////////////////////////////////////////////
// Healthd logs (battery stats)

var HEALTHD_PATTERN = regexp.MustCompile(`` +
	PRINTK_PATTERN_STRING +
	`healthd:\s*battery\s*l=(?P<l>\d+)\s+v=(?P<v>\d+)\s+t=(?P<t>\d+\.\d+)\s+h=(?P<h>-?\d+)\s+st=(?P<st>-?\d+)\s+c=(?P<c>-?\d+)\s+chg=(?P<chg>([auw]+)?)\s*`)

type Healthd struct {
	Logline   *Logline `logcat:"-"`
	Timestamp float64  `logcat:"timestamp"`
	L         int      `logcat:"l"`
	V         int      `logcat:"v"`
	T         float64  `logcat:"t"`
	H         int      `logcat:"h"`
	St        int      `logcat:"st"`
	C         int      `logcat:"c"`
	Chg       string   `logcat:"chg"`
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
	return &MsmThermalPrintk{}
}

func (p *HealthdParser) Regex() *regexp.Regexp {
	return HEALTHD_PATTERN
}

func (p *HealthdParser) Parse(line string) (interface{}, error) {
	// Currently, this just wraps the regex parser
	// TODO: Fix
	//pmp.Logline = logline
	return p.RegexParser.Parse(line)
}

// Legacy
func ParseHealthdPrintk(logline *Logline) *Healthd {
	parser := NewHealthdParser()
	if obj, err := parser.Parse(logline.Payload); err != nil {
		return nil
	} else {
		hd := obj.(*Healthd)
		hd.Logline = logline
		return hd
	}
}

///////////////////////////////////////////////////////////////////////////////
// Healthd logs (battery stats)

var PVS_BIN_PATTERN = regexp.MustCompile(`` +
	PRINTK_PATTERN_STRING +
	`acpuclk-8974 qcom,acpuclk.30: ACPU PVS: (?P<pvs_bin>\d+)`)

type PvsBin struct {
	Logline   *Logline
	Timestamp float64
	PvsBin    int
}

func ParsePvsBin(logline *Logline) *PvsBin {
	names := PVS_BIN_PATTERN.SubexpNames()
	values_raw := PVS_BIN_PATTERN.FindAllStringSubmatch(logline.Payload, -1)
	if len(values_raw) == 0 {
		fmt.Fprintln(os.Stderr, "Failed to parse:", logline.Line)
		os.Exit(-1)
	}
	values := values_raw[0]

	kv_map := map[string]string{}
	for i, value := range values {
		kv_map[names[i]] = value
	}
	timestamp, err := strconv.ParseFloat(kv_map["timestamp"], 64)
	if err != nil {
		return nil
	}
	pvsBin, err := strconv.ParseInt(kv_map["pvs_bin"], 0, 32)
	if err != nil {
		return nil
	}
	obj := PvsBin{logline, timestamp, int(pvsBin)}
	return &obj
}
