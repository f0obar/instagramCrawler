package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
)
var accounts []string

func main() {
	accounts = append(accounts, "https://www.instagram.com/elonmusk/")
	fmt.Println("Crawler starting")

	for _, element := range accounts {
		crawl(element)
	}
}

func crawl(url string)  {
	resp, err := http.Get(url)
	defer resp.Body.Close()

	if err != nil {
		panic(err)
	}
	html, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	str := fmt.Sprintf("%s", html)
	chunks := strings.Split(str,",")
	for _, element := range chunks {
		if(strings.HasPrefix(element," " + `"` + "display_src")) {
			url := strings.TrimLeft(element," " + `"` + "display_src: " + `"`)
			url = strings.TrimRight(url,`"`)
			fmt.Println(url)
		}
	}
}

func save(url string)  {

}
