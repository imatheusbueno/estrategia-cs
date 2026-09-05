package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Strategy struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Map       string `json:"map"`
	Side      string `json:"side"`
	Plan      string `json:"plan"`
	CreatedAt string `json:"createdAt,omitempty"`
}

func main() {
	db, err := sql.Open("sqlite", "file:cs2-playbook.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS strategies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		map TEXT NOT NULL,
		side TEXT NOT NULL CHECK(side IN ('ct', 'tr')),
		plan TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/strategies", strategiesHandler(db))
	mux.Handle("/", http.FileServer(http.Dir(".")))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("CS2 Playbook em http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, withCORS(mux)))
}

func strategiesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			rows, err := db.Query(`SELECT id, name, map, side, plan, created_at FROM strategies ORDER BY id DESC`)
			if err != nil {
				http.Error(w, "database query failed", http.StatusInternalServerError)
				return
			}
			defer rows.Close()
			items := make([]Strategy, 0)
			for rows.Next() {
				var item Strategy
				if err := rows.Scan(&item.ID, &item.Name, &item.Map, &item.Side, &item.Plan, &item.CreatedAt); err != nil {
					http.Error(w, "database read failed", http.StatusInternalServerError)
					return
				}
				items = append(items, item)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "database iteration failed", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(items)
		case http.MethodPost:
			var item Strategy
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
			if err := decoder.Decode(&item); err != nil || strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Plan) == "" || (item.Side != "ct" && item.Side != "tr") {
				http.Error(w, "invalid strategy", http.StatusBadRequest)
				return
			}
			result, err := db.Exec(`INSERT INTO strategies(name, map, side, plan) VALUES (?, ?, ?, ?)`, item.Name, item.Map, item.Side, item.Plan)
			if err != nil {
				http.Error(w, "database insert failed", http.StatusInternalServerError)
				return
			}
			item.ID, _ = result.LastInsertId()
			item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(item)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
