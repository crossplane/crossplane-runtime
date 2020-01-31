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

package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// ConditionalStringFormat returns based on the format string and substitution
// value. If format is not provided, substitution value is returned If format is
// provided with '%s' substitution symbol, fmt.Sprintf(fmt, val) is returned.
// NOTE: only single %s substitution is supported. If name format does not
// contain '%s' substitution, i.e. a constant string, the constant string value
// is returned back
//
// Examples:
//   For all examples assume "value" = "test-value"
//   1. format = "", ContainerName = "test-value"
//   2. format = "foo", ContainerName = "foo"
//   3. format = "foo-%s", ContainerName = "foo-test-value"
//   4. format = "foo-%s-bar-%s", ContainerName = "foo-test-value-bar-%!s(MISSING)"
func ConditionalStringFormat(format string, value string) string {
	if format == "" {
		return value
	}
	if strings.Contains(format, "%s") {
		return fmt.Sprintf(format, value)
	}
	return format
}

// GeneratePassword generates a password using random data of the given length,
// then encodes to a base64 string.
func GeneratePassword(dataLen int) (string, error) {
	randData := make([]byte, dataLen)
	if _, err := rand.Read(randData); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randData), nil
}
