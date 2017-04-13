package main

import (
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

func runnerConfsFromString(text string) ([]*phonelab.RunnerConf, error) {
	var confs []*phonelab.RunnerConf

	err := yaml.Unmarshal([]byte(text), &confs)

	if err != nil {
		return nil, err
	}

	return confs, nil
}

func runnerConfsFromFile(file string) ([]*phonelab.RunnerConf, error) {
	var err error

	if _, err = os.Stat(file); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading file %v: %v", file, err)
	}

	return runnerConfsFromString(string(data))
}

func validateFile(file, desc string) error {
	if fi, err := os.Stat(file); err != nil {
		return fmt.Errorf("Error stating %v: %v", desc, err)
	} else if fi.IsDir() {
		return fmt.Errorf("%v cannot be a directory", desc)
	}
	return nil
}
