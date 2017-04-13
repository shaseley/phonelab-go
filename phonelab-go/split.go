package main

import (
	"errors"
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var splitConfPrefix string
var splitConfIndividial bool

func splitCmdInitFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&splitConfPrefix, "prefix", "p", "out", "File prefix for generated conf files")
	cmd.Flags().BoolVarP(&splitConfIndividial, "individual-files", "i", false, "Store each conf in an individual file")
}

func doSplitSingleFile(conf *phonelab.RunnerConf, outDir string) error {
	origSource := conf.SourceConf
	sources, err := conf.SourceConf.Expand()
	if err != nil {
		return err
	} else if len(sources) == 0 {
		return errors.New("No sources. Nothing to do")
	}

	conf.SourceConf = &phonelab.PipelineSourceConf{
		Type:    origSource.Type,
		Sources: []string{sources[0]},
	}

	confs := []*phonelab.RunnerConf{conf}

	if bytes, err := yaml.Marshal(&confs); err != nil {
		return fmt.Errorf("Error marhaling Yaml: %v", err)
	} else {
		outStr := string(bytes)
		outStr = strings.Replace(outStr, "- ", "- &default\n  ", 1)
		for i := 1; i < len(sources); i++ {
			outStr += fmt.Sprintf(`
- <<: *default
  source:
    type: files
    sources: [%v]
`, sources[i])
		}

		outFile := path.Join(outDir, fmt.Sprintf("conf_%v.yaml", splitConfPrefix))

		if err = ioutil.WriteFile(outFile, []byte(outStr), 0644); err != nil {
			return fmt.Errorf("Error writing file: %v", err)
		}
		return nil

	}
}

func doSplitMultFiles(conf *phonelab.RunnerConf, outDir string) error {
	// Split into sources
	splitConfs, err := conf.ShallowSplit()
	if err != nil {
		return err
	}

	// Persist output
	prefix := "conf_" + splitConfPrefix
	count := 0

	for _, conf := range splitConfs {
		count += 1
		outFile := fmt.Sprintf("%v_%v.yaml", prefix, count)
		outFile = path.Join(outDir, outFile)

		if bytes, err := yaml.Marshal(conf); err != nil {
			return fmt.Errorf("Error marhaling Yaml: %v", err)
		} else if err = ioutil.WriteFile(outFile, bytes, 0644); err != nil {
			return fmt.Errorf("Error writing file: %v", err)
		}
	}

	return nil
}

func doSplitConf(confFile, outDir string) error {
	// Load conf
	conf, err := phonelab.RunnerConfFromFile(confFile)
	if err != nil {
		return err
	}

	if splitConfIndividial {
		return doSplitMultFiles(conf, outDir)
	} else {
		return doSplitSingleFile(conf, outDir)
	}
}

func splitCmdRun(cmd *cobra.Command, args []string) {
	if err := doSplitConf(args[0], args[1]); err != nil {
		fatalError(err)
	}
}

// Validate the arguments
func splitCmdPreRunE(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid command syntax")
	}

	if err := validateFile(args[0], "conf file"); err != nil {
		return err
	}

	if fi, err := os.Stat(args[1]); err != nil {
		return fmt.Errorf("Error stating output directory: %v", err)
	} else if !fi.IsDir() {
		return errors.New("Output directory cannot be a file")
	}

	return nil
}
