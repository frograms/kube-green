package rollouts

import (
	"context"
	"testing"

	"github.com/kube-green/kube-green/api/v1alpha1"
	"github.com/kube-green/kube-green/controllers/internal/testutil"
	"github.com/kube-green/kube-green/controllers/sleepinfo/resource"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

func TestNewResource(t *testing.T) {
	testLogger := zap.New(zap.UseDevMode(true))

	namespace := "my-namespace"
	rollout1 := GetMock(MockSpec{
		Name:      "rollout1",
		Namespace: namespace,
	})
	rollout2 := GetMock(MockSpec{
		Name:      "rollout2",
		Namespace: namespace,
	})
	rolloutOtherNamespace := GetMock(MockSpec{
		Name:      "rolloutOtherNamespace",
		Namespace: "other-namespace",
	})
	emptySleepInfo := &v1alpha1.SleepInfo{}

	listRolloutsTests := []struct {
		name      string
		client    client.Client
		sleepInfo *v1alpha1.SleepInfo
		expected  []rolloutsv1alpha1.Rollout
		throws    bool
	}{
		{
			name: "get list of rollouts",
			client: fake.
				NewClientBuilder().
				WithScheme(getScheme()).
				WithRuntimeObjects([]runtime.Object{&rollout1, &rollout2, &rolloutOtherNamespace}...).
				Build(),
			expected: []rolloutsv1alpha1.Rollout{rollout1, rollout2},
		},
		{
			name: "fails to list rollouts",
			client: &testutil.PossiblyErroringFakeCtrlRuntimeClient{
				Client: fake.NewClientBuilder().Build(),
				ShouldError: func(method testutil.Method, obj runtime.Object) bool {
					return method == testutil.List
				},
			},
			throws: true,
		},
		{
			name: "empty list rollouts",
			client: fake.
				NewClientBuilder().
				WithScheme(getScheme()).
				WithRuntimeObjects([]runtime.Object{&rolloutOtherNamespace}...).
				Build(),
			expected: []rolloutsv1alpha1.Rollout{},
		},
		{
			name: "with rollout to exclude",
			client: fake.
				NewClientBuilder().
				WithScheme(getScheme()).
				WithRuntimeObjects([]runtime.Object{&rollout1, &rollout2, &rolloutOtherNamespace}...).
				Build(),
			sleepInfo: &v1alpha1.SleepInfo{
				Spec: v1alpha1.SleepInfoSpec{
					ExcludeRef: []v1alpha1.ExcludeRef{
						{
							ApiVersion: "argoproj.io/v1alpha1",
							Kind:       "Rollout",
							Name:       rollout2.Name,
						},
						{
							ApiVersion: "argoproj.io/v1alpha1",
							Kind:       "resource",
							Name:       "foo",
						},
						{
							ApiVersion: "argoproj.io/v1alpha2",
							Kind:       "Rollout",
							Name:       rollout1.Name,
						},
					},
				},
			},
			expected: []rolloutsv1alpha1.Rollout{rollout1},
		},
	}

	for _, test := range listRolloutsTests {
		t.Run(test.name, func(t *testing.T) {
			sleepInfo := emptySleepInfo
			if test.sleepInfo != nil {
				sleepInfo = test.sleepInfo
			}
			d, err := NewResource(context.Background(), resource.ResourceClient{
				Client:    test.client,
				Log:       testLogger,
				SleepInfo: sleepInfo,
			}, namespace, map[string]int32{})
			if test.throws {
				require.EqualError(t, err, "error during list")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.expected, d.data)
		})
	}
}

func TestHasResource(t *testing.T) {
	testLogger := zap.New(zap.UseDevMode(true))

	namespace := "my-namespace"
	rollout1 := GetMock(MockSpec{
		Name:      "rollout1",
		Namespace: namespace,
	})

	t.Run("without resource", func(t *testing.T) {
		d, err := NewResource(context.Background(), resource.ResourceClient{
			Client:    fake.NewClientBuilder().WithScheme(getScheme()).Build(),
			Log:       testLogger,
			SleepInfo: &v1alpha1.SleepInfo{},
		}, namespace, map[string]int32{})
		require.NoError(t, err)

		require.False(t, d.HasResource())
	})

	t.Run("with resource", func(t *testing.T) {
		d, err := NewResource(context.Background(), resource.ResourceClient{
			Client:    fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&rollout1).Build(),
			Log:       testLogger,
			SleepInfo: &v1alpha1.SleepInfo{},
		}, namespace, map[string]int32{})
		require.NoError(t, err)

		require.True(t, d.HasResource())
	})
}

