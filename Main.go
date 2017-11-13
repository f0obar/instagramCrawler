package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"strings"
	"log"
	"os"
	"io"
)
var accounts []string

func main() {
	accounts = append(accounts, "https://www.instagram.com/elonmusk/")
	fmt.Println("Crawler starting")

	for _, element := range accounts {
		crawl(element)
	}
	fmt.Println("Crawler finished")
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
			pic := strings.TrimLeft(element," " + `"` + "display_src: " + `"`)
			pic = strings.TrimRight(pic,`"`)
			archive(pic,strings.Split(url,"/")[3])
		}
	}
}

func archive(pictureurl string, username string)  {
	//fmt.Println("saving " + pictureurl + " from " + username)
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
