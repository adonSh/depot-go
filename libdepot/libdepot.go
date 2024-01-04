package libdepot

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"

	_ "github.com/mattn/go-sqlite3"
)

type Depot struct {
	*sql.DB
	salt []byte
}

var (
	b64 = base64.StdEncoding

	ErrNotFound    = errors.New("key not found")
	ErrBadPassword = errors.New("bad password")
)

// Returns a new storage medium (sqlite3 database) or an error if
// initialization is unsuccessful.
func NewDepot(uri string) (*Depot, error) {
	conn, err := sql.Open("sqlite3", uri)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	db := Depot{conn, make([]byte, 32)}
	if err = db.QueryRow("select data from salt").Scan(&db.salt); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			if err = db.init(); err != nil {
				return nil, fmt.Errorf("cannot access database: %w", err)
			}
		}
		_, err = io.ReadFull(rand.Reader, db.salt)
		if err != nil {
			return nil, fmt.Errorf("cannot generate random salt: %w", err)
		}
		_, err = db.Exec("insert into salt (data) values (?)", db.salt)
		if err != nil {
			return nil, fmt.Errorf("cannot access database: %w", err)
		}
	}

	return &db, nil
}

func (db *Depot) init() error {
	_, err := db.Exec(`
		create table if not exists storage (
			modified   int  default (strftime('%s', 'now')),
			key        text unique not null,
			val        text not null,
			nonce      blob unique
		);

		create table if not exists salt (
			data blob not null
		);`)

	return err
}

func encrypt(password, salt, data []byte) ([]byte, []byte, error) {
	encryptionKey := pbkdf2.Key(password, salt, 4096, 32, sha1.New)
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, 12)
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	return aesgcm.Seal(nil, nonce, data, nil), nonce, nil
}

func decrypt(password, salt, nonce, data []byte) ([]byte, error) {
	encryptionKey := pbkdf2.Key(password, salt, 4096, 32, sha1.New)
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, ErrBadPassword
	}

	return plaintext, nil
}

// Stores the specified key and value in the depot. If the key exists then
// the value is updated. If password is not nil the value will be encrypted.
// Returns an error if encryption or storage fails.
func (db *Depot) Stow(key, val string, password []byte) error {
	if password == nil {
		_, err := db.Exec(`
			insert into storage (key, val, nonce)
			values (?, ?, ?)
			on conflict (key) do
			update set
				modified=(strftime('%s', 'now')),
				val=?,
				nonce=?`,
			key, val, nil, val, nil)
		if err != nil {
			return fmt.Errorf("cannot access database: %w", err)
		}

		return nil
	}

	ciphertext, nonce, err := encrypt(password, db.salt, []byte(val))
	if err != nil {
		return fmt.Errorf("cannot encrypt data: %w", err)
	}

	cval := b64.EncodeToString(ciphertext)
	_, err = db.Exec(`
		insert into storage (key, val, nonce)
		values (?, ?, ?)
		on conflict (key) do
		update set
			modified=(strftime('%s', 'now')),
			val=?,
			nonce=?`,
		key, cval, nonce, cval, nonce)
	if err != nil {
		return fmt.Errorf("cannot access database: %w", err)
	}

	return nil
}

// Returns the value from the depot associated with the specified key or an
// error if unsuccessful. A non-nil password must be supplied for encrypted
// values.
func (db *Depot) Fetch(key string, password []byte) (string, error) {
	var val string
	var nonce []byte
	err := db.QueryRow(`
		select val, nonce
		from storage
		where key = ?`,
		key).Scan(&val, &nonce)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("cannot access database: %w", err)
	}

	if nonce == nil {
		return val, nil
	}

	valbytes, err := b64.DecodeString(val)
	if err != nil {
		return "", fmt.Errorf("cannot decrypt data: %w", err)
	}

	plaintext, err := decrypt(password, db.salt, nonce, valbytes)
	if err != nil {
		return "", fmt.Errorf("cannot decrypt data: %w", err)
	}

	return string(plaintext), nil
}

// Deletes the specified key from the depot. Returns an error if unsuccessful.
func (db *Depot) Drop(key string) error {
	if _, err := db.Exec("delete from storage where key = ?", key); err != nil {
		return fmt.Errorf("cannot access database: %w", err)
	}

	return nil
}

// Tries to get a value quickly and easily. If the requested key is encrypted
// or if any errors occur then an empty string is returned. Meant to be
// followed by Fetch() if unsuccessful.
func (db *Depot) Peek(key string) string {
	var data string
	var nonce []byte

	db.QueryRow(`
		select val, nonce
		from storage
		where key = ?`,
		key).Scan(&data, &nonce)

	if nonce != nil {
		return ""
	}
	return data
}
