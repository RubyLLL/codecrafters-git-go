package commands

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CloneCommand struct{}

func (c *CloneCommand) Execute(cmd *Command) error {
	if len(cmd.Args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: clone <repository> [<directory>]\n")
		os.Exit(1)
	}

	repoURL := cmd.Args[0]
	directory := ""
	if len(cmd.Args) > 1 {
		directory = cmd.Args[1]
	} else {
		// Derive directory name from URL
		parts := strings.Split(repoURL, "/")
		directory = strings.TrimSuffix(parts[len(parts)-1], ".git")
	}

	fmt.Printf("Cloning into '%s'...\n", directory)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Change to the new directory
	oldDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %w", err)
	}
	defer os.Chdir(oldDir) // Restore original directory when done

	if err := os.Chdir(directory); err != nil {
		return fmt.Errorf("error changing to directory: %w", err)
	}

	// Initialize the repository structure
	if err := initRepository(); err != nil {
		return fmt.Errorf("error initializing repository: %w", err)
	}

	// Discover references from the remote repository
	references, err := DiscoverReferences(repoURL)
	if err != nil {
		return fmt.Errorf("error discovering references: %w", err)
	}

	// Negotiate with the server to determine what to fetch
	headRef, err := NegotiateWithServer(repoURL, references)
	if err != nil {
		return fmt.Errorf("error negotiating with server: %w", err)
	}

	// Fetch the packfile containing the objects
	packfileData, err := FetchPackfile(repoURL, headRef)
	if err != nil {
		return fmt.Errorf("error fetching packfile: %w", err)
	}

	// Validate packfile data
	if len(packfileData) < 12 {
		return fmt.Errorf("packfile too short: %d bytes", len(packfileData))
	}

	// Parse and store the objects from the packfile
	if err := ParsePackfile(packfileData); err != nil {
		return fmt.Errorf("error parsing packfile: %w", err)
	}

	// Set up the HEAD reference
	if err := setupHeadReference(headRef, references); err != nil {
		return fmt.Errorf("error setting up HEAD reference: %w", err)
	}

	// Try to checkout the working directory
	if err := checkoutWorkingDirectory(headRef); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error checking out working directory: %v\n", err)
	}

	fmt.Println("Repository cloned successfully!")
	return nil
}

func (c *CloneCommand) GetName() string {
	return "clone"
}

// initRepository initializes the basic .git directory structure
func initRepository() error {
	dirs := []string{".git", ".git/objects", ".git/refs", ".git/refs/heads", ".git/refs/tags"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	// Create HEAD file pointing to refs/heads/main as default
	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
		return fmt.Errorf("error writing HEAD file: %w", err)
	}

	return nil
}

// setupHeadReference sets up the HEAD reference and the default branch
func setupHeadReference(headSHA string, references map[string]string) error {
	// Find the branch name that corresponds to the HEAD SHA
	var branchName string
	for ref, sha := range references {
		if sha == headSHA && strings.HasPrefix(ref, "refs/heads/") {
			branchName = strings.TrimPrefix(ref, "refs/heads/")
			break
		}
	}

	if branchName == "" {
		// Default to main if we can't find a matching branch
		branchName = "main"
	}

	// Create the branch reference file
	branchRefPath := filepath.Join(".git", "refs", "heads", branchName)
	if err := os.WriteFile(branchRefPath, []byte(headSHA+"\n"), 0644); err != nil {
		return fmt.Errorf("error writing branch reference: %w", err)
	}

	// Update HEAD to point to the correct branch
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", branchName)
	if err := os.WriteFile(".git/HEAD", []byte(headContent), 0644); err != nil {
		return fmt.Errorf("error updating HEAD: %w", err)
	}

	return nil
}

// checkoutWorkingDirectory checks out files from the repository into the working directory
func checkoutWorkingDirectory(commitSHA string) error {
	// Read the commit object to get the tree SHA
	commitData, err := ReadGitObject(commitSHA)
	if err != nil {
		return fmt.Errorf("error reading commit object: %w", err)
	}

	// Parse the commit to get the tree SHA
	// Commit format: "tree <tree_sha>\nparent ...\nauthor ...\ncommitter ...\n\n<message>"
	commitLines := strings.Split(string(commitData), "\n")
	if len(commitLines) < 1 {
		return fmt.Errorf("invalid commit format")
	}

	treeLine := commitLines[0]
	if !strings.HasPrefix(treeLine, "tree ") {
		return fmt.Errorf("invalid commit format: missing tree line")
	}

	treeSHA := strings.TrimSpace(strings.TrimPrefix(treeLine, "tree "))
	fmt.Printf("Checking out tree: %s\n", treeSHA)

	// Read and parse the tree object
	return checkoutTree(treeSHA, ".")
}

// checkoutTree recursively checks out a tree object to the filesystem
func checkoutTree(treeSHA, basePath string) error {
	fmt.Printf("Checking out tree %s to %s\n", treeSHA, basePath)

	// Read the tree object
	treeData, err := ReadGitObject(treeSHA)
	if err != nil {
		return fmt.Errorf("error reading tree object %s: %w", treeSHA, err)
	}

	// Parse the tree data
	// Tree format: [<mode> <name>\0<20_byte_sha>]*
	offset := 0
	entryCount := 0
	for offset < len(treeData) {
		entryCount++
		// Parse mode and name
		nullIndex := bytes.IndexByte(treeData[offset:], 0)
		if nullIndex == -1 {
			break
		}

		entryData := treeData[offset : offset+nullIndex]
		spaceIndex := bytes.IndexByte(entryData, ' ')
		if spaceIndex == -1 {
			offset += nullIndex + 21 // Skip this entry (mode + name + \0 + 20 byte sha)
			continue
		}

		mode := string(entryData[:spaceIndex])
		name := string(entryData[spaceIndex+1:])
		sha := treeData[offset+nullIndex+1 : offset+nullIndex+21]
		shaHex := hex.EncodeToString(sha)

		fullPath := filepath.Join(basePath, name)
		fmt.Printf("  Entry %d: mode=%s name=%s sha=%s path=%s\n", entryCount, mode, name, shaHex, fullPath)

		// Determine entry type from mode
		if mode == "40000" {
			// Directory
			fmt.Printf("    Creating directory: %s\n", fullPath)
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return fmt.Errorf("error creating directory %s: %w", fullPath, err)
			}
			// Recursively checkout subtree
			if err := checkoutTree(shaHex, fullPath); err != nil {
				return fmt.Errorf("error checking out subtree %s: %w", fullPath, err)
			}
		} else {
			// File - read the blob and write it to disk
			fmt.Printf("    Creating file: %s\n", fullPath)
			blobData, err := ReadGitObject(shaHex)
			if err != nil {
				return fmt.Errorf("error reading blob %s: %w", shaHex, err)
			}

			// Write file with appropriate permissions
			perm := os.FileMode(0644)
			if mode == "100755" {
				perm = 0755 // Executable
			}

			// Ensure parent directory exists
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("error creating parent directory for %s: %w", fullPath, err)
			}

			if err := os.WriteFile(fullPath, blobData, perm); err != nil {
				return fmt.Errorf("error writing file %s: %w", fullPath, err)
			}
		}

		offset += nullIndex + 21
	}

	fmt.Printf("Finished checking out tree %s: processed %d entries\n", treeSHA, entryCount)
	return nil
}