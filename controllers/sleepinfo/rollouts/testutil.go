package rollouts

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

type MockSpec struct {
	Namespace       string
	Name            string
	Replicas        *int32
	ResourceVersion string
	PodAnnotations  map[string]string
	MatchLabels     map[string]string
}

func GetMock(opts MockSpec) rolloutsv1alpha1.Rollout {
	matchLabels := opts.MatchLabels
	if matchLabels == nil {
		opts.MatchLabels = map[string]string{
			"app": opts.Name,
		}
	}

	return rolloutsv1alpha1.Rollout{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Rollout",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            opts.Name,
			Namespace:       opts.Namespace,
			ResourceVersion: opts.ResourceVersion,
			Annotations:     opts.PodAnnotations,
		},
		Spec: rolloutsv1alpha1.RolloutSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: opts.MatchLabels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: opts.MatchLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "container",
							Image: "my-image",
						},
					},
				},
			},
			Replicas: opts.Replicas,
		},
	}
}

func getScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	rolloutsv1alpha1.AddToScheme(scheme)
	return scheme
}
