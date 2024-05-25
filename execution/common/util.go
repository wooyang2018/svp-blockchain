// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/crypto/sha3"
)

var ErrAddressLength = errors.New("unexpected address length")

func DownloadCode(url string, retry int) (*http.Response, error) {
	resp, err := http.Get(url)
	if err == nil {
		if resp.StatusCode != 200 {
			err = errors.New("status not 200")
		}
	}
	if err != nil {
		if retry <= 0 {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond)
		return DownloadCode(url, retry-1)
	}
	return resp, nil
}

func WriteCodeFile(codeDir string, codeID []byte, r io.Reader) error {
	filename := hex.EncodeToString(codeID)
	filepath := path.Join(codeDir, filename)
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return os.Chmod(filepath, 0755)
}

func CopyAndSumCode(r io.Reader) ([]byte, *bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	h := sha3.New256()
	if _, err := io.Copy(buf, io.TeeReader(r, h)); err != nil {
		return nil, nil, err
	}
	return h.Sum(nil), buf, nil
}

func StoreCode(codeDir string, r io.Reader) ([]byte, error) {
	codeID, buf, err := CopyAndSumCode(r)
	if err != nil {
		return nil, err
	}
	if err = WriteCodeFile(codeDir, codeID, buf); err != nil {
		return nil, err
	}
	return codeID, nil
}

func DecodeBalance(b []byte) uint64 {
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}

func EncodeBalance(value uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value)
	return b
}

func AssertLength(addr []byte, l int) error {
	if len(addr) == 0 || len(addr) == l {
		return nil
	}
	return ErrAddressLength
}

func ValidLength(addr []byte) error {
	if len(addr) == 20 || len(addr) == 32 {
		return nil
	}
	return ErrAddressLength
}

func AddressToString(addr []byte) string {
	if len(addr) == 20 {
		return hex.EncodeToString(addr)
	} else if len(addr) == 32 {
		return base64.StdEncoding.EncodeToString(addr)
	}
	return ""
}

func Address32ToBytes(addr string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(addr)
}

func Address20ToBytes(addr string) ([]byte, error) {
	return hex.DecodeString(addr)
}

func AddressToBytes(addr string) ([]byte, error) {
	res, err := Address32ToBytes(addr)
	if err == nil && len(res) == 32 {
		return res, nil
	}
	res, err = Address20ToBytes(addr)
	if err == nil && len(res) == 20 {
		return res, nil
	}
	return nil, ErrAddressLength
}
