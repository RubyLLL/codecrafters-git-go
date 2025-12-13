package commands

import (
	"bytes"
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
		// Format: <mode> <name>\0<20_byte_sha>
		fmt.Fprintf(&treeContent, "%s %s\x00", entry.Mode, entry.Name)
		treeContent.Write(entry.SHA)
	}

	// Write tree object using common function
	sha := WriteGitObject(TreeObject, treeContent.Bytes(), true)
	return string(sha)
}

func GetMode(info os.FileInfo) string {
	// Check if file is executable
	if info.Mode()&0111 != 0 {
		return "100755" // executable file
	}
	return "100644" // regular file
}
