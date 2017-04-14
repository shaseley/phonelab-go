package main

import (
	"fmt"
	phonelab "github.com/shaseley/phonelab-go"
	"github.com/spf13/cobra"
	"net/http"
)

var (
	submitConfServer     string
	submitConfPort       int
	submitConfUser       string
	submitConfExperiment string
)

type PhoneLabGoSubmitRequest struct {
	User           string `json:"user"`
	ExperimentName string `json:"name"`
}

func submitCmdInitFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&submitConfServer, "server", "s", "http://localhost", "The job server name or address")
	cmd.Flags().IntVarP(&submitConfPort, "port", "p", 8000, "The job server port to connect to")
	cmd.Flags().StringVar(&submitConfUser, "user", "anon", "Username")
	cmd.Flags().StringVar(&submitConfExperiment, "exp", "anon", "Experiment")
}

func doSubmit(confFile, pluginFile string) error {
	// Make sure the yaml is valid
	if _, err := phonelab.RunnerConfFromFile(confFile); err != nil {
		return fmt.Errorf("Unable to load conf file: %v", err)
	}

	// Now, connect to the job server and push the files

	submitReq := &PhoneLabGoSubmitRequest{
		User:           submitConfUser,
		ExperimentName: submitConfExperiment,
	}

	req := NewPostRequest(submitConfServer, submitConfPort, ApiEndpointSubmit)
	req.SetType(PostTypeMultipart)

	if err := req.QueueJSON(submitReq, "data"); err != nil {
		return err
	}

	req.QueueFile(confFile, "conf")
	req.QueueFile(pluginFile, "plugin")

	resp, body, errs := req.Submit()

	if len(errs) > 0 {
		return fmt.Errorf("Errors uploading files: %v", errs)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Upload failed. Status code: %v Msg: %v", resp.StatusCode, body)
	} else {
		fmt.Printf("Your PhoneLab-Go! job has been submitted!\nDetails: %v\n", body)
		return nil
	}
}

func submitCmdRun(cmd *cobra.Command, args []string) {
	if err := doSubmit(args[0], args[1]); err != nil {
		fatalError(err)
	}
}

func submitCmdPreRunE(cmd *cobra.Command, args []string) error {
	return validateConfAndPluginArgs(args)
}
