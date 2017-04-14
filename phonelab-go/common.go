package main

import (
	"errors"
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"plugin"
)

const PluginInitFuncName = "InitEnv"

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

// Get the InitEnv() Symbol in the plugin (.so) file.
func getPluginInitFunc(file string) (plugin.Symbol, error) {
	if p, err := plugin.Open(file); err != nil {
		return nil, fmt.Errorf("Unable to open plugin: %v", err)
	} else if initFunc, err := p.Lookup(PluginInitFuncName); err != nil {
		return nil, errors.New("Unable to find InitEnv() in plugin")
	} else {
		return initFunc, nil
	}
}

// Validate args of the form <conf_file> <plugin>
func validateConfAndPluginArgs(args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid command syntax")
	}

	if err := validateFile(args[0], "conf file"); err != nil {
		return err
	} else if _, err := getPluginInitFunc(args[1]); err != nil {
		return err
	}

	return nil
}
