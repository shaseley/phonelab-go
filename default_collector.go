package phonelab

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/shaseley/phonelab-go/serialize"
)

// DefaultCollector is a DataCollector that passes data to a configured
// serialize.Serializer. The serializer can be configured by the builder through
// yaml arguments.
type DefaultCollector struct {
	// Base path or URL of the output destination. The final filename will include
	// contextual information.
	Path string
	// Whether or not the output should be compressed, if possible
	Compressed bool
	// Whether or not data should be aggregated and sent as a list in OnFinish()
	AggregateData bool
	// The serializer to use for serializing data.
	Serializer serialize.Serializer

	firstContext string
	allData      []interface{}
}

// Create and return a new DefaultDataCollector from generic args.
func NewDefaultCollector(args map[string]interface{}) (DataCollector, error) {
	// Path (required)
	pathOrUrl := ""
	if v, ok := args["path"]; ok {
		if pathOrUrl, ok = v.(string); !ok {
			return nil, fmt.Errorf("Unexpected type for 'compressed'. Expected bool, got %t", v)
		}
	} else {
		return nil, errors.New("Missing 'path' argument. A path is required for the default collector")
	}

	// HDFS path (required for hdfs://)
	hdfsAddr := ""
	if v, ok := args["hdfs_addr"]; ok {
		if hdfsAddr, ok = v.(string); !ok {
			return nil, fmt.Errorf("Unexpected type for 'hdfs_addr'. Expected string, got %t", v)
		}
	}

	compressed := false

	for _, s := range []string{"compress", "compressed"} {
		if v, ok := args[s]; ok {
			if compressed, ok = v.(bool); !ok {
				return nil, fmt.Errorf("Unexpected type for 'compressed'. Expected bool, got %t", v)
			}
		}
	}

	aggregate := false
	if v, ok := args["aggregate"]; ok {
		if aggregate, ok = v.(bool); !ok {
			return nil, fmt.Errorf("Unexpected type for 'aggregate'. Expected bool, got %t", v)
		}
	}

	var serializer serialize.Serializer

	if strings.HasPrefix(pathOrUrl, "http://") || strings.HasPrefix(pathOrUrl, "https://") {
		serializer = &serialize.HTTPSerializer{}
	} else if strings.HasPrefix(pathOrUrl, "hdfs://") {
		if len(hdfsAddr) == 0 {
			return nil, errors.New("Missing 'hdfs_addr' argument. An HDFS address is required hdfs://")
		}
		serializer = serialize.NewHDFSSerializer(hdfsAddr)
	} else {
		if strings.HasPrefix(pathOrUrl, "file://") {
			pathOrUrl = pathOrUrl[7:]
		}
		if len(pathOrUrl) == 0 {
			return nil, errors.New("Invalid path")
		}
		serializer = &serialize.LocalSerializer{}
	}

	return &DefaultCollector{
		Path:          pathOrUrl,
		Compressed:    compressed,
		Serializer:    serializer,
		AggregateData: aggregate,
		allData:       make([]interface{}, 0),
	}, nil
}

func (dc *DefaultCollector) makeOutPath(context string) string {
	// We start with a base path or URL. Tack on the context.

	context = strings.Replace(context, "/", "_", -1)

	outPath := path.Join(dc.Path, context)

	// Tack on the file type
	if dc.Compressed {
		outPath += ".gz"
	} else {
		outPath += ".json"
	}

	return outPath
}

func (dc *DefaultCollector) OnData(data interface{}, info PipelineSourceInfo) {
	if dc.AggregateData {
		// Just save it for later
		dc.allData = append(dc.allData, data)
		if len(dc.firstContext) == 0 {
			dc.firstContext = info.Context()
		}
	} else {
		// Persist now.
		// FIXME: Can we use a goroutine here so we don't block the pipeline
		outPath := dc.makeOutPath(info.Context())
		if err := dc.Serializer.Serialize(data, outPath); err != nil {
			fmt.Println("Error serializing data:", err)
		}
	}
}

func (dc *DefaultCollector) Finish() {
	if dc.AggregateData {
		// Serialize the whole list
		outPath := dc.makeOutPath(dc.firstContext)
		if err := dc.Serializer.Serialize(dc.allData, outPath); err != nil {
			fmt.Println("Error serializing all data:", err)
		}
	}
}
