package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGetFromBintray(t *testing.T) {

	ts := startBintrayTestServer()
	defer ts.Close()
	bintrayURL = ts.URL + "/%s"

	r, err := getFromBintray(http.DefaultClient, "exampleLib1")

	if err != nil {
		t.Errorf("getFromBintray returned an error: %v", err)
	}
	assert.Equal(t, "0.14.0", r.Name)
}

func TestGetFromBintray_404(t *testing.T) {

	ts := startBintrayTestServer()
	defer ts.Close()
	bintrayURL = ts.URL + "/%s"

	_, err := getFromBintray(http.DefaultClient, "libNotFound")

	assert.EqualError(t, err, "[404]")
}

func TestGetFromBintray_unmarshalErr(t *testing.T) {

	ts := startBintrayTestServer()
	defer ts.Close()
	bintrayURL = ts.URL + "/%s"

	_, err := getFromBintray(http.DefaultClient, "nilBody")

	assert.Error(t, err)
}

func TestGetFromNexus(t *testing.T) {

	ts := startNexusTestServer()
	defer ts.Close()
	nexusURL = ts.URL + "/%s"

	r, err := getFromNexus(http.DefaultClient, "exampleLib2")

	if err != nil {
		t.Errorf("getFromNexus returned an error: %v", err)
	}
	assert.Equal(t, "5.14.0", r.Name)
}

func TestGetFromNexus_404(t *testing.T) {

	ts := startNexusTestServer()
	defer ts.Close()
	nexusURL = ts.URL + "/%s"

	_, err := getFromNexus(http.DefaultClient, "libNotFound")

	assert.EqualError(t, err, "[404]")
}

func TestGetFromNexus_empty(t *testing.T) {

	ts := startNexusTestServer()
	defer ts.Close()
	nexusURL = ts.URL + "/%s"

	_, err := getFromNexus(http.DefaultClient, "nilBody")

	assert.Error(t, err)
}

func TestGetLatestVersion(t *testing.T) {
	f := false
	migration = &f

	bts := startBintrayTestServer()
	defer bts.Close()
	bintrayURL = bts.URL + "/%s"

	nts := startNexusTestServer()
	defer nts.Close()
	nexusURL = nts.URL + "/%s"

	err := make(chan string)
	go func() {
		for _ = range err {
		}
	}()

	var wg sync.WaitGroup
	getLatestVersion(http.DefaultClient, []string{"exampleLib:0.0.0", "exampleLib1", "0.0.0"}, &err, &wg)
	getLatestVersion(http.DefaultClient, []string{"exampleLib:0.0.0", "exampleLib2", "0.0.0"}, &err, &wg)
	getLatestVersion(http.DefaultClient, []string{"exampleLib:0.0.0", "libNotFound", "0.0.0"}, &err, &wg)
	getLatestVersion(http.DefaultClient, []string{"exampleLib:0.0.0", "nilBody", "0.0.0"}, &err, &wg)

	wg.Wait()
}

func startBintrayTestServer() *httptest.Server {
	localResp, _ := ioutil.ReadFile("test-data/bintray-resp.json")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "exampleLib1"):
			fmt.Fprintln(w, string(localResp))
		case strings.Contains(r.URL.Path, "nilBody"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, nil)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "")
		}
	}))

	return ts
}

func startNexusTestServer() *httptest.Server {
	localResp, _ := ioutil.ReadFile("test-data/nexus-resp.html")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch {
		case strings.Contains(r.URL.Path, "exampleLib2"):
			fmt.Fprintln(w, string(localResp))
		case strings.Contains(r.URL.Path, "nilBody"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, nil)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "")
		}
	}))

	return ts
}
