package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// GitObjectType represents the type of Git object
type GitObjectType string

const (
	BlobObject   GitObjectType = "blob"
	TreeObject   GitObjectType = "tree"
	CommitObject GitObjectType = "commit"
)

// WriteGitObject writes a Git object to the .git/objects directory
// Returns the SHA-1 hash as a hex string or raw bytes based on hexEncoded flag
func WriteGitObject(objectType GitObjectType, content []byte, hexEncoded bool) []byte {
	// Create object with header: <type> <size>\0<content>
	header := fmt.Sprintf("%s %d\x00", objectType, len(content))
	fullContent := append([]byte(header), content...)

	// Compress the content
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, err := w.Write(fullContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compressing git object: %s\n", err)
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
		fmt.Fprintf(os.Stderr, "Error writing git object to disk: %s\n", err)
		os.Exit(1)
	}

	if hexEncoded {
		return []byte(sha)
	}
	return objSHA[:]
}

// ReadGitObject reads and decompresses a Git object from .git/objects
// Returns the decompressed content (including header)
func ReadGitObject(sha string) ([]byte, error) {
	// Read from .git/objects/<first_2>/<remaining_38>
	dirName := sha[:2]
	fileName := sha[2:]
	filePath := fmt.Sprintf(".git/objects/%s/%s", dirName, fileName)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading git object: %w", err)
	}

	// Decompress using zlib
	reader, err := zlib.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("error creating zlib reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading from zlib reader: %w", err)
	}

	return decompressed, nil
}

// ParseGitObject parses a Git object and returns its type and content
func ParseGitObject(data []byte) (GitObjectType, []byte, error) {
	// Find the null byte that separates header from content
	nullIndex := bytes.IndexByte(data, 0)
	if nullIndex == -1 {
		return "", nil, fmt.Errorf("invalid git object: no null byte found")
	}

	// Parse header: <type> <size>
	header := string(data[:nullIndex])
	content := data[nullIndex+1:]

	// Extract type
	parts := bytes.Split([]byte(header), []byte(" "))
	if len(parts) < 2 {
		return "", nil, fmt.Errorf("invalid git object header: %s", header)
	}

	objectType := GitObjectType(parts[0])
	return objectType, content, nil
}
