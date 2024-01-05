package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/adonSh/depot/libdepot"
	"golang.org/x/term"
)

const (
	// Commands
	actStow  = "stow"
	actFetch = "fetch"
	actDrop  = "drop"
	actHelp  = "help"

	// Environment Variables
	envPath = "DEPOT_PATH"
	envPass = "DEPOT_PASS"
)

func main() {
	// Parse command line
	log.SetFlags(0)
	action, key, secret, newline, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("Invalid args: %v\n", err)
	}

	// Initialize
	dbPath, err := choosePath()
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	storage, err := libdepot.NewDepot(dbPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Do the thing
	switch action {
	case actStow:
		val, err := getVal(secret)
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}

		password, err := getPassword(secret)
		if err != nil {
			log.Fatalf("Error: %v\n")
		}

		err = storage.Stow(key, strings.TrimSpace(val), password)
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case actFetch:
		val := storage.Peek(key)
		if val == "" {
			password, err := getPassword(true)
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			val, err = storage.Fetch(key, password)
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}
		}

		if newline {
			fmt.Println(val)
		} else {
			fmt.Print(val)
		}
	case actDrop:
		err = storage.Drop(key)
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case actHelp:
		fmt.Println(usage())
	default:
		log.Fatalf("Unrecognized action: %v\n", action)
	}
}

// Returns the key, options, and action to perform specified in the
// command-line arguments or an error if unsuccessful
func parseArgs(args []string) (string, string, bool, bool, error) {
	action := ""
	key := ""
	secret := false
	newline := true

	for _, a := range args {
		if a == "-h" || a == "--help" || a == "-?" || a == actHelp {
			return actHelp, key, secret, newline, nil
		}

		if strings.HasPrefix(a, "-") {
			secret = secret || strings.Contains(a, "s")
			newline = !(!newline || strings.Contains(a, "n"))
		} else if action == "" {
			action = a
		} else if key == "" {
			key = a
		} else {
			return action, key, secret, newline, fmt.Errorf("one key at a time")
		}
	}

	if action == "" {
		return action, key, secret, newline, fmt.Errorf("no action specified")
	}
	if key == "" {
		return action, key, secret, newline, fmt.Errorf("no key specified")
	}

	return action, key, secret, newline, nil
}

// Returns the location of the database in the filesystem depending on the
// environment or an error if unsuccessful
func choosePath() (string, error) {
	path := os.Getenv(envPath)
	if path != "" {
		return path, nil
	}

	basedir := os.Getenv("XDG_CONFIG_HOME")
	if basedir == "" {
		path = filepath.Join(os.Getenv("HOME"), ".depot")
	} else {
		path = filepath.Join(basedir, "depot")
	}

	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return path, err
	}

	return filepath.Join(path, "depot.db"), nil
}

// Returns the password from either an environment variable or console input.
// If secret is false, returns nil. Returns an error if unsuccessful.
func getPassword(secret bool) ([]byte, error) {
	if !secret {
		return nil, nil
	}

	if p := os.Getenv(envPass); p != "" {
		return []byte(p), nil
	}

	tty, err := os.Create("/dev/tty")
	if err != nil {
		return nil, err
	}
	defer tty.Close()

	fmt.Fprint(tty, "PASSWORD: ")
	password, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty, "")
	if err != nil {
		return nil, err
	}

	return password, nil
}

// Returns the value read from stdin or an error if unsuccessful
func getVal(secret bool) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) && secret {
		val, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		return string(val), nil
	}

	return bufio.NewReader(os.Stdin).ReadString('\n')
}

// Returns the help message
func usage() string {
	return strings.Join([]string{
		"Usage: depot [-nsh?] <action> <key>",
		"",
		"Actions:",
		"    stow        Read a value from stdin and associate it with the given key",
		"    fetch       Print the value associated with the given key to stdout",
		"    drop        Remove the given key from the depot",
		"",
		"Options:",
		"    -n          No newline character will be printed after fetching a value",
		"    -s          The provided value is secret and will be encrypted",
		"    -h, -?      Print this help message and exit",
		"",
		"Environment Variables:",
		"    DEPOT_PATH  Specifies a non-standard path to the depot's database",
		"                (Defaults to $XDG_CONFIG_HOME/depot/depot.db)",
		"    DEPOT_PASS  Specifies the password to be used to encrypt/decrypt values",
		"                (Be careful with this! It is certainly less secure!)",
	}, "\n")
}
