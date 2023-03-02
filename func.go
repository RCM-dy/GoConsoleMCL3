package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tidwall/gjson"
)

func GetByteInInternet(url string) ([]byte, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
func GetByteInInternetWithHash(url string, algorithm string, hashcode string) ([]byte, error) {
	b, err := GetByteInInternet(url)
	if err != nil {
		return nil, err
	}
	if !IsBytesSameHash(algorithm, hashcode, b) {
		return nil, NewHashNotSame(hashcode, "")
	}
	return b, nil
}
func Readbyte(filename string) ([]byte, error) {
	f, err := os.OpenFile(filename, os.O_RDWR, 0666)
	if err != nil {
		return []byte{}, err
	}
	defer f.Close()
	frb, err := io.ReadAll(f)
	if err != nil {
		return []byte{}, err
	}
	return frb, err
}
func WriteBytes(filename string, data []byte) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}
func WriteFmtJsonBytes(filename string, jsondata []byte) error {
	jsonb, err := FmtJsonBytes(jsondata)
	if err != nil {
		return err
	}
	return WriteBytes(filename, jsonb)
}
func FmtJsonBytes(data []byte) ([]byte, error) {
	var s bytes.Buffer
	err := json.Indent(&s, data, "", "    ")
	if err != nil {
		return []byte(""), err
	}
	return s.Bytes(), nil
}
func Sha1Bytes(datas []byte) string {
	s := sha1.New()
	s.Write(datas)
	return hex.EncodeToString(s.Sum(nil))
}
func ReplaceByMap(s string, c map[string]string) string {
	ss := s
	for k, v := range c {
		ss = strings.ReplaceAll(ss, k, v)
	}
	return ss
}
func IsBytesSameHash(algorithm string, need string, got []byte) bool {
	switch algorithm {
	case "sha1":
		return need == Sha1Bytes(got)
	}
	return false
}
func FileNameIsExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil {
		return os.IsExist(err)
	}
	return true
}
func DownloadFmtJsonWithHash(filename string, url string, algorithm string, needHash string) error {
	getByte, err := GetByteInInternet(url)
	if err != nil {
		return err
	}
	if !IsBytesSameHash(algorithm, needHash, getByte) {
		return NewHashNotSame(needHash, "")
	}
	dirs, _ := filepath.Split(filename)
	if !FileNameIsExist(dirs) {
		err = os.MkdirAll(dirs, 0666)
		if err != nil {
			return err
		}
	}
	return WriteFmtJsonBytes(filename, getByte)
}
func DownloadWithHash(filename string, url string, algorithm string, needHash string) error {
	getByte, err := GetByteInInternet(url)
	if err != nil {
		return err
	}
	if !IsBytesSameHash(algorithm, needHash, getByte) {
		return NewHashNotSame(needHash, "")
	}
	dirs, _ := filepath.Split(filename)
	if !FileNameIsExist(dirs) {
		err = os.MkdirAll(dirs, 0666)
		if err != nil {
			return err
		}
	}
	return WriteBytes(filename, getByte)
}
func GetStrFmtJsonInInternetWithHash(url, algorithm, needHash string) (string, error) {
	getsB, err := GetByteInInternet(url)
	if err != nil {
		return "", err
	}
	if !IsBytesSameHash(algorithm, needHash, getsB) {
		return "", NewHashNotSame(needHash, "")
	}
	getsB, err = FmtJsonBytes(getsB)
	return string(getsB), err
}

func RandStringBytes(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

type AssertExpention struct{}

func (a *AssertExpention) Error() string {
	return "assertExpention"
}
func AssertsTrue(got bool) {
	if !got {
		panic(&AssertExpention{})
	}
}
func PostMapGotBytes(url string, header map[string]string, value map[string]interface{}) ([]byte, error) {
	client := &http.Client{}
	postValue, err := json.Marshal(&value)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(postValue))
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header.Add(k, v)
	}
	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
func IsRuleSameFrom_gjson_Result(rules gjson.Result, isdomouser bool) bool {
	if !rules.Exists() {
		return true
	}
	var notsame bool = false
	for _, vr := range rules.Array() {
		action := vr.Get("action")
		if action.Exists() {
			if action.String() != "allow" {
				continue
			}
		}
		features := vr.Get("features")
		if features.Exists() {
			features.ForEach(func(key, value gjson.Result) bool {
				if key.String() == "is_demo_user" && value.Bool() == isdomouser {
					notsame = true
					return false
				}
				if key.String() == "has_custom_resolution" && value.Bool() {
					notsame = true
					return false
				}
				return true
			})
			if notsame {
				break
			}
		}
		oses := vr.Get("os")
		if oses.Exists() {
			oses.ForEach(func(key, value gjson.Result) bool {
				if key.String() == "name" && value.String() == "windows" {
					notsame = true
					return false
				}
				if key.String() == "arch" && value.String() == "x"+strings.TrimLeft(runtime.GOARCH, "amd") {
					notsame = true
					return false
				}
				return true
			})
		}
	}
	return !notsame
}
func ReadString(filename string) (string, error) {
	f, err := os.OpenFile(filename, os.O_RDWR, 0666)
	if err != nil {
		return "", err
	}
	defer f.Close()
	frb, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(frb), err
}
func WriteString(filename string, data string) error {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(data)
	if err != nil {
		return err
	}
	return nil
}
