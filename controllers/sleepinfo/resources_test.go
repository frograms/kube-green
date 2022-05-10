package sleepinfo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kube-green/kube-green/api/v1alpha1"
	"github.com/kube-green/kube-green/controllers/internal/testutil"
	"github.com/kube-green/kube-green/controllers/sleepinfo/cronjobs"
	"github.com/kube-green/kube-green/controllers/sleepinfo/deployments"
	"github.com/kube-green/kube-green/controllers/sleepinfo/resource"
	"github.com/kube-green/kube-green/controllers/sleepinfo/rollouts"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

func TestNewResources(t *testing.T) {
	namespace := "my-namespace"
	replica1 := int32(1)

	cronJob := cronjobs.GetMock(cronjobs.MockSpec{
		Name:      "cronjob",
		Namespace: namespace,
	})
	deployments := deployments.GetMock(deployments.MockSpec{
		Name:      "deploy",
		Replicas:  &replica1,
		Namespace: namespace,
	})
	rollouts := rollouts.GetMock(rollouts.MockSpec{
		Name:      "rollout",
		Replicas:  &replica1,
		Namespace: namespace,
	})

	t.Run("errors if client is not valid", func(t *testing.T) {
		resClient := resource.ResourceClient{}
		res, err := NewResources(context.Background(), resClient, namespace, SleepInfoData{})
		require.True(t, strings.HasPrefix(err.Error(), "invalid client"))
		require.Empty(t, res)
	})

	t.Run("retrieve deployments data", func(t *testing.T) {
		resClient := resource.ResourceClient{
			Client:    getFakeClient().WithRuntimeObjects(&cronJob, &deployments, &rollouts).Build(),
			Log:       zap.New(zap.UseDevMode(true)),
			SleepInfo: &v1alpha1.SleepInfo{},
		}
		res, err := NewResources(context.Background(), resClient, namespace, SleepInfoData{})
		require.NoError(t, err)
		require.True(t, res.deployments.HasResource())
		require.True(t, res.rollouts.HasResource())
		require.False(t, res.cronjobs.HasResource())
	})

	t.Run("retrieve deployments and cron jobs data", func(t *testing.T) {
		resClient := resource.ResourceClient{
			Client: getFakeClient().WithRuntimeObjects(&cronJob, &deployments, &rollouts).Build(),
			Log:    zap.New(zap.UseDevMode(true)),
			SleepInfo: &v1alpha1.SleepInfo{
				Spec: v1alpha1.SleepInfoSpec{
					SuspendCronjobs: true,
				},
			},
		}
		res, err := NewResources(context.Background(), resClient, namespace, SleepInfoData{})
		require.NoError(t, err)
		require.True(t, res.deployments.HasResource())
		require.True(t, res.cronjobs.HasResource())
		require.True(t, res.rollouts.HasResource())
	})

	t.Run("throws if fetch deployments fails", func(t *testing.T) {
		resClient := resource.ResourceClient{
			Client: testutil.PossiblyErroringFakeCtrlRuntimeClient{
				Client: getFakeClient().WithRuntimeObjects(&cronJob, &deployments, &rollouts).Build(),
				ShouldError: func(method testutil.Method, obj runtime.Object) bool {
					_, ok := obj.(*appsv1.DeploymentList)
					return method == testutil.List && ok
				},
			},
			Log:       zap.New(zap.UseDevMode(true)),
			SleepInfo: &v1alpha1.SleepInfo{},
		}
		res, err := NewResources(context.Background(), resClient, namespace, SleepInfoData{})
		require.EqualError(t, err, "error during list")
		require.Empty(t, res)
	})

	t.Run("throws if fetch cron job fails", func(t *testing.T) {
		resClient := resource.ResourceClient{
			Client: testutil.PossiblyErroringFakeCtrlRuntimeClient{
				Client: getFakeClient().WithRuntimeObjects(&cronJob, &deployments, &rollouts).Build(),
				ShouldError: func(method testutil.Method, obj runtime.Object) bool {
					kind := obj.GetObjectKind().GroupVersionKind().Kind
					return method == testutil.List && kind == "CronJob"
				},
			},
			Log: zap.New(zap.UseDevMode(true)),
			SleepInfo: &v1alpha1.SleepInfo{
				Spec: v1alpha1.SleepInfoSpec{
					SuspendCronjobs: true,
				},
			},
		}
		res, err := NewResources(context.Background(), resClient, namespace, SleepInfoData{})
		require.EqualError(t, err, fmt.Sprintf("%s: error during list", cronjobs.ErrFetchingCronJobs))
		require.Empty(t, res)
	})

	t.Run("throws if fetch rollouts fails", func(t *testing.T) {
		resClient := resource.ResourceClient{
			Client: testutil.PossiblyErroringFakeCtrlRuntimeClient{
				Client: getFakeClient().WithRuntimeObjects(&cronJob, &deployments, &rollouts).Build(),
				ShouldError: func(method testutil.Method, obj runtime.Object) bool {
					_, ok := obj.(*rolloutsv1alpha1.RolloutList)
					return method == testutil.List && ok
				},
			},
			Log:       zap.New(zap.UseDevMode(true)),
			SleepInfo: &v1alpha1.SleepInfo{},
		}
		res, err := NewResources(context.Background(), resClient, namespace, SleepInfoData{})
		require.EqualError(t, err, "error during list")
		require.Empty(t, res)
	})
}

