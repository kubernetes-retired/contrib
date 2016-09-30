/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// The base URL for GCS's HTTP API.
const gcsBaseURL = "https://storage.googleapis.com"
const gcsPath = "/gcs" // path for GCS browsing on this server

var flPort = flag.Int("p", 80, "port number on which to listen")

type strslice []string

// String prints the strlice as a string.
func (ss *strslice) String() string {
	return fmt.Sprintf("%v", *ss)
}

// Set appends a value onto the strslice.
func (ss *strslice) Set(value string) error {
	*ss = append(*ss, value)
	return nil
}

// Only buckets in this list will be served.
var allowedBuckets strslice

func main() {
	flag.Var(&allowedBuckets, "bucket", "GCS bucket to serve (may be specified more than once)")
	flag.Parse()

	log.Printf("starting")
	rand.Seed(time.Now().UTC().UnixNano())

	// Canonicalize allowed buckets.
	for i := range allowedBuckets {
		allowedBuckets[i] = joinPath(gcsPath, allowedBuckets[i])
		log.Printf("allowing %s", allowedBuckets[i])
		http.HandleFunc(allowedBuckets[i]+"/", gcsRequest)
		http.HandleFunc(allowedBuckets[i], func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, allowedBuckets[i]+"/", http.StatusMovedPermanently)
		})
	}

	// Serve HTTP.
	http.HandleFunc("/", otherRequest)
	log.Printf("serving on port %d", *flPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *flPort), nil))
}

func otherRequest(w http.ResponseWriter, r *http.Request) {
	newTxnLogger(r)
	http.NotFound(w, r)
}

