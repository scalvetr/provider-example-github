/*
Copyright 2020 The Crossplane Authors.

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

package membership

import (
	"context"
	"fmt"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/hasheddan/kc-provider-github/apis/org/v1alpha1"
	kcgitclient "github.com/hasheddan/kc-provider-github/pkg/client"
)

const (
	errNotMembership = "managed resource is not a MyType custom resource"
	errCreateService = "failed to create client service"
)

// SetupM adds a controller that reconciles MyType managed resources.
func SetupMembership(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.MembershipGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.MembershipGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube: mgr.GetClient()},
		),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Membership{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube client.Client
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the ProviderConfig's credentials secret.
// 4. Using the credentials secret to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	_, ok := mg.(*v1alpha1.Membership)
	if !ok {
		return nil, errors.New(errNotMembership)
	}
	svc, err := kcgitclient.UseProviderConfig(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errCreateService)
	}
	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service *github.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Membership)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotMembership)
	}

	membership, _, err := c.service.Teams.GetTeamMembershipBySlug(
		ctx,
		cr.Spec.ForProvider.Org,
		pointer.StringDeref(cr.Spec.ForProvider.Team, ""),
		cr.Spec.ForProvider.User,
	)
	if err != nil {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if membership.State != nil {
		cr.Status.AtProvider.State = *membership.State
	}

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: true,

		ConnectionDetails: managed.ConnectionDetails{
			"username": []byte(cr.Spec.ForProvider.User),
		},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Membership)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotMembership)
	}

	fmt.Printf("Creating: %+v", cr)

	_, _, err := c.service.Teams.AddTeamMembershipBySlug(
		ctx,
		cr.Spec.ForProvider.Org,
		pointer.StringDeref(cr.Spec.ForProvider.Team, ""),
		cr.Spec.ForProvider.User,
		nil,
	)

	return managed.ExternalCreation{}, err
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Membership)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotMembership)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Membership)
	if !ok {
		return errors.New(errNotMembership)
	}

	fmt.Printf("Deleting: %+v", cr)

	_, err := c.service.Teams.RemoveTeamMembershipBySlug(
		ctx,
		cr.Spec.ForProvider.Org,
		pointer.StringDeref(cr.Spec.ForProvider.Team, ""),
		cr.Spec.ForProvider.User,
	)

	return err
}
