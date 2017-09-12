/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package downloader

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
)

// Downloader is an interface for downloading files
type Downloader interface {
	// Donwload donwloads file specificed by location
	// and return downloaded file path
	Download(rawUrl string) (string, error)
	// DonwloadStream donwloads file specificed by location
	// and return io.ReadCloser as stream
	DownloadStream(rawUrl string) (io.ReadCloser, error)
}

type basicDownloader struct {
	protocol string
}

func NewBasicDownloader(protocol string) Downloader {
	return &basicDownloader{protocol}
}

func (b *basicDownloader) Download(rawUrl string) (string, error) {
	url := fmt.Sprintf("%s://%s", b.protocol, rawUrl)

	tmpFile, err := ioutil.TempFile("", "frakti_")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func (b *basicDownloader) DownloadStream(rawUrl string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s://%s", b.protocol, rawUrl)

	glog.V(3).Infof("Downloading file from %q", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
