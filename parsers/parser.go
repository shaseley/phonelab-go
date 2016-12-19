package phonelab

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Tags we know how to handle
const (
	TAG_PRINTK           = "KernelPrintk"
	TAG_TRACE            = "Kernel-Trace"
	TAG_PL_POWER_BATTERY = "Power-Battery-PhoneLab"
)

// This is for subparsers, though the top-level logline parser also
// kind of implements this interface.
type Parser interface {
	Parse(payload string) (interface{}, error)
}

type ParserController interface {
	SetParser(tag string, p Parser)
	ClearParser(tag string)
}

type LoglineParser struct {
	LogcatParser    *LogcatParser
	TagParsers      map[string]Parser
	ErrOnUnknownTag bool
}

func NewLoglineParser() *LoglineParser {
	parser := &LoglineParser{
		LogcatParser:    NewLogcatParser(),
		TagParsers:      make(map[string]Parser),
		ErrOnUnknownTag: false,
	}

	return parser
}

func (pc *LoglineParser) SetParser(tag string, p Parser) {
	pc.TagParsers[tag] = p
}

func (pc *LoglineParser) ClearParser(tag string) {
	delete(pc.TagParsers, tag)
}

func (pc *LoglineParser) AddKnownTags() {
	pkparser := NewPrintkParser()
	pkparser.ErrOnUnknownTag = false
	pc.SetParser(TAG_PRINTK, pkparser)

	tparser := NewKernelTraceParser()
	tparser.ErrOnUnknownTag = false
	pc.SetParser(TAG_TRACE, tparser)

	pc.SetParser(TAG_PL_POWER_BATTERY, NewPLPowerBatteryParser())
}

// For the logline parser, the payload is the whole log line
func (pc *LoglineParser) Parse(line string) (interface{}, error) {

	var ll *Logline

	if obj, err := pc.LogcatParser.Parse(line); err != nil {
		return obj, err
	} else {
		//ll = obj.(*Logline)
		ll = obj
	}

	// Do we have a payload parser?
	if parser, ok := pc.TagParsers[ll.Tag]; ok {
		// Yes
		payload := ll.Payload.(string)
		if obj, err := parser.Parse(payload); err != nil {
			return ll, err
		} else {
			ll.Payload = obj
			return ll, nil
		}
	} else if pc.ErrOnUnknownTag {
		// No, and we should
		return nil, fmt.Errorf("No tag parser for tag '%v'", ll.Tag)
	} else {
		// No subparser, just return the logline with the unparsed payload.
		return ll, nil
	}
}

// Helper function to unpack values from a map into a structure.
func UnpackLogcatEntry(dest interface{}, values map[string]string) error {

	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("Expected a ptr type, but got '%v'", val.Kind())
	}

	elem := val.Elem()
	tp := elem.Type()
	n := elem.NumField()

	var key string

	for i := 0; i < n; i++ {
		f := elem.Field(i)
		bits := 32
		if tag := tp.Field(i).Tag.Get("logcat"); len(tag) > 0 {
			key = tag
			if key == "-" {
				continue
			}
		} else {
			key = strings.ToLower(tp.Field(i).Name)
		}

		if v, ok := values[key]; ok && len(v) > 0 {
			// We have a value and where to put it
			//fmt.Println(key, v, f.Kind())
			switch f.Kind() {
			case reflect.String:
				f.SetString(v)
			case reflect.Int64:
				bits = 64
				fallthrough
			case reflect.Int:
				fallthrough
			case reflect.Int32:
				if tmp, err := strconv.ParseInt(v, 10, bits); err != nil {
					return err
				} else {
					f.SetInt(int64(tmp))
				}
			case reflect.Uint64:
				bits = 64
				fallthrough
			case reflect.Uint:
				fallthrough
			case reflect.Uint32:
				if tmp, err := strconv.ParseUint(v, 10, bits); err != nil {
					return err
				} else {
					f.SetUint(uint64(tmp))
				}
			case reflect.Float64:
				bits = 64
				fallthrough
			case reflect.Float32:
				if tmp, err := strconv.ParseFloat(v, bits); err != nil {
					return err
				} else {
					f.SetFloat(float64(tmp))
				}
			default:
				return fmt.Errorf("Unsupported kind: %v", f.Kind())
			}
		}
	}

	return nil
}

// Helper function to create a map from two slices of the same size.
// This panics if the slices have different lengths.
func zipToDict(keys []string, vals []string) map[string]string {
	if len(keys) != len(vals) {
		panic("len(keys) != len(vals)")
	}

	m := map[string]string{}
	for i, value := range vals {
		m[keys[i]] = value
	}
	return m
}

// Parse source based on the regex and populate dest using the field names
// and refletion.
func unpackFromRegex(source string, re *regexp.Regexp, dest interface{}) (map[string]string, error) {
	keys := re.SubexpNames()
	values := re.FindAllStringSubmatch(source, -1)

	if len(values) == 0 {
		return nil, fmt.Errorf("The regex failed to parse the source text")
	}

	m := zipToDict(keys, values[0])
	return m, UnpackLogcatEntry(dest, m)
}

///////////////////////////////////////////////////////////////////////////////
// A parser that populates a struct based on a regex with names.

type RegexParserProps interface {
	// Create a new default object to populate. The result must be a pointer.
	New() interface{}

	// The regex to use to populate the object.
	Regex() *regexp.Regexp
}

type RegexParser struct {
	Props   RegexParserProps
	LastMap map[string]string
}

func NewRegexParser(props RegexParserProps) *RegexParser {
	return &RegexParser{
		Props: props,
	}
}

func (p *RegexParser) Parse(line string) (interface{}, error) {
	obj := p.Props.New()
	m, err := unpackFromRegex(line, p.Props.Regex(), obj)
	p.LastMap = m
	return obj, err
}

///////////////////////////////////////////////////////////////////////////////
// A parser that handles multiple regexes

type MultRegexParserProps interface {
	// Create a new default object to populate. The result must be a pointer.
	New() interface{}

	// The regex to use to populate the object.
	Regex() []*regexp.Regexp
}

type MultRegexParser struct {
	Props        MultRegexParserProps
	LastMap      map[string]string
	startPattern int
}

func NewMultRegexParser(props MultRegexParserProps) *MultRegexParser {
	return &MultRegexParser{
		Props:        props,
		startPattern: 0,
	}
}

func (p *MultRegexParser) Parse(line string) (interface{}, error) {

	var err error
	var m map[string]string
	// Assuming this parser is reused for a long stream, and all of the logs
	// within the same stream have the same format, this will learn the format
	// after one log. This has a huge performance benefit, and safes the developer
	// the burden of reordering the patterns.
	i := p.startPattern
	patterns := p.Props.Regex()

	for {
		obj := p.Props.New()
		if m, err = unpackFromRegex(line, patterns[i], obj); err == nil {
			p.startPattern = i
			p.LastMap = m
			return obj, err
		}

		// Advance and wrap, checking if we're back at the start
		if i = (i + 1) % len(patterns); i == p.startPattern {
			break
		}
	}

	return nil, err
}

///////////////////////////////////////////////////////////////////////////////
// JSON parser. This simply unmarshals a JSON object into and object created
// with the New() function.

type JSONParserProps interface {
	New() interface{}
}

type JSONParser struct {
	Props JSONParserProps
}

func NewJSONParser(props JSONParserProps) *JSONParser {
	return &JSONParser{
		Props: props,
	}
}

func (p *JSONParser) Parse(line string) (interface{}, error) {
	obj := p.Props.New()
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, err
	} else {
		return obj, nil
	}
}
