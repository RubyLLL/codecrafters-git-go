package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
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
	// 1. Compose blob content
	fileContent, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}
	contentLength := len(fileContent)

	content := fmt.Appendf(nil, "blob %d\x00%s", contentLength, fileContent)

	// 2. Compress the content
	b := new(bytes.Buffer)
	w := zlib.NewWriter(b)
	_, err = w.Write(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compressing git object: %s\n", err)
		return []byte("\n")
	}
	w.Close()

	// 3. Generate SHA-1 hash
	objSHA := sha1.Sum(content)
	sha := hex.EncodeToString(objSHA[:])

	// 4. Write to the file
	dirName := sha[:2]
	fileName := sha[2:]
	if err := os.MkdirAll(".git/objects/"+dirName, 0755); err != nil {
		fmt.Println("Error creating directory:", err)
		os.Exit(1)
	}
	err = os.WriteFile(".git/objects/"+dirName+"/"+fileName, b.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing git object to disk: %s\n", err)
		return []byte("\n")
	}

	if hexCoded {
		return []byte(sha)
	}
	return objSHA[:]
}
