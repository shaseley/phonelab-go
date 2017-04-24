package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kr/beanstalk"
	phonelab "github.com/shaseley/phonelab-go"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

var logger = log.New(os.Stderr, "phonelab-go-server", log.LstdFlags)

// Details about the non-split experiment
type metaJob struct {
	Id   int64
	Dir  string
	User string
	Name string
}

func (job *metaJob) tubeName() string {
	return fmt.Sprintf("%v_%v", job.User, job.Name)
}

type SubmissionServer struct {
	Port            int
	BeanstalkPort   int
	BeanstalkServer string
	JobDir          string

	conn      *beanstalk.Conn
	isTempDir bool
	nextId    int64

	sync.Mutex
}

var server *SubmissionServer

func NewSubmissionServer(port, beanstalkPort int, jobDir, beanstalkServer string) *SubmissionServer {
	return &SubmissionServer{
		Port:            port,
		BeanstalkPort:   beanstalkPort,
		BeanstalkServer: beanstalkServer,
		JobDir:          jobDir,
		nextId:          1,
	}
}

var (
	submitConfPort          int
	submitConfBeanstalkPort int
	submitConfJobDir        string
)

func serverCmdInitFlags(cmd *cobra.Command) {
	cmd.Flags().IntVarP(&submitConfPort, "port", "p", 8000, "The job server port to listen on")
	cmd.Flags().IntVarP(&submitConfBeanstalkPort, "beanstalk", "b", 14000, "The beanstalkd port to use for jobs")
	cmd.Flags().StringVarP(&submitConfJobDir, "jobdir", "j", "", "The directory to store job details. By default, this is stored in /tmp")
}

// Entry point for phonelab-go-server
func serverCmdRun(cmd *cobra.Command, args []string) {
	server = NewSubmissionServer(submitConfPort, submitConfBeanstalkPort, submitConfJobDir, "localhost")
	server.Start()
}

// Kick off the server
func (s *SubmissionServer) Start() {
	// Init
	if len(s.JobDir) == 0 {
		var err error
		s.JobDir, err = ioutil.TempDir("", "phonelab-go")
		if err != nil {
			panic(err)
		}
		s.isTempDir = true
	}

	if c, err := beanstalk.Dial("tcp", fmt.Sprintf("%v:%v", s.BeanstalkServer, s.BeanstalkPort)); err != nil {
		panic(fmt.Sprintf("Unable to connect to beanstalk: %v", err))
	} else {
		s.conn = c
	}

	// Run
	logger.Printf("Starting PhoneLab-Go! server on port %v\n", s.Port)
	logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", s.Port), NewRouter()))
}

// Stop is called when we're being killed. Clean up and free any resources.
func (s *SubmissionServer) Stop() {
	if s.isTempDir {
		os.RemoveAll(s.JobDir)
	}
	s.conn.Close()
}

// Find the next available id and create the directory for the files.
func (s *SubmissionServer) prepareJob(user, name string) (*metaJob, error) {
	s.Lock()
	defer s.Unlock()

	for {
		id := s.nextId
		s.nextId += 1
		d := path.Join(s.JobDir, fmt.Sprintf("%v", id))

		if _, err := os.Stat(d); err == nil {
			logger.Println("Skipping directory", d)
			continue
		}

		if err := os.Mkdir(d, 0775); err != nil {
			logger.Printf("Error creating directory %v: %v\n", d, err)
			return nil, err
		}

		return &metaJob{
			Id:   id,
			Dir:  d,
			User: user,
			Name: name,
		}, nil
	}
}

func (s *SubmissionServer) genFileName(prefix, ext string) string {
	s.Lock()
	defer s.Unlock()

	nanos := time.Now().UnixNano()
	return path.Join(s.JobDir, fmt.Sprintf("%v_%v.%v", prefix, nanos, ext))
}

type BeanstalkJob struct {
	MetaId int64  `json:"meta_id"`
	Index  int    `json:"index"`
	User   string `json:"user"`
	Name   string `json:"name"`
}

// Split a full job into individual jobs and queue on the beanstalk server
func (s *SubmissionServer) QueueJob(job *metaJob) (int, error) {
	confFile := path.Join(job.Dir, "conf.yaml")
	conf, err := phonelab.RunnerConfFromFile(confFile)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Split into sources
	logger.Println("Spitting conf file...")
	splitConfs, err := conf.ShallowSplit()
	if err != nil {
		return http.StatusBadRequest, err
	}
	logger.Printf("Conf file split into %v files\n", len(splitConfs))

	// Persist output
	count := 0

	for _, conf := range splitConfs {
		count += 1
		outFile := path.Join(path.Dir(confFile), fmt.Sprintf("conf_%v.yaml", count))

		if bytes, err := yaml.Marshal(conf); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("Error marhaling Yaml: %v", err)
		} else if err = ioutil.WriteFile(outFile, bytes, 0644); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("Error writing file: %v", err)
		}
	}

	// Set up the experiment tube. We only need to separate for the UI.
	tubeName := job.tubeName()
	logger.Printf("Adding %v jobs to tube %v\n", count, tubeName)
	tube := &beanstalk.Tube{s.conn, tubeName}

	// Add all individual jobs to beanstalk
	for i := 0; i < count; i++ {
		newJob := &BeanstalkJob{
			MetaId: job.Id,
			Index:  i + 1,
			User:   job.User,
			Name:   job.Name,
		}

		if bytes, err := json.Marshal(newJob); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("Json Marshal Error: %v", err)
		} else if _, err = tube.Put(bytes, 1, 0, time.Second*60*60*8); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("Beanstalk Queue Error: %v", err)
		}
	}

	return http.StatusOK, nil
}

