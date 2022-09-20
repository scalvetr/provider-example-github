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

package team

import (
	"context"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/hasheddan/kc-provider-github/apis/org/v1alpha1"
	apisv1alpha1 "github.com/hasheddan/kc-provider-github/apis/v1alpha1"
	kcgitclient "github.com/hasheddan/kc-provider-github/pkg/client"
)

const (
	errNotTeam       = "managed resource is not a Team custom resource"
	errCreateService = "failed to create client service"
	errGetTeam       = "Failed to get team"
	errCreateTeam    = "Failed to create team"
	errUpdateTeam    = "Failed to update team"
	errDeleteTeam    = "Failed to delete team"
)

// Setup adds a controller that reconciles MyType managed resources.
func SetupTeam(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.TeamGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.TeamGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{})}),
		managed.WithInitializers(
			managed.NewNameAsExternalName(mgr.GetClient())),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Team{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube  client.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the ProviderConfig's credentials secret.
// 4. Using the credentials secret to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	_, ok := mg.(*v1alpha1.Team)
	if !ok {
		return nil, errors.New(errNotTeam)
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
	cr, ok := mg.(*v1alpha1.Team)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotTeam)
	}

	team, _, err := c.service.Teams.GetTeamBySlug(
		ctx,
		cr.Spec.ForProvider.Organization,
		meta.GetExternalName(cr))

	if err != nil {
		return managed.ExternalObservation{ResourceExists: false}, errors.Wrap(resource.Ignore(isNotFound, err), errGetTeam)
	}

	previousSpec := cr.Spec.ForProvider.DeepCopy()
	cr.Spec.ForProvider = lateInitialize(cr.Spec.ForProvider, team)
	isLateInitialized := cmp.Equal(previousSpec, cr.Spec.ForProvider)
	isUpToDate := isUpToDate(cr.Spec.ForProvider, team)

	cr.Status.SetConditions(xpv1.Available())
	cr.Status.AtProvider.MembersCount = team.MembersCount

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isUpToDate,
		ResourceLateInitialized: isLateInitialized,
	}, nil

	// can be changed in kubernetes
	// special case late initialized by the API
	// can be changed by the external api (readonly)

	// init with nil description
	// description is late initilized
	// set description to nil again
	// description is late initilized
}

func isUpToDate(spec v1alpha1.TeamParameters, current *github.Team) bool {
	if current == nil {
		return false
	}
	if pointer.StringDeref(spec.Description, "") != pointer.StringDeref(current.Description, "") {
		return false
	}
	return true
}

func lateInitialize(spec v1alpha1.TeamParameters, current *github.Team) v1alpha1.TeamParameters {
	if current != nil {
		if spec.Description == nil {
			spec.Description = current.Description
		}
	}
	return spec
}

func isNotFound(err error) bool {
	// err == HTTP 404 ?
	e, ok := err.(*github.ErrorResponse)
	if !ok {
		return false
	}
	return e.Response.StatusCode == 404
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Team)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotTeam)
	}
	_, _, err := c.service.Teams.CreateTeam(ctx, cr.Spec.ForProvider.Organization, github.NewTeam{
		Name:        meta.GetExternalName(cr),
		Description: cr.Spec.ForProvider.Description,
	})
	return managed.ExternalCreation{}, errors.Wrap(err, errCreateTeam)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Team)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotTeam)
	}
	_, _, err := c.service.Teams.EditTeamBySlug(
		ctx,
		meta.GetExternalName(cr),
		cr.Spec.ForProvider.Organization, github.NewTeam{
			Name:        meta.GetExternalName(cr),
			Description: cr.Spec.ForProvider.Description,
		}, false)
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateTeam)
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Team)
	if !ok {
		return errors.New(errNotTeam)
	}
	_, err := c.service.Teams.DeleteTeamBySlug(
		ctx,
		cr.Spec.ForProvider.Organization,
		meta.GetExternalName(cr))
	return errors.Wrap(err, errDeleteTeam)
}
