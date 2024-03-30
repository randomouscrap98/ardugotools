package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <filename> <length>")
		return
	}

	// Get the length
	length, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error: can't parse length: ", err)
	}

	// Open the binary file
	filename := os.Args[1]
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	data := make([]byte, length)

	// Write very obvious data: constantly increasing values
	for i := 0; i < length; i++ {
		data[i] = uint8(i & 0xFF)
	}

	_, err = file.Write(data)
	if err != nil {
		fmt.Println("Error writing file: ", err)
		return
	}

	fmt.Println("Wrote file ", filename)
}
