package main

import (
	// "database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	// "github.com/mattn/go-sqlite3"
)

var (
	addr = flag.String("addr", ":8090", "[ip]:port to listen on")
	// dbfile = flag.String("dbfile", "fivecalls.db", "filename for sqlite db")
)

var pagetemplate *template.Template

func main() {
	flag.Parse()

	p, err := template.ParseFiles("index.html")
	if err != nil {
		log.Println("can't parse template:", err)
	}
	pagetemplate = p

	// db, err := sql.Open("sqlite3", fmt.Sprintf("./%s", *dbfile))
	// if err != nil {
	// 	log.Printf("can't open databse: %s", err)
	// 	return
	// }
	// // cachedb = db
	// defer db.Close()

	// load the current csv files
	loadIssuesCSV()

	r := mux.NewRouter()
	r.HandleFunc("/issues/{zip}", pageHandler)
	r.HandleFunc("/", pageHandler)
	http.Handle("/", r)

	log.Printf("running fivecalls-web on port %v", *addr)

	log.Fatal(http.ListenAndServe(*addr, nil))
}

func loadIssuesCSV() {
	issuesCSV, err := os.Open("issues.csv")
	if err != nil {
		log.Fatal(err)
	}

	r := csv.NewReader(issuesCSV)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(record)
	}
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zip := vars["zip"]

	contacts := []Contact{}

	if len(zip) == 5 && zip != "" {
		log.Printf("zip %s", zip)

		googResponse, err := getReps(zip)
		if err != nil {
			panic(err)
		}

		// remove president and vice president from officials
		validOfficials := []GoogleOfficial{}
		for _, office := range googResponse.Offices {
			if strings.Contains(office.Name, "President") {
				continue
			}

			for _, index := range office.OfficialIndices {
				official := googResponse.Officials[index]

				if strings.Contains(office.Name, "Senate") {
					official.Area = "Senate"
				} else if strings.Contains(office.Name, "House") {
					official.Area = "House"
				} else {
					official.Area = "Other"
				}

				validOfficials = append(validOfficials, official)
			}
		}

		// swap to our own model for sane JSON
		for _, rep := range validOfficials {
			contact := Contact{Name: rep.Name, Phone: rep.Phones[0], PhotoURL: rep.PhotoURL, Area: rep.Area}

			contacts = append(contacts, contact)
		}
	} else {
		log.Printf("no zip")
	}

	jsonData, err := json.Marshal(contacts)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// Issue is a thing to care about and call on
type Issue struct {
	Name string
	Reps []Contact
}

// Contact is a single point of contact related to an issue
type Contact struct {
	Name     string //`json:"name"`
	Phone    string
	PhotoURL string
	Area     string
}

func getReps(zip string) (*GoogleRepResponse, error) {
	url := fmt.Sprintf("https://www.googleapis.com/civicinfo/v2/representatives?address=%s&fields=offices(name,officialIndices),officials(name,phones,urls,photoUrl)&levels=country&key=AIzaSyCNNKXRLCny-ZGGZliWjXz2JvVRBeXBeU8", zip)

	client := http.DefaultClient
	r, e := client.Get(url)
	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)
	if r.StatusCode >= 200 && r.StatusCode <= 400 && e == nil {
		parsedResponse := GoogleRepResponse{}
		json.Unmarshal(body, &parsedResponse)
		return &parsedResponse, nil
	}

	return nil, fmt.Errorf("rep error code:%d error:%v body:%s", r.StatusCode, e, body)
}

// GoogleRepResponse is the whole response
type GoogleRepResponse struct {
	Offices   []GoogleOffice
	Officials []GoogleOfficial
}

// GoogleOffice is an office in government
type GoogleOffice struct {
	Name            string
	OfficialIndices []int
}

// GoogleOfficial is a local official
type GoogleOfficial struct {
	Name     string
	Phones   []string
	Urls     []string
	PhotoURL string
	Area     string
}