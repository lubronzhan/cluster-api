package main

import (
	"fmt"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	machinesv1 "k8s.io/kube-deploy/cluster-api/api/machines/v1alpha1"
	gceconfig "k8s.io/kube-deploy/cluster-api/examples/gce-machines-controller/apis/gceproviderconfig"
	gceconfigv1 "k8s.io/kube-deploy/cluster-api/examples/gce-machines-controller/apis/gceproviderconfig/v1alpha1"
)

type GCEClient struct {
	service      *compute.Service
	scheme       *runtime.Scheme
	codecFactory *serializer.CodecFactory
}

func New() (*GCEClient, error) {
	client, err := google.DefaultClient(context.TODO(), compute.ComputeScope)
	if err != nil {
		return nil, err
	}

	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	scheme, codecFactory, err := gceconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	return &GCEClient{
		service:      service,
		scheme:       scheme,
		codecFactory: codecFactory,
	}, nil
}

func (gce *GCEClient) CreateVM(machine *machinesv1.Machine) error {
	config, err := gce.providerconfig(machine)
	if err != nil {
		return err
	}

	// TODO: still need to specify startup script to actually install/run Kubernetes
	_, err = gce.service.Instances.Insert(config.Project, config.Zone, &compute.Instance{
		Name:        machine.ObjectMeta.Name,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", config.Zone, config.MachineType),
		Zone:        config.Zone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: "global/networks/default",
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
			},
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: config.Image,
					DiskSizeGb:  10,
				},
			},
		},
	}).Do()
	return err
}

func (gce *GCEClient) DeleteVM(machine *machinesv1.Machine) error {
	config, err := gce.providerconfig(machine)
	if err != nil {
		return err
	}

	_, err = gce.service.Instances.Delete(config.Project, config.Zone, machine.ObjectMeta.Name).Do()
	return err
}

func (gce *GCEClient) providerconfig(machine *machinesv1.Machine) (*gceconfig.GCEProviderConfig, error) {
	obj, gvk, err := gce.codecFactory.UniversalDecoder().Decode([]byte(machine.Spec.ProviderConfig), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decoding failure: %v", err)
	}

	config, ok := obj.(*gceconfig.GCEProviderConfig)
	if !ok {
		return nil, fmt.Errorf("failure to cast to gce; type: %v", gvk)
	}

	return config, nil
}
