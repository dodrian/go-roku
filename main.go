package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type SeasonResponse struct {
	Items []Season
}

type Season struct {
	Id string
}

type EpisodeResponse struct {
	Items []Episode
}

type Episode struct {
	Id string
}

type Item struct {
	Id        string
	Name      string
	ImageTags ImageTag
}

type ImageTag struct {
	Primary string
}

type ItemResponse struct {
	Items        []Item
	JELLYFIN_URL string
	GOROKU_URL   string
	ROKU_URL     string
}

var ROKU_URL string
var JELLYFIN_URL string
var JELLYFIN_API_KEY string
var JELLYFIN_CHANNEL_ID string
var JELLYFIN_DEFAULT_LIBRARY string
var JELLYFIN_USER_ID string
var GOROKU_URL string
var client *http.Client

func getEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return ""
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func getSeasons(series string) []Season {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Shows/%s/Seasons", JELLYFIN_URL, series), nil)
	req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
	resp, _ := client.Do(req)
	var sr SeasonResponse
	_ = json.NewDecoder(resp.Body).Decode(&sr)
	return sr.Items
}

func getEpisodes(series, season string) []Episode {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Shows/%s/Episodes?SeasonId=%s", JELLYFIN_URL, series, season), nil)
	req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
	resp, _ := client.Do(req)
	var er EpisodeResponse
	_ = json.NewDecoder(resp.Body).Decode(&er)
	return er.Items
}

func getItems() (ItemResponse, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Items?userId=%s&Fields=Name,Id&SortBy=SortName&SortOrder=Ascending&IncludeItemTypes=Series&Recursive=true&StartIndex=0&Limit=100&EnableImageTypes=Primary&ParentId=%s", JELLYFIN_URL, JELLYFIN_USER_ID, JELLYFIN_DEFAULT_LIBRARY), nil)
	req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
	resp, err := client.Do(req)
	if err != nil {
		return ItemResponse{}, err
	}
	var ir ItemResponse
	err = json.NewDecoder(resp.Body).Decode(&ir)
	if err != nil {
		return ItemResponse{}, err
	}
	ir.GOROKU_URL = GOROKU_URL
	ir.JELLYFIN_URL = JELLYFIN_URL
	ir.ROKU_URL = ROKU_URL
	return ir, nil
}

func main() {

	ROKU_URL = getEnv("ROKU_URL")
	JELLYFIN_URL = getEnv("JELLYFIN_URL")
	JELLYFIN_API_KEY = getEnv("JELLYFIN_API_KEY")
	JELLYFIN_CHANNEL_ID = getEnv("JELLYFIN_CHANNEL_ID")
	JELLYFIN_DEFAULT_LIBRARY = getEnv("JELLYFIN_DEFAULT_LIBRARY")
	JELLYFIN_USER_ID = getEnv("JELLYFIN_USER_ID")
	GOROKU_URL = getEnv("GOROKU_URL")

	log.SetOutput(os.Stdout)

	client = &http.Client{}

	r := mux.NewRouter()

	r.HandleFunc("/series/{series}/seasons", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		series := vars["series"]
		seasons := getSeasons(series)
		_ = json.NewEncoder(w).Encode(&seasons)
		// fmt.Fprintf(w, "%v\n", seasons)
	})
	r.HandleFunc("/series/{series}/episodes", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		series := vars["series"]
		seasons := getSeasons(series)
		episodes := []Episode{}
		for _, season := range seasons {
			episodes = append(episodes, getEpisodes(series, season.Id)...)
		}
		if r.Method == "GET" {
			_ = json.NewEncoder(w).Encode(&episodes)
		} else if r.Method == "POST" {
			episode := episodes[rand.Intn(len(episodes))].Id

			// delete played state to play from beginning
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/Users/%s/PlayedItems/%s", JELLYFIN_URL, JELLYFIN_USER_ID, episode), strings.NewReader(""))
			req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
			_, err := client.Do(req)

			if err != nil {
				fmt.Fprintf(w, "Delete Request: %v\n", err)
			}

			req, err = http.NewRequest("POST", fmt.Sprintf("%s/launch/%s?contentID=%s&MediaType=Episode", ROKU_URL, JELLYFIN_CHANNEL_ID, episode), strings.NewReader(""))
			if err != nil {
				fmt.Fprintf(w, "Create Request: %v\n", err)
				return
			}
			_, err = client.Do(req)
			if err != nil {
				fmt.Fprintf(w, "Post Request: %v\n", err)
			}
		}
		//fmt.Fprintf(w, "%v\n", seasons)
	})
	r.HandleFunc("/series/", func(w http.ResponseWriter, r *http.Request) {
		ir, _ := getItems()
		_ = json.NewEncoder(w).Encode(&ir)
	})
	tmpl := template.Must(template.ParseFiles("assets/index.html"))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ir, _ := getItems()
		tmpl.Execute(w, ir)

	})
	r.Use(loggingMiddleware)

	log.Fatal(http.ListenAndServe(":8000", r))
}
