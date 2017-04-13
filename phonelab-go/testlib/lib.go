package main

import (
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
)

type testProcessor struct {
	file  string
	count int
}

func (t *testProcessor) Handle(log interface{}) interface{} {
	t.count += 1
	return nil
}

func (t *testProcessor) Finish() {
	fmt.Printf("%v has %v lines\n", t.file, t.count)
}

type testGen struct{}

func (tg *testGen) GenerateProcessor(source *phonelab.PipelineSourceInstance,
	kwargs map[string]interface{}) phonelab.Processor {

	return phonelab.NewSimpleProcessor(source.Processor,
		&testProcessor{
			file:  source.Info["file_name"].(string),
			count: 0,
		})
}

func InitEnv(env *phonelab.Environment) {
	env.Processors["test"] = &testGen{}
}
