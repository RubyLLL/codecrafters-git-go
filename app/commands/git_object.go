package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
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

// ParsePktLine parses a pkt-line formatted response
func ParsePktLine(data []byte) (string, []byte, error) {
	if len(data) < 4 {
		return "", nil, fmt.Errorf("invalid pkt-line: too short")
	}

	// First 4 bytes are the hex length
	lengthStr := string(data[:4])
	length, err := strconv.ParseInt(lengthStr, 16, 32)
	if err != nil {
		return "", nil, fmt.Errorf("invalid pkt-line length: %s", lengthStr)
	}

	// Handle special packet types
	if length == 0 {
		return "flush", nil, nil
	}

	if length < 4 {
		return "", nil, fmt.Errorf("invalid pkt-line length: %d", length)
	}

	// Extract the payload
	payload := data[4:length]
	return "data", payload, nil
}

// ReadPktLines reads all pkt-lines from a response
func ReadPktLines(resp *http.Response) ([]string, error) {
	var lines []string
	buf := make([]byte, 1024)

	for {
		n, err := resp.Body.Read(buf)
		if n == 0 {
			break
		}
		if err != nil && err != io.EOF {
			return nil, err
		}

		// Process the buffer in pkt-line chunks
		data := buf[:n]
		for len(data) > 0 {
			if len(data) < 4 {
				break
			}

			lengthStr := string(data[:4])
			length, err := strconv.ParseInt(lengthStr, 16, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid pkt-line length: %s", lengthStr)
			}

			if length == 0 {
				// Flush packet
				lines = append(lines, "")
				data = data[4:]
				continue
			}

			if int(length) > len(data) {
				// Not enough data, need to read more
				break
			}

			payload := string(data[4:length])
			lines = append(lines, strings.TrimSpace(payload))
			data = data[length:]
		}

		if err == io.EOF {
			break
		}
	}

	return lines, nil
}

// MakePktLine creates a pkt-line from data
func MakePktLine(data string) string {
	if data == "" {
		return "0000" // flush packet
	}

	length := len(data) + 4 // +4 for the length header itself
	lengthStr := fmt.Sprintf("%04x", length)
	return lengthStr + data
}

// DiscoverReferences discovers references from a remote repository
func DiscoverReferences(repoURL string) (map[string]string, error) {
	// Ensure the URL ends with .git for HTTP Git requests
	if !strings.HasSuffix(repoURL, ".git") {
		repoURL += ".git"
	}

	// Make HTTP request to info/refs endpoint
	url := repoURL + "/info/refs?service=git-upload-pack"

	// Create HTTP request with proper headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set required Git headers
	req.Header.Set("User-Agent", "git/2.0 (CodeCrafters)")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching references: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching references: HTTP %d", resp.StatusCode)
	}

	// Parse pkt-line responses
	lines, err := ReadPktLines(resp)
	if err != nil {
		return nil, fmt.Errorf("error parsing pkt-lines: %w", err)
	}

	// Parse references from the response
	references := make(map[string]string)
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse reference line: <sha-1> <refname>
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			sha := parts[0]
			ref := strings.TrimSpace(parts[1])
			references[ref] = sha
		}
	}

	return references, nil
}

// NegotiateWithServer negotiates with the server to determine what objects to fetch
func NegotiateWithServer(repoURL string, references map[string]string) (string, error) {
	// For simplicity, we'll just want the HEAD reference
	headRef, exists := references["HEAD"]
	if !exists {
		// Try to find a default branch
		if mainRef, exists := references["refs/heads/main"]; exists {
			headRef = mainRef
		} else if masterRef, exists := references["refs/heads/master"]; exists {
			headRef = masterRef
		} else {
			return "", fmt.Errorf("no suitable reference found")
		}
	}

	return headRef, nil
}

// Packfile object types
const (
	OBJ_COMMIT    = 1
	OBJ_TREE      = 2
	OBJ_BLOB      = 3
	OBJ_TAG       = 4
	OBJ_OFS_DELTA = 6
	OBJ_REF_DELTA = 7
)

