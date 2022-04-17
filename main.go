package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
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
	Type      string
}

type ImageTag struct {
	Primary string
}

type ItemResponse struct {
	Items            []Item
	TotalRecordCount int
}

type Library struct {
	ItemResponse

	Name string
	Id   string
}

type Index struct {
	Libraries []Library

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

func playItemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item_id"]
	log.Printf("Play request for item %s", itemID)
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Items?userId=%s&ids=%s&SortBy=SortName&SortOrder=Ascending&Recursive=False&StartIndex=0&Limit=100&EnableImageTypes=Primary", JELLYFIN_URL, JELLYFIN_USER_ID, itemID), nil)
	req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error getting item %s: %s", itemID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var ir ItemResponse
	err = json.NewDecoder(resp.Body).Decode(&ir)
	if err != nil {
		log.Printf("error decoding ItemResponse for item %s: %s", itemID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var playable_id string
	itemType := ir.Items[0].Type
	switch {
	case itemType == "Movie" || itemType == "Episode":
		playable_id = ir.Items[0].Id

	case ir.Items[0].Type == "Series":
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Items?userId=%s&ParentId=%s&SortBy=SortName&SortOrder=Ascending&IncludeItemTypes=Episode&Recursive=True&StartIndex=0&EnableImageTypes=Primary", JELLYFIN_URL, JELLYFIN_USER_ID, itemID), nil)
		req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("error getting item as parent %s: %s", itemID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var ir ItemResponse
		err = json.NewDecoder(resp.Body).Decode(&ir)
		if err != nil {
			log.Printf("error decoding ItemResponse for item parent %s: %s", itemID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		playable_id = ir.Items[rand.Intn(len(ir.Items))].Id

	default:
		log.Printf("error switching type %s for item %s: %s", ir.Items[0].Type, itemID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// delete played state to play from beginning
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/Users/%s/PlayedItems/%s", JELLYFIN_URL, JELLYFIN_USER_ID, playable_id), strings.NewReader(""))
	req.Header.Add("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, JELLYFIN_API_KEY))
	_, err = client.Do(req)

	if err != nil {
		fmt.Fprintf(w, "Delete Request: %v\n", err)
	}
	log.Printf("Attempting to play item %s type %s", playable_id, itemType)
	req, err = http.NewRequest("POST", fmt.Sprintf("%s/launch/%s?contentID=%s&mediaType=%s", ROKU_URL, JELLYFIN_CHANNEL_ID, playable_id, itemType), strings.NewReader(""))
	if err != nil {
		fmt.Fprintf(w, "Create Request: %v\n", err)
		return
	}
	_, err = client.Do(req)
	if err != nil {
		fmt.Fprintf(w, "Post Request: %v\n", err)
	}

}

func getLibraryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	library := vars["library_id"]
	ir, err := getItems(library)
	if err != nil {
		log.Printf("error getting library %s: %s", library, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(&ir)
}

func getItems(library string) (ItemResponse, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Items?userId=%s&Fields=Name,Id,IsSeries&SortBy=SortName&SortOrder=Ascending&Recursive=False&StartIndex=0&Limit=100&EnableImageTypes=Primary&ParentId=%s", JELLYFIN_URL, JELLYFIN_USER_ID, library), nil)
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

	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatal("Error when opening file: ", err)
	}

	// Now let's unmarshall the data into `payload`
	var libraries []Library
	err = json.Unmarshal(content, &libraries)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
	}

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
	r.HandleFunc("/play/{item_id}", playItemHandler)
	r.HandleFunc("/series/", func(w http.ResponseWriter, r *http.Request) {
		ir, _ := getItems(JELLYFIN_DEFAULT_LIBRARY)
		_ = json.NewEncoder(w).Encode(&ir)
	})
	r.HandleFunc("/library/{library_id}", getLibraryHandler)
	tmpl := template.Must(template.ParseFiles("assets/index.html"))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for i := range libraries {
			ir, err := getItems(libraries[i].Id)
			if err != nil {
				log.Printf("err getting library %s %s", libraries[i].Name, libraries[i].Id)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			libraries[i].ItemResponse = ir

		}

		index := Index{
			Libraries:    libraries,
			GOROKU_URL:   GOROKU_URL,
			JELLYFIN_URL: JELLYFIN_URL,
			ROKU_URL:     ROKU_URL,
		}
		tmpl.Execute(w, index)

	})
	r.Use(loggingMiddleware)

	log.Fatal(http.ListenAndServe(":8000", r))
}
