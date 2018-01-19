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
	"io"
	"sync"
	"github.com/anaskhan96/soup"
	"encoding/json"
	"github.com/gosuri/uiprogress"
	"regexp"
	"errors"
)

const errorDelay = 5
var waitGroup = sync.WaitGroup{}
var pages = -1
var interval = -1

//variables just for user information
var doneCount = 0
var savedImages = 0

// Maximum simultanious http connections open at any given time (workerpool size)
var maxConnections = 50

var imageChan = make(chan Image,10000)
var pageChan = make(chan Page,1000)
var galleryChan = make(chan Gallery,1000)

var bar *uiprogress.Bar// Add a new bar


func main() {
	fmt.Println("Crawler starting")
	params := append(os.Args[:0], os.Args[1:]...)

	for _, element := range params {
		if strings.HasPrefix(element, "c") {
			reg, err := regexp.Compile("[^0-9]+")
			if err != nil {
				log.Fatal(err)
			}
			processedString := reg.ReplaceAllString(element, "")
			if num, err := strconv.Atoi(processedString); err == nil {
				maxConnections = num
			}
		}
		if strings.HasPrefix(element, "r") {
			reg, err := regexp.Compile("[^0-9]+")
			if err != nil {
				log.Fatal(err)
			}
			processedString := reg.ReplaceAllString(element, "")
			if num, err := strconv.Atoi(processedString); err == nil {
				interval = num
			}
		}
		if strings.HasPrefix(element, "p") {
			reg, err := regexp.Compile("[^0-9]+")
			if err != nil {
				log.Fatal(err)
			}
			processedString := reg.ReplaceAllString(element, "")
			if num, err := strconv.Atoi(processedString); err == nil {
				pages = num
			}
		}
	}

	fmt.Println(">>>SETTINGS LOADED<<<")
	fmt.Println("Maximum concurrent connections",maxConnections)
	fmt.Println("Pages to crawl per profile:", pages)
	fmt.Println("Refreshing interval:", interval, "seconds")

	if interval > 0 {
		t := time.NewTicker(time.Duration(interval) * time.Second)
		for {
			fmt.Println("\n>>>CRAWLING STARTED<<<\n")
			startCrawling()
			fmt.Println(">>>CRAWLING FINISHED, next crawling will start in " + strconv.Itoa(interval) + " seconds\n\n<<<")
			<-t.C
		}
	} else {
		fmt.Println("\n>>>CRAWLING STARTED<<\n")
		startCrawling()
		fmt.Println("\n>>>CRAWLING FINISHED<<\n")
	}
}


func startCrawling() {
	uiprogress.Start()            // start rendering
	bar = uiprogress.AddBar(100) // Add a new bar
	bar.AppendCompleted()

	doneCount = 0
	readAccountsFile()
	for i := 1; i <= maxConnections; i++ {
		go workerRoutine()
	}
	//go status()
	time.Sleep(100)
	waitGroup.Wait()
	fmt.Println("Saved " + strconv.Itoa(savedImages) + " new Images!")
}

func readAccountsFile()  {
	b, err := ioutil.ReadFile("accounts.txt")
	if err != nil {
		fmt.Print("Couldn't read accounts.txt, please provide a file named accounts.txt in the same directory")
		panic(err)
	}
	for _, element := range strings.Split(string(b),",") {
		//Checking / Creating folder for the account
		if _, err := os.Stat(element); os.IsNotExist(err) {
			err = os.MkdirAll(element, 0777)
			if err != nil {
				panic(err)
			}
		}
		//Adding base page to the que
		pageChan <- Page{"https://www.instagram.com/" + element + "/",element,pages}
		waitGroup.Add(1)
	}
}

func workerRoutine(){
	for {
		select {
		case page := <-pageChan:
			handlePage(page)
			continue
		default:
		}
		select {
		case gallery := <-galleryChan:
			handleGallery(gallery)
			continue
		default:
		}
		select {
		case page := <-pageChan:
			handlePage(page)
			continue
		case gallery := <-galleryChan:
			handleGallery(gallery)
			continue
		case i := <-imageChan:
			handleImage(i)
			continue
		}
		break
	}
}

