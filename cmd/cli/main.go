package main

import (
	"fmt"
	"github.com/randomouscrap98/ardugotools/arduboy"
)

func main() {
	fmt.Println("Wow?")
	arduboy.About()
	arduboy.ListDevices()
	arduboy.ListDetailedDevices()
}
