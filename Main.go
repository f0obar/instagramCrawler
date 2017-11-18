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
	"regexp"
	"sync"
	"github.com/anaskhan96/soup"
	"encoding/json"
)

var waitGroup = sync.WaitGroup{}
var workers = 2
var imageConnections = 10
var pages = -1
var interval = -1
var accountsToCrawl chan string
var imagesToSave chan MyImage


func main() {
	fmt.Println("Crawler starting")
	params := append(os.Args[:0], os.Args[1:]...)

	for _, element := range params {
		if strings.HasPrefix(element, "w") {
			reg, err := regexp.Compile("[^0-9]+")
			if err != nil {
				log.Fatal(err)
			}
			processedString := reg.ReplaceAllString(element, "")
			if num, err := strconv.Atoi(processedString); err == nil {
				workers = num
			}
		}
		if strings.HasPrefix(element, "i") {
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
		if strings.HasPrefix(element, "i") {
			reg, err := regexp.Compile("[^0-9]+")
			if err != nil {
				log.Fatal(err)
			}
			processedString := reg.ReplaceAllString(element, "")
			if num, err := strconv.Atoi(processedString); err == nil {
				imageConnections = num
			}
		}
	}

	fmt.Println("Workerpool size:", workers)
	fmt.Println("Maximum concurrent image requests",imageConnections)
	fmt.Println("Pages to crawl per profile:", pages)
	fmt.Println("Refreshing interval:", interval, "seconds")

	if interval > 0 {
		t := time.NewTicker(time.Duration(interval) * time.Second)
		for {
			fmt.Println(">>CRAWLING<<\n\n\n\n")
			startCrawling()
			fmt.Println(">>CRAWLING FINISHED, next crawling will start in " + strconv.Itoa(interval) + " seconds\n\n\n\n<<")
			<-t.C
		}
	} else {
		fmt.Println(">>CRAWLING<<\n\n\n\n")
		startCrawling()
		fmt.Println("Crawler finished")
	}
}

func statusNotification()  {
	for len(imagesToSave) > 0 || len(accountsToCrawl) > 0{
		fmt.Println("###################","Profiles in Queue:",len(accountsToCrawl),"Images in Queue",len(imagesToSave),"###################")
		time.Sleep(2*time.Second)
	}
}

func startCrawling() {
	accountsToCrawl = make(chan string, 100)
	imagesToSave = make(chan MyImage,10000)

	readAccountsFile()
	for w := 1; w <= workers; w++ {
		go workerRun()
	}
	for i := 1; i <= imageConnections; i++ {
		go imageSaverRun()
	}
	time.Sleep(100)
	go statusNotification()
	waitGroup.Wait()
}

func readAccountsFile()  {
	b, err := ioutil.ReadFile("accounts.txt")
	if err != nil {
		fmt.Print("Couldn't read accounts.txt")
		panic(err)
	}
	for _, element := range strings.Split(string(b),",") {
		addAccountToQueue(element)
	}
}

func workerRun() {
	for account := range accountsToCrawl {
		fmt.Println("Worker crawling: ",account)
		crawl(account)
		waitGroup.Done()
	}
}

func imageSaverRun()  {
	for image := range imagesToSave {
		archive(image.Url,image.Username,image.Timestamp)
		waitGroup.Done()
	}
}

func addAccountToQueue(acc string)  {
	waitGroup.Add(1)
	accountsToCrawl <- acc
}

func crawl(account string)  {
	fmt.Println("+++crawling:",account,"+++")

	//Checking / Creating Foler for the account
	if _, err := os.Stat(account); os.IsNotExist(err) {
		err = os.MkdirAll(account, 0777)
		if err != nil {
			panic(err)
		}
	}
	baseUrl:= "https://www.instagram.com/" + account + "/"

	if pages == -1 {
		//crawl full page
		nextID := openPage(baseUrl, account)
		for nextID != "" {
			nextID = openPage(baseUrl + "?max_id=" + nextID, account)
		}
	} else {
		if pages > 0 {
			pagesLeft := pages - 1
			nextID := openPage(baseUrl, account)
			for nextID != "" && pagesLeft > 0{
				nextID = openPage(baseUrl + "?max_id=" + nextID, account)
				pagesLeft--
			}
		}
	}
	fmt.Println("---crawling finished:",account,"---")
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


func openPage(url string, accountname string)(nextPageID string){
	resp, err := soup.Get(url)
	if err != nil {
		panic(err)
	}
	doc := soup.HTMLParse(resp)
	script := doc.FindAll("script")[2].Text()
	script = script[21:len(script)-1]

	var data map[string]interface{}
	e := json.Unmarshal([]byte(script), &data)
	if e != nil {
		panic(e)
	}

	page := MainPage{}
	json.Unmarshal([]byte(script), &page)


	for _, element := range page.EntryData.ProfilePage[0].User.Media.Nodes {
		if !element.IsVideo{
			if element.Typename == "GraphImage" {
				waitGroup.Add(1)
				imagesToSave <- MyImage{element.DisplaySrc,accountname,element.Date}
			}
			if element.Typename == "GraphSidecar" {
				waitGroup.Add(1)
			 	go openGallery("https://www.instagram.com/p/" + element.Code,accountname,element.Date)
			}
		}
	}

	if page.EntryData.ProfilePage[0].User.Media.PageInfo.HasNextPage {
		return page.EntryData.ProfilePage[0].User.Media.Nodes[11].Id
	}
	return ""
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

func openGallery(url string, accountname string,timestamp int)  {
	resp, err := soup.Get(url)
	if err != nil {
		panic(err)
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
			imagesToSave <- MyImage{element.Node.DisplaySrc,accountname,timestamp}
		}
	}
	waitGroup.Done()
}

type MyImage struct {
	Url string
	Username string
	Timestamp int
}

func archive(pictureurl string, username string,timestamp int)  {
	fullpath := username + "/" + strconv.Itoa(timestamp) + "_" +strings.Split(pictureurl,"/")[len(strings.Split(pictureurl,"/")) - 1]

	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		save(pictureurl, fullpath)
	}
}

func save(pictureurl string, fullpath string) {
	response, e := http.Get(pictureurl)
	if e != nil {
		log.Fatal(e)
	}
	defer response.Body.Close()

	file, err := os.Create(fullpath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
}
