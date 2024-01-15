package libdepot

import (
	"errors"
	"log"
	"testing"
)

var db *Depot

func init() {
	var err error
	db, err = NewDepot("test.db")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err.Error())
	}
}

func TestPlain(t *testing.T) {
	key := "plaintext"
	data := "testing123"

	// Stow
	err := db.Stow(key, data, nil)
	if err != nil {
		t.Errorf("error inserting plaintext into database: %v", err.Error())
	}

	// Peek
	val, err := db.Peek(key)
	if err != nil {
		t.Errorf("unexpected error in Peek(): %v", err)
	}
	if val == "" {
		t.Errorf("expected to be able to Peek() at %v but it failed", key)
	}

	// Fetch
	val, err = db.Fetch(key, nil)
	if err != nil {
		t.Errorf("error fetching %v from database: %v", key, err.Error())
	}
	if val != data {
		t.Errorf("expected %v but %v was retrieved for key %v", data, val, key)
	}

	// Drop
	err = db.Drop(key)
	if err != nil {
		t.Errorf("error deleting %v from database: %v", key, err.Error())
	}
	_, err = db.Peek(key)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected Error: %v, but instead error was %v", ErrNotFound, err)
	}

	_, err = db.Fetch(key, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected Error: %v retrieving deleted data %v, but error was %v", ErrNotFound, key, err)
	}
}

func TestCipher(t *testing.T) {
	key := "ciphertext"
	data := "testing123"
	password := []byte("password")

	// Stow
	err := db.Stow(key, data, password)
	if err != nil {
		t.Errorf("error inserting ciphertext into database: %v", err.Error())
	}

	// Peek
	pk, err := db.Peek(key)
	if err != nil {
		t.Errorf("unexpected error in Peek(): %v", err)
	}
	if pk != "" {
		t.Errorf("expected Peek(%v) to fail but it returned %v", key, pk)
	}

	// Fetch
	val, err := db.Fetch(key, password)
	if err != nil {
		t.Errorf("error fetching %v from database: %v", key, err.Error())
	}
	if val != data {
		t.Errorf("expected %v but %v was retrieved for key %v", data, val, key)
	}

	// Drop
	err = db.Drop(key)
	if err != nil {
		t.Errorf("error deleting %v from database: %v", key, err.Error())
	}
	_, err = db.Peek(key)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected Error: %v, but instead error was %v", ErrNotFound, err)
	}
	_, err = db.Fetch(key, password)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected %v retrieving deleted data %v but error was %v", ErrNotFound, key, err)
	}
}

func TestBadDecrypt(t *testing.T) {
	key := "ciphertext"
	data := "testing123"
	goodpassword := []byte("goodpassword")
	badpassword := []byte("badpassword")

	err := db.Stow(key, data, goodpassword)
	if err != nil {
		t.Errorf("error inserting ciphertext into database: %v", err.Error())
	}

	_, err = db.Fetch(key, badpassword)
	if !errors.Is(err, ErrBadPassword) {
		t.Errorf("Expected %v from Fetch() but the error was %v", ErrBadPassword, err)
	}

	t.Cleanup(func() { db.Drop(key) })
}

func TestBadKey(t *testing.T) {
	_, err := db.Fetch("badkey", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected %v from Fetch() but the error was %v", ErrNotFound, err)
	}
}
