package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
	"log"
	"os"
	"strconv"
	"time"
	"sync"
	"io"
	"regexp"
)
var waitGroup sync.WaitGroup
var mutex sync.Mutex

var allAccounts []string
var madness = false

func main() {
	fmt.Println("Crawler starting")

	config := getConfig()
	interval := -1
	pagesToCrawlBack := 0

	if num, err := strconv.Atoi(config[0]); err == nil {
		interval = num
		fmt.Println("Setting repeat interval to " + config[0] + " seconds")
		config = append(config[:0], config[1:]...)
	} else if strings.HasPrefix(config[0],"P") {
		pagesToCrawlBack, err = strconv.Atoi(strings.TrimLeft(config[0],"P"))
		if(err != nil){
			panic(err)
		}
		if pagesToCrawlBack == -2 {
			madness = true
		}
		fmt.Println("Crawlback mode: " + config[0] + " Pages")
		config = append(config[:0], config[1:]...)
	}
	fmt.Println("Found " + strconv.Itoa(len(config)) + " Accounts to crawl")
	allAccounts = config

	if interval > 0 {
		t := time.NewTicker(time.Duration(interval)*time.Second)
		for {
			fmt.Println(">>CRAWLING<<")
			for _, element := range config {
				go crawl(element,pagesToCrawlBack)
			}
			waitGroup.Wait()
			fmt.Println(">>CRAWLING FINISHED, next crawling will start in " + strconv.Itoa(interval) + " seconds<<")
			<-t.C
		}
	} else {
		for _, element := range config {
			go crawl(element,pagesToCrawlBack)
		}
		time.Sleep(100)
		waitGroup.Wait()
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

func crawl(url string, pages int)  {
	waitGroup.Add(1)
	defer waitGroup.Done()

	//create directory
	if _, err := os.Stat(strings.Split(url,"/")[3]); os.IsNotExist(err) {
		err = os.MkdirAll(strings.Split(url,"/")[3], 0777)
		if err != nil {
			panic(err)
		}
	}


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

	nextPageId := ""
	crawlSubImages := false
	idCount := 0


	for _, element := range chunks {
		/*
		When media preview is null the image actually is a gallery
		 */
		if strings.HasPrefix(element," " + `"` + "media_preview") {
			mediaType := strings.TrimLeft(element," " + `"` + "media_preview" + `"` + ": ")
			if mediaType == "null"{
				crawlSubImages = true
			}
		}
		/*
		Code tag for gallery id to crawl containing images
		 */
		if strings.HasPrefix(element," " + `"` + "code") && crawlSubImages {
			crawlSubImages = false
			id := strings.TrimLeft(element," " + `"` + "code" + `"` + ": " + `"`)
			id = strings.TrimRight(id,`"`)
			go crawlSub(id,strings.Split(url,"/")[3])
		}
		/*
		Simple image, save directly.
		 */
		if strings.HasPrefix(element," " + `"` + "display_src") && crawlSubImages == false{
			pic := strings.TrimLeft(element," " + `"` + "display_src: " + `"`)
			pic = strings.TrimRight(pic,`"`)
			go archive(pic,strings.Split(url,"/")[3])
		}
		/*
		Tag for crawling older page
		 */
		if strings.HasPrefix(element," " + `"` + "id"){
			nextPageId = strings.TrimLeft(element," " + `"` + "id" + `"` + ": " + `"`)
			nextPageId = strings.TrimRight(nextPageId,`"`)
			idCount++
		}
		/**
		madness mode
		 */
		 if madness {
			 if strings.HasPrefix(element," " + `"` + "caption") && strings.Contains(element,"@"){
				 description := strings.TrimLeft(element," " + `"` + "caption" + `"` + ": ")
				 description = strings.TrimRight(description,`"`)
				 possibleAccs := strings.Split(description, " ")

				 for _, acc := range possibleAccs {
				 	if strings.HasPrefix(acc,"@"){
						reg, err := regexp.Compile("[^a-zA-Z0-9_]+")
						if err != nil {
							log.Fatal(err)
						}
						processedString := reg.ReplaceAllString(acc, "")
						if len(processedString) > 0 {
							go addNewlyCralwedAccount(strings.TrimLeft(processedString, "@"))
						}
					}
				 }
			 }
		 }
	}
	if(pages != 0 && idCount == 13){
		if !strings.HasSuffix(url,"/"){
			//need to trim max_id off
			url = strings.TrimRight(url,strings.Split(url,"/")[len(strings.Split(url,"/")) -1])
		}

		url+= "?max_id="
		url+= nextPageId

		pages--
		crawl(url,pages)
	}
}


func crawlSub(id string, username string){
	waitGroup.Add(1)
	defer waitGroup.Done()
	resp, err := http.Get("https://www.instagram.com/p/" + id)
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
		if strings.HasPrefix(element," " + `"` + "display_url") {
			pic := strings.TrimLeft(element," " + `"` + "display_url: " + `"`)
			pic = strings.TrimRight(pic,`"`)
			go archive(pic,username)
		}
	}
}


func archive(pictureurl string, username string)  {
	waitGroup.Add(1)
	defer waitGroup.Done()
	if !alreadySaved(username + "/" + strings.Split(pictureurl,"/")[len(strings.Split(pictureurl,"/")) - 1]) {
		save(pictureurl, username)
	}
}

func save(pictureurl string, username string) {
	response, e := http.Get(pictureurl)
	if e != nil {
		log.Fatal(e)
	}
	defer response.Body.Close()

	file, err := os.Create(username + "/" + strings.Split(pictureurl, "/")[len(strings.Split(pictureurl, "/"))-1])
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
}

func alreadySaved(fullpath string)(exists bool)  {
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		return false
	}
	return true
}

func addNewlyCralwedAccount(acc string)  {
	waitGroup.Add(1)
	mutex.Lock()
	if !contains(allAccounts,acc){
		fmt.Println("New Account Found:",acc)
		//Write it to file
		f, _ := os.OpenFile("config.txt", os.O_APPEND|os.O_WRONLY, 0777)
		f.WriteString(",https://www.instagram.com/"+acc+"/")
		f.Close()
		//add it to array
		allAccounts = append(allAccounts,acc)
		//start new routine
		//go crawl("",-2)
	}
	mutex.Unlock()
	waitGroup.Done()
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
