package wordpress

import (
	"fmt"

	websitev1 "github.com/k8s-controllers/wordpress-builder/pkg/apis/website/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiutil "k8s.io/apimachinery/pkg/util/intstr"
)

const (
	wordpressHTTPPort = 80
)

// Wordpress embeds wordpressv1alpha1.Wordpress and adds utility functions
type Wordpress struct {
	*websitev1.Wordpress
}

// New wraps a wordpressv1alpha1.Wordpress into a Wordpress object
func New(obj *websitev1.Wordpress) *Wordpress {
	return &Wordpress{obj}
}

// Unwrap returns the wrapped wordpressv1alpha1.Wordpress object
func (wp *Wordpress) Unwrap() *websitev1.Wordpress {
	return wp.Wordpress
}

func (wp *Wordpress) image() string {
	return fmt.Sprintf("%s:%s", wp.Spec.Image, wp.Spec.Tag)
}
func (wp *Wordpress) getNamespace() string {
	namespace := apiv1.NamespaceDefault
	if wp.Namespace != "" {
		namespace = wp.Namespace
	}
	return namespace
}

const (
	defaultTag   = "latest"
	defaultImage = "ajipsum/wordpress-apache"
	// codeSrcMountPath     = "/var/run/presslabs.org/code/src"
	// defaultCodeMountPath = "/var/www/html/wp-content"
)

// SetDefaults sets Wordpress field defaults
func (wp *Wordpress) SetDefaults() {
	if len(wp.Spec.Image) == 0 {
		wp.Spec.Image = defaultImage
	}

	if len(wp.Spec.Tag) == 0 {
		wp.Spec.Tag = defaultTag
	}

	// if o.Spec.CodeVolumeSpec != nil && len(o.Spec.CodeVolumeSpec.MountPath) == 0 {
	// 	o.Spec.CodeVolumeSpec.MountPath = defaultCodeMountPath
	// }
}

func (wp *Wordpress) env() []apiv1.EnvVar {
	// scheme := "http"
	// if false {
	// 	scheme = "https"
	// }

	out := append([]apiv1.EnvVar{
		// {
		// 	Name:  "WP_HOME",
		// 	Value: fmt.Sprintf("%s://%s", scheme, wp.Spec.Domains[0]),
		// },
		// {
		// 	Name:  "WP_SITEURL",
		// 	Value: fmt.Sprintf("%s://%s/wp", scheme, wp.Spec.Domains[0]),
		// },
	}, wp.Spec.Env...)

	return out
}

func (wp *Wordpress) createDeployment() (out *appsv1.Deployment) {
	deploymentName := wp.Spec.DeploymentName
	// deploy := apiv1.PodTemplateSpec{}
	// deploy.ObjectMeta = metav1.ObjectMeta{
	// 	Name:      wp.deploymentName,
	// 	Namespace: wp.Namespace,
	// 	OwnerReferences: []metav1.OwnerReference{
	// 		{
	// 			APIVersion: "wordpresses.website.k8s.io/v1",
	// 			Kind:       "Wordpress",
	// 			Name:       wp.Name,
	// 			UID:        wp.UID,
	// 		},
	// 	},
	// }
	// deploy.Spec.Replicas = int32Ptr(1)
	// deploy.Spec.Selector = &metav1.LabelSelector{
	// 	MatchLabels: map[string]string{
	// 		"deployment": deploymentName,
	// 	},
	// }

	// deploy.Spec.Template.ObjectMeta = metav1.ObjectMeta{
	// 	Labels: map[string]string{
	// 		"deployment": deploymentName,
	// 	},
	// }

	// deploy.Spec.Containers = []apiv1.Container{
	// 	{
	// 		Name:  wp.deploymentName,
	// 		Image: wp.image(),
	// 		// VolumeMounts: wp.volumeMounts(),
	// 		Env:     wp.env(),
	// 		EnvFrom: wp.Spec.EnvFrom,
	// 		Ports: []apiv1.ContainerPort{
	// 			{
	// 				Name:          "http",
	// 				ContainerPort: int32(wordpressHTTPPort),
	// 			},
	// 		},
	// 		ReadinessProbe: &apiv1.Probe{
	// 			Handler: apiv1.Handler{
	// 				TCPSocket: &apiv1.TCPSocketAction{
	// 					Port: apiutil.FromInt(80),
	// 				},
	// 			},
	// 			InitialDelaySeconds: 5,
	// 			TimeoutSeconds:      60,
	// 			PeriodSeconds:       2,
	// 		},
	// 	},
	// }

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: wp.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "wordpresses.website.k8s.io/v1",
					Kind:       "Wordpress",
					Name:       wp.Name,
					UID:        wp.UID,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deployment": deploymentName,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"deployment": deploymentName,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  deploymentName,
							Image: wp.image(),
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: int32(wordpressHTTPPort),
								},
							},
							ReadinessProbe: &apiv1.Probe{
								Handler: apiv1.Handler{
									TCPSocket: &apiv1.TCPSocketAction{
										Port: apiutil.FromInt(80),
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      60,
								PeriodSeconds:       2,
							},
							Env: wp.env(),
						},
					},
				},
			},
		},
	}
	return deploy
}

func int32Ptr(i int32) *int32 { return &i }
