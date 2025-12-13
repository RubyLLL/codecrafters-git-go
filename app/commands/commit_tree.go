package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

type CommitTreeCommand struct{}

func (c *CommitTreeCommand) GetName() string {
	return "commit-tree"
}

func (c *CommitTreeCommand) Execute(cmd *Command) error {
	// Parse command line arguments
	// Format: commit-tree <tree_sha> -p <parent_sha> -m <message>
	// or:     commit-tree <tree_sha> -m <message>
	if len(cmd.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: commit-tree <tree_sha> [-p <parent_sha>] -m <message>\n")
		os.Exit(1)
	}

	var treeSHA, parentSHA, commitMsg string
	treeSHA = cmd.Args[0]

	// Parse flags
	for i := 1; i < len(cmd.Args); i++ {
		switch cmd.Args[i] {
		case "-p":
			if i+1 < len(cmd.Args) {
				parentSHA = cmd.Args[i+1]
				i++
			}
		case "-m":
			if i+1 < len(cmd.Args) {
				commitMsg = cmd.Args[i+1]
				i++
			}
		}
	}

	// Get current timestamp
	timestamp := time.Now().Unix()

	// Compose commit content
	var contentBytes bytes.Buffer
	/*
		Commit format:
		tree <tree_sha>
		parent <parent_sha>  (optional)
		author <name> <email> <timestamp> <timezone>
		committer <name> <email> <timestamp> <timezone>

		<commit message>
	*/
	fmt.Fprintf(&contentBytes, "tree %s\n", treeSHA)
	if parentSHA != "" {
		fmt.Fprintf(&contentBytes, "parent %s\n", parentSHA)
	}
	fmt.Fprintf(&contentBytes, "author %s <%s> %d +0000\n", "Xiaoyue Lyu", "xlyu00green@gmail.com", timestamp)
	fmt.Fprintf(&contentBytes, "committer %s <%s> %d +0000\n", "Xiaoyue Lyu", "xlyu00green@gmail.com", timestamp)
	fmt.Fprintf(&contentBytes, "\n%s\n", commitMsg)

	// Create tree object with header
	header := fmt.Sprintf("commit %d\x00", contentBytes.Len())
	fullContent := append([]byte(header), contentBytes.Bytes()...)

	// Compress the content
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, err := w.Write(fullContent)
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

	fmt.Print(sha)
	return nil
}
