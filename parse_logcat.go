package phonelab

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gurupras/gocommons"
	"github.com/pbnjay/strptime"
)

type Logline struct {
	Line          string
	BootId        string
	Datetime      time.Time
	DatetimeNanos int64
	LogcatToken   int64
	TraceTime     float64
	Pid           int32
	Tid           int32
	Level         string
	Tag           string
	Payload       string
	PayloadObj    interface{}
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

// The order can be swapped in case we know the format a priori.
var allLCPatterns = []*regexp.Regexp{
	PHONELAB_PATTERN, PATTERN,
}

func ParseLogline(line string) (*Logline, error) {
	var err error
	var names []string
	var values_raw [][]string

	for _, pattern := range allLCPatterns {
		names = pattern.SubexpNames()
		values_raw = pattern.FindAllStringSubmatch(line, -1)
		if len(values_raw) > 0 {
			break
		}
	}

	if len(values_raw) != 1 {
		return nil, fmt.Errorf("Unsupported logcat format or invalid logline")
	}

	values := values_raw[0]

	kv_map := map[string]string{}
	for i, value := range values {
		kv_map[names[i]] = value
	}

	// Convert values
	// Some datetimes are 9 digits instead of 6
	// TODO: Get rid of the last 3

	datetimeNanos, err := strconv.ParseInt(kv_map["datetime"][20:], 0, 64)
	if err != nil {
		return nil, err
	}

	if len(kv_map["datetime"]) > 26 {
		kv_map["datetime"] = kv_map["datetime"][:26]
	}

	datetime, err := strptime.Parse(kv_map["datetime"], "%Y-%m-%d %H:%M:%S.%f")
	if err != nil {
		return nil, err
	}

	token, err := strconv.ParseInt(kv_map["LogcatToken"], 0, 64)
	if err != nil {
		return nil, err
	}

	tracetime, err := strconv.ParseFloat(kv_map["tracetime"], 64)
	if err != nil {
		return nil, err
	}

	pid, err := strconv.ParseInt(kv_map["pid"], 0, 32)
	if err != nil {
		return nil, err
	}

	tid, err := strconv.ParseInt(kv_map["tid"], 0, 32)
	if err != nil {
		return nil, err
	}

	ll := &Logline{
		Line:          line,
		BootId:        kv_map["boot_id"],
		Datetime:      datetime,
		DatetimeNanos: datetimeNanos,
		LogcatToken:   token,
		TraceTime:     tracetime,
		Pid:           int32(pid),
		Tid:           int32(tid),
		Level:         kv_map["level"],
		Tag:           kv_map["tag"],
		Payload:       kv_map["payload"],
	}

	return ll, nil
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
			err = errors.New(fmt.Sprintf("Failed to convert from SortInterface to *Logline:", reflect.TypeOf(s)))
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
