package commands

import (
	"fmt"
	"os"
	"strings"
)

type CatFileCommand struct{}

func (c *CatFileCommand) Execute(cmd *Command) {
	if len(cmd.Args) != 2 {
		fmt.Println(cmd.Usage)
		os.Exit(1)
	}

	// Read and decompress the git object
	data, err := ReadGitObject(cmd.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading git object: %s\n", err)
		os.Exit(1)
	}

	// Parse the object to extract content
	_, content, err := ParseGitObject(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing git object: %s\n", err)
		os.Exit(1)
	}

	// Print the content to stdout
	fmt.Printf("%s", strings.TrimSuffix(string(content), "\n"))
}

func (c *CatFileCommand) GetName() string {
	return "cat-file"
}
