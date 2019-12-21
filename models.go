package main

import "os/exec"

import "strings"

// Config service configuration
type Config struct {
	KeyHash  string            `json:"keyHash,omitempty"`
	Services map[string]Target `json:"services,omitempty"`
}

// RedeploymentRequest request body for redeployments.
type RedeploymentRequest struct {
	Target string `json:"target,omitempty"`
	Image  string `json:"image,omitempty"`
}

// ResponseMessage response containing a string message.
type ResponseMessage struct {
	Message string `json:"message,omitempty"`
}

// Target defines a script to be run by a webhook trigger.
type Target struct {
	ID     string `json:"id,omitempty"`
	Binary string `json:"binary,omitempty"`
	Script string `json:"script,omitempty"`
}

func (t Target) execute(ctx *Context, args ...string) (string, error) {
	allArgs := make([]string, len(args)+1)
	allArgs[0] = t.Script
	for i, arg := range args {
		allArgs[i+1] = arg
	}

	rawOutput, err := exec.CommandContext(ctx, t.Binary, allArgs...).CombinedOutput()
	output := string(rawOutput)
	if strings.HasSuffix(output, "\n") {
		output = output[:len(output)-1]
	}

	return output, err
}
