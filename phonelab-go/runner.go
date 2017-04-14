package main

import (
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
	"github.com/spf13/cobra"
)

func runCmdInitFlags(cmd *cobra.Command) {}

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
	return validateConfAndPluginArgs(args)
}
