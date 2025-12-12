package commands

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"strings"
)

type CatFileCommand struct{}

func (c *CatFileCommand) Execute(cmd *Command) {
	if len(cmd.Args) != 2 {
		fmt.Println(cmd.Usage)
		os.Exit(1)
	}

	// 1. Read the contents of the blob object file from the .git/objects directory
	dir := cmd.Args[1][:2]
	file := cmd.Args[1][2:]

	content, err := os.ReadFile(".git/objects/" + dir + "/" + file)
	if err != nil {
		fmt.Println("Error opening file:", err)
		os.Exit(1)
	}

	// 2. Decompress the contents using Zlib
	z, err := zlib.NewReader(bytes.NewReader(content))
	if err != nil {
		fmt.Println("Error creating zlib reader:", err)
		os.Exit(1)
	}
	defer z.Close()

	p, err := io.ReadAll(z)
	if err != nil {
		fmt.Println("Error reading from zlib reader:", err)
		os.Exit(1)
	}

	// 3. Extract the actual "content" from the decompressed data
	contentStr := string(p)
	actualContent := contentStr[5:] // chop off the "blob " prefix
	parts := strings.Split(actualContent, "\x00")

	// 4.Print the content to stdout
	fmt.Printf("%s", strings.TrimSuffix(parts[1], "\n"))
}

func (c *CatFileCommand) GetName() string {
	return "cat-file"
}
