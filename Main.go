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
		fmt.Println("Crawlback mode: " + config[0] + " Pages")
		config = append(config[:0], config[1:]...)
	}
	fmt.Println("Found " + strconv.Itoa(len(config)) + " Accounts to crawl")


	if interval > 0 {
		t := time.NewTicker(time.Duration(interval)*time.Second)
		for {
			fmt.Println(">>CRAWLING<<")
			for _, element := range config {
				crawl(element,pagesToCrawlBack)
			}
			fmt.Println(">>CRAWLING FINISHED, next crawling will start in " + strconv.Itoa(interval) + " seconds<<")
			<-t.C
		}
	} else {
		for _, element := range config {
			crawl(element,pagesToCrawlBack)
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

func crawl(url string, pages int)  {
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

	nextPageId := ""
	crawlSubImages := false

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
			crawlSub(id,strings.Split(url,"/")[3])
		}
		/*
		Simple image, save directly.
		 */
		if strings.HasPrefix(element," " + `"` + "display_src") && crawlSubImages == false{
			pic := strings.TrimLeft(element," " + `"` + "display_src: " + `"`)
			pic = strings.TrimRight(pic,`"`)
			archive(pic,strings.Split(url,"/")[3])
		}
		/*
		Tag for crawling older page
		 */
		if strings.HasPrefix(element," " + `"` + "id"){
			nextPageId = strings.TrimLeft(element," " + `"` + "id" + `"` + ": ")
			nextPageId = strings.TrimRight(nextPageId,`"`)
		}
	}
	if(pages > 0){
		if !strings.HasSuffix(url,"/"){
			fmt.Println("I need to trim")
			//need to trim max_id off
			url = strings.TrimRight(url,strings.Split(url,"/")[len(strings.Split(url,"/")) -1])
		}

		url+= "?max_id="
		url+= nextPageId

		fmt.Println("I OPEN OLD PAGE: ", url)

		pages--
		crawl(url,pages)
	}
}


func crawlSub(id string, username string){
	fmt.Println("NEED TO CRAWL GALLERY", "https://www.instagram.com/p/" + id)
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
			archive(pic,username)
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
