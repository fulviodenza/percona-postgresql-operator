package pgbackup

import (
	"context"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fulviodenza/percona-postgresql-operator/internal/naming"
	pNaming "github.com/fulviodenza/percona-postgresql-operator/percona/naming"
	v2 "github.com/fulviodenza/percona-postgresql-operator/pkg/apis/pgv2.percona.com/v2"
	"github.com/fulviodenza/percona-postgresql-operator/pkg/apis/postgres-operator.crunchydata.com/v1beta1"
)

type fakeClient struct {
	client.Client
}

var _ = client.Client(new(fakeClient))

func buildFakeClient(ctx context.Context, cr *v2.PerconaPGCluster, objs ...client.Object) (client.Client, error) {
	s := scheme.Scheme

	if err := v1beta1.AddToScheme(s); err != nil {
		return nil, err
	}
	if err := v2.AddToScheme(s); err != nil {
		return nil, err
	}

	objs = append(objs, cr)
	cr.Default()
	postgresCluster, err := cr.ToCrunchy(ctx, nil, s)
	if err != nil {
		return nil, err
	}
	objs = append(objs, postgresCluster)

	dcs := &corev1.Endpoints{ObjectMeta: naming.PatroniDistributedConfiguration(postgresCluster)}
	dcs.Annotations = map[string]string{
		"initialize": "system-identifier",
	}
	objs = append(objs, dcs)

	cl := new(fakeClient)
	cl.Client = fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).WithStatusSubresource(objs...).
		WithIndex(new(v2.PerconaPGBackup), v2.IndexFieldPGCluster, v2.PGClusterIndexerFunc).
		Build()

	return cl, nil
}

func readDefaultCR(name, namespace string) (*v2.PerconaPGCluster, error) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "cr.yaml"))
	if err != nil {
		return nil, err
	}

	cr := &v2.PerconaPGCluster{}

	if err := yaml.Unmarshal(data, cr); err != nil {
		return nil, err
	}

	cr.Name = name
	if cr.Annotations == nil {
		cr.Annotations = make(map[string]string)
	}
	cr.Spec.InitContainer = &v1beta1.InitContainerSpec{
		Image: "some-image",
	}
	cr.Annotations[pNaming.AnnotationCustomPatroniVersion] = "4.0.0"
	cr.Namespace = namespace
	cr.Status.Postgres.Version = cr.Spec.PostgresVersion
	return cr, nil
}
