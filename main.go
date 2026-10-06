package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/OGA45/gomatalk/pkg/activity"
	"github.com/OGA45/gomatalk/pkg/boltdb"
	"github.com/OGA45/gomatalk/pkg/config"
	"github.com/OGA45/gomatalk/pkg/db"
	"github.com/OGA45/gomatalk/pkg/discord"
	global "github.com/OGA45/gomatalk/pkg/global_vars"
)

// WavGC periodically removes stale temp synthesis files (/tmp/voice-*.wav and
// .txt) that were orphaned by a hard kill before their deferred cleanup ran.
func WavGC() {
	go func() {
		t := time.NewTicker(30 * time.Minute) // 30分おきに検索
		defer t.Stop()
		for range t.C {
			files, err := filepath.Glob("/tmp/voice-*")
			if err != nil {
				// A glob error must not permanently disable the janitor.
				log.Println("ERROR: WavGC glob:", err)
				continue
			}
			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					continue // already removed, skip
				}
				if info.ModTime().Before(time.Now().Add(-10 * time.Minute)) { // 10分前以前に作られたファイルは消去
					log.Println("INFO: Garbage temp file found. Deleting...: " + file)
					os.Remove(file)
				}
			}
		}
	}()
}

func main() {
	filename := flag.String("f", "bot.toml", "Set path for the config file.")
	flag.Parse()
	log.Println("INFO: Opening", *filename)
	if err := config.Load(*filename); err != nil {
		log.Fatal("FATAL: config: ", err)
	}

	// Hot reload
	config.Watch()

	global.DB = db.NewDatabase("data/gomatalk-sqlite.db")

	// Connect to Discord
	if err := discord.DiscordConnect(); err != nil {
		log.Fatal("FATAL: Discord ", err)
	}
	boltdb.Migrate()
	WavGC()

	// Optional Discord Activity backend (HTTP server on :8080 by default).
	var activitySrv *activity.Server
	if config.O().Activity.Enabled {
		srv, err := activity.Start()
		if err != nil {
			log.Println("ERROR: cannot start activity server:", err)
		} else {
			activitySrv = srv
		}
	}

	// Block until interrupted, then shut down gracefully.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	log.Println("INFO: shutting down...")
	// Shutdown takes the session locks; if they are wedged, leave a goroutine
	// dump before docker's stop timeout (stop_grace_period) SIGKILLs us.
	time.AfterFunc(25*time.Second, func() { discord.DumpAndExit("shutdown did not finish within 25s") })
	discord.Shutdown()
	if activitySrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := activitySrv.Shutdown(ctx); err != nil {
			log.Println("ERROR: activity shutdown:", err)
		}
	}
}
