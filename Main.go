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

var waitGorup = sync.WaitGroup{}
var workers = 5
var pages = -1
var interval = -1
var accountsToCrawl chan string

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
	}

	fmt.Println("Workerpool Size:", workers)
	fmt.Println("Pages to Crawl per profile:", pages)
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

func startCrawling() {
	accountsToCrawl = make(chan string, 1000)

	readAccountsFile()
	for w := 1; w <= workers; w++ {
		go workerRun(w)
	}
	time.Sleep(100)
	waitGorup.Wait()
}

func readAccountsFile()  {
	b, err := ioutil.ReadFile("accounts.txt")
	if err != nil {
		fmt.Print("Couldn't read accounts.txt")
		panic(err)
	}
	for _, element := range strings.Split(string(b),",") {
		addAccount(element)
	}
}

func workerRun(i int) {
	for account := range accountsToCrawl {
		fmt.Println("Worker",i,"crawling: ",account)
		crawl(account)
		waitGorup.Done()
	}
}

func addAccount(acc string)  {
	waitGorup.Add(1)
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
				waitGorup.Add(1)
				go archive(element.DisplaySrc,accountname,element.Date)
			}
			if element.Typename == "GraphSidecar" {
				waitGorup.Add(1)
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
			waitGorup.Add(1)
			go archive(element.Node.DisplaySrc,accountname,timestamp)
		}
	}
	waitGorup.Done()
}

func archive(pictureurl string, username string,timestamp int)  {
	fullpath := username + "/" + strconv.Itoa(timestamp) + "_" +strings.Split(pictureurl,"/")[len(strings.Split(pictureurl,"/")) - 1]

	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		save(pictureurl, fullpath)
	}
	waitGorup.Done()
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
