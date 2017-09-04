package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/kyokomi/emoji"
	"sync"
	"time"
	"flag"
	"sort"
	"sync/atomic"
)

type searchResp struct {
	Source  string
	Name    string
	Updated string
}

const (
	dateLayout string = "2006-01-02T15:04:05.000Z"
)

var wg sync.WaitGroup
var wgErr sync.WaitGroup
var remove map[string]interface{}
var migration *bool
var counter uint32
var AppVersion string
var Sha string

var bintrayUrl = "https://api.bintray.com/packages/hmrc/releases/%s/versions/_latest"
var nexusUrl = "https://nexus-dev.tax.service.gov.uk/content/repositories/hmrc-releases/uk/gov/hmrc/%s_2.11/"

func main() {

	start := time.Now()

	filename := flag.String("file", "", "Filename to parse in the current dir, i.e. -file=MicroServiceBuild.scala")
	printVersion := flag.Bool("version", false, "Prints the version of this application")
	migration = flag.Bool("migration", false, "If present will highlight libraries to be removed for library upgrade")
	flag.Parse()

	if *printVersion {
		fmt.Printf("Current version is: %s\nGit commit: %s", AppVersion, Sha)
		return
	}

	if *filename == "" {
		fmt.Println("Filename must be specified via -file= flag.")
		return
	}

	var migrationLibs = []string{"http-verbs", "play-auditing", "play-graphite", "play-config", "play-authorisation", "play-authorised-frontend", "play-health", "crypto", "logback-json-logger", "play-json-logger", "govuk-template", "play-ui"}
	remove = make(map[string]interface{})
	for _, l := range migrationLibs {
		remove[l] = nil
	}

	timeout := time.Duration(2 * time.Second)
	httpClient := http.Client{
		Timeout: timeout,
	}

	var errors chan string = make(chan string)
	var libs chan []string = make(chan []string)
	var errs = make([]string, 0, 0)

	go errorProc(&errors, &errs, &wgErr)

	go getLibraries(*filename, &libs)

	fmt.Printf("\n|------------------------------|----------|----------|----------|------------|\n")
	fmt.Printf("|%30s|%10s|%10s|%10s|%12s|\n", "Library", "Current", "Latest", "On", "Updated")
	fmt.Printf("|------------------------------|----------|----------|----------|------------|\n")

	wg.Add(1)
	for lib := range libs {
		go getLatestVersion(&httpClient, lib, &errors, &wg)
	}

	wg.Done()
	wg.Wait()
	close(errors)

	fmt.Printf("|------------------------------|----------|----------|----------|------------|\n")
	printHelp()
	fmt.Printf("\nElapsed:%s\n", time.Since(start))

	wgErr.Wait()
	if len(errs) != 0 {
		fmt.Println("\nErrors:\n")
		for _, e := range errs {
			fmt.Println(e)
		}
	}
}

func errorProc(errCh *chan string, errors *[]string, wg *sync.WaitGroup) {
	wg.Add(1)
	for e := range *errCh {
		*errors = append(*errors, e)
	}
	wg.Done()
}

func getLibraries(filename string, libs *chan []string) {
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

func getLatestVersion(httpClient *http.Client, lib []string, errors *chan string, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	libName := lib[1]
	libCurVersion := lib[2]

	var resp = searchResp{}
	var err error

	var errId uint32

	resp, err = getFromBintray(httpClient, libName)
	if err != nil {
		errId = atomic.AddUint32(&counter, 1)
		if err.Error() == "[404]" {
			resp, err = getFromNexus(httpClient, libName)
		}
		if err != nil {
			*errors <- fmt.Sprintf("[%d] - %s [%s]\n\tCouldn't get version because of error %v", errId, libName, libCurVersion, err)
		}
	}

	printLine(libName, libCurVersion, resp, int(errId))
}

func getFromBintray(httpClient *http.Client, libName string) (searchResp, error) {
	url := fmt.Sprintf(bintrayUrl, libName)
	r, err := httpClient.Get(url)
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

func getFromNexus(httpClient *http.Client, libName string) (searchResp, error) {
	url := fmt.Sprintf(nexusUrl, libName)
	r, err := httpClient.Get(url)
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
	r, _ := regexp.Compile("https:\\/\\/nexus-dev.tax.service.gov.uk\\/content\\/repositories\\/hmrc-releases\\/uk\\/gov\\/hmrc\\/.*?>(.*?)\\/<\\/a>")
	c := string(body)
	m := r.FindAllStringSubmatch(c, -1)

	if len(m) == 0 {
		return searchResp{}, fmt.Errorf("No matches.")
	}

	versions := make([]*version.Version, len(m))
	for i, raw := range m {
		v, _ := version.NewVersion(raw[1])
		versions[i] = v
	}
	sort.Sort(version.Collection(versions))

	return searchResp{
		Source: "Nexus",
		Name:   versions[len(versions)-1].String(),
	}, nil
}

func printLine(libName string, libCurVersion string, resp searchResp, errId int) {

	libLatestVersion := resp.Name
	libLatestUpdate, _ := time.Parse(dateLayout, resp.Updated)
	updFmt := ""
	if resp.Updated != "" {
		updFmt = libLatestUpdate.Format("2006-01-02")
	}

	_, exists := remove[libName]

	lV, _ := version.NewVersion(libLatestVersion)
	cV, _ := version.NewVersion(libCurVersion)

	switch {
	case *migration && exists:
		color.Magenta("|%30s|%10s|%10s|%10s|%12s|\n", libName, libCurVersion, libLatestVersion, resp.Source, updFmt)
	case libLatestVersion == "":
		color.Yellow("|%30s|%10s|%10s|%10s|%12s|\n", libName, libCurVersion, fmt.Sprintf("err[%d]", errId), "", "")
	case cV.LessThan(lV):
		color.Red("|%30s|%10s|%10s|%10s|%12s|\n", libName, libCurVersion, libLatestVersion, resp.Source, updFmt)
	default:
		color.Green("|%30s|%10s|%10s|%10s|%12s|\n", libName, libCurVersion, libLatestVersion, resp.Source, "")
	}
}

func printHelp() {
	fmt.Printf("\nSo colorful! What does it mean?\n")
	color.Green(emoji.Sprint(":smile: Library up to date!"))
	color.Red(emoji.Sprint(":anguished: Auch! Not the latest..."))
	color.Yellow(emoji.Sprint(":disappointed: Something went wrong! VPN issues??? If the library is on Nexus I need to access it..."))
	color.Magenta(emoji.Sprint(":construction: To be removed for upgrade to the latest version of bootstraps"))
}
