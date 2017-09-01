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
)

type bintrayResp struct {
	Name string
}

var wg sync.WaitGroup

func main() {

	start := time.Now()

	var errors []string = make([]string, 0, 0)

	fmt.Printf("|%30s|%10s|%10s|%8s|\n", "Library", "Current", "Latest", "Update")
	fmt.Printf("|------------------------------|----------|----------|--------|\n")

	var libs chan []string = make(chan []string)

	go func(){
		r, _ := regexp.Compile("uk.gov.hmrc\".*?%%.*?\"(.*?)\".*?%.*?\"(.*?)\"")

		b, err := ioutil.ReadFile("./MicroServiceBuild.scala")
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

			switch {
			case gr.Name == "": color.Yellow("|%30s|%10s|%10s|%8s|\n", libName, libCurVersion, fmt.Sprintf("err[%d]", len(errors)), "")
			case gr.Name > lib[2]: color.Red("|%30s|%10s|%10s|%8s|\n", libName, libCurVersion, gr.Name, "[*]")
			default: color.Green("|%30s|%10s|%10s|%8s|\n", libName, libCurVersion, gr.Name, "")
			}

			wg.Done()
		}(lib, &wg)
	}

	wg.Wait()

	fmt.Printf("\nElapsed:%s\n\nErrors:\n", time.Since(start))
	for i, e := range errors {
		fmt.Printf("[%d] %s", i+1, e)
	}
}
