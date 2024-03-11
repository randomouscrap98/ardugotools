package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Most commands need this, so... yeah
func PrintJson(obj interface{}) {
	rawjson, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatalln("Couldn't serialize json: ", err)
	}
	fmt.Println(string(rawjson))
}

// Get a filesafe datetime, condensed (local time, I hope)
func FileSafeDateTime() string {
	currentTime := time.Now()
	return currentTime.Format("20060102-150405")
}
