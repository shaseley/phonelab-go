package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/kr/beanstalk"
	"github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var logger = log.New(os.Stderr, "phonelab-go-worker", log.LstdFlags)

type PhoneLabWorkerManager struct {
	Server          string
	Port            int
	BeanstalkServer string
	BeanstalkPort   int
	MaxJobs         int

	tempDir string

	sync.Mutex
}

// The global work manager
var workerManager *PhoneLabWorkerManager

func NewPhoneLabWorkerManager() *PhoneLabWorkerManager {
	return &PhoneLabWorkerManager{
		Server:          workerConfServer,
		Port:            workerConfPort,
		BeanstalkServer: workerConfBeanstalkServer,
		BeanstalkPort:   workerConfBeanstalkPort,
		MaxJobs:         workerConfMaxJobs,
	}
}

// An individual worker thread.
type PhoneLabWorker struct {
	Id      int
	mgr     *PhoneLabWorkerManager
	server  string
	conn    *beanstalk.Conn
	tempDir string
}

func NewPhoneLabWorker(mgr *PhoneLabWorkerManager, id int) *PhoneLabWorker {
	return &PhoneLabWorker{
		Id:      id,
		server:  fmt.Sprintf("%v:%v", mgr.Server, mgr.Port),
		tempDir: mgr.tempDir,
		mgr:     mgr,
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
	workerManager = NewPhoneLabWorkerManager()
	workerManager.Start()
}

const (
	beanstalkConnectSleepS = 10
	beanstalkMaxRetries    = -1
)

func (w *PhoneLabWorker) connectToBeanstalk() error {
	attempt := 0

	for beanstalkMaxRetries < 0 || attempt < beanstalkMaxRetries {
		if w.conn != nil {
			// This will probably error, but just in case the connection is in a weird state
			w.conn.Close()
		}

		attempt += 1
		logger.Printf("Worker %v connecting to beanstalk...\n", w.Id)

		c, err := beanstalk.Dial("tcp", fmt.Sprintf("%v:%v", w.mgr.BeanstalkServer, w.mgr.BeanstalkPort))
		if err != nil {
			logger.Printf("Worker %v unable to connect to beanstalk: %v. Trying again in %v sec.\n", w.Id, err, beanstalkConnectSleepS)
		} else {
			logger.Printf("Worker %v connected to beanstalk\n", w.Id)
			w.conn = c
			return nil
		}
		time.Sleep(beanstalkConnectSleepS * time.Second)
	}

	return errors.New("Timed out connecting to beanstalk")
}

// Kick off the worker
func (w *PhoneLabWorkerManager) Start() {

	if d, err := ioutil.TempDir("", "phonelab-go-worker"); err != nil {
		panic(fmt.Sprintf("Cannot create temp dir: %v", err))
	} else {
		w.tempDir = d
		logger.Printf("Temp dir created: %v\n", w.tempDir)
	}

	// Run
	w.mainLoop()
}

func (mgr *PhoneLabWorkerManager) mainLoop() {
	// TODO: Allow MaxJobs to change

	workers := make([]*PhoneLabWorker, 0, mgr.MaxJobs)
	for i := 0; i < mgr.MaxJobs; i++ {
		workers = append(workers, NewPhoneLabWorker(mgr, i+1))
	}

	done := make(chan int)

	for _, w := range workers {
		go func(worker *PhoneLabWorker) {
			if err := worker.connectToBeanstalk(); err != nil {
				// Timeout
				panic(err)
			}

			for {
				if err := worker.runOneJob(); err != nil {
					logger.Printf("Error running job: %v\n", err)
				}
			}
		}(w)
	}

	// Block forever
	<-done
}

func (w *PhoneLabWorker) runOneJob() error {
	var bid uint64
	var body []byte
	var err error

	id := w.Id

	// We loop here because we don't know when a new tube will be added.
	// We keep the timeout relatively short and poll for tube changes.
	// This would be far better if beanstalk allowed us to reserve a job on
	// any tube.

	// FIXME: Ideally, one goroutine would poll for jobs which would allow
	// the easy addition of latency to try and ensure that different
	// clients pick up jobs without saturating one client. Instead, this
	// tries to achieve the same thing with least code change
	w.mgr.Lock()
	time.Sleep(time.Duration((1 + rand.Int31n(5))) * time.Second)
	for {
		// Get all tubes
		tubes, err := w.conn.ListTubes()
		if err != nil {
			logger.Printf("Error listing tubes: '%v'. Reconnecting...\n", err)
			if err = w.connectToBeanstalk(); err != nil {
				w.mgr.Unlock()
				return err
			}
			// The connection has been re-established, start the loop again
			continue
		}

		// Get a job from beanstalk from one of the tubes
		logger.Printf("Worker %v retrieving job...\n", id)

		ts := beanstalk.NewTubeSet(w.conn, tubes...)

		bid, body, err = ts.Reserve(30 * time.Second)
		if err != nil {
			continue
		}
		break
	}
	w.mgr.Unlock()

	logger.Printf("Worker %v starting job %v...\n", id, bid)

	var job BeanstalkJob
	if err = json.Unmarshal(body, &job); err != nil {
		return err
	}

	// Download resources
	logger.Printf("Worker %v downloading conf file...\n", id)
	ep := fmt.Sprintf("%v/conf/%v/%v", w.server, job.MetaId, job.Index)

	confFile := path.Join(w.tempDir, fmt.Sprintf("conf_%v_%v.yaml", job.MetaId, job.Index))

	if err = downloadFileBase(ep, confFile, 0644, false); err != nil {
		return err
	}
	defer os.Remove(confFile)

	logger.Printf("Worker %v downloading plugin file...\n", id)

	ep = fmt.Sprintf("%v/plugin/%v", w.server, job.MetaId)
	pluginFile := path.Join(w.tempDir, fmt.Sprintf("plugin_%v_%v.yaml", job.MetaId, job.Index))

	if err = downloadFileBase(ep, pluginFile, 0744, true); err != nil {
		return err
	}
	defer os.Remove(pluginFile)

	// Execute it
	if err = w.execPhoneLabGo(confFile, pluginFile); err != nil {
		logger.Printf("worker %v error running phonelab-go: %v\n", id, err)
	}

	logger.Printf("Worker %v attempting delete job %v...\n", id, bid)

	// Done
	w.conn.Delete(bid)

	logger.Printf("Worker %v attempting delete conf file on the server...\n", id)
	// best effort delete job files on server
	ep = fmt.Sprintf("%v/conf/%v/%v", w.server, job.MetaId, job.Index)
	gorequest.New().Delete(ep).End()

	logger.Printf("Worker %v done!\n", id)

	return nil
}

func (w *PhoneLabWorker) execPhoneLabGo(confFile, pluginFile string) error {
	// TODO: What to do with the output?
	// Ideally, we'd send this back to the server.

	cmd := exec.Command("phonelab-go", "run", confFile, pluginFile)
	output, err := cmd.CombinedOutput()

	logger.Printf("Command Output:\n%v", string(output))
	return err
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
func (w *PhoneLabWorkerManager) Stop() {
	os.RemoveAll(w.tempDir)
}
