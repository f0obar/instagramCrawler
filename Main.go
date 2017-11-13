package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
	"log"
	"os"
	"io"
	"strconv"
	"time"
)

func main() {
	fmt.Println("Crawler starting")

	config := getConfig()
	interval := -1

	if num, err := strconv.Atoi(config[0]); err == nil {
		interval = num
		fmt.Println("Setting repeat interval to " + config[0] + " seconds")
		config = append(config[:0], config[1:]...)
	}
	fmt.Println("Found " + strconv.Itoa(len(config)) + " Accounts to crawl")


	if interval > 0 {
		t := time.NewTicker(time.Duration(interval)*time.Second)
		for {
			fmt.Println(">>CRAWLING<<")
			for _, element := range config {
				crawl(element)
			}
			fmt.Println(">>CRAWLING FINISHED, next crawling will start in " + strconv.Itoa(interval) + " seconds<<")
			<-t.C
		}
	} else {
		for _, element := range config {
			crawl(element)
		}
		fmt.Println("Crawler finished")
	}
}

func getConfig()(accounts []string)  {
	b, err := ioutil.ReadFile("config.txt")
	if err != nil {
		fmt.Print("Couldn't read config.txt")
		panic(err)
	}
	for _, element := range strings.Split(string(b),",") {
		accounts = append(accounts, element)
	}
	return accounts
}

func crawl(url string)  {
	fmt.Println("Crawling: " + url)
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
			pic := strings.TrimLeft(element," " + `"` + "display_src: " + `"`)
			pic = strings.TrimRight(pic,`"`)
			archive(pic,strings.Split(url,"/")[3])
		}
	}
}

func archive(pictureurl string, username string)  {
	if !alreadySaved(username + "/" + strings.Split(pictureurl,"/")[len(strings.Split(pictureurl,"/")) - 1]) {
		save(pictureurl, username)
	}
}

func save(pictureurl string, username string)  {
	fmt.Println("Saving new Image " + pictureurl)
	response, e := http.Get(pictureurl)
	if e != nil {
		log.Fatal(e)
	}
	defer response.Body.Close()

	//create directory
	if _, err := os.Stat(username); os.IsNotExist(err) {
		err = os.MkdirAll(username, 0777)
		if err != nil {
			panic(err)
		}
	}

	//open a file for writing
	file, err := os.Create(username + "/" + strings.Split(pictureurl,"/")[len(strings.Split(pictureurl,"/")) - 1])
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
	file.Close()
}

func alreadySaved(fullpath string)(exists bool)  {
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		return false
	}
	return true
}
