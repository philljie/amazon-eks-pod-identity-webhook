package config

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const (
	namespaceFoo               = "foo"
	namespaceFooServiceAccount = "ns-foo-sa"
	namespaceBar               = "bar"
	namespaceBarServiceAccount = "ns-bar-sa"

	containerCredentialsAudience = "containerCredentialsAudience"
	containersCredentialsFullUri = "containersCredentialsFullUri"

	defaultTimeout      = 10 * time.Second
	defaultPollInterval = 1 * time.Second
)

func TestFileConfig_Watcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirPath, err := os.MkdirTemp("", "test")
	assert.NoError(t, err)
	defer os.RemoveAll(dirPath)

	filePath := filepath.Join(dirPath, "file")
	assert.NoError(t, os.WriteFile(filePath, defaultConfigObjectBytes(), 0666))

	fileConfig := NewFileConfig(containerCredentialsAudience, containersCredentialsFullUri)
	assert.NoError(t, fileConfig.StartWatcher(ctx, filePath))
	verifyConfigObject(t, fileConfig, defaultConfigObject())

	newConfigObject := defaultConfigObject()
	newConfigObject.Identities = append(newConfigObject.Identities, Identity{
		Namespace:      "new-ns",
		ServiceAccount: "new-sa",
	})
	newConfigObjectBytes, err := json.Marshal(newConfigObject)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filePath, newConfigObjectBytes, 0666))
	verifyConfigObject(t, fileConfig, newConfigObject)
}

func TestFileConfig_WatcherNotStarted(t *testing.T) {
	fileConfig := NewFileConfig(containerCredentialsAudience, containersCredentialsFullUri)
	patchConfig := fileConfig.Get("non-existent", "non-existent")
	assert.Nil(t, patchConfig)
}

func TestFileConfig_Load(t *testing.T) {
	testcases := []struct {
		name                 string
		input                []byte
		expectedConfigObject *IdentityConfigObject
		expectError          bool
	}{
		{
			name:                 "Nil byte slice",
			input:                nil,
			expectedConfigObject: nil,
		},
		{
			name:                 "Empty byte slice",
			input:                make([]byte, 0),
			expectedConfigObject: nil,
		},
		{
			name:                 "Basic Test",
			input:                defaultConfigObjectBytes(),
			expectedConfigObject: defaultConfigObject(),
		},
		{
			name:        "Malformed JSON bytes",
			input:       []byte("bad json"),
			expectError: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fileConfig := NewFileConfig(containerCredentialsAudience, containersCredentialsFullUri)
			err := fileConfig.Load(tc.input)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				verifyConfigObject(t, fileConfig, tc.expectedConfigObject)
			}
		})
	}

}

func TestFileConfig_Get(t *testing.T) {
	fileConfig := NewFileConfig(containerCredentialsAudience, containersCredentialsFullUri)
	err := fileConfig.Load(defaultConfigObjectBytes())
	assert.NoError(t, err)

	assert.NotNil(t, fileConfig.cache)
	assert.Len(t, fileConfig.cache, 2)

	patchConfig := fileConfig.Get(namespaceFoo, namespaceFooServiceAccount)
	assert.NotNil(t, patchConfig)
	assert.Equal(t, containerCredentialsAudience, patchConfig.Audience)
	assert.Equal(t, containersCredentialsFullUri, patchConfig.FullUri)

	patchConfig = fileConfig.Get("non-existent", "non-existent")
	assert.Nil(t, patchConfig)
}

func defaultConfigObject() *IdentityConfigObject {
	return &IdentityConfigObject{
		Identities: []Identity{
			{
				Namespace:      namespaceFoo,
				ServiceAccount: namespaceFooServiceAccount,
			},
			{
				Namespace:      namespaceBar,
				ServiceAccount: namespaceBarServiceAccount,
			},
		},
	}
}

func defaultConfigObjectBytes() []byte {
	configObject := defaultConfigObject()
	jsonBytes, err := json.Marshal(configObject)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}

func verifyConfigObject(t *testing.T, fileConfig *FileConfig, expected *IdentityConfigObject) {
	assert.Eventually(t, func() bool {
		return reflect.DeepEqual(fileConfig.identityConfigObject, expected)
	}, defaultTimeout, defaultPollInterval)
}
