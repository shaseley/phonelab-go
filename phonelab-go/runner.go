package main

import (
	"errors"
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
	"github.com/spf13/cobra"
	"plugin"
)

func runCmdInitFlags(cmd *cobra.Command) {

}

func getPluginInitFunc(file string) (plugin.Symbol, error) {
	if p, err := plugin.Open(file); err != nil {
		return nil, fmt.Errorf("Unable to open plugin: %v", err)
	} else if initFunc, err := p.Lookup("InitEnv"); err != nil {
		return nil, errors.New("Unable to find InitEnv() in plugin")
	} else {
		return initFunc, nil
	}
}

func doRun(confFile, pluginFile string) error {
	// Load conf
	conf, err := phonelab.RunnerConfFromFile(confFile)
	if err != nil {
		return err
	}

	// Find the InitEnv function
	initFunc, err := getPluginInitFunc(pluginFile)
	if err != nil {
		return err
	}

	// Create and initialize runner environment
	env := phonelab.NewEnvironment()
	initFunc.(func(*phonelab.Environment))(env)

	// Create runner
	runner, err := conf.ToRunner(env)
	if err != nil {
		return fmt.Errorf("Error creating runner: %v\n", err)
	}

	// Run experiment
	if errs := runner.Run(); len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}

	return nil
}

func runCmdRun(cmd *cobra.Command, args []string) {
	if err := doRun(args[0], args[1]); err != nil {
		fatalError(err)
	}
}

func runCmdPreRunE(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid command syntax")
	}

	if err := validateFile(args[0], "conf file"); err != nil {
		return err
	} else if _, err := getPluginInitFunc(args[1]); err != nil {
		return err
	}

	p, err := plugin.Open(args[1])
	if err != nil {
		return fmt.Errorf("Unable to open plugin: %v", err)
	}

	if _, err = p.Lookup("InitEnv"); err != nil {
		return errors.New("Unable to find InitEnv() in plugin")
	}

	return nil
}