func gcsRequest(w http.ResponseWriter, r *http.Request) {
	logger := newTxnLogger(r)

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// e.g. "/gcs/bucket/path/to/object" -> "/bucket/path/to/object"
	path := strings.TrimPrefix(r.URL.Path, gcsPath)
	// e.g. "/bucket/path/to/object" -> ["bucket", "path/to/object"]
	bucket, object := splitBucketObject(path)

	url := joinPath(gcsBaseURL, bucket)
	url += "?delimiter=/"

	if object != "" {
		// Adding the last slash forces the server to give me a clue about
		// whether the object is a file or a dir.  If it is a dir, the
		// contents will include a record for itself.  If it is a file it
		// will not.
		url += "&prefix=" + object + "/"
	}

	if markers, found := r.URL.Query()["marker"]; found {
		url += "&marker=" + markers[0]
	}

	resp, err := http.Get(url)
	if err != nil {
		logger.Printf("GET %s: %s", url, err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "http.Get: %v", err)
		return
	}
	defer resp.Body.Close()

	logger.Printf("GET %s: %s", url, resp.Status)

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("ioutil.ReadAll: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "ioutil.ReadAll: %v", err)
		return
	}
	dir, err := parseXML(body, object+"/")
	if err != nil {
		logger.Printf("xml.Unmarshal: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "xml.Unmarshal: %v", err)
		return
	}
	if dir == nil {
		// It was a request for a file, send them there directly.
		url := joinPath(gcsBaseURL, bucket, object)
		logger.Printf("redirect to %s", url)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return
	}
	dir.Render(w, path)
}

// splitBucketObject breaks a path into the first part (the bucket), and
// everything else (the object).
func splitBucketObject(path string) (string, string) {
	path = strings.Trim(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// joinPath joins a set of path elements, but does not remove duplicate /
// characters, making it URL-safe.
func joinPath(paths ...string) string {
	return strings.Join(paths, "/")
}

// dirname returns the logical parent directory of the path.  This is different
// than path.Split() in that we want dirname("foo/bar/") -> "foo/", whereas
// path.Split() returns "foo/bar/".
func dirname(path string) string {
	leading := ""
	if strings.HasPrefix(path, "/") {
		leading = "/"
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 1 {
		return leading + strings.Join(parts[0:len(parts)-1], "/") + "/"
	}
	return leading
}

// parseXML extracts a gcsDir object from XML.  If this returns a nil gcsDir,
// the XML indicated that this was not a directory at all.
func parseXML(body []byte, object string) (*gcsDir, error) {
	dir := new(gcsDir)
	if err := xml.Unmarshal(body, &dir); err != nil {
		return nil, err
	}
	// We think this is a dir if the object is "/" (just the bucket) or if we
	// find any Contents or CommonPrefixes.
	isDir := object == "/" || len(dir.Contents)+len(dir.CommonPrefixes) > 0
	selfIndex := -1
	for i := range dir.Contents {
		rec := &dir.Contents[i]
		name := strings.TrimPrefix(rec.Name, object)
		if name == "" {
			selfIndex = i
			continue
		}
		rec.Name = name
		if strings.HasSuffix(name, "/") {
			rec.isDir = true
		}
	}

	for i := range dir.CommonPrefixes {
		cp := &dir.CommonPrefixes[i]
		cp.Prefix = strings.TrimPrefix(cp.Prefix, object)
	}

	if !isDir {
		return nil, nil
	}

	if selfIndex >= 0 {
		// Strip out the record that indicates this object.
		dir.Contents = append(dir.Contents[:selfIndex], dir.Contents[selfIndex+1:]...)
	}
	return dir, nil
}

// gcsDir represents a bucket in GCS, decoded from XML.
type gcsDir struct {
	XMLName        xml.Name `xml:"ListBucketResult"`
	Name           string   `xml:"Name"`
	Prefix         string   `xml:"Prefix"`
	Marker         string   `xml:"Marker"`
	NextMarker     string   `xml:"NextMarker"`
	Contents       []Record `xml:"Contents"`
	CommonPrefixes []Prefix `xml:"CommonPrefixes"`
}

const header string = `<!doctype html>
	<html>
	<head>
	<link rel="stylesheet" href="http://yui.yahooapis.com/pure/0.6.0/pure-min.css">
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>GCS browser</title>
	</head>
	<body>`
const footer string = `</body></html>`

func gridItem(url, name, size, modified string) string {
	return fmt.Sprintf(`<li class="pure-g">
		<div class="pure-u-1-3"><a href="%s">%s</a></div>
		<div class="pure-u-1-3">%s</div>
		<div class="pure-u-1-3">%s</div>
		</li>`,
		url, name, size, modified)
}

// Render writes HTML representing this gcsDir to the provided output.
func (dir *gcsDir) Render(out http.ResponseWriter, inPath string) {
	fmt.Fprintf(out, header)

	fmt.Fprintf(out, `<header style="margin-left:10px;">`)
	fmt.Fprintf(out, "<h1>%s</h1>\n", dir.Name)
	fmt.Fprintf(out, "<h3>%s</h3>\n", inPath)
	fmt.Fprintf(out, "</header>")

	fmt.Fprintf(out, "<ul>\n")
	var nextButton string
	if dir.NextMarker != "" {
		nextButton = fmt.Sprintf(`<a href="%s?marker=%s" class="pure-button" style="margin: 10px 0;">Next page</a>`+"\n", gcsPath+inPath, dir.NextMarker)
		fmt.Fprintf(out, nextButton)
	}
	fmt.Fprintf(out, `<li class="pure-g">`+
		`<div class="pure-u-1-3">Name</div>`+
		`<div class="pure-u-1-3">Size</div>`+
		`<div class="pure-u-1-3">Modified</div>`+
		"</li>")
	if parent := dirname(inPath); parent != "" {
		url := gcsPath + parent
		fmt.Fprintf(out, gridItem(url, "..", "-", "-"))
	}

	for i := range dir.CommonPrefixes {
		dir.CommonPrefixes[i].Render(out, inPath)
	}
	for i := range dir.Contents {
		dir.Contents[i].Render(out, inPath)
	}
	if dir.NextMarker != "" {
		fmt.Fprintf(out, nextButton)
	}
	fmt.Fprintf(out, "</ul>\n")
	fmt.Fprintf(out, footer)
}

// Record represents a single "Contents" entry in a GCS bucket.
type Record struct {
	Name  string `xml:"Key"`
	MTime string `xml:"LastModified"`
	Size  int64  `xml:"Size"`
	isDir bool
}

// Render writes HTML representing this Record to the provided output.
func (rec *Record) Render(out http.ResponseWriter, inPath string) {
	mtime := "<unknown>"
	ts, err := time.Parse(time.RFC3339, rec.MTime)
	if err == nil {
		mtime = ts.Format("02 Jan 2006 15:04:05")
	}
	var url, size string
	if rec.isDir {
		url = gcsPath + inPath + rec.Name
		size = "-"
	} else {
		url = gcsBaseURL + inPath + rec.Name
		size = fmt.Sprintf("%v", rec.Size)
	}
	fmt.Fprintf(out, gridItem(url, rec.Name, size, mtime))
}

// Prefix represents a single "CommonPrefixes" entry in a GCS bucket.
type Prefix struct {
	Prefix string `xml:"Prefix"`
}

// Render writes HTML representing this Prefix to the provided output.
func (pfx *Prefix) Render(out http.ResponseWriter, inPath string) {
	url := gcsPath + inPath + pfx.Prefix
	fmt.Fprintf(out, gridItem(url, pfx.Prefix, "-", "-"))
}

// A logger-wrapper that logs a transaction's metadata.
type txnLogger struct {
	nonce string
}

// Printf logs a formatted line to the logging output.
func (tl txnLogger) Printf(fmt string, args ...interface{}) {
	args = append([]interface{}{tl.nonce}, args...)
	log.Printf("[txn-%s] "+fmt, args...)
}

func newTxnLogger(r *http.Request) txnLogger {
	nonce := fmt.Sprintf("%08x", rand.Int31())
	logger := txnLogger{nonce}
	logger.Printf("request: %s %s", r.Method, r.URL.Path)
	return logger
}