func TestHasResources(t *testing.T) {
	testLogger := zap.New(zap.UseDevMode(true))

	namespace := "my-namespace"
	tests := []struct {
		name                     string
		deploy                   bool
		cronJob                  bool
		rollout                  bool
		expectToPerformOperation bool
	}{
		{
			name:                     "empty resources",
			expectToPerformOperation: false,
		},
		{
			name:                     "some deployments",
			deploy:                   true,
			expectToPerformOperation: true,
		},
		{
			name:                     "some cronjobs",
			cronJob:                  true,
			expectToPerformOperation: true,
		},
		{
			name:                     "some rollouts",
			rollout:                  true,
			expectToPerformOperation: true,
		},
		{
			name:                     "cronjobs and deployments and rollouts",
			cronJob:                  true,
			deploy:                   true,
			rollout:                  true,
			expectToPerformOperation: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resources, err := NewResources(context.Background(), resource.ResourceClient{
				Log:       testLogger,
				Client:    getFakeClient().Build(),
				SleepInfo: &v1alpha1.SleepInfo{},
			}, namespace, SleepInfoData{})
			require.NoError(t, err)

			resources.deployments = resource.GetResourceMock(resource.ResourceMock{
				HasResourceResponseMock: test.deploy,
			})

			resources.cronjobs = resource.GetResourceMock(resource.ResourceMock{
				HasResourceResponseMock: test.cronJob,
			})

			resources.rollouts = resource.GetResourceMock(resource.ResourceMock{
				HasResourceResponseMock: test.rollout,
			})

			require.Equal(t, test.expectToPerformOperation, resources.hasResources())
		})
	}
}

