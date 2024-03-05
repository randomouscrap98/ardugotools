package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/randomouscrap98/ardugotools/arduboy"
)

func main() {
	devices, err := arduboy.GetBasicDevices()
	if err != nil {
		log.Fatalln("Couldn't pull devices: ", err)
	}
	rawjson, err := json.Marshal(devices)
	if err != nil {
		log.Fatalln("Couldn't serialize json: ", err)
	}
	fmt.Println(string(rawjson))
	//fmt.Println("Done")

	//fmt.Println("Wow?")
	//arduboy.About()
	//arduboy.ListDevices()
	//arduboy.ListDetailedDevices()
}
