package phonelab

import (
	"fmt"
	"strconv"
)

const (
	FieldTypeString = iota
	FieldTypeInt
	FieldTypeInt32
	FieldTypeInt64
	FieldTypeFloat32
	FieldTypeFloat64
	FieldTypeRemainder
)

const (
	LengthTypeNone = iota
	LengthTypeFixed
	LengthTypeMax
)

const (
	StopTypeWhiteSpace = iota
	StopTypeCharacterInclusive
	StopTypeCharacterExclusive
)

type FieldInfo struct {
	Name       string // The name of the field to be used in the parse map
	Skip       bool
	FieldType  int // The type of field
	Length     int // The fixed or max length of the field
	LengthType int // The method for handling length
	StopChars  []uint8
	StopType   int // The method for field termination
}

var DefaultStopChars = []uint8{' ', '\t'}

type Fielder interface {
	Set(interface{})
	Info() *FieldInfo
}

type StringField struct {
	*FieldInfo
	Destination *string
}

func NewStringField(f *FieldInfo, destination *string) *StringField {
	if f.FieldType != FieldTypeString && f.FieldType != FieldTypeRemainder {
		panic("StringField must be either FieldTypeString or FieldTypeString")
	}

	return &StringField{
		FieldInfo:   f,
		Destination: destination,
	}
}

func (f *StringField) Set(v interface{}) {
	*f.Destination = v.(string)
}

func (f *StringField) Info() *FieldInfo {
	return f.FieldInfo
}

type Int64Field struct {
	*FieldInfo
	Destination *int64
}

func NewInt64Field(f *FieldInfo, destination *int64) *Int64Field {

	f.FieldType = FieldTypeInt64

	return &Int64Field{
		FieldInfo:   f,
		Destination: destination,
	}
}

func (f *Int64Field) Set(v interface{}) {
	*f.Destination = v.(int64)
}

func (f *Int64Field) Info() *FieldInfo {
	return f.FieldInfo
}

type Int32Field struct {
	*FieldInfo
	Destination *int32
}

func NewInt32Field(f *FieldInfo, destination *int32) *Int32Field {
	return &Int32Field{
		FieldInfo:   f,
		Destination: destination,
	}
}

func (f *Int32Field) Set(v interface{}) {
	*f.Destination = v.(int32)
}

func (f *Int32Field) Info() *FieldInfo {
	return f.FieldInfo
}

type IntField struct {
	*FieldInfo
	Destination *int
}

func NewIntField(f *FieldInfo, destination *int) *IntField {
	return &IntField{
		FieldInfo:   f,
		Destination: destination,
	}
}

func (f *IntField) Set(v interface{}) {
	*f.Destination = v.(int)
}

func (f *IntField) Info() *FieldInfo {
	return f.FieldInfo
}

type Float64Field struct {
	*FieldInfo
	Destination *float64
}

func NewFloat64Field(f *FieldInfo, destination *float64) *Float64Field {
	return &Float64Field{
		FieldInfo:   f,
		Destination: destination,
	}
}

func (f *Float64Field) Set(v interface{}) {
	*f.Destination = v.(float64)
}

func (f *Float64Field) Info() *FieldInfo {
	return f.FieldInfo
}

type Float32Field struct {
	*FieldInfo
	Destination *float32
}

func NewFloat32Field(f *FieldInfo, destination *float32) *Float32Field {
	return &Float32Field{
		FieldInfo:   f,
		Destination: destination,
	}
}

func (f *Float32Field) Set(v interface{}) {
	*f.Destination = v.(float32)
}

func (f *Float32Field) Info() *FieldInfo {
	return f.FieldInfo
}

type LineParser struct {
	fields []*FieldInfo
}

func NewLineParser(fields []*FieldInfo) *LineParser {
	return &LineParser{
		fields: fields,
	}
}

func (lp *LineParser) Parse(line string) error {
	lpi := newLineParserImpl(lp.fields, line)
	return lpi.Parse()
}

