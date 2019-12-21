package main

import "fmt"

var (
	errNoSuchContainer = fmt.Errorf("No such container")
)

// DockerClient interface for interacting with docker
type DockerClient interface {
	Pull(ctx *Context, image string) error
	GetImageID(ctx *Context, name string) (string, error)
	RemoveContainer(ctx *Context, name string) error
	RemoveImage(ctx *Context, image string) error
}

type cliDockerClient struct{}

func (c *cliDockerClient) Pull(ctx *Context, image string) error {
	log.Debugw("Pulling image", "image", image, "requestId", ctx.id)
	if true {
		return nil
	}

	target := Target{
		Binary: "docker",
		Script: "pull",
	}

	output, err := target.execute(ctx, image)
	if err != nil {
		log.Errorw("Failed to pull image", "output", output, "error", err, "requestId", ctx.id)
	}

	return err
}

func (c *cliDockerClient) GetImageID(ctx *Context, name string) (string, error) {
	log.Debugw("Retrieving image id", "name", name, "requestId", ctx.id)
	target := Target{
		Binary: "docker",
		Script: "ps",
	}

	filter := fmt.Sprintf("name=%s", name)
	output, err := target.execute(ctx, "-a", "--filter", filter, "--format", "{{.Image}}")
	if output == "" {
		return "", errNoSuchContainer
	}

	if err != nil {
		log.Errorw("Failed to get image id", "output", output, "error", err, "requestId", ctx.id)
	}

	return output, err
}

func (c *cliDockerClient) RemoveContainer(ctx *Context, name string) error {
	log.Debugw("Stopping and removing container", "name", name, "requestId", ctx.id)
	target := Target{
		Binary: "docker",
		Script: "rm",
	}

	output, err := target.execute(ctx, "-f", name)
	if err != nil {
		log.Errorw("Failed to remove container", "output", output, "error", err, "requestId", ctx.id)
	}

	return err
}

func (c *cliDockerClient) RemoveImage(ctx *Context, image string) error {
	log.Debugw("Removing image", "name", image, "requestId", ctx.id)
	target := Target{
		Binary: "docker",
		Script: "rmi",
	}

	output, err := target.execute(ctx, image)
	if err != nil {
		log.Errorw("Failed to remove image", "output", output, "error", err, "requestId", ctx.id)
	}

	return err
}
