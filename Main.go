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
	"github.com/anaskhan96/soup"
	"encoding/json"
	"github.com/gosuri/uiprogress"
	"regexp"
	"errors"
)

var saveVideos = false
const errorDelay = 30
var waitGroup = sync.WaitGroup{}
var pages = -1
var interval = -1

//variables just for user information
var doneCount = 0
var savedImages = 0

// Maximum simultaneously http connections open at any given time (worker pool size)
var maxConnections = 50

var pageChan = make(chan Page,1000)
var mediaChan = make(chan Resource,10000)
var galleryPageChan = make(chan Resource,1000)
var videoPageChan = make(chan Resource,1000)

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
		if strings.HasPrefix(element, "v") {
			saveVideos = true
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
		case galleryPage := <-galleryPageChan:
			handleGalleryPage(galleryPage)
			continue
		default:
		}
		select {
		case videoPage := <-videoPageChan:
			handleVideoPage(videoPage)
			continue
		default:
		}
		select {
		case page := <-pageChan:
			handlePage(page)
			continue
		case gallery := <-galleryPageChan:
			handleGalleryPage(gallery)
			continue
		case video := <-videoPageChan:
			handleVideoPage(video)
			continue
		case m := <-mediaChan:
			handleMedia(m)
			continue
		}
		break
	}
}

func handlePage(page Page){
	script, err := getJson(page.Url)
	if err != nil {
		updateProgressBar()
		waitGroup.Done()
		return
	}

	mainPage := JsonMainPage{}
	json.Unmarshal([]byte(script), &mainPage)

	for _, element := range mainPage.EntryData.ProfilePage[0].User.Media.Nodes {
		if !element.IsVideo{
			if element.Typename == "GraphImage" {
				waitGroup.Add(1)
				mediaChan <- Resource{element.DisplaySrc,page.Username,element.Date}
			}
			if element.Typename == "GraphSidecar" {
				waitGroup.Add(1)
				galleryPageChan <- Resource{"https://www.instagram.com/p/" + element.Code,page.Username,element.Date}
			}
		} else if saveVideos{
			waitGroup.Add(1)
			videoPageChan <- Resource{"https://www.instagram.com/p/" + element.Code,page.Username,element.Date}
		}
	}

	if page.Remaining != 0 && mainPage.EntryData.ProfilePage[0].User.Media.PageInfo.HasNextPage {
		waitGroup.Add(1)
		pageChan <- Page{"https://www.instagram.com/" + page.Username + "/?max_id=" + mainPage.EntryData.ProfilePage[0].User.Media.Nodes[11].Id,page.Username,page.Remaining - 1}
	}
	updateProgressBar()
	waitGroup.Done()
}

func handleGalleryPage(resource Resource){
	script, err := getJson(resource.Url)
	if err != nil {
		updateProgressBar()
		waitGroup.Done()
		return
	}

	page := JsonGalleryPage{}
	json.Unmarshal([]byte(script), &page)

	for _, element := range page.EntryData.PostPage[0].Graphql.ShortcodeMedia.EdgeSidecarToChildren.Edges {
		if element.Node.Typename == "GraphImage" {
			waitGroup.Add(1)
			mediaChan <- Resource{element.Node.DisplaySrc, resource.Username, resource.Timestamp}
		}
	}
	updateProgressBar()
	waitGroup.Done()
}

func handleVideoPage(resource Resource) {
	script, err := getJson(resource.Url)
	if err != nil {
		updateProgressBar()
		waitGroup.Done()
		return
	}

	page := JsonVideoPage{}
	json.Unmarshal([]byte(script), &page)

	waitGroup.Add(1)
	mediaChan <- Resource{page.EntryData.PostPage[0].Graphql.ShortcodeMedia.VideoUrl, resource.Username, resource.Timestamp}

	updateProgressBar()
	waitGroup.Done()
}

func handleMedia(resource Resource){
	fullpath := resource.Username + "/" + strconv.Itoa(resource.Timestamp) + "_" +strings.Split(resource.Url,"/")[len(strings.Split(resource.Url,"/")) - 1]

	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		resp, err := get(resource.Url)
		if err != nil {
			updateProgressBar()
			waitGroup.Done()
			return
		}

		file, err := os.Create(fullpath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		file.Write(resp)
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
		f := float64(doneCount)/float64(len(mediaChan) + len(galleryPageChan) + len(pageChan) + len(videoPageChan) + doneCount)
		bar.Set(int(f * 100))
	}
}


func get(url string)([]byte, error) {
	resp, e := http.Get(url)
	if e != nil {
		fmt.Println("Connection Issue")
		time.Sleep(errorDelay * time.Second)
		return get(url)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429{
		fmt.Println("Throtteling")
		time.Sleep(errorDelay * time.Second)
		return get(url)
	}
	if resp.StatusCode == 404 {
		fmt.Println("Could not find",url)
		return nil,errors.New("not found")
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("unable to read the response body")
	}
	return bytes, nil
}

func getJson(url string)(string,error){
	resp, err := get(url)
	if err != nil {
		return "", err
	}
	doc := soup.HTMLParse(string(resp))

	script := doc.FindAll("script")[2].Text()
	script = script[21:len(script)-1]

	var data map[string]interface{}
	e := json.Unmarshal([]byte(script), &data)
	if e != nil {
		panic(e)
	}
	return script,nil
}

type Page struct {
	Url string
	Username string
	Remaining int
}

type Resource struct {
	Url string
	Username string
	Timestamp int
}


type JsonMainPage struct {
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

type JsonGalleryPage struct {
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

type JsonVideoPage struct {
	EntryData struct {
		PostPage []struct {
			Graphql struct {
				ShortcodeMedia struct {
					Typename string `json:"__typename"`
					Id       string `json:"id"`
					VideoUrl string `json:"video_url"`
				} `json:"shortcode_media"`
			} `json:"graphql"`
		} `json:"PostPage"`
	} `json:"entry_data"`
}