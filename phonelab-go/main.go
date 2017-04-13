package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func initCommands() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "phoneLab-go",
		Short: "PhoneLab-Go! CLI",
	}

	splitCmd := &cobra.Command{
		Use:     "split <conf_file> <output_dir>",
		Short:   "Split a yaml runner conf into individual source confs",
		Run:     splitCmdRun,
		PreRunE: splitCmdPreRunE,
	}

	splitCmdInitFlags(splitCmd)
	rootCmd.AddCommand(splitCmd)

	return rootCmd
}

func fatalError(err error) {
	fmt.Fprintf(os.Stderr, "Error processing command: %v\n", err)
	os.Exit(1)
}

func main() {

	mainCmd := initCommands()

	if err := mainCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
