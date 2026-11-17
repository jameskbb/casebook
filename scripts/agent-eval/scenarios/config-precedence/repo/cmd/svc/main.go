// Command svc serves the API.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"svc/internal/config"
)

func main() {
	confPath := flag.String("config", "/etc/svc/svc.conf", "config file")
	port := flag.String("port", "", "listen port (overrides everything)")
	printOnly := flag.Bool("print-config", false, "print the effective config and exit")
	flag.Parse()

	overrides := map[string]string{}
	if *port != "" {
		overrides["port"] = *port
	}
	cfg, err := config.Load(*confPath, overrides)
	if err != nil {
		log.Fatal(err)
	}
	if *printOnly {
		fmt.Printf("port=%d log_level=%s\n", cfg.Port, cfg.LogLevel)
		os.Exit(0)
	}
	log.Printf("listening on :%d", cfg.Port)
}
