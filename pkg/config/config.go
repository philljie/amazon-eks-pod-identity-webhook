/*
  Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.

  Licensed under the Apache License, Version 2.0 (the "License").
  You may not use this file except in compliance with the License.
  A copy of the License is located at

      http://www.apache.org/licenses/LICENSE-2.0

  or in the "license" file accompanying this file. This file is distributed
  on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
  express or implied. See the License for the specific language governing
  permissions and limitations under the License.
*/

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-eks-pod-identity-webhook/pkg/filesystem"
	"k8s.io/klog/v2"
	"sync"
)

type Config interface {
	Get(namespace string, serviceAccount string) *ContainerCredentialsPatchConfig
}

type FileConfig struct {
	containerCredentialsAudience string
	containersCredentialsFullUri string

	watcher              *filesystem.FileWatcher
	identityConfigObject *IdentityConfigObject
	cache                map[Identity]bool
	mu                   sync.RWMutex // guards cache
}

type ContainerCredentialsPatchConfig struct {
	Audience string
	FullUri  string
}

func NewFileConfig(containersCredentialsAudience, containersCredentialsFullUri string) *FileConfig {
	return &FileConfig{
		containerCredentialsAudience: containersCredentialsAudience,
		containersCredentialsFullUri: containersCredentialsFullUri,
		identityConfigObject:         nil,
		cache:                        make(map[Identity]bool),
	}
}

// StartWatcher creates and starts a fsnotify watcher on the target config file.
// The watcher runs continuously until the context is cancelled.  When the file is updated,
// Load will be invoked, and thus will refresh the cache.
func (f *FileConfig) StartWatcher(ctx context.Context, filePath string) error {
	f.watcher = filesystem.NewFileWatcher("local-file-config", filePath, f.Load)
	return f.watcher.Watch(ctx)
}

func (f *FileConfig) Load(content []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if content == nil || len(content) == 0 {
		klog.Info("Config file is empty, clearing cache")
		f.identityConfigObject = nil
		f.cache = nil
		return nil
	}

	var configObject IdentityConfigObject
	if err := json.Unmarshal(content, &configObject); err != nil {
		return fmt.Errorf("error Unmarshalling config file: %v", err)
	}

	newCache := make(map[Identity]bool)
	for _, item := range configObject.Identities {
		klog.V(5).Infof("Adding SA %s/%s to config cache", item.Namespace, item.ServiceAccount)
		newCache[item] = true
	}
	f.identityConfigObject = &configObject
	f.cache = newCache
	klog.Info("Successfully loaded config file")

	return nil
}

func (f *FileConfig) Get(namespace string, serviceAccount string) *ContainerCredentialsPatchConfig {
	key := Identity{
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
	}
	if f.getCacheItem(key) {
		return &ContainerCredentialsPatchConfig{
			Audience: f.containerCredentialsAudience,
			FullUri:  f.containersCredentialsFullUri,
		}
	}

	return nil
}

func (f *FileConfig) getCacheItem(identity Identity) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cache[identity]
}
