package phonelab

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gurupras/gocommons"
	"github.com/pbnjay/strptime"
)

type Logline struct {
	Line          string    `logcat:"-"`
	BootId        string    `logcat:"boot_id"`
	Datetime      time.Time `logcat:"-"`
	DatetimeNanos int64     `logcat:"-"`
	LogcatToken   int64     `logcat:"LogcatToken"`
	TraceTime     float64   `logcat:"tracetime"`
	Pid           int32     `logcat:"pid"`
	Tid           int32     `logcat:"tid"`
	Level         string    `logcat:"level"`
	Tag           string    `logcat:"tag"`

	// This will be a string or object, depending on if it has been parsed.
	Payload interface{} `logcat:"-"`
}

func (ll *Logline) MonotonicTimestamp() float64 {
	return ll.TraceTime
}

type Loglines []*Logline

var PATTERN = regexp.MustCompile(`` +
	`\s*(?P<line>` +
	`\s*(?P<boot_id>[a-z0-9\-]{36})` +
	`\s*(?P<datetime>\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d+)` +
	`\s*(?P<LogcatToken>\d+)` +
	`\s*\[\s*(?P<tracetime>\d+\.\d+)\]` +
	`\s*(?P<pid>\d+)` +
	`\s*(?P<tid>\d+)` +
	`\s*(?P<level>[A-Z]+)` +
	`\s*(?P<tag>.*?)\s*: ` +
	`\s*(?P<payload>.*)` +
	`)`)

var PHONELAB_PATTERN = regexp.MustCompile(`` +
	`(?P<line>` +
	`(?P<deviceid>[a-z0-9]+)` +
	`\s+(?P<logcat_timestamp>\d+)` +
	`\s+(?P<logcat_timestamp_sub>\d+(\.\d+)?)` +
	`\s+(?P<boot_id>[a-z0-9\-]{36})` +
	`\s+(?P<LogcatToken>\d+)` +
	`\s+(?P<tracetime>\d+\.\d+)` +
	`\s+(?P<datetime>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+)` +
	`\s+(?P<pid>\d+)` +
	`\s+(?P<tid>\d+)` +
	`\s+(?P<level>[A-Z]+)` +
	`\s+(?P<tag>\S+)` +
	`\s+(?P<payload>.*)` +
	`)`)

type LogcatParser struct {
	RegexParser *MultRegexParser
	Patterns    []*regexp.Regexp

	// Parameters
	StoreLogline bool

	// Private
	curPattern   int
	startPattern int
	l            sync.Mutex
}

func NewLogcatParser() *LogcatParser {
	parser := &LogcatParser{
		Patterns: []*regexp.Regexp{
			PHONELAB_PATTERN, PATTERN,
		},
		curPattern:   0,
		startPattern: 0,
		StoreLogline: true,
	}
	parser.RegexParser = NewMultRegexParser(parser)
	return parser
}

func (p *LogcatParser) New() interface{} {
	return &Logline{}
}

func (p *LogcatParser) Regex() []*regexp.Regexp {
	return p.Patterns
}

func (p *LogcatParser) Parse(line string) (*Logline, error) {
	p.l.Lock()
	defer p.l.Unlock()

	var logline *Logline = nil

	if obj, err := p.RegexParser.Parse(line); err == nil {
		logline = obj.(*Logline)
	}

	if logline == nil {
		return nil, fmt.Errorf("Unsupported logcat format or invalid logline")
	}

	// The RegexParser handles most of the fields, we just need to patch
	// up the remainder.

	// Some datetimes are 9 digits instead of 6
	// TODO: Get rid of the last 3

	// Text line
	if p.StoreLogline {
		logline.Line = line
	}

	// Payload
	logline.Payload = p.RegexParser.LastMap["payload"]

	// Datetime Nanoseconds
	if res, err := strconv.ParseInt(p.RegexParser.LastMap["datetime"][20:], 0, 64); err != nil {
		return nil, err
	} else {
		logline.DatetimeNanos = res
	}

	// Datetime (seconds)
	dt := p.RegexParser.LastMap["datetime"]
	if len(dt) > 26 {
		dt = dt[:26]
	}
	if res, err := strptime.Parse(dt, "%Y-%m-%d %H:%M:%S.%f"); err != nil {
		return nil, err
	} else {
		logline.Datetime = res
	}

	return logline, nil
}

var legacyParser = NewLogcatParser()

// Legacy
func ParseLogline(line string) (*Logline, error) {
	ll, err := legacyParser.Parse(line)
	if err != nil {
		return nil, err
	} else {
		return ll, nil
		//return ll.(*Logline), nil
	}
}

// TODO: Should this move?
func ParseLoglineConvert(line string) gocommons.SortInterface {
	if ll, _ := ParseLogline(line); ll != nil {
		return ll
	} else {
		return nil
	}
}

var (
	LoglineSortParams = gocommons.SortParams{LineConvert: ParseLoglineConvert, Lines: make(gocommons.SortCollection, 0)}
)

func (l *Logline) String() string {
	return l.Line
	//	return fmt.Sprintf("%v %v %v [%v] %v %v %v %v: %v",
	//		l.boot_id, l.datetime, l.LogcatToken, l.tracetime,
	//		l.pid, l.tid, l.level, l.tag, l.payload)
}

func (l *Logline) Less(s gocommons.SortInterface) (ret bool, err error) {
	var o *Logline
	var ok bool
	if s != nil {
		if o, ok = s.(*Logline); !ok {
			err = fmt.Errorf("Failed to convert from SortInterface to *Logline: %v", reflect.TypeOf(s))
			ret = false
			goto out
		}
	}
	if l != nil && o != nil {
		bootComparison := strings.Compare(l.BootId, o.BootId)
		if bootComparison == -1 {
			ret = true
		} else if bootComparison == 1 {
			ret = false
		} else {
			// Same boot ID..compare the other fields
			if l.LogcatToken == o.LogcatToken {
				ret = l.TraceTime < o.TraceTime
			} else {
				ret = l.LogcatToken < o.LogcatToken
			}
		}
	} else if l != nil {
		ret = true
	} else {
		ret = false
	}
out:
	return
}
