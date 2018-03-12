package main

//go:generate rice embed-go

import (
	"fmt"
	"log"
	"os"

	"github.com/asdine/storm"
	"github.com/namsral/flag"
)

var (
	cfg Config
	db  *storm.DB
)

func main() {
	var (
		version bool
		config  string
		dbpath  string
		bind    string
	)

	flag.BoolVar(&version, "v", false, "display version information")
	flag.StringVar(&config, "config", "", "config file")
	flag.StringVar(&dbpath, "dbpath", "je.db", "Database path")
	flag.StringVar(&bind, "bind", "0.0.0.0:8000", "[int]:<port> to bind to")
	flag.Parse()

	if version {
		fmt.Printf("je v%s", FullVersion())
		os.Exit(0)
	}

	var err error
	db, err = storm.Open(dbpath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	NewServer(bind, cfg).ListenAndServe()
}
