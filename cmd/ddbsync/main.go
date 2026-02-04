package main

import (
	"flag"
	"log"
	"time"

	"github.com/brightcove/playback_ddbsync"
)

func main() {
	var (
		table      = flag.String("table", "", "Table Name")
		region     = flag.String("region", "", "AWS Region")
		endpoint   = flag.String("endpoint", "", "DynamoDB Endpoint")
		disableSSL = flag.Bool("disable-ssl", false, "Disable SSL")
		ttl        = flag.Duration("ttl", 1*time.Minute, "TTL")
		reattempt  = flag.Duration("reattempt", ddbsync.DefaultReattemptWait, "Reattempt wait")
		cutoff     = flag.Duration("cutoff", ddbsync.DefaultCutoff, "Cutoff")
	)
	flag.Parse()
	if len(flag.Args()) != 2 {
		log.Fatal("must provide operation & key")
	}
	var (
		operation   = flag.Arg(0)
		key         = flag.Arg(1)
		lockService = ddbsync.NewLockService(
			*table,
			*region,
			*endpoint,
			*disableSSL)
		mutex = lockService.NewLock(key, *ttl, *reattempt, *cutoff)
	)
	switch operation {
	case "lock":
		if err := mutex.Lock(); err != nil {
			log.Fatal("could not acquire lock", err)
		}
		log.Println("locked", key, "for", ttl)
	case "unlock":
		mutex.Unlock()
		log.Println("unlocked", key)
	default:
		log.Fatalf("unknown operation: %q", operation)
	}
}
