package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kr/beanstalk"
	"github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

var logger = log.New(os.Stderr, "phonelab-go-worker", log.LstdFlags)

type PhoneLabWorker struct {
	Server          string
	Port            int
	BeanstalkServer string
	BeanstalkPort   int
	MaxJobs         int

	tempDir string

	sync.Mutex
}

var worker *PhoneLabWorker

func NewPhoneLabWorker() *PhoneLabWorker {
	return &PhoneLabWorker{
		Server:          workerConfServer,
		Port:            workerConfPort,
		BeanstalkServer: workerConfBeanstalkServer,
		BeanstalkPort:   workerConfBeanstalkPort,
		MaxJobs:         workerConfMaxJobs,
	}
}

type BeanstalkJob struct {
	MetaId int64  `json:"meta_id"`
	Index  int    `json:"index"`
	User   string `json:"user"`
	Name   string `json:"name"`
}

var (
	workerConfServer          string
	workerConfPort            int
	workerConfBeanstalkPort   int
	workerConfBeanstalkServer string
	workerConfMaxJobs         int
)

func workerCmdInitFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&workerConfServer, "server", "s", "http://localhost", "The job server name or address")
	cmd.Flags().IntVarP(&workerConfPort, "port", "p", 8000, "The job server port to connect to")
	cmd.Flags().StringVar(&workerConfBeanstalkServer, "beanstalk-server", "localhost", "The beanstalkd server name or address")
	cmd.Flags().IntVar(&workerConfBeanstalkPort, "beanstalk-port", 14000, "The beanstalkd port to use for jobs")
	cmd.Flags().IntVar(&workerConfMaxJobs, "max-jobs", 4, "The maximum number of concurrent jobs")
}

// Entry point for phonelab-go-server
func workerCmdRun(cmd *cobra.Command, args []string) {
	worker = NewPhoneLabWorker()
	worker.Start()
}

// Kick off the worker
func (w *PhoneLabWorker) Start() {

	if d, err := ioutil.TempDir("", "phonelab-go-worker"); err != nil {
		panic(fmt.Sprintf("Cannot create temp dir: %v", err))
	} else {
		w.tempDir = d
		logger.Printf("Temp dir created: %v\n", w.tempDir)
	}

	// Run
	w.mainLoop()
}

func (w *PhoneLabWorker) mainLoop() {

	sem := make(chan int, w.MaxJobs)
	id := int64(0)

	for {
		sem <- 1
		id += 1

		// TODO: shut it down after N errors

		// Get a job, run it
		go func(wid int64) {
			if err := w.runOneJob(wid); err != nil {
				logger.Printf("Error running job: %v\n", err)
			}
			<-sem
		}(id)
	}
}

func (w *PhoneLabWorker) runOneJob(id int64) error {
	conn, err := beanstalk.Dial("tcp", fmt.Sprintf("%v:%v", w.BeanstalkServer, w.BeanstalkPort))
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to beanstalk: %v", err))
	}

	// Get all tubes
	tubes, err := conn.ListTubes()
	if err != nil {
		logger.Fatalf("Error listing tubes: '%v'. Shutting down.\n", err)
	}

	// Get a job from beanstalk from one of the tubes
	logger.Printf("Worker %v retrieving job...\n", id)

	ts := beanstalk.NewTubeSet(conn, tubes...)
	bid, body, err := ts.Reserve(10 * time.Hour)
	if err != nil {
		return err
	}

	logger.Printf("Worker %v starting job %v...\n", id, bid)

	var job BeanstalkJob
	if err = json.Unmarshal(body, &job); err != nil {
		return err
	}

	// Download resources
	logger.Printf("Worker %v downloading conf file...\n", id)
	ep := fmt.Sprintf("%v:%v/conf/%v/%v", w.Server, w.Port, job.MetaId, job.Index)

	confFile := path.Join(w.tempDir, fmt.Sprintf("conf_%v_%v.yaml", job.MetaId, job.Index))

	if err = downloadFileBase(ep, confFile, 0644, false); err != nil {
		return err
	}
	//defer os.Remove(confFile)

	logger.Printf("Worker %v downloading plugin file...\n", id)

	ep = fmt.Sprintf("%v:%v/plugin/%v", w.Server, w.Port, job.MetaId)
	pluginFile := path.Join(w.tempDir, fmt.Sprintf("plugin_%v_%v.yaml", job.MetaId, job.Index))

	if err = downloadFileBase(ep, pluginFile, 0744, true); err != nil {
		return err
	}
	//defer os.Remove(pluginFile)

	// Execute it
	// TODO: Actually run experiment

	logger.Printf("Worker %v attempting delete job %v...\n", id, bid)

	// Done
	conn.Delete(bid)

	logger.Printf("Worker %v attempting delete conf file on the server...\n", id)
	// best effort delete job files on server
	ep = fmt.Sprintf("%v:%v/conf/%v/%v", w.Server, w.Port, job.MetaId, job.Index)
	gorequest.New().Delete(ep).End()

	logger.Printf("Worker %v done!\n", id)

	return nil
}

// Download a single file from the server and unmarshal the JSON mody into obj.
func downloadFileBase(endpoint, dest string, mode os.FileMode, isBase64 bool) error {

	resp, body, errs := gorequest.New().Get(endpoint).End()

	if errs != nil {
		return errors.New(fmt.Sprintf("%v", errs))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unable to retrieve remote targets: %v", resp.Status)
	}

	var err error
	var payload []byte

	if isBase64 {
		payload, err = base64.StdEncoding.DecodeString(body)
	} else {
		payload = []byte(body)
	}

	if err == nil {
		err = ioutil.WriteFile(dest, payload, mode)
	}

	return err
}

// Stop is called when we're being killed. Clean up and free any resources.
func (w *PhoneLabWorker) Stop() {
}