// ParsePackfile parses a packfile and extracts objects
func ParsePackfile(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("packfile too short")
	}

	// Check header
	if string(data[:4]) != "PACK" {
		return fmt.Errorf("invalid packfile header")
	}

	// Parse version (should be 2)
	version := (uint32(data[4]) << 24) | (uint32(data[5]) << 16) | (uint32(data[6]) << 8) | uint32(data[7])
	if version != 2 {
		return fmt.Errorf("unsupported packfile version: %d", version)
	}

	// Parse object count
	objectCount := (uint32(data[8]) << 24) | (uint32(data[9]) << 16) | (uint32(data[10]) << 8) | uint32(data[11])

	fmt.Printf("Parsing packfile with %d objects\n", objectCount)

	// If object count is 0, there's nothing to parse
	if objectCount == 0 {
		fmt.Printf("No objects to parse in packfile\n")
		return nil
	}

	// Process objects
	offset := 12
	for i := uint32(0); i < objectCount; i++ {
		if offset >= len(data) {
			return fmt.Errorf("unexpected end of packfile at object %d", i)
		}

		// Parse object header
		objType, _, headerSize := parseObjectHeader(data[offset:])
		if headerSize == 0 {
			return fmt.Errorf("invalid object header at object %d", i)
		}

		offset += headerSize

		if offset >= len(data) {
			return fmt.Errorf("unexpected end of packfile after header of object %d", i)
		}

		// Handle different object types
		switch objType {
		case OBJ_COMMIT, OBJ_TREE, OBJ_BLOB, OBJ_TAG:
			// Regular object - decompress with zlib
			// We need to decompress the data starting from the current offset
			reader := bytes.NewReader(data[offset:])
			zlibReader, err := zlib.NewReader(reader)
			if err != nil {
				return fmt.Errorf("error creating zlib reader for object %d: %w", i, err)
			}

			content, err := io.ReadAll(zlibReader)
			if err != nil {
				zlibReader.Close()
				return fmt.Errorf("error reading zlib data for object %d: %w", i, err)
			}
			zlibReader.Close()

			// Determine how much data was consumed by checking the reader position
			consumed := int(reader.Size()) - int(reader.Len())

			// Determine object type
			var gitObjType GitObjectType
			switch objType {
			case OBJ_COMMIT:
				gitObjType = CommitObject
			case OBJ_TREE:
				gitObjType = TreeObject
			case OBJ_BLOB:
				gitObjType = BlobObject
			default:
				return fmt.Errorf("unsupported object type: %d", objType)
			}

			// Store the object
			shaBytes := WriteGitObject(gitObjType, content, true)
			_ = shaBytes // We don't need to use this for now

			// Move offset forward by the amount of data consumed
			offset += consumed

		case OBJ_OFS_DELTA, OBJ_REF_DELTA:
			// Delta object - for MVP, we'll skip these
			// In a full implementation, we'd need to resolve the base object and apply the delta

			// Skip some data for now - in a real implementation we'd properly parse this
			// For now, let's try to parse the delta header to get the correct size
			deltaHeaderSize := parseDeltaHeaderSize(data[offset:])
			offset += deltaHeaderSize

			// For delta objects, we need to consume the compressed data as well
			// Let's try to read it with a zlib reader to see how much data it consumes
			if offset < len(data) {
				reader := bytes.NewReader(data[offset:])
				zlibReader, err := zlib.NewReader(reader)
				if err != nil {
					// If we can't create a zlib reader, just skip a fixed amount
					if offset + 50 < len(data) {
						offset += 50
					} else {
						offset = len(data)
					}
				} else {
					// Read the data to consume it
					_, err := io.ReadAll(zlibReader)
					zlibReader.Close()
					if err != nil {
						// If there's an error, skip a fixed amount
						if offset + 50 < len(data) {
							offset += 50
						} else {
							offset = len(data)
						}
					} else {
						// Calculate how much data was consumed
						consumed := int(reader.Size()) - int(reader.Len())
						offset += consumed
					}
				}
			}

		default:
			return fmt.Errorf("unknown object type: %d", objType)
		}

		if offset >= len(data) {
			if i < objectCount-1 {
				return fmt.Errorf("unexpected end of packfile after object %d of %d", i+1, objectCount)
			}
			break
		}
	}

	return nil
}

