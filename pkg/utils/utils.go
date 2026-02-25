// (C) Copyright IBM Corp.  2025,2026 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	klog "k8s.io/klog/v2"

	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
)

var (
	uuidGenerateMaxRetry = 10
)

// createNewConfigFolder generates new folder with the unique ID, returns host mount path or error
func CreateNewConfigFolder(configPath string) (string, error) {
	var err error
	for attempt := 1; attempt <= uuidGenerateMaxRetry; attempt++ {
		newUuid := uuid.NewString()
		newFolder := filepath.Join(configPath, newUuid)
		err = CreateFolderIfNotExists(newFolder)
		if err == nil {
			return newFolder, nil
		}
	}
	return "", fmt.Errorf("cannot create new config folder (max retry: %d): %v", uuidGenerateMaxRetry, err)
}

func CreateFolderIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModeDir|0755)
	}
	return nil
}

func GetEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Printf("%s value is empty, set `%s`", key, defaultValue)
		return defaultValue
	}
	return value
}

func GetNodeName() string {
	var nodeName string
	var err error
	var found bool
	nodeName, found = os.LookupEnv(cst.NodeNameEnvKey)
	if !found {
		klog.Info("NODENAME_ENV is not set, use os.Hostname()")
		nodeName, err = os.Hostname()
		if err != nil {
			klog.Warning("failed to get host name")
		}
	}
	klog.Infof("nodeName=%s\n", nodeName)
	return nodeName
}

func PathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func CopyFile(src, dest string) error {
	if srcFile, err := os.Open(src); err != nil {
		return err
	} else {
		defer func() { _ = srcFile.Close() }()
		if destFile, err := os.Create(dest); err != nil {
			return err
		} else {
			defer func() { _ = destFile.Close() }()
			if _, err = io.Copy(destFile, srcFile); err != nil {
				return err
			}
		}
	}
	return nil
}
