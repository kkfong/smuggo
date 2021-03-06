package main

import (
	"crypto/md5"
	"fmt"
	"go-oauth/oauth"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// upload transfers a single file to the SmugMug album identifed by key.
func upload(albumKey string, filename string) {
	userToken, err := loadUserToken()
	if err != nil {
		fmt.Println("Error reading OAuth token: " + err.Error())
		return
	}

	var client = http.Client{}

	err = postImage(&client, userToken, albumKey, filename)
	if err != nil {
		fmt.Println("Error uploading: " + err.Error())
	}
}

// expandFileNames applies pattern matching to the given list of filenames.
// Pass filepath.Glob as the expander function.  The pattern matching function
// is a parameter for testing purposes.
func expandFileNames(
	filenames []string, expander func(pattern string) ([]string, error)) []string {

	expanded := make([]string, 0, 20)

	for _, fname := range filenames {
		matches, err := expander(fname)
		if err != nil {
			continue
		}
		expanded = append(expanded, matches...)
	}

	return expanded
}

// multiUpload uploads files in parallel to the given SmugMug album.
func multiUpload(numParallel int, albumKey string, filenames []string) {
	if numParallel < 1 {
		fmt.Println("Error, must upload at least 1 file at a time!")
		return
	}

	userToken, err := loadUserToken()
	if err != nil {
		fmt.Println("Error reading OAuth token: " + err.Error())
		return
	}

	expFileNames := expandFileNames(filenames, filepath.Glob)
	fmt.Println(expFileNames)
	var client = http.Client{}

	semaph := make(chan int, numParallel)
	for _, filename := range expFileNames {
		semaph <- 1
		go func(filename string) {
			fmt.Println("go " + filename)
			err := postImage(&client, userToken, albumKey, filename)
			if err != nil {
				fmt.Println("Error uploading: " + err.Error())
			}
			<-semaph
		}(filename)
	}

	for {
		time.Sleep(time.Second)
		if len(semaph) == 0 {
			break
		}
	}
}

// getMediaType determines the value for the Content-Type header field based
// on the file extension.
func getMediaType(filename string) string {
	ext := filepath.Ext(filename)
	return mime.TypeByExtension(ext)
}

// calcMd5 generates the MD5 sum for the given file.
func calcMd5(imgFileName string) (string, int64, error) {
	file, err := os.Open(imgFileName)
	if err != nil {
		return "", 0, err
	}

	defer file.Close()

	hash := md5.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}

	var md5Sum []byte
	md5Sum = hash.Sum(md5Sum)
	return fmt.Sprintf("%x", md5Sum), size, nil
}

// postImage uploads a single image to SmugMug via the POST method.
func postImage(client *http.Client, credentials *oauth.Credentials,
	albumKey string, imgFileName string) error {

	md5Str, imgSize, err := calcMd5(imgFileName)
	if err != nil {
		return err
	}

	uploadUri := "https://upload.smugmug.com/"
	file, err := os.Open(imgFileName)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", uploadUri, file)
	if err != nil {
		return err
	}

	req.ContentLength = imgSize

	for key, val := range oauthClient.Header {
		req.Header[key] = val
	}

	_, justImgFileName := filepath.Split(imgFileName)
	var headers = url.Values{
		"Accept":              {"application/json"},
		"Content-Type":        {getMediaType(justImgFileName)},
		"Content-MD5":         {md5Str},
		"Content-Length":      {strconv.FormatInt(imgSize, 10)},
		"X-Smug-ResponseType": {"JSON"},
		"X-Smug-AlbumUri":     {"/api/v2/album/" + albumKey},
		"X-Smug-Version":      {"v2"},
		"X-Smug-Filename":     {justImgFileName},
	}

	for key, val := range headers {
		req.Header[key] = val
	}

	if err := oauthClient.SetAuthorizationHeader(
		req.Header, credentials, "POST", req.URL, url.Values{}); err != nil {
		return nil
	}

	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(resp.Status)
	fmt.Println(string(bytes))

	return nil
}
