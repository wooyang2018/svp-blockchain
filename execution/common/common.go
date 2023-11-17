// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/crypto/sha3"
)

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
		time.Sleep(100 * time.Millisecond)
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
