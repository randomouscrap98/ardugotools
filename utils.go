package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// Most commands need this, so... yeah
func PrintJson(obj interface{}) {
	rawjson, err := json.Marshal(obj)
	if err != nil {
		log.Fatalln("Couldn't serialize json: ", err)
	}
	fmt.Println(string(rawjson))
}
