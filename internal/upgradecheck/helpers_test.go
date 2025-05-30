// Copyright 2021 - 2024 Crunchy Data Solutions, Inc.
//
// SPDX-License-Identifier: Apache-2.0

package upgradecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr/funcr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/version"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fulviodenza/percona-postgresql-operator/internal/controller/runtime"
	"github.com/fulviodenza/percona-postgresql-operator/internal/logging"
	"github.com/fulviodenza/percona-postgresql-operator/pkg/apis/postgres-operator.crunchydata.com/v1beta1"
)

type fakeClientWithError struct {
	crclient.Client
	errorType string
}

// Get returns the client.get OR an Error (`get error`) if the fakeClientWithError is set to error that way
func (f *fakeClientWithError) Get(ctx context.Context, key types.NamespacedName, obj crclient.Object, opts ...crclient.GetOption) error {
	switch f.errorType {
	case "get error":
		return fmt.Errorf("get error")
	default:
		return f.Client.Get(ctx, key, obj, opts...)
	}
}

// Patch returns the client.get OR an Error (`patch error`) if the fakeClientWithError is set to error that way
// TODO: PatchType is not supported currently by fake
// - https://github.com/kubernetes/client-go/issues/970
// Once that gets fixed, we can test without envtest
func (f *fakeClientWithError) Patch(ctx context.Context, obj crclient.Object,
	patch crclient.Patch, opts ...crclient.PatchOption) error {
	switch {
	case f.errorType == "patch error":
		return fmt.Errorf("patch error")
	default:
		return f.Client.Patch(ctx, obj, patch, opts...)
	}
}

// List returns the client.get OR an Error (`list error`) if the fakeClientWithError is set to error that way
func (f *fakeClientWithError) List(ctx context.Context, objList crclient.ObjectList,
	opts ...crclient.ListOption) error {
	switch f.errorType {
	case "list error":
		return fmt.Errorf("list error")
	default:
		return f.Client.List(ctx, objList, opts...)
	}
}

// setupDeploymentID returns a UUID
func setupDeploymentID(t *testing.T) string {
	t.Helper()
	deploymentID = string(uuid.NewUUID())
	return deploymentID
}

// setupFakeClientWithPGOScheme returns a fake client with the PGO scheme added;
// if `includeCluster` is true, also adds some empty PostgresCluster and CrunchyBridgeCluster
// items to the client
func setupFakeClientWithPGOScheme(t *testing.T, includeCluster bool) crclient.Client {
	t.Helper()
	if includeCluster {
		pc := &v1beta1.PostgresClusterList{
			Items: []v1beta1.PostgresCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "hippo",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "elephant",
					},
				},
			},
		}

		bcl := &v1beta1.CrunchyBridgeClusterList{
			Items: []v1beta1.CrunchyBridgeCluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "hippo",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "elephant",
					},
				},
			},
		}

		return fake.NewClientBuilder().
			WithScheme(runtime.Scheme).
			WithLists(pc, bcl).
			Build()
	}
	return fake.NewClientBuilder().WithScheme(runtime.Scheme).Build()
}

// setupVersionServer sets up and tears down a server and version info for testing
func setupVersionServer(t *testing.T, works bool) (version.Info, *httptest.Server) {
	t.Helper()
	expect := version.Info{
		Major:     "1",
		Minor:     "22",
		GitCommit: "v1.22.2",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		req *http.Request) {
		if works {
			output, _ := json.Marshal(expect)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// We don't need to check the error output from this
			_, _ = w.Write(output)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	return expect, server
}

// setupLogCapture captures the logs and keeps count of the logs captured
func setupLogCapture(ctx context.Context) (context.Context, *[]string) {
	calls := []string{}
	testlog := funcr.NewJSON(func(object string) {
		calls = append(calls, object)
	}, funcr.Options{
		Verbosity: 1,
	})
	return logging.NewContext(ctx, testlog), &calls
}
