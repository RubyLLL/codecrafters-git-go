package commands

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
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

	// Read from the file
	filePath := fmt.Sprintf(".git/objects/%s/%s", cmd.Args[1][:2], cmd.Args[1][2:])
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("failed to read the file")
		os.Exit(1)
	}

	// Decompress the content
	b := bytes.NewReader(fileBytes)
	r, err := zlib.NewReader(b)
	if err != nil {
		fmt.Println("failed to decompress the content")
		os.Exit(1)
	}
	defer r.Close()

	decompressedBytes, err := io.ReadAll(r)
	if err != nil {
		fmt.Println("failed to read the content")
		os.Exit(1)
	}

	// Parse the content
	var result string
	//   tree <size>\0<mode> <name>\0<20_byte_sha><mode> <name>\0<20_byte_sha>
	lines := bytes.Split(decompressedBytes, []byte("\x00"))[1:]
	for _, line := range lines {
		parts := bytes.Split(line, []byte(" "))
		if len(parts) >= 2 {
			result = fmt.Sprintf("%s\n%s", result, string(parts[len(parts)-1]))
		}
	}
	result += "\n"
	fmt.Printf("%s", strings.TrimPrefix(result, "\n"))
}
