package commands

import (
	"fmt"
	"os"
)

type HashObjectCommand struct{}

func (c *HashObjectCommand) GetName() string {
	return "hash-object"
}

func (c *HashObjectCommand) Execute(cmd *Command) {
	if len(cmd.Args) != 2 {
		fmt.Println(cmd.Usage)
		os.Exit(1)
	}
	sha := string(CreateBlob(cmd.Args[1], true))

	// 5. Print the SHA-1 hash
	fmt.Printf("%s", sha)
}

func CreateBlob(path string, hexCoded bool) []byte {
	// Read file content
	fileContent, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
		os.Exit(1)
	}

	// Write blob object using common function
	return WriteGitObject(BlobObject, fileContent, hexCoded)
}
