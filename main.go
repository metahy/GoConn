package main

import (
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http service address")
var rooms = make(map[string]*Room)

func serveHome1(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "home.html")
}

func serveHome2(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "home2.html")
}

func serveHome3(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "home3.html")
}

func serveHome4(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "home4.html")
}

func main() {
	flag.Parse()
	http.HandleFunc("/1", serveHome1)
	http.HandleFunc("/2", serveHome2)
	http.HandleFunc("/3", serveHome3)
	http.HandleFunc("/4", serveHome4)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
