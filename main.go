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
)

type bintrayResp struct {
	Name string
	Updated string
}

var wg sync.WaitGroup

func main() {

	dateLayout := "2006-01-02T15:04:05.000Z"
	start := time.Now()

	filename := flag.String("file", "", "Filename to parse in the current dir, i.e. -file=MicroServiceBuild.scala")
	flag.Parse()

	if *filename == "" {
		fmt.Println("Filename must be specified via -file= flag.")
		return
	}

	var errors []string = make([]string, 0, 0)
	var libs chan []string = make(chan []string)

	go func(){
		r, _ := regexp.Compile("uk.gov.hmrc\".*?%%.*?\"(.*?)\".*?%.*?\"(.*?)\"")

		b, err := ioutil.ReadFile(*filename)
		if err != nil {
			fmt.Println("Couldn't open file:", err)
		}

		c := string(b)
		m := r.FindAllStringSubmatch(c, -1)
		for _, l := range m {
			libs <- l
		}
		close(libs)
	}()

	fmt.Printf("|%30s|%10s|%10s|%12s|%8s|\n", "Library", "Current", "Latest", "On", "Update")
	fmt.Printf("|------------------------------|----------|----------|------------|--------|\n")

	wg.Add(1)
	for lib := range libs {
		go func(lib []string, wg *sync.WaitGroup) {
			wg.Add(1)
			libName := lib[1]
			libCurVersion := lib[2]

			url := fmt.Sprintf("https://api.bintray.com/packages/hmrc/releases/%s/versions/_latest", libName)
			r, err := http.Get(url)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s [%s] - Couldn't get url: %s because of error [%v]", libName, libCurVersion, url, err))
			}
			if r.StatusCode != http.StatusOK {
				errors = append(errors, fmt.Sprintf("%s [%s] - Couldn't get url: %s because of error [%s]", libName, libCurVersion, url, r.Status))
			}

			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s [%s] - Couldn't read body of response from: %s because of error [%v]", libName, libCurVersion, url, err))
			}
			var gr bintrayResp
			if err := json.Unmarshal(b, &gr); err != nil {
				errors = append(errors, fmt.Sprintln("Error while unmarshalling", string(b), err))
			}

			upd, _ := time.Parse(dateLayout, gr.Updated)

			switch {
			case gr.Name == "": color.Yellow("|%30s|%10s|%10s|%12s|%8s|\n", libName, libCurVersion, fmt.Sprintf("err[%d]", len(errors)), "", "")
			case gr.Name > lib[2]: color.Red("|%30s|%10s|%10s|%12s|%8s|\n", libName, libCurVersion, gr.Name, upd.Format("2006-01-02"), "[*]")
			default: color.Green("|%30s|%10s|%10s|%12s|%8s|\n", libName, libCurVersion, gr.Name, upd.Format("2006-01-02"), "")
			}

			wg.Done()
		}(lib, &wg)
	}

	wg.Done()
	wg.Wait()

	fmt.Printf("|------------------------------|----------|----------|------------|--------|\n")
	fmt.Printf("\nElapsed:%s\n\nErrors:\n", time.Since(start))
	for i, e := range errors {
		fmt.Printf("[%d] %s\n", i+1, e)
	}
}