func TestSleep(t *testing.T) {
	testLogger := zap.New(zap.UseDevMode(true))

	var replica0 int32 = 0
	var replica1 int32 = 1
	var replica5 int32 = 5
	namespace := "my-namespace"

	r1 := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "r1",
		Replicas:        &replica1,
		ResourceVersion: "2",
	})
	r2 := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "r2",
		Replicas:        &replica5,
		ResourceVersion: "1",
	})
	rZeroReplicas := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "rZeroReplicas",
		Replicas:        &replica0,
		ResourceVersion: "1",
	})

	ctx := context.Background()
	emptySleepInfo := &v1alpha1.SleepInfo{}
	listOptions := &client.ListOptions{
		Namespace: namespace,
		Limit:     500,
	}

	t.Run("update deploy to have zero replicas", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&r1, &r2, &rZeroReplicas).Build()
		fakeClient := &testutil.PossiblyErroringFakeCtrlRuntimeClient{
			Client: c,
		}

		resource, err := NewResource(ctx, resource.ResourceClient{
			Client:    fakeClient,
			Log:       testLogger,
			SleepInfo: emptySleepInfo,
		}, namespace, map[string]int32{})
		require.NoError(t, err)

		require.NoError(t, resource.Sleep(ctx))

		list := rolloutsv1alpha1.RolloutList{}
		err = c.List(context.Background(), &list, listOptions)
		require.NoError(t, err)
		require.Equal(t, rolloutsv1alpha1.RolloutList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "RolloutList",
				APIVersion: "argoproj.io/v1alpha1",
			},
			Items: []rolloutsv1alpha1.Rollout{
				GetMock(MockSpec{
					Namespace:       namespace,
					Name:            "r1",
					Replicas:        &replica0,
					ResourceVersion: "3",
				}),
				GetMock(MockSpec{
					Namespace:       namespace,
					Name:            "r2",
					Replicas:        &replica0,
					ResourceVersion: "2",
				}),
				rZeroReplicas,
			},
		}, list)
	})

	t.Run("fails to patch rollout", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&r1, &r2, &rZeroReplicas).Build()
		fakeClient := &testutil.PossiblyErroringFakeCtrlRuntimeClient{
			Client: c,
			ShouldError: func(method testutil.Method, obj runtime.Object) bool {
				return method == testutil.Patch
			},
		}

		resource, err := NewResource(ctx, resource.ResourceClient{
			Client:    fakeClient,
			Log:       testLogger,
			SleepInfo: emptySleepInfo,
		}, namespace, map[string]int32{})
		require.NoError(t, err)

		require.EqualError(t, resource.Sleep(ctx), "error during patch")
	})

	t.Run("not fails if rollouts not found", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&r1, &r2, &rZeroReplicas).Build()
		fakeClient := &testutil.PossiblyErroringFakeCtrlRuntimeClient{
			Client: c,
		}

		resource, err := NewResource(ctx, resource.ResourceClient{
			Client:    fakeClient,
			Log:       testLogger,
			SleepInfo: emptySleepInfo,
		}, namespace, map[string]int32{})
		require.NoError(t, err)

		err = c.DeleteAllOf(ctx, &rolloutsv1alpha1.Rollout{}, &client.DeleteAllOfOptions{})
		require.NoError(t, err)

		require.NoError(t, resource.Sleep(ctx))
	})
}

