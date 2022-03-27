package main

import (
	"flag"
	"github.com/pressly/goose/v3"
	"log"
	"requester/internal/repository"

	_ "github.com/jackc/pgx/v4/stdlib"
	_ "github.com/joho/godotenv/autoload"
)

var (
	command string
	dir     string
	args    []string
)

func init() {
	flag.StringVar(&dir, "dir", "migrations", "directory with migration files")
	flag.Parse()
	args = flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return
	}
	command, args = args[0], args[1:]
	goose.SetSequential(true)
	goose.SetVerbose(true)
}

func main() {
	dbCfg, err := repository.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	db, err := goose.OpenDBWithDriver("pgx", dbCfg.URL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = goose.Run(command, db, dir, args...)
	if err != nil {
		log.Fatalf("goose %v: %v", command, err)
	}
}