func TestResourcesSleep(t *testing.T) {
	t.Run("correctly sleep all resources", func(t *testing.T) {
		numberOfCalledDeploymentSleep := 0
		numberOfCalledCronJobSleep := 0
		numberOfCalledRolloutSleep := 0
		cronJobMock := resource.ResourceMock{
			MockSleep: func(ctx context.Context) error {
				numberOfCalledCronJobSleep++
				return nil
			},
		}
		deploymentMock := resource.ResourceMock{
			MockSleep: func(ctx context.Context) error {
				numberOfCalledDeploymentSleep++
				return nil
			},
		}
		rolloutMock := resource.ResourceMock{
			MockSleep: func(ctx context.Context) error {
				numberOfCalledRolloutSleep++
				return nil
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.sleep(context.Background())
		require.NoError(t, err)

		require.Equal(t, 1, numberOfCalledDeploymentSleep, "calls deployments sleep")
		require.Equal(t, 1, numberOfCalledCronJobSleep, "calls cron job sleep")
		require.Equal(t, 1, numberOfCalledRolloutSleep, "calls rollouts job sleep")
	})

	t.Run("throws if deployment sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{
			MockSleep: func(ctx context.Context) error {
				return fmt.Errorf("some error")
			},
		}
		cronJobMock := resource.ResourceMock{}
		rolloutMock := resource.ResourceMock{}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.sleep(context.Background())
		require.EqualError(t, err, "some error")
	})

	t.Run("throws if cron job sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{}
		cronJobMock := resource.ResourceMock{
			MockSleep: func(ctx context.Context) error {
				return fmt.Errorf("some error")
			},
		}
		rolloutMock := resource.ResourceMock{}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.sleep(context.Background())
		require.EqualError(t, err, "some error")
	})

	t.Run("throws if rollout sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{}
		cronJobMock := resource.ResourceMock{}
		rolloutMock := resource.ResourceMock{
			MockSleep: func(ctx context.Context) error {
				return fmt.Errorf("some error")
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.sleep(context.Background())
		require.EqualError(t, err, "some error")
	})
}

func TestResourcesWakeUp(t *testing.T) {
	t.Run("correctly wake up all resources", func(t *testing.T) {
		numberOfCalledDeploymentWakeUp := 0
		numberOfCalledCronJobWakeUp := 0
		numberOfCalledRolloutWakeUp := 0
		cronJobMock := resource.ResourceMock{
			MockWakeUp: func(ctx context.Context) error {
				numberOfCalledCronJobWakeUp++
				return nil
			},
		}
		deploymentMock := resource.ResourceMock{
			MockWakeUp: func(ctx context.Context) error {
				numberOfCalledDeploymentWakeUp++
				return nil
			},
		}
		rolloutMock := resource.ResourceMock{
			MockWakeUp: func(ctx context.Context) error {
				numberOfCalledRolloutWakeUp++
				return nil
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.wakeUp(context.Background())
		require.NoError(t, err)

		require.Equal(t, 1, numberOfCalledDeploymentWakeUp, "calls deployments wake up")
		require.Equal(t, 1, numberOfCalledCronJobWakeUp, "calls cron job wake up")
		require.Equal(t, 1, numberOfCalledRolloutWakeUp, "calls rollouts wake up")
	})

	t.Run("throws if deployment sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{
			MockWakeUp: func(ctx context.Context) error {
				return fmt.Errorf("some error")
			},
		}
		cronJobMock := resource.ResourceMock{}
		rolloutMock := resource.ResourceMock{}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.wakeUp(context.Background())
		require.EqualError(t, err, "some error")
	})

	t.Run("throws if cron job sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{}
		cronJobMock := resource.ResourceMock{
			MockWakeUp: func(ctx context.Context) error {
				return fmt.Errorf("some error")
			},
		}
		rolloutMock := resource.ResourceMock{}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.wakeUp(context.Background())
		require.EqualError(t, err, "some error")
	})

	t.Run("throws if rollout sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{}
		cronJobMock := resource.ResourceMock{}
		rolloutMock := resource.ResourceMock{
			MockWakeUp: func(ctx context.Context) error {
				return fmt.Errorf("some error")
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		err := r.wakeUp(context.Background())
		require.EqualError(t, err, "some error")
	})
}