func TestWakeUp(t *testing.T) {
	testLogger := zap.New(zap.UseDevMode(true))

	var replica0 int32 = 0
	var replica1 int32 = 1
	var replica5 int32 = 5
	namespace := "my-namespace"

	r1 := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "r1",
		Replicas:        &replica0,
		ResourceVersion: "2",
	})
	r2 := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "r2",
		Replicas:        &replica0,
		ResourceVersion: "1",
	})
	rZeroReplicas := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "rZeroReplicas",
		Replicas:        &replica0,
		ResourceVersion: "1",
	})
	dAfterSleep := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "aftersleep",
		Replicas:        &replica1,
		ResourceVersion: "1",
	})

	ctx := context.Background()
	emptySleepInfo := &v1alpha1.SleepInfo{}
	listOptions := &client.ListOptions{
		Namespace: namespace,
		Limit:     500,
	}

	t.Run("wake up deploy", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&r1, &r2, &rZeroReplicas, &dAfterSleep).Build()
		r, err := NewResource(ctx, resource.ResourceClient{
			Client:    c,
			Log:       testLogger,
			SleepInfo: emptySleepInfo,
		}, namespace, map[string]int32{
			r1.Name: replica1,
			r2.Name: replica5,
		})
		require.NoError(t, err)

		err = r.WakeUp(ctx)
		require.NoError(t, err)

		list := rolloutsv1alpha1.RolloutList{}
		err = c.List(context.Background(), &list, listOptions)
		require.NoError(t, err)
		require.Equal(t, rolloutsv1alpha1.RolloutList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "RolloutList",
				APIVersion: "argoproj.io/v1alpha1",
			},
			Items: []rolloutsv1alpha1.Rollout{
				dAfterSleep,
				GetMock(MockSpec{
					Namespace:       namespace,
					Name:            "r1",
					Replicas:        &replica1,
					ResourceVersion: "3",
				}),
				GetMock(MockSpec{
					Namespace:       namespace,
					Name:            "r2",
					Replicas:        &replica5,
					ResourceVersion: "2",
				}),
				rZeroReplicas,
			},
		}, list)
	})

	t.Run("wake up fails", func(t *testing.T) {
		c := testutil.PossiblyErroringFakeCtrlRuntimeClient{
			Client: fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&r1).Build(),
			ShouldError: func(method testutil.Method, obj runtime.Object) bool {
				return method == testutil.Patch
			},
		}
		r, err := NewResource(ctx, resource.ResourceClient{
			Client:    c,
			Log:       testLogger,
			SleepInfo: emptySleepInfo,
		}, namespace, map[string]int32{
			r1.Name: replica1,
			r2.Name: replica5,
		})
		require.NoError(t, err)

		err = r.WakeUp(ctx)
		require.EqualError(t, err, "error during patch")
	})
}

func TestRolloutOriginalReplicas(t *testing.T) {
	testLogger := zap.New(zap.UseDevMode(true))

	ctx := context.Background()
	namespace := "my-namespace"
	emptySleepInfo := &v1alpha1.SleepInfo{}
	var replica0 int32 = 0
	var replica1 int32 = 1
	var replica5 int32 = 5

	r1 := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "r1",
		Replicas:        &replica1,
		ResourceVersion: "2",
	})
	r2 := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "r2",
		Replicas:        &replica5,
		ResourceVersion: "1",
	})
	rZeroReplicas := GetMock(MockSpec{
		Namespace:       namespace,
		Name:            "dZeroReplica",
		Replicas:        &replica0,
		ResourceVersion: "1",
	})

	t.Run("save and restore replicas info", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(getScheme()).WithRuntimeObjects(&r1, &r2, &rZeroReplicas).Build()
		r, err := NewResource(ctx, resource.ResourceClient{
			Client:    c,
			Log:       testLogger,
			SleepInfo: emptySleepInfo,
		}, namespace, map[string]int32{
			r1.Name: replica1,
			r2.Name: replica5,
		})
		require.NoError(t, err)

		res, err := r.GetOriginalInfoToSave()
		require.NoError(t, err)

		expectedInfoToSave := `[{"name":"r1","replicas":1},{"name":"r2","replicas":5}]`
		require.JSONEq(t, expectedInfoToSave, string(res))

		t.Run("restore saved info", func(t *testing.T) {
			infoToSave := []byte(expectedInfoToSave)
			restoredInfo, err := GetOriginalInfoToRestore(infoToSave)
			require.NoError(t, err)
			require.Equal(t, map[string]int32{
				r1.Name: replica1,
				r2.Name: replica5,
			}, restoredInfo)
		})
	})

	t.Run("restore info with data nil", func(t *testing.T) {
		info, err := GetOriginalInfoToRestore(nil)
		require.Equal(t, map[string]int32{}, info)
		require.NoError(t, err)
	})

	t.Run("fails if saved data are not valid json", func(t *testing.T) {
		info, err := GetOriginalInfoToRestore([]byte(`{}`))
		require.EqualError(t, err, "json: cannot unmarshal object into Go value of type []rollouts.OriginalReplicas")
		require.Nil(t, info)
	})
}
