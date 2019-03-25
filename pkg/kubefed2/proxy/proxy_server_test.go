package proxy

import (
	"fmt"
	"regexp"
	"testing"
)

func TestGetNamespaceFromPath(t *testing.T) {
	paths := []string{
		"api/v1/namespaces/test1/pods",
		"api/v1/namespaces/test1/pods/abc",
		"apis/rbac.authorization.k8s.io/v1",
	}

	r, _ := regexp.Compile(".*/namespaces/([^/]*)/([^/]*)/?(.*)")
	for _, path := range paths {
		sub := r.FindStringSubmatch(path)
		fmt.Printf("%s: len %d, %v\n", path, len(sub), sub)
	}
}
