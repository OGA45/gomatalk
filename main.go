package main

import (
	"flag"
	"log"
)

func main() {
	// runtime.GOMAXPROCS(1)
	filename := flag.String("f", "bot.toml", "Set path for the config file.")
	flag.Parse()
	log.Println("INFO: Opening", *filename)
	err := LoadConfig(*filename)
	if err != nil {
		log.Println("FATA:", err)
		return
	}

	err = LoadVoiceConfig(*filename)
	if err != nil {
		log.Println("FATA:", err)
		return
	}

	// Hot reload
	Watch()

	err = CreateDB()
	if err != nil {
		log.Println("FATA: DB", err)
		return
	}

	// Connecto to Discord
	err = DiscordConnect()
	if err != nil {
		log.Println("FATA: Discord", err)
		return
	}

	WavGC()

	<-make(chan struct{})
}