func TestGetOriginalResourceInfoToSave(t *testing.T) {
	t.Run("correctly get original resources", func(t *testing.T) {
		numberOfCalledDeploymentInfoToSave := 0
		numberOfCalledCronJobInfoToSave := 0
		numberOfCalledRolloutInfoToSave := 0
		deploymentMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledDeploymentInfoToSave++
				return nil, nil
			},
		}
		cronJobMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledCronJobInfoToSave++
				return []byte("[]"), nil
			},
		}
		rolloutMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledRolloutInfoToSave++
				return nil, nil
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		data, err := r.getOriginalResourceInfoToSave()
		require.NoError(t, err)
		require.Equal(t, map[string][]byte{
			replicasBeforeSleepKey:        nil,
			originalCronjobStatusKey:      []byte("[]"),
			replicasRolloutBeforeSleepKey: nil,
		}, data)

		require.Equal(t, 1, numberOfCalledDeploymentInfoToSave, "calls deployments wake up")
		require.Equal(t, 1, numberOfCalledCronJobInfoToSave, "calls cron job wake up")
		require.Equal(t, 1, numberOfCalledRolloutInfoToSave, "calls rollouts wake up")
	})

	t.Run("correctly get original resources only for deployments", func(t *testing.T) {
		numberOfCalledDeploymentInfoToSave := 0
		numberOfCalledCronJobInfoToSave := 0
		numberOfCalledRolloutInfoToSave := 0
		cronJobMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledCronJobInfoToSave++
				return nil, nil
			},
		}
		deploymentMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledDeploymentInfoToSave++
				return []byte("[]"), nil
			},
		}
		rolloutMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledRolloutInfoToSave++
				return nil, nil
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		data, err := r.getOriginalResourceInfoToSave()
		require.NoError(t, err)
		require.Equal(t, map[string][]byte{
			replicasBeforeSleepKey: []byte("[]"),
		}, data)

		require.Equal(t, 1, numberOfCalledDeploymentInfoToSave, "calls deployments wake up")
		require.Equal(t, 1, numberOfCalledCronJobInfoToSave, "calls cron job wake up")
		require.Equal(t, 1, numberOfCalledRolloutInfoToSave, "calls rollouts wake up")
	})

	t.Run("correctly get original resources only for rollouts", func(t *testing.T) {
		numberOfCalledDeploymentInfoToSave := 0
		numberOfCalledCronJobInfoToSave := 0
		numberOfCalledRolloutInfoToSave := 0
		cronJobMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledCronJobInfoToSave++
				return nil, nil
			},
		}
		deploymentMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledDeploymentInfoToSave++
				return nil, nil
			},
		}
		rolloutMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				numberOfCalledRolloutInfoToSave++
				return []byte("[]"), nil
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		data, err := r.getOriginalResourceInfoToSave()
		require.NoError(t, err)
		require.Equal(t, map[string][]byte{
			replicasRolloutBeforeSleepKey: []byte("[]"),
		}, data)

		require.Equal(t, 1, numberOfCalledDeploymentInfoToSave, "calls deployments wake up")
		require.Equal(t, 1, numberOfCalledCronJobInfoToSave, "calls cron job wake up")
		require.Equal(t, 1, numberOfCalledRolloutInfoToSave, "calls rollouts wake up")
	})

	t.Run("throws if deployment sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				return nil, fmt.Errorf("some error")
			},
		}
		cronJobMock := resource.ResourceMock{}
		rolloutMock := resource.ResourceMock{}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		_, err := r.getOriginalResourceInfoToSave()
		require.EqualError(t, err, "some error")
	})

	t.Run("throws if cron job sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{}
		cronJobMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				return nil, fmt.Errorf("some error")
			},
		}
		rolloutMock := resource.ResourceMock{}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		_, err := r.getOriginalResourceInfoToSave()
		require.EqualError(t, err, "some error")
	})

	t.Run("throws if rollout sleep fails", func(t *testing.T) {
		deploymentMock := resource.ResourceMock{}
		cronJobMock := resource.ResourceMock{}
		rolloutMock := resource.ResourceMock{
			MockOriginalInfoToSave: func() ([]byte, error) {
				return nil, fmt.Errorf("some error")
			},
		}

		r := newResourcesMock(t, deploymentMock, cronJobMock, rolloutMock)
		_, err := r.getOriginalResourceInfoToSave()
		require.EqualError(t, err, "some error")
	})
}

