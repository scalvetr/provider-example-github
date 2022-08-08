package client

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apisv1alpha1 "github.com/hasheddan/kc-provider-github/apis/v1alpha1"
)

const (
	errEmptyToken   = "no token provided"
	errNotMyType    = "managed resource is not a MyType custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errNoSecretRef  = "ProviderConfig does not reference a credentials Secret"
	errGetSecret    = "cannot get credentials Secret"

	errNewClient = "cannot create new Service"
)

// NewClient creates a new client.
func NewClient(token string) (*github.Client, error) {
	if token == "" {
		return nil, errors.New(errEmptyToken)
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc), nil
}

func UseProviderConfig(ctx context.Context, c client.Client, mg resource.Managed) (*github.Client, error) {
	usage := resource.NewProviderConfigUsageTracker(c, &apisv1alpha1.ProviderConfigUsage{})

	if err := usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	// A secret is the most common way to authenticate to a provider, but some
	// providers additionally support alternative authentication methods such as
	// IAM, so a reference is not required.
	ref := pc.Spec.Credentials.SecretRef
	if ref == nil {
		return nil, errors.New(errNoSecretRef)
	}

	s := &v1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, s); err != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}

	svc, err := NewClient(string(s.Data[ref.Key]))
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	return svc, nil
}
