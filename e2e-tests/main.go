package main

import (
	"encoding/json"
	goflag "flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/golang/glog"
	//"github.com/kr/pretty"
	"github.com/spf13/cobra"

	"k8s.io/contrib/e2e-tests/e2e"
	"k8s.io/contrib/test-utils/admin"
	"k8s.io/contrib/test-utils/utils"
)

type config struct {
	e2e e2e.E2ETester

	port      int // where to expose API endpoints
	adminPort int

	BlockingJobNames     []string
	WeakBlockingJobNames []string
	NonBlockingJobNames  []string
}

// This serves little purpose other than to show updates every minute in the
// web UI. Stable() will get called as needed against individual PRs as well.
func (c *config) updateTests() {
	for {
		c.e2e.UpdateTests()
		time.Sleep(1 * time.Minute)
	}
}

func marshal(data interface{}) []byte {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		glog.Errorf("Unable to Marshal data: %#v: %v", data, err)
		return nil
	}
	return b
}

func serve(data []byte, res http.ResponseWriter, req *http.Request) {
	if data == nil {
		res.Header().Set("Content-type", "text/plain")
		res.WriteHeader(http.StatusInternalServerError)
	} else {
		res.Header().Set("Content-type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(data)
	}
}

func (c *config) serveBlockingStatus(res http.ResponseWriter, req *http.Request) {

	data := marshal(c.e2e.GetBlockingTestStatus())
	serve(data, res, req)
}

func (c *config) serveNonBlockingStatus(res http.ResponseWriter, req *http.Request) {
	data := marshal(c.e2e.GetNonBlockingTestStatus())
	serve(data, res, req)
}

func zip(f http.HandlerFunc) http.Handler {
	return gziphandler.GzipHandler(f)
}

// Given a string slice with a single empty value this function will return an empty slice.
// This is extremely useful for StringSlice flags, so the user can do --flag="" and instead
// of getting []string{""} they will get []string{}
func cleanStringSlice(in []string) []string {
	if len(in) == 1 && len(in[0]) == 0 {
		return []string{}
	}
	return in
}

// Clean up all of our flags which we wish --flag="" to mean []string{}
func (c *config) cleanStringSlices() {
	c.BlockingJobNames = cleanStringSlice(c.BlockingJobNames)
	c.NonBlockingJobNames = cleanStringSlice(c.NonBlockingJobNames)
	c.WeakBlockingJobNames = cleanStringSlice(c.WeakBlockingJobNames)
}

func (c *config) initializeE2E(overrideUrl string) {
	c.cleanStringSlices()

	var gcs *utils.Utils
	if overrideUrl != "" {
		gcs = utils.NewTestUtils(overrideUrl)
	} else {
		gcs = utils.NewUtils(utils.KubekinsBucket, utils.LogDir)
	}
	realE2E := &e2e.RealE2ETester{
		BlockingJobNames:     c.BlockingJobNames,
		NonBlockingJobNames:  c.NonBlockingJobNames,
		WeakBlockingJobNames: c.WeakBlockingJobNames,
		GoogleGCSBucketUtils: gcs,
	}
	realE2E.Init(admin.Mux)
	c.e2e = realE2E

	if c.adminPort != 0 {
		address := fmt.Sprintf(":%d", c.adminPort)
		go http.ListenAndServe(address, admin.Mux)
	}
	if c.port != 0 {
		address := fmt.Sprintf(":%d", c.port)
		go http.ListenAndServe(address, nil)
	}
	http.Handle("/blocking-status", zip(c.serveBlockingStatus))
	http.Handle("/nonblocking-status", zip(c.serveNonBlockingStatus))
	return
}

func main() {
	config := &config{}
	root := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "A program to watch continuously running e2e tests",
		RunE: func(_ *cobra.Command, _ []string) error {
			config.initializeE2E("")
			go config.updateTests()
			select {} // sleep forever!
			return nil
		},
	}
	root.Flags().StringSliceVar(&config.BlockingJobNames, "blocking-jobs", []string{
		"kubelet-gce-e2e-ci",
		"kubernetes-build",
		"kubernetes-test-go",
		"kubernetes-e2e-gce",
		"kubernetes-e2e-gce-slow",
		"kubernetes-e2e-gce-serial",
		"kubernetes-e2e-gke-serial",
		"kubernetes-e2e-gke",
		"kubernetes-e2e-gke-slow",
		"kubernetes-e2e-gce-scalability",
		"kubernetes-kubemark-5-gce",
	}, "Comma separated list of jobs that should block merges if failing.")
	root.Flags().StringSliceVar(&config.NonBlockingJobNames, "nonblocking-jobs", []string{
		"kubernetes-e2e-gke-staging",
		"kubernetes-e2e-gke-staging-parallel",
		"kubernetes-e2e-gke-subnet",
		"kubernetes-e2e-gke-test",
		"kubernetes-e2e-gce-examples",
	}, "Comma separated list of jobs that should not block merges but which are still watched and managed.")
	root.Flags().StringSliceVar(&config.WeakBlockingJobNames, "weak-blocking-jobs", []string{
		"kubernetes-kubemark-500-gce",
	}, "Comma separated list of jobs that block merges which only require weak success")
	root.Flags().IntVar(&config.port, "port", 8082, "The port to listen on for non-admin API")
	root.Flags().IntVar(&config.adminPort, "admin-port", 9999, "If non-zero, will serve administrative actions on this port.")
	root.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
	root.Execute()
}
