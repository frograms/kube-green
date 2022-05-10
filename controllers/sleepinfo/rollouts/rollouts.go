package rollouts

import (
	"context"
	"encoding/json"

	kubegreenv1alpha1 "github.com/kube-green/kube-green/api/v1alpha1"
	"github.com/kube-green/kube-green/controllers/sleepinfo/resource"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apisrollouts "github.com/argoproj/argo-rollouts/pkg/apis/rollouts"
	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

type rollouts struct {
	resource.ResourceClient
	data             []rolloutsv1alpha1.Rollout
	OriginalReplicas map[string]int32
}

func NewResource(ctx context.Context, res resource.ResourceClient, namespace string, originalReplicas map[string]int32) (rollouts, error) {
	r := rollouts{
		ResourceClient:   res,
		OriginalReplicas: originalReplicas,
	}

	if err := r.fetch(ctx, namespace); err != nil {
		return rollouts{}, err
	}

	return r, nil
}

func (r rollouts) HasResource() bool {
	return len(r.data) > 0
}

func (r rollouts) Sleep(ctx context.Context) error {
	for _, rollout := range r.data {
		rollout := rollout

		rolloutReplicas := *rollout.Spec.Replicas
		if rolloutReplicas == 0 {
			continue
		}
		newRollout := rollout.DeepCopy()
		*newRollout.Spec.Replicas = 0

		if err := r.Patch(ctx, &rollout, newRollout); err != nil {
			return err
		}
	}
	return nil
}

func (r rollouts) WakeUp(ctx context.Context) error {
	for _, rollout := range r.data {
		rollout := rollout

		rolloutLogger := r.Log.WithValues("rollout", rollout.Name, "namespace", rollout.Namespace)
		if *rollout.Spec.Replicas != 0 {
			rolloutLogger.Info("replicas not 0 during wake up")
			continue
		}

		replica, ok := r.OriginalReplicas[rollout.Name]
		if !ok {
			rolloutLogger.Info("original rollout info not correctly set")
			continue
		}

		newRollout := rollout.DeepCopy()
		*newRollout.Spec.Replicas = replica

		if err := r.Patch(ctx, &rollout, newRollout); err != nil {
			return err
		}
	}
	return nil
}

func (r *rollouts) fetch(ctx context.Context, namespace string) error {
	log := r.Log.WithValues("namespace", namespace)

	rolloutList, err := r.getListByNamespace(ctx, namespace)
	if err != nil {
		return err
	}
	log.V(1).Info("rollouts in namespace", "number of rollout", len(rolloutList))
	r.data = r.filterExcludedRollout(rolloutList)
	return nil
}

func (r rollouts) getListByNamespace(ctx context.Context, namespace string) ([]rolloutsv1alpha1.Rollout, error) {
	listOptions := &client.ListOptions{
		Namespace: namespace,
		Limit:     500,
	}
	rollouts := rolloutsv1alpha1.RolloutList{}
	if err := r.Client.List(ctx, &rollouts, listOptions); err != nil {
		return rollouts.Items, client.IgnoreNotFound(err)
	}
	return rollouts.Items, nil
}

func (r rollouts) filterExcludedRollout(rolloutsList []rolloutsv1alpha1.Rollout) []rolloutsv1alpha1.Rollout {
	excludedRolloutName := getExcludedRolloutName(r.SleepInfo)
	filteredList := []rolloutsv1alpha1.Rollout{}
	for _, rollout := range rolloutsList {
		if !excludedRolloutName[rollout.Name] {
			filteredList = append(filteredList, rollout)
		}
	}
	return filteredList
}

func getExcludedRolloutName(sleepInfo *kubegreenv1alpha1.SleepInfo) map[string]bool {
	excludedRolloutName := map[string]bool{}
	for _, exclusion := range sleepInfo.GetExcludeRef() {
		if exclusion.Kind == apisrollouts.RolloutKind && exclusion.ApiVersion == apisrollouts.Group {
			excludedRolloutName[exclusion.Name] = true
		}
	}
	return excludedRolloutName
}

type OriginalReplicas struct {
	Name     string `json:"name"`
	Replicas int32  `json:"replicas"`
}

func (r rollouts) GetOriginalInfoToSave() ([]byte, error) {
	originalRolloutsReplicas := []OriginalReplicas{}
	for _, rollout := range r.data {
		originalReplicas := *rollout.Spec.Replicas
		if replica, ok := r.OriginalReplicas[rollout.Name]; ok {
			if ok && replica != 0 {
				originalReplicas = replica
			}
		}
		if originalReplicas == 0 {
			continue
		}
		originalRolloutsReplicas = append(originalRolloutsReplicas, OriginalReplicas{
			Name:     rollout.Name,
			Replicas: originalReplicas,
		})
	}
	return json.Marshal(originalRolloutsReplicas)
}

func GetOriginalInfoToRestore(data []byte) (map[string]int32, error) {
	if data == nil {
		return map[string]int32{}, nil
	}
	originalRolloutsReplicas := []OriginalReplicas{}
	originalRolloutsReplicasData := map[string]int32{}
	if err := json.Unmarshal(data, &originalRolloutsReplicas); err != nil {
		return nil, err
	}
	for _, replicaInfo := range originalRolloutsReplicas {
		if replicaInfo.Name != "" {
			originalRolloutsReplicasData[replicaInfo.Name] = replicaInfo.Replicas
		}
	}
	return originalRolloutsReplicasData, nil
}
