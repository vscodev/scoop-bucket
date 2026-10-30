package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
)

const (
	_versionEnvKey         = "NEOKIKOERU_VERSION"
	_assetNameWindowsAmd64 = "neokikoeru-windows-amd64.zip"
	_assetNameWindowsArm64 = "neokikoeru-windows-arm64.zip"
)

var (
	_versionRegex = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
)

type Bucket struct {
	Version string

	DownloadUrlWindowsAmd64 string
	Sha256WindowsAmd64      string

	DownloadUrlWindowsArm64 string
	Sha256WindowsArm64      string
}

type Asset struct {
	Name               string `json:"name"`
	Digest             string `json:"digest"`
	BrowserDownloadUrl string `json:"browser_download_url"`
}

type Release struct {
	Name   string  `json:"name"`
	Assets []Asset `json:"assets"`
}

type Error struct {
	Message string `json:"message"`
}

func fetchRelease(ctx context.Context, version string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/vscodev/neokikoeru/releases/tags/v%s", version)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		v := new(Error)
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return nil, err
		}

		return nil, errors.New(v.Message)
	}

	v := new(Release)
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return nil, err
	}

	return v, nil
}

func main() {
	version := os.Getenv(_versionEnvKey)
	if version == "" || !_versionRegex.MatchString(version) {
		log.Fatalf("$%s is not a valid version. Please provide a valid semver", _versionEnvKey)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release, err := fetchRelease(ctx, version)
	if err != nil {
		log.Fatal(err)
	}

	bucket := &Bucket{
		Version: version,
	}
	for _, asset := range release.Assets {
		switch asset.Name {
		case _assetNameWindowsAmd64:
			bucket.DownloadUrlWindowsAmd64 = asset.BrowserDownloadUrl
			bucket.Sha256WindowsAmd64 = strings.TrimPrefix(asset.Digest, "sha256:")
		case _assetNameWindowsArm64:
			bucket.DownloadUrlWindowsArm64 = asset.BrowserDownloadUrl
			bucket.Sha256WindowsArm64 = strings.TrimPrefix(asset.Digest, "sha256:")
		}
	}

	tmpl := template.Must(template.ParseFiles("./templates/neokikoeru.json.tmpl"))
	bucketFile, err := os.OpenFile("./bucket/neokikoeru.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer bucketFile.Close()

	if err = tmpl.Execute(bucketFile, bucket); err != nil {
		log.Fatal(err)
	}
}
