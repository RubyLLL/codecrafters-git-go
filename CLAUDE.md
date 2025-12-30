# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is a Git implementation project written in Go for the CodeCrafters "Build Your Own Git" challenge. The project implements a subset of Git functionality capable of initializing repositories, creating commits, and handling Git's core data model (blobs, trees, commits).

## Code Architecture

### Command Layer Pattern
The codebase follows a command pattern architecture:
- `commands.Command`: Base struct containing args and usage string
- `commands.CommandRunner`: Interface with `GetName()` and `Execute()` methods
- Each Git command is implemented as a separate struct that handles its specific logic

### Git Object Model
Core Git object types and handling are defined in `app/commands/git_object.go`:
- `BlobObject`, `TreeObject`, `CommitObject` types
- `WriteGitObject()`: Creates Git objects with zlib compression and SHA-1 hashing
- `ReadGitObject()`: Reads and decompresses Git objects using zlib
- `ParseGitObject()`: Parses object headers to extract type and content

### Implemented Git Commands
- `init`: Creates `.git` directory, objects, refs, and HEAD file (`app/main.go`)
- `cat-file`: Reads and displays content of a Git object (`app/commands/cat_file.go`)
- `hash-object`: Computes SHA-1 hash of a file and stores as blob (`app/commands/hash_object.go`)
- `ls-tree`: Lists contents of a tree object (`app/commands/ls_tree.go`)
- `write-tree`: Creates a tree object from current directory (`app/commands/write_tree.go`)
- `commit-tree`: Creates a commit object from tree + optional parent (`app/commands/commit_tree.go`)

## Development Commands

### Building the Project
```bash
# Local build (uses your_program.sh script)
go build -o /tmp/codecrafters-build-git-go app/*.go

# Or simply run the script
./your_program.sh
```

### Running the Program
```bash
# Test locally (should be run in a separate directory to avoid damaging .git folder)
mkdir -p /tmp/testing && cd /tmp/testing
/path/to/your/repo/your_program.sh <command> [<args>...]

# Or with an alias for convenience
alias mygit=/path/to/your/repo/your_program.sh
mkdir -p /tmp/testing && cd /tmp/testing
mygit <command> [<args>...]
```

### Testing Approach
The project uses CodeCrafters' automated CLI testing framework. There are no local unit tests - testing is done via:
- CodeCrafters remote test runner
- Manual testing via `your_program.sh` in a separate directory

## Key Implementation Details

1. **Git Object Format**: Objects are stored with headers: `<type> <size>\0<content>`
2. **Compression**: Uses zlib for all Git objects
3. **Hashing**: SHA-1 hashes are computed on the pre-compressed content
4. **Storage Convention**: Objects stored at `.git/objects/<first_2_hex>/<remaining_38_hex>`
5. **Tree Entries**: Format: `<mode> <name>\0<20_byte_sha>`

## File Structure
```
app/
├── main.go              # Entry point with command router
└── commands/
    ├── commands.go      # Command interface and base struct
    ├── cat_file.go      # git cat-file command
    ├── commit_tree.go   # git commit-tree command
    ├── git_object.go    # Core Git object handling (read/write/parse)
    ├── hash_object.go   # git hash-object command
    ├── ls_tree.go       # git ls-tree command
    └── write_tree.go    # git write-tree command
```