type lineParserImpl struct {
	pos    int
	length int
	line   string

	fields []*FieldInfo
}

func newLineParserImpl(fields []*FieldInfo, line string) *lineParserImpl {
	return &lineParserImpl{
		pos:    0,
		length: len(line),
		line:   line,
		fields: fields,
	}
}

func (lpi *lineParserImpl) reset(line string, fields []*FieldInfo) {
	lpi.pos = 0
	lpi.length = len(line)
	lpi.line = line
	lpi.fields = fields
}

func (p *lineParserImpl) Parse() error {
	/*
		for _, f := range p.fields {
			if err := p.parseField(f); err != nil {
				return err
			}
		}
	*/
	return nil
}

func inStopList(list []uint8, c uint8) bool {
	for _, sc := range list {
		if sc == c {
			return true
		}
	}
	return false
}

// Skip preliminary whitespace
func (p *lineParserImpl) advance() {
	for p.pos < p.length && inStopList(DefaultStopChars, p.line[p.pos]) {
		p.pos += 1
	}
}

func (p *lineParserImpl) parseField(info *FieldInfo, dest interface{}) error {
	//info := f.Info()

	// Skip leading space and check if we've run off the edge
	p.advance()

	if p.pos >= p.length {
		return fmt.Errorf("Parser Error: Expected field '%v', got EOF", info.Name)
	}

	if info.FieldType == FieldTypeRemainder {
		// We're done.
		//f.Set(p.line[p.pos:])
		*(dest.(*string)) = p.line[p.pos:]
		return nil
	}

	start := p.pos

	// Advance until we get a stop character or the end
	for p.pos < p.length && !inStopList(info.StopChars, p.line[p.pos]) {
		p.pos += 1
	}

	// If we want the stop char, increment the position so we procede in the
	// manner.
	if info.StopType == StopTypeCharacterInclusive {
		p.pos += 1
	}

	fieldStr := p.line[start:p.pos]

	// Check the field length
	switch info.LengthType {
	case LengthTypeFixed:
		if len(fieldStr) != info.Length {
			return fmt.Errorf("Invalid length for field '%v', data='%v'. Expected %v got %v",
				info.Name, fieldStr, info.Length, len(fieldStr))
		}
	case LengthTypeMax:
		if len(fieldStr) > info.Length {
			return fmt.Errorf("Invalid length for field '%v', data='%v'. Expected at most %v got %v",
				info.Name, fieldStr, info.Length, len(fieldStr))
		}
	}

	if info.Skip {
		// Just need to advance the state, but don't acutally need the field.
		return nil
	}

	// Now, convert it into the right type
	switch info.FieldType {
	default:
		{
			return fmt.Errorf("Unsupported field type: '%v'", info.FieldType)
		}
	case FieldTypeString:
		{
			*(dest.(*string)) = fieldStr
		}
	case FieldTypeInt:
		{
			if v, err := strconv.Atoi(fieldStr); err != nil {
				return err
			} else {
				*(dest.(*int)) = v
			}
		}
	case FieldTypeInt32:
		{
			if v, err := strconv.ParseInt(fieldStr, 10, 32); err != nil {
				return err
			} else {
				*(dest.(*int32)) = int32(v)
			}
		}
	case FieldTypeInt64:
		{
			if v, err := strconv.ParseInt(fieldStr, 10, 64); err != nil {
				return err
			} else {
				*(dest.(*int64)) = v
			}
		}
	case FieldTypeFloat32:
		{
			if v, err := strconv.ParseFloat(fieldStr, 32); err != nil {
				return err
			} else {
				*(dest.(*float32)) = float32(v)
			}
		}
	case FieldTypeFloat64:
		{
			if v, err := strconv.ParseFloat(fieldStr, 64); err != nil {
				return err
			} else {
				*(dest.(*float64)) = v
			}
		}
	}

	// Go past the stop character, it shouldn't be part of the next field
	if info.StopType == StopTypeCharacterExclusive {
		p.pos += 1
	}

	return nil
}