// parseDeltaHeaderSize parses the size information from a delta object header
func parseDeltaHeaderSize(data []byte) int {
	offset := 0

	// Parse base object size
	for offset < len(data) && (data[offset] & 0x80) != 0 {
		offset++
	}
	if offset < len(data) {
		offset++
	}

	// Parse result object size
	for offset < len(data) && (data[offset] & 0x80) != 0 {
		offset++
	}
	if offset < len(data) {
		offset++
	}

	return offset
}

// parseObjectHeader parses the variable-length header of a packfile object
func parseObjectHeader(data []byte) (int, int, int) {
	if len(data) == 0 {
		return 0, 0, 0
	}

	// Parse the first byte
	firstByte := data[0]
	objType := (firstByte >> 4) & 0x7
	size := int(firstByte & 0xF)

	// Check if more bytes follow
	if (firstByte & 0x80) == 0 {
		// No more bytes
		return int(objType), size, 1
	}

	// Parse additional bytes for size
	offset := 1
	shift := 4
	for offset < len(data) {
		b := data[offset]
		size |= int(b&0x7F) << shift
		shift += 7
		offset++

		if (b & 0x80) == 0 {
			break
		}
	}

	return int(objType), size, offset
}

// FetchPackfile fetches a packfile from a remote repository
func FetchPackfile(repoURL, wantSHA string) ([]byte, error) {
	// Ensure the URL ends with .git for HTTP Git requests
	if !strings.HasSuffix(repoURL, ".git") {
		repoURL += ".git"
	}

	// Make HTTP POST request to git-upload-pack endpoint
	url := repoURL + "/git-upload-pack"

	// Create the request body with want and done commands
	body := MakePktLine(fmt.Sprintf("want %s side-band-64k\n", wantSHA))
	body += MakePktLine("") // flush packet
	body += MakePktLine("done\n")

	// Create HTTP request
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	fmt.Printf("Sending request to %s\n", url)
	fmt.Printf("Request body: %q\n", body)

	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching packfile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching packfile: HTTP %d", resp.StatusCode)
	}

	// Read the raw response body first
	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	fmt.Printf("Raw response size: %d bytes\n", len(rawData))
	if len(rawData) > 20 {
		fmt.Printf("First 20 bytes: %q\n", string(rawData[:20]))
	}

	// Try to unwrap the response body (handle side-band encoding)
	data, err := unwrapSideBand(bytes.NewReader(rawData))
	if err != nil {
		return nil, fmt.Errorf("error unwrapping side-band: %w", err)
	}
	fmt.Printf("Unwrapped data size: %d bytes\n", len(data))
	if len(data) > 20 {
		fmt.Printf("First 20 bytes of unwrapped data: %q\n", string(data[:20]))
	}

	return data, nil
}

// unwrapSideBand unwraps side-band encoded data
func unwrapSideBand(reader io.Reader) ([]byte, error) {
	var result []byte
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Process the data in pkt-line chunks
	for len(data) > 0 {
		if len(data) < 4 {
			// Not enough data for a pkt-line header
			break
		}

		lengthStr := string(data[:4])
		length, err := strconv.ParseInt(lengthStr, 16, 32)
		if err != nil {
			// Not a pkt-line, treat as raw data (this shouldn't happen in a proper Git response)
			return nil, fmt.Errorf("invalid pkt-line header: %q", lengthStr)
		}

		if length == 0 {
			// Flush packet
			data = data[4:]
			continue
		}

		if int(length) > len(data) {
			// Not enough data
			return nil, fmt.Errorf("incomplete pkt-line: expected %d bytes, got %d", length, len(data))
		}

		// Extract payload (skip the first byte which is the side-band channel)
		if length >= 5 { // Need at least 4 bytes for header + 1 byte for channel
			channel := data[4]
			payload := data[5:length]

			// Channel 1 is packfile data
			if channel == 1 {
				result = append(result, payload...)
			}
			// Channel 2 is progress messages (we can ignore)
			// Channel 3 is error messages (we should handle)
		}

		data = data[length:]
	}

	return result, nil
}
