// main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/showwin/speedtest-go/speedtest"
)

type SpeedTestResult struct {
	ID        int
	Timestamp time.Time
	Ping      float64
	Download  float64
	Upload    float64
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./speedtest.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTable()

	go startSpeedTest()

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/results", resultsHandler).Methods("GET")

	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func createTable() {
	createTableSQL := `CREATE TABLE IF NOT EXISTS speedtest_results (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"timestamp" DATETIME,
		"ping" REAL,
		"download" REAL,
		"upload" REAL
	);`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}
}

func startSpeedTest() {
	for {
		result, err := performSpeedTest()
		if err != nil {
			log.Println("Error performing speed test:", err)
		} else {
			log.Println(result)
			storeResult(result)
		}
		time.Sleep(10 * time.Second)
	}
}

func performSpeedTest() (SpeedTestResult, error) {
	// Retrieve available servers
	var speedtestClient = speedtest.New()
	serverList, _ := speedtestClient.FetchServers()
	targets, _ := serverList.FindServer([]int{})

	// user, _ := speedtest.FetchUserInfo()
	// serverList, _ := speedtest.FetchServerList(user)
	// targets, _ := serverList.FindServer([]int{})

	var result SpeedTestResult
	result.Timestamp = time.Now()

	for _, s := range targets {
		err := s.PingTest(nil)
		if err != nil {
			return result, err
		}
		result.Ping = s.Latency.Seconds() * 1000

		err = s.DownloadTest()
		if err != nil {
			return result, err
		}
		result.Download = float64(s.DLSpeed)

		err = s.UploadTest()
		if err != nil {
			return result, err
		}
		result.Upload = float64(s.ULSpeed)

		break
	}
	return result, nil
}

func storeResult(result SpeedTestResult) {
	insertSQL := `INSERT INTO speedtest_results (timestamp, ping, download, upload) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(insertSQL, result.Timestamp, result.Ping, result.Download, result.Upload)
	if err != nil {
		log.Println("Error storing result:", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.html"))
	tmpl.Execute(w, nil)
}

func resultsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, timestamp, ping, download, upload FROM speedtest_results ORDER BY timestamp DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []SpeedTestResult
	for rows.Next() {
		var result SpeedTestResult
		err := rows.Scan(&result.ID, &result.Timestamp, &result.Ping, &result.Download, &result.Upload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