////////////////////////////////////////////////////////////////////////////////
// API

const (
	JsonHeader = "application/json; charset=UTF-8"
	TextHeader = "text/plain; charset=UTF-8"
)

func sendErrorCode(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", TextHeader)
	w.WriteHeader(code)
	fmt.Fprintf(w, "%v", err)
}

const (
	ConfFileName   = "conf.yaml"
	PluginFileName = "plugin.so"
)

// POST /submit
func routeSubmit(w http.ResponseWriter, r *http.Request) {

	metaDetails := struct {
		User string `json:"user"`
		Name string `json:"name"`
	}{"", ""}

	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		logger.Printf("Parse error. Status: %v, Error: %v\n", http.StatusBadRequest, err)
		sendErrorCode(w, http.StatusBadRequest, errors.New("Parse error"))
		return
	}

	// Get the meta-details
	if data, ok := r.MultipartForm.Value["data"]; ok {
		json.Unmarshal([]byte(data[0]), &metaDetails)
	}
	if len(metaDetails.User) == 0 || len(metaDetails.Name) == 0 {
		sendErrorCode(w, http.StatusBadRequest, errors.New("Expected user and experiment names"))
		return
	}

	logger.Printf("User: %v, Name: %v\n", metaDetails.User, metaDetails.Name)

	job, err := server.prepareJob(metaDetails.User, metaDetails.Name)
	if err != nil {
		logger.Printf("Prepare error. Status: %v, Error: %v\n", http.StatusInternalServerError, err)
		sendErrorCode(w, http.StatusInternalServerError, errors.New("Download error"))
		return
	}

	confFile := path.Join(job.Dir, ConfFileName)
	pluginFile := path.Join(job.Dir, PluginFileName)

	if status, err := downloadFile("conf", confFile, r); err != nil {
		logger.Printf("Download error. Status: %v, Error: %v\n", status, err)
		sendErrorCode(w, status, errors.New("Download error"))
		return
	}

	if status, err := downloadFile("plugin", pluginFile, r); err != nil {
		logger.Printf("Download error. Status: %v, Error: %v\n", status, err)
		sendErrorCode(w, status, errors.New("Download error"))
		return
	}

	if status, err := server.QueueJob(job); err != nil {
		logger.Printf("Download error. Status: %v, Error: %v\n", status, err)
		sendErrorCode(w, status, err)
		return
	}

	// Send success, with a message
	sendErrorCode(w, http.StatusOK,
		fmt.Errorf("Success, your experiment has been queued in the tube '%v'", job.tubeName()))
}

// GET /conf{id}/{index}
func routeConf(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fmt.Println(vars)

	id := vars["meta_id"]
	index := vars["bean_id"]
	dest := path.Join(server.JobDir, id, fmt.Sprintf("conf_%v.yaml", index))

	if _, err := os.Stat(dest); err != nil {
		sendErrorCode(w, http.StatusNotFound, err)
		return
	}

	bytes, err := ioutil.ReadFile(dest)
	if err != nil {
		sendErrorCode(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", TextHeader)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%v", string(bytes))
}

// DELETE /conf{id}/{index}
func routeDeleteConf(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	id := vars["meta_id"]
	index := vars["bean_id"]

	root := path.Join(server.JobDir, id)
	dest := path.Join(root, fmt.Sprintf("conf_%v.yaml", index))

	if _, err := os.Stat(dest); err != nil {
		sendErrorCode(w, http.StatusNotFound, err)
		return
	}

	os.Remove(dest)

	// If the directory is empty, except for the conf and plugin, we can remove
	// it.
	files, err := ioutil.ReadDir(root)
	if err == nil {
		// (ReadDir sorts files by name)
		if len(files) == 2 && files[0].Name() == ConfFileName && files[1].Name() == PluginFileName {
			os.RemoveAll(root)
		}
	} else {
		logger.Println("Error reading job dir:", err)
	}

	w.WriteHeader(http.StatusOK)
}

// GET /plugin/{id}
func routePlugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["meta_id"]
	dest := path.Join(server.JobDir, id, PluginFileName)
	logger.Println("plugin dest:", dest)

	if _, err := os.Stat(dest); err != nil {
		sendErrorCode(w, http.StatusNotFound, err)
		return
	}

	bytes, err := ioutil.ReadFile(dest)
	if err != nil {
		sendErrorCode(w, http.StatusInternalServerError, err)
		return
	}

	pluginStr := base64.StdEncoding.EncodeToString(bytes)

	w.Header().Set("Content-Type", TextHeader)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%v", pluginStr)
}

// Download an individual file
func downloadFile(key, outFileName string, r *http.Request) (int, error) {
	if file, _, err := r.FormFile(key); err != nil {
		return http.StatusBadRequest, err
	} else {
		defer file.Close()

		if f, err := os.OpenFile(outFileName, os.O_WRONLY|os.O_CREATE, 0744); err != nil {
			return http.StatusInternalServerError, err
		} else {

			defer f.Close()

			if _, err = io.Copy(f, file); err != nil {
				return http.StatusInternalServerError, err
			}

			return http.StatusOK, nil
		}
	}
}
