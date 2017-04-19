package phonelab

import (
	"errors"
)

type DefaultCollector struct {
}

func NewDefaultCollector(args map[string]interface{}) (DataCollector, error) {
	tp, ok := args["type"]
	if !ok {
		return nil, errors.New("Missing required 'type' argument")
	}

	switch tp {
	default:
		return nil,
			errors.New("Currently, DefaultCollector only supports 'http', 'hdfs', and 'file' types")
	case "http":
	case "hdfs":
	case "file":
	}

	return nil, nil
}

func (dc *DefaultCollector) OnData(data interface{}, info PipelineSourceInfo) {

}

func (dc *DefaultCollector) Finish() {

}
