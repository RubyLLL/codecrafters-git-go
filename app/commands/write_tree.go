package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type WriteTreeCommand struct{}

type TreeEntry struct {
	Mode string
	Name string
	SHA  []byte
}

func (c *WriteTreeCommand) GetName() string {
	return "write-tree"
}

func (c *WriteTreeCommand) Execute(cmd *Command) error {
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %s\n", err)
		return err
	}

	sha := CreateTree(workDir)
	fmt.Print(sha)
	return nil
}

func CreateTree(dirPath string) string {
	var entries []TreeEntry

	// Read directory entries
	files, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory: %s\n", err)
		os.Exit(1)
	}

	for _, file := range files {
		// Skip .git directory
		if file.Name() == ".git" {
			continue
		}

		fullPath := filepath.Join(dirPath, file.Name())
		info, err := file.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting file info: %s\n", err)
			continue
		}

		var sha []byte
		var mode string

		if file.IsDir() {
			// Recursively create tree for subdirectory
			subTreeSHA := CreateTree(fullPath)
			sha, _ = hex.DecodeString(subTreeSHA)
			mode = "40000" // directory mode
		} else {
			// Create blob for file
			sha = CreateBlob(fullPath, false)
			mode = GetMode(info)
		}

		entries = append(entries, TreeEntry{
			Mode: mode,
			Name: file.Name(),
			SHA:  sha,
		})
	}

	// Sort entries alphabetically by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	// Build tree content
	var treeContent bytes.Buffer
	for _, entry := range entries {
		// <mode> <name>\0<20_byte_sha>
		fmt.Fprintf(&treeContent, "%s %s\x00", entry.Mode, entry.Name)
		treeContent.Write(entry.SHA)
	}

	// Create tree object with header
	header := fmt.Sprintf("tree %d\x00", treeContent.Len())
	fullContent := append([]byte(header), treeContent.Bytes()...)

	// Compress the content
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, err = w.Write(fullContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compressing tree object: %s\n", err)
		os.Exit(1)
	}
	w.Close()

	// Generate SHA-1 hash
	objSHA := sha1.Sum(fullContent)
	sha := hex.EncodeToString(objSHA[:])

	// Write to .git/objects directory
	dirName := sha[:2]
	fileName := sha[2:]
	if err := os.MkdirAll(".git/objects/"+dirName, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
		os.Exit(1)
	}
	err = os.WriteFile(".git/objects/"+dirName+"/"+fileName, compressed.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing tree object to disk: %s\n", err)
		os.Exit(1)
	}

	return sha
}

func GetMode(info os.FileInfo) string {
	// Check if file is executable
	if info.Mode()&0111 != 0 {
		return "100755" // executable file
	}
	return "100644" // regular file
}
