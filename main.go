package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"github.com/fatih/color"
	"sync"
	"time"
	"flag"
	"sort"
)

type searchResp struct {
	Source string
	Name string
	Updated string
}

var wg sync.WaitGroup
const dateLayout string = "2006-01-02T15:04:05.000Z"

func main() {

	start := time.Now()

	filename := flag.String("file", "", "Filename to parse in the current dir, i.e. -file=MicroServiceBuild.scala")
	flag.Parse()

	if *filename == "" {
		fmt.Println("Filename must be specified via -file= flag.")
		return
	}

	var errors []string = make([]string, 0, 0)
	var libs chan []string = make(chan []string)

	go getLibraries(*filename, &libs)

	fmt.Printf("|%30s|%10s|%10s|%12s|%8s|\n", "Library", "Current", "Latest", "On", "Update")
	fmt.Printf("|------------------------------|----------|----------|------------|--------|\n")

	wg.Add(1)
	for lib := range libs {
		go getLatestVersion(lib, &errors, &wg)
	}
	wg.Done()
	wg.Wait()

	fmt.Printf("|------------------------------|----------|----------|------------|--------|\n")
	fmt.Printf("\nElapsed:%s\n\nErrors:\n", time.Since(start))
	for i, e := range errors {
		fmt.Printf("[%d] %s\n", i+1, e)
	}
}

func getLibraries(filename string, libs *chan []string){
	r, _ := regexp.Compile("uk.gov.hmrc\".*?%%.*?\"(.*?)\".*?%.*?\"(.*?)\"")

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("Couldn't open file:", err)
	}

	c := string(b)
	m := r.FindAllStringSubmatch(c, -1)
	for _, l := range m {
		*libs <- l
	}
	close(*libs)
}

func getLatestVersion(lib []string, errors *[]string, wg *sync.WaitGroup) {
	wg.Add(1)
	libName := lib[1]
	libCurVersion := lib[2]

	var resp = searchResp{}
	var err error

	resp, err = getFromBintray(libName)
	if err != nil {
		if err.Error() != "[404]" {
			*errors = append(*errors, fmt.Sprintf("%s [%s] - Couldn't get version because of error %v", libName, libCurVersion, err))
		} else {
			resp, err = getFromNexus(libName)
			if err != nil {
				*errors = append(*errors, fmt.Sprintf("%s [%s] - Couldn't get version because of error %v", libName, libCurVersion, err))
			}
		}
	}

	printLine(libName, libCurVersion, resp, len(*errors))

	wg.Done()
}

func getFromBintray(libName string) (searchResp, error) {
	url := fmt.Sprintf("https://api.bintray.com/packages/hmrc/releases/%s/versions/_latest", libName)
	r, err := http.Get(url)
	if err != nil {
		return searchResp{}, err
	}

	if r.StatusCode != http.StatusOK {
		return searchResp{}, fmt.Errorf("[%d]", r.StatusCode)
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return searchResp{}, err
	}

	var resp searchResp
	if err := json.Unmarshal(b, &resp); err != nil {
		return searchResp{}, err
	}

	resp.Source = "Bintray"
	return resp, nil
}

func getFromNexus(libName string) (searchResp, error) {
	url := fmt.Sprintf("https://nexus-dev.tax.service.gov.uk/content/repositories/hmrc-releases/uk/gov/hmrc/%s_2.11/", libName)
	r, err := http.Get(url)
	if err != nil {
		return searchResp{}, err
	}

	if r.StatusCode != http.StatusOK {
		return searchResp{}, fmt.Errorf("[%d]", r.StatusCode)
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return searchResp{}, err
	}

	return parseLatestNexus(b)
}

func parseLatestNexus(body []byte) (searchResp, error) {
	r, _ := regexp.Compile("https:\\/\\/nexus-dev.tax.service.gov.uk\\/content\\/repositories\\/hmrc-releases\\/uk\\/gov\\/hmrc\\/.*?>(.*?)\\/")
	c := string(body)
	m := r.FindAllStringSubmatch(c, -1)

	if len(m) == 0 {
		return searchResp{}, fmt.Errorf("No matches.")
	}

	var versions []string = make([]string, len(m))
	for i, v := range m {
		versions[i] = v[1]
	}
	sort.Strings(versions)
	return searchResp{
		Source: "Nexus",
		Name: versions[len(versions)-1],
	}, nil
}

func printLine(libName string, libCurVersion string, br searchResp, errId int) {

	libLatestVersion := br.Name
	//libLatestUpdate, _ := time.Parse(dateLayout, br.Updated)
	//updFmt := libLatestUpdate.Format("2006-01-02")

	switch {
	case libLatestVersion == "": color.Yellow("|%30s|%10s|%10s|%12s|%8s|\n", libName, libCurVersion, fmt.Sprintf("err[%d]", errId), "", "")
	case libLatestVersion > libCurVersion: color.Red("|%30s|%10s|%10s|%12s|%8s|\n", libName, libCurVersion, libLatestVersion, br.Source, "[*]")
	default: color.Green("|%30s|%10s|%10s|%12s|%8s|\n", libName, libCurVersion, libLatestVersion, br.Source, "")
	}
}