package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

const chunkSize = 256
const magicString = "ARDUBOY"

func main() {
	// Check if a filename is provided as a command-line argument
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <filename>")
		return
	}

	// Open the binary file
	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Create a new directory to store the found chunks
	outputDir := "found_chunks"
	err = os.Mkdir(outputDir, 0755)
	if err != nil && !os.IsExist(err) {
		fmt.Println("Error creating output directory:", err)
		return
	}

	// Create a reader for the file
	reader := bufio.NewReader(file)

	// Read the file in 256 byte chunks
	chunk := make([]byte, chunkSize)
	chunkCounter := 0
	for {
		bytesRead, err := reader.Read(chunk)
		if err != nil {
			break
		}

		// Check if the chunk starts with the magic string
		if string(chunk[:len(magicString)]) == magicString {
			// Write out the chunk to a new file
			chunkFilename := filepath.Join(outputDir, fmt.Sprintf("chunk_%d.bin", chunkCounter))
			chunkFile, err := os.Create(chunkFilename)
			if err != nil {
				fmt.Println("Error creating chunk file:", err)
				return
			}
			defer chunkFile.Close()

			_, err = chunkFile.Write(chunk[:bytesRead])
			if err != nil {
				fmt.Println("Error writing chunk to file:", err)
				return
			}
			chunkCounter++
		}

		// Move to the next chunk
		_, err = reader.Discard(chunkSize - bytesRead)
		if err != nil {
			break
		}
	}

	fmt.Printf("Found and saved %d chunks to directory '%s'\n", chunkCounter, outputDir)
}

