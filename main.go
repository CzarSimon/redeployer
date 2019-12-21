package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	stdLog "log"
	"net/http"

	"go.uber.org/zap"
)

var log = getLogger()

var (
	port       = flag.Int("port", 9000, "Port to expose webhooks on")
	configPath = flag.String("config", "/etc/redeployer/config.json", "Path to configuration")

	errTargetNotFound = fmt.Errorf("Redeployment target not found")
)

type env struct {
	cfg    Config
	docker DockerClient
}

func main() {
	flag.Parse()

	env := newEnv()

	r := newRouter(env.cfg.KeyHash)
	r.GET("/health", checkHealth, false)
	r.POST("/redeploy", env.triggerRedeployment, true)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: r,
	}

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

	target, ok := e.cfg.Services[req.Target]
	if !ok {
		return http.StatusNotFound, errTargetNotFound
	}

	go e.redeploy(ctx, target, req.Image)

	ctx.sendJSON(ResponseMessage{
		Message: "Redeployment triggered",
	})
	return http.StatusOK, nil
}

func (e *env) redeploy(ctx *Context, target Target, image string) {
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
		err = e.docker.RemoveImage(ctx, previous)
		if err != nil {
			log.Errorw("Redeployment failed", "requestId", ctx.id)
		}
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

func newEnv() *env {
	raw, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatalw("Failed to read config file", "error", err)
	}

	var cfg Config
	err = json.Unmarshal(raw, &cfg)
	if err != nil {
		log.Fatalw("Failed to parse config file", "error", err)
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