func handlePage(page Page){
	resp, err, retry := get(page.Url)
	if err != nil {
		if retry {
			handlePage(page)
			return
		} else {
			updateProgressBar()
			waitGroup.Done()
			return
		}
	}
	doc := soup.HTMLParse(resp)

	script := doc.FindAll("script")[2].Text()
	script = script[21:len(script)-1]

	var data map[string]interface{}
	e := json.Unmarshal([]byte(script), &data)
	if e != nil {
		panic(e)
	}

	mainPage := MainPage{}
	json.Unmarshal([]byte(script), &mainPage)


	for _, element := range mainPage.EntryData.ProfilePage[0].User.Media.Nodes {
		if !element.IsVideo{
			if element.Typename == "GraphImage" {
				waitGroup.Add(1)
				imageChan <- Image{element.DisplaySrc,page.Username,element.Date}
			}
			if element.Typename == "GraphSidecar" {
				waitGroup.Add(1)
				galleryChan <- Gallery{"https://www.instagram.com/p/" + element.Code,page.Username,element.Date}
			}
		}
	}

	if page.Remaining != 0 && mainPage.EntryData.ProfilePage[0].User.Media.PageInfo.HasNextPage {
		waitGroup.Add(1)
		pageChan <- Page{"https://www.instagram.com/" + page.Username + "/?max_id=" + mainPage.EntryData.ProfilePage[0].User.Media.Nodes[11].Id,page.Username,page.Remaining - 1}
	}
	updateProgressBar()
	waitGroup.Done()
}

func handleGallery(gallery Gallery){
	resp, err, retry := get(gallery.Url)
	if err != nil {
		if retry {
			handleGallery(gallery)
			return
		} else {
			updateProgressBar()
			waitGroup.Done()
			return
		}
	}
	doc := soup.HTMLParse(resp)

	script := doc.FindAll("script")[2].Text()
	script = script[21:len(script)-1]

	var data map[string]interface{}
	e := json.Unmarshal([]byte(script), &data)
	if e != nil {
		panic(e)
	}

	page := GalleryPage{}
	json.Unmarshal([]byte(script), &page)

	for _, element := range page.EntryData.PostPage[0].Graphql.ShortcodeMedia.EdgeSidecarToChildren.Edges {
		if element.Node.Typename == "GraphImage" {
			waitGroup.Add(1)
			imageChan <- Image{element.Node.DisplaySrc,gallery.Username,gallery.Timestamp}
		}
	}
	updateProgressBar()
	waitGroup.Done()
}

func handleImage(image Image){
	fullpath := image.Username + "/" + strconv.Itoa(image.Timestamp) + "_" +strings.Split(image.Url,"/")[len(strings.Split(image.Url,"/")) - 1]

	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		resp, e := http.Get(image.Url)
		if e != nil {
			time.Sleep(errorDelay * time.Second)
			handleImage(image)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429{
			time.Sleep(errorDelay * time.Second)
			handleImage(image)
			return
		}
		if(resp.StatusCode == 404) {
			return
		}

		file, err := os.Create(fullpath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		savedImages ++
	}
	updateProgressBar()
	waitGroup.Done()
}

func updateProgressBar() {
	doneCount ++
	if doneCount == 0 {
		bar.Set(0)
	} else {
		f := float64(doneCount)/float64(len(imageChan) + len(galleryChan) + len(pageChan) + doneCount)
		bar.Set(int(f * 100))
	}
}


func get(url string)(string, error, bool) {
	resp, e := http.Get(url)
	if e != nil {
		time.Sleep(errorDelay * time.Second)
		return "",errors.New("Connection Issues"),true
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429{
		time.Sleep(errorDelay * time.Second)
		return "",errors.New("Throtteling"),true
	}
	if(resp.StatusCode == 404) {
		return "",errors.New("Not found"),false
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Unable to read the response body"),false
	}
	return string(bytes), nil,false
}

func checkAndHandleThrottle(s string) {

}

func checkForError(s string)(bool) {
	return false
}

type Page struct {
	Url string
	Username string
	Remaining int
}

type Gallery struct {
	Url string
	Username string
	Timestamp int
}

type Image struct {
	Url string
	Username string
	Timestamp int
}

type MainPage struct {
	EntryData struct{
		ProfilePage []struct{
			User struct{
				Media struct{
					Nodes []struct{
						Typename string `json:"__typename"`
						Id string `json:"id"`
						MediaPreview string `json:"media_preview"`
						IsVideo bool `json:"is_video"`
						Code string `json:"code"`
						Date int `json:"date"`
						DisplaySrc string `json:"display_src"`
						Caption string `json:"caption"`
					} `json:"nodes"`
					PageInfo struct{
						HasNextPage bool `json:"has_next_page"`
					} `json:"page_info"`
				} `json:"media"`
			} `json:"user"`
		} `json:"ProfilePage"`
	} `json:"entry_data"`
}

type GalleryPage struct {
	EntryData struct {
		PostPage []struct {
			Graphql struct {
				ShortcodeMedia struct {
					EdgeSidecarToChildren struct {
						Edges []struct {
							Node struct {
								Typename     string `json:"__typename"`
								Id           string `json:"id"`
								MediaPreview string `json:"media_preview"`
								IsVideo      bool   `json:"is_video"`
								DisplaySrc   string `json:"display_url"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"edge_sidecar_to_children"`
				} `json:"shortcode_media"`
			} `json:"graphql"`
		} `json:"PostPage"`
	} `json:"entry_data"`
}