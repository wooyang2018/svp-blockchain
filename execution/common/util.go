// Copyright (C) 2023 Wooyang2018
// Licensed under the GNU General Public License v3.0

package common

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/wooyang2018/svp-blockchain/logger"
)

var ErrAddressLength = errors.New("unexpected address length")

// genesisCodes include addresses of XCoin and TAddr
var genesisCodes map[string][]byte

func RegisterCode(key string, addr []byte) {
	if genesisCodes == nil {
		genesisCodes = make(map[string][]byte)
	}
	if len(addr) != 32 {
		panic("unexpected genesis chaincode")
	}
	genesisCodes[key] = addr
}

func GetCodeAddr(key string) []byte {
	if _, ok := genesisCodes[key]; !ok {
		panic("unexpected genesis chaincode")
	}
	return genesisCodes[key]
}

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

func ConcatBytes(srcs ...[]byte) []byte {
	buf := bytes.NewBuffer(nil)
	size := 0
	for _, src := range srcs {
		size += len(src)
	}
	buf.Grow(size)
	for _, src := range srcs {
		buf.Write(src)
	}
	return buf.Bytes()
}

func CheckResponse(resp *http.Response, err error) error {
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("status code %d, %s", resp.StatusCode, string(msg))
	}
	return nil
}

func CreateRequestBody(filePath string) (*bytes.Buffer, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	buf := bytes.NewBuffer(nil)
	mw := multipart.NewWriter(buf)
	defer mw.Close()

	fw, err := mw.CreateFormFile("file", "chaincode")
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, "", err
	}
	return buf, mw.FormDataContentType(), nil
}

func DumpFile(data []byte, directory, file string) {
	if !Exists(directory) {
		Check(os.MkdirAll(directory, os.ModePerm))
	}
	f, err := os.Create(path.Join(directory, file))
	Check(err)
	defer f.Close()
	_, err = f.Write(data)
	Check(err)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func Check(err error) {
	if err != nil {
		logger.I().Fatal(err)
	}
}

func Check2(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func DownloadCodeIfRequired(codeDir string, codeID, data []byte) error {
	filepath := path.Join(codeDir, hex.EncodeToString(codeID))
	if _, err := os.Stat(filepath); err == nil {
		return nil // code file already exist
	}
	resp, err := DownloadCode(string(data), 5)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	sum, buf, err := CopyAndSumCode(resp.Body)
	if err != nil {
		return err
	}
	if !bytes.Equal(codeID, sum) {
		return errors.New("invalid code hash")
	}
	return WriteCodeFile(codeDir, codeID, buf)
}
