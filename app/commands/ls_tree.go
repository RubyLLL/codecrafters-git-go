package commands

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

type LsTreeComand struct{}

func (c *LsTreeComand) GetName() string {
	return "ls-tree"
}

func (c *LsTreeComand) Execute(cmd *Command) {
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

	// Parse the tree object
	_, content, err := ParseGitObject(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing git object: %s\n", err)
		os.Exit(1)
	}

	// Parse tree entries: <mode> <name>\0<20_byte_sha>
	var result string
	lines := bytes.Split(content, []byte("\x00"))
	for _, line := range lines {
		parts := bytes.Split(line, []byte(" "))
		if len(parts) >= 2 {
			result = fmt.Sprintf("%s\n%s", result, string(parts[len(parts)-1]))
		}
	}
	result += "\n"
	fmt.Printf("%s", strings.TrimPrefix(result, "\n"))
}
