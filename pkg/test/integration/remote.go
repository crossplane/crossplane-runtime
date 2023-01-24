/*
Copyright 2019 The Crossplane Authors.

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

package integration

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"

	getter "github.com/hashicorp/go-getter"
)

// Downloads the given url into a subdirectory in the given path.
func downloadPath(url, path string) (string, error) {
	// Pwd is necessary for downloading local files via relative path.
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Subdirectory is given name of fnv hash of url.
	hasher := fnv.New32a()
	hasher.Write([]byte(url))
	dst := filepath.Join(path, fmt.Sprintf("%x", hasher.Sum32()))

	c := getter.Client{
		Src:  url,
		Dst:  dst,
		Pwd:  pwd,
		Mode: getter.ClientModeAny,
	}

	return dst, c.Get()
}
