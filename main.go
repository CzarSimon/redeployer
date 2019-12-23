package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	stdLog "log"
	"net/http"
	"regexp"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var log = getLogger()

var (
	port       = flag.Int("port", 9000, "Port to expose webhooks on")
	configPath = flag.String("config", "/etc/redeployer/config.yaml", "Path to configuration")
)

type env struct {
	cfg    Config
	docker DockerClient
}

func main() {
	flag.Parse()

	env := newEnv()
	server := newServer(env, *port)

	log.Infow("Starting redeployer service", "port", port)
	err := server.ListenAndServe()
	if err != nil {
		log.Errorw("Service failed", "error", err)
	}
}

func (e *env) triggerRedeployment(ctx *Context) (int, error) {
	log.Debugw("Redeployment triggered", "requestId", ctx.id)
	var req RedeploymentRequest
	err := json.NewDecoder(ctx.r.Body).Decode(&req)
	if err != nil {
		log.Errorw("Failed to parse request body", "error", err)
		return http.StatusBadRequest, errBadRequest
	}

	target, status, err := e.findTarget(ctx, req)
	if err != nil {
		return status, err
	}

	go e.redeploy(ctx, target, req.Image)

	ctx.sendJSON(ResponseMessage{
		Message: "Redeployment triggered",
	})
	return http.StatusOK, nil
}

func (e *env) findTarget(ctx *Context, req RedeploymentRequest) (Target, int, error) {
	var target Target
	target, ok := e.cfg.Services[req.Target]
	if !ok {
		return target, http.StatusNotFound, errNotFound
	}

	pattern, err := regexp.Compile(target.MustMatch)
	if err != nil {
		log.Errorw("Failed to compile regex", "service", target.ID, "error", err, "requestId", ctx.id)
		return target, http.StatusInternalServerError, errInternalError
	}

	ok = pattern.MatchString(req.Image)
	if !ok {
		log.Warnw("Image did not match target", "image", req.Image, "regex", target.MustMatch, "requestId", ctx.id)
		return target, http.StatusForbidden, errForbidden
	}

	return target, http.StatusOK, nil
}

func (e *env) redeploy(ctx *Context, target Target, image string) {
	defer recoverFromPanic(ctx, "env.redeploy", false)

	log.Debugw("Redeploying service", "service", target.ID, "image", image, "requestId", ctx.id)
	previous, removeOld, err := e.prepareDeployment(ctx, target, image)
	if err != nil {
		log.Errorw("Redeployment failed", "requestId", ctx.id)
	}

	output, err := target.execute(ctx, image)
	if err != nil {
		log.Errorw("Failed to execute redeployment", "error", err, "output", output, "requestId", ctx.id)
	}
	log.Infow(output, "requestId", ctx.id)

	if removeOld && image != previous {
		e.docker.RemoveImage(ctx, previous)
	}

	log.Debugw("Redeployment succeded", "executionTime", ctx.latency(), "requestId", ctx.id)
}

func (e *env) prepareDeployment(ctx *Context, target Target, image string) (string, bool, error) {
	log.Debugw("Preparing redeployment", "requestId", ctx.id)
	removeOld := true

	err := e.docker.Pull(ctx, image)
	if err != nil {
		return "", removeOld, err
	}

	previous, err := e.docker.GetImageID(ctx, target.ID)
	if err == errNoSuchContainer {
		removeOld = false
		return "", removeOld, nil
	} else if err != nil {
		return "", removeOld, err
	}

	err = e.docker.RemoveContainer(ctx, target.ID)
	return previous, removeOld, err
}

func checkHealth(ctx *Context) (int, error) {
	ctx.sendOK()
	return http.StatusOK, nil
}

func newServer(e *env, port int) *http.Server {
	r := newRouter(e.cfg.Authentication)
	r.GET("/health", checkHealth, false)
	r.POST("/redeploy", e.triggerRedeployment, true)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}
}

func newEnv() *env {
	raw, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatalw("Failed to read config file", "error", err)
	}

	var cfg Config
	err = yaml.Unmarshal(raw, &cfg)
	if err != nil {
		log.Fatalw("Failed to parse config file", "error", err)
	}

	for _, target := range cfg.Services {
		_, err = regexp.Compile(target.MustMatch)
		if err != nil {
			msg := fmt.Sprintf("Invalid regex [%s] for target: %s", target.MustMatch, target.ID)
			log.Fatalw(msg, "error", err)
		}
	}

	return &env{
		cfg:    cfg,
		docker: &cliDockerClient{},
	}
}

func getLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)

	logger, err := cfg.Build()
	if err != nil {
		stdLog.Fatalf("Failed to create logger. Error: %s", err)
		return nil
	}

	return logger.Sugar()
}
