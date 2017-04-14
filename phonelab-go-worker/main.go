package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
)

func initCommands() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "phonelab-go-worker",
		Short: "PhoneLab-Go! Worker Bee",
		Run:   workerCmdRun,
	}

	workerCmdInitFlags(rootCmd)

	return rootCmd
}

func fatalError(err error) {
	fmt.Fprintf(os.Stderr, "Error processing command: %v\n", err)
	os.Exit(1)
}

// Modified from http://nathanleclaire.com/blog/2014/08/24/handling-ctrl-c-interrupt-signal-in-golang-programs/
func waitForSignal() {
	signalChan := make(chan os.Signal, 1)
	doneChan := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)

	go func() {
		for _ = range signalChan {
			fmt.Println("Killing...")
			if worker != nil {
				worker.Stop()
			}
			doneChan <- true
		}
	}()

	<-doneChan
}

func main() {

	mainCmd := initCommands()

	go func() {
		if err := mainCmd.Execute(); err != nil {
			os.Exit(-1)
		} else {
			os.Exit(0)
		}
	}()

	waitForSignal()
}