func TestSetOriginalResourceInfoToRestoreInSleepInfo(t *testing.T) {
	t.Run("empty sleepInfoData if data is nil", func(t *testing.T) {
		sleepInfoData := SleepInfoData{}
		err := setOriginalResourceInfoToRestoreInSleepInfo(nil, &sleepInfoData)
		require.Equal(t, sleepInfoData, sleepInfoData)
		require.NoError(t, err)
	})

	t.Run("empty sleepInfoData if data is empty", func(t *testing.T) {
		sleepInfoData := SleepInfoData{}
		err := setOriginalResourceInfoToRestoreInSleepInfo(map[string][]byte{}, &sleepInfoData)
		require.Equal(t, sleepInfoData, sleepInfoData)
		require.NoError(t, err)
	})

	t.Run("deployment throws if data is not a correct json", func(t *testing.T) {
		sleepInfoData := SleepInfoData{}
		data := map[string][]byte{
			replicasBeforeSleepKey: []byte("{}"),
		}
		err := setOriginalResourceInfoToRestoreInSleepInfo(data, &sleepInfoData)
		require.EqualError(t, err, "json: cannot unmarshal object into Go value of type []deployments.OriginalReplicas")
	})

	t.Run("cronjob throws if data is not a correct json", func(t *testing.T) {
		sleepInfoData := SleepInfoData{}
		data := map[string][]byte{
			originalCronjobStatusKey: []byte("{}"),
		}
		err := setOriginalResourceInfoToRestoreInSleepInfo(data, &sleepInfoData)
		require.EqualError(t, err, "json: cannot unmarshal object into Go value of type []cronjobs.OriginalCronJobStatus")
	})

	t.Run("rollout throws if data is not a correct json", func(t *testing.T) {
		sleepInfoData := SleepInfoData{}
		data := map[string][]byte{
			replicasRolloutBeforeSleepKey: []byte("{}"),
		}
		err := setOriginalResourceInfoToRestoreInSleepInfo(data, &sleepInfoData)
		require.EqualError(t, err, "json: cannot unmarshal object into Go value of type []rollouts.OriginalReplicas")
	})

	t.Run("correctly set sleep info data for deployments and cronjobs and rollouts", func(t *testing.T) {
		sleepInfoData := SleepInfoData{}
		data := map[string][]byte{
			originalCronjobStatusKey:      []byte(`[{"name":"cj1","suspend":true}]`),
			replicasBeforeSleepKey:        []byte(`[{"name":"deploy1","replicas":5}]`),
			replicasRolloutBeforeSleepKey: []byte(`[{"name":"rollout1","replicas":5}]`),
		}
		err := setOriginalResourceInfoToRestoreInSleepInfo(data, &sleepInfoData)
		require.NoError(t, err)
		require.Equal(t, SleepInfoData{
			OriginalCronJobStatus:       map[string]bool{"cj1": true},
			OriginalDeploymentsReplicas: map[string]int32{"deploy1": 5},
			OriginalRolloutsReplicas:    map[string]int32{"rollout1": 5},
		}, sleepInfoData)
	})
}

func newResourcesMock(t *testing.T, deploymentsMock resource.ResourceMock, cronjobsMock resource.ResourceMock, rolloutsMock resource.ResourceMock) Resources {
	t.Helper()
	return Resources{
		deployments: resource.GetResourceMock(deploymentsMock),
		cronjobs:    resource.GetResourceMock(cronjobsMock),
		rollouts:    resource.GetResourceMock(rolloutsMock),
	}
}

func getFakeClient() *fake.ClientBuilder {
	groupVersion := []schema.GroupVersion{
		{Group: "batch", Version: "v1"},
		{Group: "batch", Version: "v1beta1"},
	}
	restMapper := meta.NewDefaultRESTMapper(groupVersion)
	restMapper.Add(schema.GroupVersionKind{
		Group:   "batch",
		Version: "v1",
		Kind:    "CronJob",
	}, meta.RESTScopeNamespace)
	restMapper.Add(schema.GroupVersionKind{
		Group:   "batch",
		Version: "v1beta1",
		Kind:    "CronJob",
	}, meta.RESTScopeNamespace)

	s := k8sscheme.Scheme
	rolloutsv1alpha1.AddToScheme(s)

	return fake.
		NewClientBuilder().
		WithScheme(s).
		WithRESTMapper(restMapper)
}
