package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type DockerRunner interface {
	// BuildImage runs building a docker image from
	// the specified Dockerfile
	// * dockerfile – path to a dockerfile
	// * tags – tags of the image, tags[0] – the name of the image
	// * args - arguments values for the building the image
	// * includeToContext – file paths to include to the building context
	// returns the error from the docker service
	BuildImage(dockerfile string, tags []string, args map[string]string, includeToContext []string) error
	// RunContainer starts the container
	// from the specified image name
	// * imageName – which image will be used to start the container
	// * containerName – the name of the started container
	// * portsToExpose – list of the ports which will be exposed. Ex: []string{"8080", "8081"}
	// * inputEnv – environment variables. Ex: []string{"VAR_NAME_1=VALUE_1", "VAR_NAME_2=VALUE_2"}
	RunContainer(imageName string, containerName string, portsToExpose []string, volumeBinds []string, inputEnv []string) error
	// StopAndRemoveContainer stops and removes the container by name
	// if container has already been stopped it will delete it too
	StopAndRemoveContainer(containerName string) error
	// KillRunningContainers stops and removes all running containers
	KillRunningContainers(containerNameToDelete string) error
	// CheckRunningContainer checks if is the container running right now by name
	CheckRunningContainer(containerName string) (bool, error)
}

// DockerClient is the implementation of the DockerRunner interface
type DockerClient struct {
	client *client.Client
}

func NewDockerClient(client *client.Client) *DockerClient {
	return &DockerClient{client}
}

func (d *DockerClient) BuildImage(dockerfile string, tags []string, args map[string]string, includeToContext []string) error {
	ctx := context.Background()

	reader, err := archive.TarWithOptions(".", &archive.TarOptions{IncludeFiles: includeToContext})
	if err != nil {
		return err
	}

	log.Printf("ARGS TO BUILD: %+v\n", args)
	// Define the build options to use for the file
	// https://godoc.org/github.com/docker/docker/api/types#ImageBuildOptions
	buildOptions := types.ImageBuildOptions{
		Context:    reader,
		Dockerfile: dockerfile,
		Remove:     true,
		Tags:       tags,
		BuildArgs:  convertMapToDockerArgs(args),
	}

	// Build the actual image
	imageBuildResponse, err := d.client.ImageBuild(
		ctx,
		reader,
		buildOptions,
	)
	if err != nil {
		return err
	}

	// Read the STDOUT from the build process
	defer imageBuildResponse.Body.Close()
	_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
	if err != nil {
		return err
	}

	return nil
}

func (d *DockerClient) RunContainer(imageName string, containerName string, portsToExpose []string, volumeBinds []string, inputEnv []string) error {
	portMap := nat.PortMap{}
	exposedPorts := make(map[nat.Port]struct{})
	for _, port := range portsToExpose {
		// Define a PORT opening
		hostPort := port
		containerPort := port
		if strings.Contains(port, ":") {
			hostPort = strings.Split(port, ":")[0]
			containerPort = strings.Split(port, ":")[1]
		}
		newport, err := nat.NewPort("tcp", containerPort)
		if err != nil {
			fmt.Println("Unable to create docker port")
			return err
		}
		portMap[newport] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		}

		exposedPorts[newport] = struct{}{}
	}

	// Configured hostConfig:
	// https://godoc.org/github.com/docker/docker/api/types/container#HostConfig
	hostConfig := &container.HostConfig{
		Binds:        volumeBinds,
		PortBindings: portMap,
		// RestartPolicy: container.RestartPolicy{
		// 	Name: "always",
		// },
		LogConfig: container.LogConfig{
			Type:   "json-file",
			Config: map[string]string{},
		},
		// special factor conf
		// see: https://confluence.hflabs.ru/pages/viewpage.action?pageId=972227051
		OomScoreAdj: -1000,
		Resources: container.Resources{
			Ulimits: []*units.Ulimit{
				{
					Name: "nofile",
					Hard: 65535,
					Soft: 65535,
				},
				{
					Name: "nproc",
					Hard: 8192,
					Soft: 8192,
				},
			},
		},
	}

	// Define Network config (why isn't PORT in here...?:
	// https://godoc.org/github.com/docker/docker/api/types/network#NetworkingConfig
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{},
	}
	gatewayConfig := &network.EndpointSettings{
		Gateway: "gatewayname",
	}
	networkConfig.EndpointsConfig["bridge"] = gatewayConfig

	// Configuration
	// https://godoc.org/github.com/docker/docker/api/types/container#Config
	config := &container.Config{
		Image:        imageName,
		Env:          inputEnv,
		ExposedPorts: exposedPorts,
		Hostname:     imageName,
	}

	// Creating the actual container. This is "nil,nil,nil" in every example.
	cont, err := d.client.ContainerCreate(
		context.Background(),
		config,
		hostConfig,
		networkConfig,
		&specs.Platform{
			Architecture: "amd64",
			OS:           "linux",
		},
		containerName,
	)
	if err != nil {
		log.Println(err)
		return err
	}

	// Run the actual container
	err = d.client.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	log.Printf("Container %s is created", cont.ID)

	return nil
}

// Stop and remove a container
func (d *DockerClient) StopAndRemoveContainer(containerName string) error {
	ctx := context.Background()

	if err := d.client.ContainerStop(ctx, containerName, nil); err != nil {
		log.Printf("Unable to stop container %s: %s", containerName, err)
	}

	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	if err := d.client.ContainerRemove(ctx, containerName, removeOptions); err != nil {
		log.Printf("Unable to remove container: %s", err)
		return err
	}

	return nil
}

// List contaiers tags
func (d *DockerClient) listContainers() ([]types.Container, error) {
	ctx := context.Background()

	containers, err := d.client.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	return containers, nil
}

// Check weather the container is running now
func (d *DockerClient) CheckRunningContainer(containerName string) (bool, error) {
	containers, err := d.listContainers()
	if err != nil {
		return false, err
	}
	if len(containers) == 0 {
		return false, nil
	}
	for _, container := range containers {
		if containerName == strings.TrimLeft(container.Names[0], "/") {
			return true, nil
		}
	}
	return false, nil
}

func (d *DockerClient) KillRunningContainers(containerNameToDelete string) error {
	containers, err := d.listContainers()
	if err != nil {
		return err
	}
	errors := make([]error, 0)
	for _, container := range containers {
		containerName := strings.TrimLeft(container.Names[0], "/")
		if containerNameToDelete != "" && containerName != containerNameToDelete {
			continue
		}
		err = d.StopAndRemoveContainer(containerName)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) != 0 {
		errStr := ""
		for _, err := range errors {
			errStr += fmt.Sprintf("%v\n", err)
		}
		return fmt.Errorf(errStr)
	}
	return nil
}
