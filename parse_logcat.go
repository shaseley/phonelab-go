package phonelab

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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

	fieldParser *logcatFieldParser
}

func NewLogcatParser() *LogcatParser {
	parser := &LogcatParser{
		Patterns: []*regexp.Regexp{
			PHONELAB_PATTERN, PATTERN,
		},
		curPattern:   0,
		startPattern: 0,
		StoreLogline: true,
		fieldParser:  newLogcatFieldParser(),
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

var ConfigDoNewParse = true
var ConfigDoNewNewParse = false

func (p *LogcatParser) Parse(line string) (*Logline, error) {

	if ConfigDoNewParse {
		return parseLoglineString(line)
	} else if ConfigDoNewNewParse {
		return p.fieldParser.Parse(line)
	}

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

func (l *Logline) String() string {
	return l.Line
	//	return fmt.Sprintf("%v %v %v [%v] %v %v %v %v: %v",
	//		l.boot_id, l.datetime, l.LogcatToken, l.tracetime,
	//		l.pid, l.tid, l.level, l.tag, l.payload)
}

func (l *Logline) Less(s interface{}) (ret bool, err error) {
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

type logcatStringParser struct {
	pos       int
	length    int
	line      string
	lastToken string
}

func ws(c uint8) bool {
	switch c {
	default:
		return false
	case '\n':
		fallthrough
	case '\r':
		fallthrough
	case '\t':
		fallthrough
	case ' ':
		return true
	}
}

func (p *logcatStringParser) advance() {

	// Skip preliminary whitespace
	for p.pos < p.length && ws(p.line[p.pos]) {
		p.pos += 1
	}
}

func (p *logcatStringParser) nextToken() (string, bool) {

	p.advance()

	if p.pos >= p.length {
		return "", false
	}

	start := p.pos

	// increment until we get a whitespace character or the end
	for p.pos < p.length && !ws(p.line[p.pos]) {
		p.pos += 1
	}

	//fmt.Println("Next token:", p.line[start:p.pos])

	// Position is now just past the token
	p.lastToken = p.line[start:p.pos]
	return p.lastToken, true
}

func (p *logcatStringParser) parseFixedLenString(expected int) (string, error) {
	if data, ok := p.nextToken(); !ok {
		return "", errors.New("LC Parser Error: Expected string token, got EOF")
	} else if len(data) != expected {
		return "", fmt.Errorf("LC Parser Error: Invalid string length. Expected %v got %v", expected, len(data))
	} else {
		return data, nil
	}
}

func (p *logcatStringParser) parseInt64() (int64, error) {
	if data, ok := p.nextToken(); !ok {
		return 0, errors.New("LC Parser Error: Expected int token, got EOF")
	} else if i, err := strconv.ParseInt(data, 10, 64); err != nil {
		return 0, err
	} else {
		return i, nil
	}
}

func (p *logcatStringParser) parseFloat64() (float64, error) {
	if data, ok := p.nextToken(); !ok {
		return 0.0, errors.New("LC Parser Error: Expected int token, got EOF")
	} else if f, err := strconv.ParseFloat(data, 64); err != nil {
		return 0.0, err
	} else {
		return f, nil
	}
}

func (p *logcatStringParser) parseTagAndPayload() (string, string, error) {
	p.advance()

	if p.pos >= p.length {
		return "", "", errors.New("LC Parser Error: Missing tag and payload")
	}

	start := p.pos

	// increment until we get a whitespace character or the end
	for p.pos < p.length-1 && p.line[p.pos:p.pos+2] != ": " {
		p.pos += 1
	}

	if p.pos >= p.length-1 {
		return "", "", errors.New("LC Parser Error: Missing tag and payload")
	}

	// Position is now just past the token or at the end
	tag := strings.TrimSpace(p.line[start:p.pos])
	payload := strings.TrimSpace(p.line[p.pos+2:])
	return tag, payload, nil
}

func parseInts(s string, starts, lengths []int) ([]int, error) {
	if len(starts) != len(lengths) {
		return nil, errors.New("starts and lengths must be same length")
	}

	l := len(starts)
	res := make([]int, l, l)

	var err error

	for i := 0; i < l; i++ {
		start, end := starts[i], starts[i]+lengths[i]
		res[i], err = strconv.Atoi(s[start:end])
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (parser *logcatStringParser) parseDateTime() (timeOut time.Time, nanoTime int, err error) {

	var datePart string

	datePart, err = parser.parseFixedLenString(10)
	if err != nil {
		return
	}

	// Now parse
	if datePart[4] != '-' || datePart[7] != '-' {
		err = errors.New("LC Parser Error: Invalid date format")
		return
	}

	var dateParsed []int

	dateParsed, err = parseInts(datePart, []int{0, 5, 8}, []int{4, 2, 2})
	if err != nil {
		return
	}

	timePart, ok := parser.nextToken()

	if !ok {
		err = errors.New("LC Parser Error: Expected date string token, got EOF")
		return
	}

	if len(timePart) < 10 || timePart[2] != ':' || timePart[5] != ':' || timePart[8] != '.' {
		err = errors.New("LC Parser Error: Invalid time format")
		return
	}

	var timeParsed []int

	timeParsed, err = parseInts(timePart, []int{0, 3, 6}, []int{2, 2, 2})
	if err != nil {
		return
	}

	// Finally, ns
	nsPart := timePart[9:]
	nanoTime, err = strconv.Atoi(nsPart)
	if err != nil {
		return
	}

	if len(nsPart) > 9 {
		err = fmt.Errorf("Invalid nanotime: %v", nsPart)
	}

	for i := 0; i < 9-len(nsPart); i++ {
		nanoTime *= 10
	}

	timeOut = time.Date(dateParsed[0], time.Month(dateParsed[1]), dateParsed[2],
		timeParsed[0], timeParsed[1], timeParsed[2], nanoTime, est)

	return
}

var est *time.Location

func init() {
	var err error

	if est, err = time.LoadLocation("EST"); err != nil {
		panic(fmt.Sprintf("Unable to set EST: %v", err))
	}

	initLocatFieldInfo()
}

func (parser *logcatStringParser) parseTraceTimeBrackets() (float64, error) {
	parser.advance()

	if parser.pos >= parser.length || parser.line[parser.pos] != '[' {
		return 0.0, errors.New("Invalid tracetime format. Expected [tracetime] (1)")
	}

	// Skip open [ and whitespace
	parser.pos += 1
	parser.advance()

	t, ok := parser.nextToken()
	if !ok || parser.pos >= parser.length || t[len(t)-1] != ']' {
		return 0.0, errors.New("Invalid tracetime format. Expected [tracetime] (2)")
	}
	t = t[:len(t)-1]

	return strconv.ParseFloat(t, 64)
}

func (parser *logcatStringParser) parseLoglinePhonelabFmt() (*Logline, error) {

	// Skip the next 2 fields after device id
	const numSkips = 2

	for i := 0; i < numSkips; i += 1 {
		if _, ok := parser.nextToken(); !ok {
			return nil, errors.New("LC Parser Error: unexpected EOF")
		}
	}

	ll := &Logline{}

	var err error
	var ok bool

	if ll.BootId, ok = parser.nextToken(); !ok {
		return nil, errors.New("LC Parser Error: Expected boot_id token, got EOF")
	}

	if ll.LogcatToken, err = parser.parseInt64(); err != nil {
		return nil, err
	}

	if ll.TraceTime, err = parser.parseFloat64(); err != nil {
		return nil, err
	}

	var nanos int
	if ll.Datetime, nanos, err = parser.parseDateTime(); err != nil {
		return nil, err
	} else {
		ll.DatetimeNanos = int64(nanos)
	}

	if v, err := parser.parseInt64(); err != nil {
		return nil, err
	} else {
		ll.Pid = int32(v)
	}

	if v, err := parser.parseInt64(); err != nil {
		return nil, err
	} else {
		ll.Tid = int32(v)
	}

	if ll.Level, err = parser.parseFixedLenString(1); err != nil {
		return nil, err
	}

	if ll.Tag, ok = parser.nextToken(); !ok {
		return nil, errors.New("LC Parser Error: Expected tag token, got EOF")
	}

	ll.Payload = strings.TrimSpace(parser.line[parser.pos:])
	ll.Line = parser.line

	return ll, nil

}

func (parser *logcatStringParser) parseLoglineTraceTimeFmt() (*Logline, error) {
	// Assume we've already parsed the first field.
	ll := &Logline{
		BootId: parser.lastToken,
	}

	var err error
	var nanos int

	if ll.Datetime, nanos, err = parser.parseDateTime(); err != nil {
		return nil, err
	} else {
		ll.DatetimeNanos = int64(nanos)
	}

	if ll.LogcatToken, err = parser.parseInt64(); err != nil {
		return nil, err
	}

	if ll.TraceTime, err = parser.parseTraceTimeBrackets(); err != nil {
		return nil, err
	}

	if v, err := parser.parseInt64(); err != nil {
		return nil, err
	} else {
		ll.Pid = int32(v)
	}

	if v, err := parser.parseInt64(); err != nil {
		return nil, err
	} else {
		ll.Tid = int32(v)
	}

	if ll.Level, err = parser.parseFixedLenString(1); err != nil {
		return nil, err
	}

	if ll.Tag, ll.Payload, err = parser.parseTagAndPayload(); err != nil {
		return nil, err
	}

	ll.Line = parser.line

	return ll, nil
}

func parseLoglineString(line string) (*Logline, error) {
	parser := &logcatStringParser{
		line:   line,
		length: len(line),
	}

	firstField, ok := parser.nextToken()
	if !ok {
		return nil, errors.New("LC Parser Error: Invalid line")
	}

	if len(firstField) == 40 {
		return parser.parseLoglinePhonelabFmt()
	} else if len(firstField) == 36 {
		return parser.parseLoglineTraceTimeFmt()
	} else {
		return nil, errors.New("LC Parser Error: Unsupported logcat format")
	}
}

////////////////////////////////////////////////////////////////////////////////
// New Format (Field Declaration)

type logcatFieldParser struct {
	addrs       []interface{}
	fieldParser *lineParserImpl
	temp        *intermediateLine
	sync.Mutex
}

func newLogcatFieldParser() *logcatFieldParser {
	return &logcatFieldParser{
		addrs:       make([]interface{}, numLineParserAddrs, numLineParserAddrs),
		fieldParser: newLineParserImpl(nil, ""),
		temp:        &intermediateLine{},
	}
}

func (p *logcatFieldParser) Parse(line string) (*Logline, error) {
	p.Lock()
	defer p.Unlock()

	if len(line) > 40 && line[36] != ' ' && line[36] != '\t' {
		return p.parseLoglinePhoneLabFmt(line)
	} else {
		return p.parseLoglineTraceTimeFmt(line)
	}
}

func (p *logcatFieldParser) parseLoglineTraceTimeFmt(line string) (*Logline, error) {

	ll := &Logline{}
	var skip string

	p.addrs[0] = &ll.BootId
	p.addrs[1] = &p.temp.Year
	p.addrs[2] = &p.temp.Month
	p.addrs[3] = &p.temp.Day
	p.addrs[4] = &p.temp.Hour
	p.addrs[5] = &p.temp.Minutes
	p.addrs[6] = &p.temp.Sec
	p.addrs[7] = &p.temp.Nanos
	p.addrs[8] = &ll.LogcatToken
	p.addrs[9] = &skip
	p.addrs[10] = &ll.TraceTime
	p.addrs[11] = &ll.Pid
	p.addrs[12] = &ll.Tid
	p.addrs[13] = &ll.Level
	p.addrs[14] = &ll.Tag
	p.addrs[15] = &p.temp.Payload

	return p.commonParse(traceLogLineFields, line, ll)
}

type intermediateLine struct {
	Year    int
	Month   int
	Day     int
	Hour    int
	Minutes int
	Sec     int
	Nanos   string
	Payload string
}

func loglineFromIntermediate(ll *Logline, temp *intermediateLine) error {
	// These need to be trimmed
	ll.Tag = strings.TrimSpace(ll.Tag)
	ll.Payload = strings.TrimSpace(temp.Payload)

	nanoTime, err := strconv.Atoi(temp.Nanos)
	if err != nil {
		return fmt.Errorf("Error parsing nano time: %v", err)
	}

	if len(temp.Nanos) > 9 {
		return fmt.Errorf("Invalid nanotime: %v", temp.Nanos)
	}

	for i := 0; i < 9-len(temp.Nanos); i++ {
		nanoTime *= 10
	}

	ll.Datetime = time.Date(temp.Year, time.Month(temp.Month), temp.Day,
		temp.Hour, temp.Minutes, temp.Sec, nanoTime, est)

	ll.DatetimeNanos = int64(nanoTime)

	return nil
}

func (p *logcatFieldParser) commonParse(fields []*FieldInfo, line string, ll *Logline) (*Logline, error) {
	p.fieldParser.reset(line, fields)

	for i, f := range fields {
		if err := p.fieldParser.parseField(f, p.addrs[i]); err != nil {
			return nil, err
		}
	}

	ll.Line = line

	if err := loglineFromIntermediate(ll, p.temp); err != nil {
		return nil, err
	}

	return ll, nil
}

func (p *logcatFieldParser) parseLoglinePhoneLabFmt(line string) (*Logline, error) {

	ll := &Logline{}

	var skip string

	p.addrs[0] = &skip
	p.addrs[1] = &skip
	p.addrs[2] = &skip
	p.addrs[3] = &ll.BootId
	p.addrs[4] = &ll.LogcatToken
	p.addrs[5] = &ll.TraceTime
	p.addrs[6] = &p.temp.Year
	p.addrs[7] = &p.temp.Month
	p.addrs[8] = &p.temp.Day
	p.addrs[9] = &p.temp.Hour
	p.addrs[10] = &p.temp.Minutes
	p.addrs[11] = &p.temp.Sec
	p.addrs[12] = &p.temp.Nanos
	p.addrs[13] = &ll.Pid
	p.addrs[14] = &ll.Tid
	p.addrs[15] = &ll.Level
	p.addrs[16] = &ll.Tag
	p.addrs[17] = &p.temp.Payload

	return p.commonParse(phoneLabLogLineFields, line, ll)
}

var (
	traceLogLineFields    []*FieldInfo
	phoneLabLogLineFields []*FieldInfo
)

const (
	numLineParserAddrs = 18
)

func initLocatFieldInfo() {
	traceLogLineFields = []*FieldInfo{
		&FieldInfo{
			Name:       "Boot ID",
			FieldType:  FieldTypeString,
			Length:     36,
			LengthType: LengthTypeFixed,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Year",
			FieldType:  FieldTypeInt,
			Length:     4,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'-'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Month",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'-'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Day",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Hours",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{':'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Minutes",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{':'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Seconds",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'.'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Nanos",
			FieldType:  FieldTypeString,
			Length:     9,
			LengthType: LengthTypeMax,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Token",
			FieldType: FieldTypeInt64,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Skip",
			Skip:       true,
			FieldType:  FieldTypeString,
			Length:     1,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'['},
			StopType:   StopTypeCharacterInclusive,
		},
		&FieldInfo{
			Name:      "Tracetime",
			FieldType: FieldTypeFloat64,
			StopChars: []uint8{']'},
			StopType:  StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:      "Pid",
			FieldType: FieldTypeInt32,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Tid",
			FieldType: FieldTypeInt32,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Level",
			FieldType:  FieldTypeString,
			Length:     1,
			LengthType: LengthTypeFixed,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Tag",
			FieldType: FieldTypeString,
			StopChars: []uint8{':'},
			StopType:  StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:      "Payload",
			FieldType: FieldTypeRemainder,
			StopChars: DefaultStopChars,
		},
	}

	phoneLabLogLineFields = []*FieldInfo{
		&FieldInfo{
			Name:       "Skip1",
			Skip:       true,
			FieldType:  FieldTypeString,
			StopChars:  DefaultStopChars,
			Length:     40,
			LengthType: LengthTypeFixed,
		},
		&FieldInfo{
			Name:      "Skip2",
			Skip:      true,
			FieldType: FieldTypeString,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Skip3",
			Skip:      true,
			FieldType: FieldTypeString,
			StopChars: DefaultStopChars,
		},

		&FieldInfo{
			Name:       "Boot ID",
			FieldType:  FieldTypeString,
			Length:     36,
			LengthType: LengthTypeFixed,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Token",
			FieldType: FieldTypeInt64,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Tracetime",
			FieldType: FieldTypeFloat64,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Year",
			FieldType:  FieldTypeInt,
			Length:     4,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'-'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Month",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'-'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Day",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Hours",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{':'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Minutes",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{':'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Seconds",
			FieldType:  FieldTypeInt,
			Length:     2,
			LengthType: LengthTypeFixed,
			StopChars:  []uint8{'.'},
			StopType:   StopTypeCharacterExclusive,
		},
		&FieldInfo{
			Name:       "Nanos",
			FieldType:  FieldTypeString,
			Length:     9,
			LengthType: LengthTypeMax,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Pid",
			FieldType: FieldTypeInt32,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Tid",
			FieldType: FieldTypeInt32,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:       "Level",
			FieldType:  FieldTypeString,
			Length:     1,
			LengthType: LengthTypeFixed,
			StopChars:  DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Tag",
			FieldType: FieldTypeString,
			StopChars: DefaultStopChars,
		},
		&FieldInfo{
			Name:      "Payload",
			FieldType: FieldTypeRemainder,
			StopChars: DefaultStopChars,
		},
	}
}
