package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	milvussdk "github.com/milvus-io/milvus/client/v2/milvusclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SDKMilvusClientFactory struct{}

func (SDKMilvusClientFactory) New(ctx context.Context, cfg model.MilvusConfig, timeout time.Duration) (MilvusClient, error) {
	connectCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		connectCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	client, err := milvussdk.New(connectCtx, &milvussdk.ClientConfig{
		Address:  cfg.URI,
		Username: cfg.Username,
		Password: cfg.Password,
		APIKey:   cfg.Token,
	})
	if err != nil {
		return nil, err
	}
	return &sdkMilvusClient{
		client:      client,
		baseConfig:  cfg,
		callTimeout: timeout,
	}, nil
}

type sdkMilvusClient struct {
	client      *milvussdk.Client
	baseConfig  model.MilvusConfig
	callTimeout time.Duration
}

func (c *sdkMilvusClient) Ping(ctx context.Context) error {
	_, err := c.GetServerVersion(ctx)
	return err
}

func (c *sdkMilvusClient) GetServerVersion(ctx context.Context) (string, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.client.GetServerVersion(callCtx, milvussdk.NewGetServerVersionOption())
}

func (c *sdkMilvusClient) ListDatabases(ctx context.Context) ([]string, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	databases, err := c.client.ListDatabase(callCtx, milvussdk.NewListDatabaseOption())
	if err != nil && isMilvusCapabilityUnavailable(err) {
		return nil, ErrCapabilityUnavailable
	}
	return databases, err
}

func (c *sdkMilvusClient) ListCollections(ctx context.Context, database string) ([]MilvusCollection, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if database != "" {
		scopedCfg := c.baseConfig
		scopedClient, err := c.newScopedClient(ctx, scopedCfg, database)
		if err != nil {
			return nil, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	names, err := client.ListCollections(callCtx, milvussdk.NewListCollectionOption())
	if err != nil {
		return nil, err
	}

	collections := make([]MilvusCollection, 0, len(names))
	for _, name := range names {
		callCtx, cancel := c.withTimeout(ctx)
		desc, err := client.DescribeCollection(callCtx, milvussdk.NewDescribeCollectionOption(name))
		cancel()
		if err != nil {
			return nil, err
		}
		fieldCount := 0
		if desc.Schema != nil {
			fieldCount = len(desc.Schema.Fields)
		}
		collections = append(collections, MilvusCollection{
			Database:   databaseOrDefault(database),
			Name:       desc.Name,
			ShardNum:   desc.ShardNum,
			FieldCount: fieldCount,
		})
	}

	return collections, nil
}

func (c *sdkMilvusClient) Close(ctx context.Context) error {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.client.Close(callCtx)
}

func (c *sdkMilvusClient) newScopedClient(ctx context.Context, cfg model.MilvusConfig, database string) (*milvussdk.Client, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return milvussdk.New(callCtx, &milvussdk.ClientConfig{
		Address:  cfg.URI,
		Username: cfg.Username,
		Password: cfg.Password,
		APIKey:   cfg.Token,
		DBName:   database,
	})
}

func (c *sdkMilvusClient) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.callTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.callTimeout)
}

func databaseOrDefault(database string) string {
	if database == "" {
		return "default"
	}
	return database
}

func isMilvusCapabilityUnavailable(err error) bool {
	if err == nil {
		return false
	}
	code := status.Code(err)
	if code == codes.Unimplemented {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unimplemented") || strings.Contains(message, "not implemented")
}

type ClientGoK8sClientFactory struct{}

func (ClientGoK8sClientFactory) New(ctx context.Context, cfg model.K8sConfig, timeout time.Duration) (K8sClient, error) {
	_ = ctx
	restCfg, err := buildRestConfig(cfg, timeout)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &clientGoK8sClient{clientset: clientset}, nil
}

type clientGoK8sClient struct {
	clientset kubernetes.Interface
}

func (c *clientGoK8sClient) ListPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]PodInfo, 0, len(list.Items))
	for _, pod := range list.Items {
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" {
				ready = condition.Status == "True"
				break
			}
		}
		var restartCount int32
		for _, status := range pod.Status.ContainerStatuses {
			restartCount += status.RestartCount
		}
		pods = append(pods, PodInfo{
			Name:         pod.Name,
			Phase:        string(pod.Status.Phase),
			Ready:        ready,
			RestartCount: restartCount,
		})
	}
	return pods, nil
}

func (c *clientGoK8sClient) ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	list, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := make([]ServiceInfo, 0, len(list.Items))
	for _, service := range list.Items {
		ports := make([]string, 0, len(service.Spec.Ports))
		for _, port := range service.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", port.Port, strings.ToLower(string(port.Protocol))))
		}
		services = append(services, ServiceInfo{
			Name:  service.Name,
			Type:  string(service.Spec.Type),
			Ports: ports,
		})
	}
	return services, nil
}

func (c *clientGoK8sClient) ListEndpoints(ctx context.Context, namespace string) ([]EndpointInfo, error) {
	list, err := c.clientset.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	endpoints := make([]EndpointInfo, 0, len(list.Items))
	for _, endpoint := range list.Items {
		addresses := make([]string, 0)
		for _, subset := range endpoint.Subsets {
			for _, address := range subset.Addresses {
				addresses = append(addresses, address.IP)
			}
		}
		endpoints = append(endpoints, EndpointInfo{
			Name:      endpoint.Name,
			Addresses: addresses,
		})
	}
	return endpoints, nil
}

func buildRestConfig(cfg model.K8sConfig, timeout time.Duration) (*rest.Config, error) {
	if cfg.Kubeconfig != "" {
		restCfg, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, err
		}
		restCfg.Timeout = timeout
		return restCfg, nil
	}

	restCfg, err := rest.InClusterConfig()
	if err == nil {
		restCfg.Timeout = timeout
		return restCfg, nil
	}
	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, err
	}

	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	restCfg, err = loader.ClientConfig()
	if err != nil {
		return nil, err
	}
	restCfg.Timeout = timeout
	return restCfg, nil
}
